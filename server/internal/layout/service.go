package layout

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/realtime"
	"hookah/server/internal/store"
)

type Service struct {
	pool *pgxpool.Pool
	repo *Repository
	hub  realtime.Broadcaster
}

func NewService(pool *pgxpool.Pool, repo *Repository, hub realtime.Broadcaster) *Service {
	return &Service{pool: pool, repo: repo, hub: hub}
}

// LayoutUpdated is the payload of layout.updated, tech.md section 9.
type LayoutUpdated struct {
	HallID string         `json:"hall_id"`
	Tables []domain.Table `json:"tables"`
}

func taken(field string) error {
	v := &domain.ValidationError{}
	v.Add(field, domain.ReasonTaken)
	return v
}

func notFound(what, id string) error {
	return fmt.Errorf("%s %q: %w", what, id, domain.ErrNotFound)
}

func (s *Service) hall(ctx context.Context, q store.Querier, c auth.Claims, hallID string) (domain.Hall, error) {
	if !domain.ValidUUID(hallID) {
		return domain.Hall{}, notFound("hall", hallID)
	}
	return s.repo.GetHall(ctx, q, c.VenueID, hallID)
}

type HallInput struct {
	Name      *string
	SortOrder *int
}

func (s *Service) CreateHall(ctx context.Context, c auth.Claims, name string, sortOrder int) (domain.Hall, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Hall{}, err
	}
	name, err := domain.ValidateHall(name, sortOrder)
	if err != nil {
		return domain.Hall{}, err
	}
	var h domain.Hall
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		dup, err := s.repo.HallNameTaken(ctx, tx, c.VenueID, name, "")
		if err != nil {
			return err
		}
		if dup {
			return taken("name")
		}
		h, err = s.repo.CreateHall(ctx, tx, c.VenueID, name, sortOrder)
		return err
	})
	if store.IsUniqueViolation(err, "halls_venue_id_name_key") {
		return domain.Hall{}, taken("name")
	}
	return h, err
}

func (s *Service) UpdateHall(ctx context.Context, c auth.Claims, hallID string, in HallInput) (domain.Hall, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Hall{}, err
	}
	var h domain.Hall
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if h, err = s.hall(ctx, tx, c, hallID); err != nil {
			return err
		}
		if in.Name != nil {
			h.Name = *in.Name
		}
		if in.SortOrder != nil {
			h.SortOrder = *in.SortOrder
		}
		if h.Name, err = domain.ValidateHall(h.Name, h.SortOrder); err != nil {
			return err
		}
		dup, err := s.repo.HallNameTaken(ctx, tx, c.VenueID, h.Name, h.ID)
		if err != nil {
			return err
		}
		if dup {
			return taken("name")
		}
		return s.repo.UpdateHall(ctx, tx, h)
	})
	if store.IsUniqueViolation(err, "halls_venue_id_name_key") {
		return domain.Hall{}, taken("name")
	}
	return h, err
}

func (s *Service) DeleteHall(ctx context.Context, c auth.Claims, hallID string) error {
	if err := auth.RequireManager(c); err != nil {
		return err
	}
	return store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.hall(ctx, tx, c, hallID); err != nil {
			return err
		}
		if err := s.repo.LockHall(ctx, tx, hallID); err != nil {
			return err
		}
		busy, err := s.repo.HallHasLiveHookah(ctx, tx, hallID)
		if err != nil {
			return err
		}
		if busy {
			return fmt.Errorf("delete hall %s: %w", hallID, domain.ErrTableBusy)
		}
		return s.repo.DeleteHall(ctx, tx, hallID)
	})
}

// publish sends the full active table list of a hall after a commit.
func (s *Service) publish(ctx context.Context, venueID, hallID string) ([]domain.Table, error) {
	tables, err := s.repo.ListTables(ctx, s.pool, hallID)
	if err != nil {
		return nil, err
	}
	if tables == nil {
		tables = []domain.Table{}
	}
	id := hallID
	s.hub.Publish(venueID, &id, realtime.TypeLayoutUpdated, LayoutUpdated{HallID: hallID, Tables: tables})
	return tables, nil
}

func (s *Service) CreateTable(ctx context.Context, c auth.Claims, hallID string, f domain.TableFields) (domain.Table, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Table{}, err
	}
	f, err := domain.ValidateTable(f, "")
	if err != nil {
		return domain.Table{}, err
	}
	var t domain.Table
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.hall(ctx, tx, c, hallID); err != nil {
			return err
		}
		dup, err := s.repo.LabelTaken(ctx, tx, hallID, f.Label, "")
		if err != nil {
			return err
		}
		if dup {
			return taken("label")
		}
		t, err = s.repo.CreateTable(ctx, tx, hallID, f)
		return err
	})
	if store.IsUniqueViolation(err, "tables_hall_label_key") {
		return domain.Table{}, taken("label")
	}
	if err != nil {
		return domain.Table{}, err
	}
	_, err = s.publish(ctx, c.VenueID, hallID)
	return t, err
}

// PatchTable applies a partial update, including archive and restore.
func (s *Service) PatchTable(ctx context.Context, c auth.Claims, tableID string, p domain.TablePatch) (domain.Table, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Table{}, err
	}
	if !domain.ValidUUID(tableID) {
		return domain.Table{}, notFound("table", tableID)
	}
	var out domain.Table
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.repo.GetTableForUpdate(ctx, tx, c.VenueID, tableID)
		if err != nil {
			return err
		}
		archived := cur.Archived
		if p.Archived != nil {
			archived = *p.Archived
		}
		f, err := domain.ValidateTable(p.Apply(cur.Fields()), "")
		if err != nil {
			return err
		}
		if archived && !cur.Archived {
			busy, err := s.repo.TableHasLiveHookah(ctx, tx, tableID)
			if err != nil {
				return err
			}
			if busy {
				return fmt.Errorf("archive table %s: %w", tableID, domain.ErrTableBusy)
			}
		}
		if !archived {
			dup, err := s.repo.LabelTaken(ctx, tx, cur.HallID, f.Label, tableID)
			if err != nil {
				return err
			}
			if dup {
				return taken("label")
			}
		}
		if err := s.repo.UpdateTable(ctx, tx, tableID, f, archived); err != nil {
			return err
		}
		out = domain.Table{ID: tableID, HallID: cur.HallID, Label: f.Label, Shape: f.Shape, X: f.X, Y: f.Y, W: f.W, H: f.H, Rotation: f.Rotation, Seats: f.Seats}
		return nil
	})
	if store.IsUniqueViolation(err, "tables_hall_label_key") {
		return domain.Table{}, taken("label")
	}
	if err != nil {
		return domain.Table{}, err
	}
	_, err = s.publish(ctx, c.VenueID, out.HallID)
	return out, err
}

func (s *Service) ArchiveTable(ctx context.Context, c auth.Claims, tableID string) error {
	archived := true
	_, err := s.PatchTable(ctx, c, tableID, domain.TablePatch{Archived: &archived})
	return err
}

// SetLayout saves a drag session in one transaction, tech.md section 19.6.
func (s *Service) SetLayout(ctx context.Context, c auth.Claims, hallID string, batch []domain.Placement) ([]domain.Table, error) {
	if err := auth.RequireManager(c); err != nil {
		return nil, err
	}
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.hall(ctx, tx, c, hallID); err != nil {
			return err
		}
		current, err := s.repo.LockTables(ctx, tx, hallID)
		if err != nil {
			return err
		}
		for _, p := range batch {
			if _, ok := current[p.ID]; !ok {
				return notFound("table", p.ID)
			}
		}
		if err := domain.ValidateLayout(batch, current); err != nil {
			return err
		}
		for _, p := range batch {
			if err := s.repo.SetPlacement(ctx, tx, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.publish(ctx, c.VenueID, hallID)
}
