package layout

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

type Repository struct{}

func NewRepository() *Repository { return &Repository{} }

func (Repository) ListHalls(ctx context.Context, q store.Querier, venueID string) ([]domain.Hall, error) {
	rows, err := q.Query(ctx,
		`select id, venue_id, name, sort_order from halls
		 where venue_id = $1 order by sort_order, name`, venueID)
	if err != nil {
		return nil, fmt.Errorf("list halls: %w", err)
	}
	halls, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Hall, error) {
		var h domain.Hall
		return h, row.Scan(&h.ID, &h.VenueID, &h.Name, &h.SortOrder)
	})
	if err != nil {
		return nil, fmt.Errorf("list halls: %w", err)
	}
	return halls, nil
}

func (Repository) GetHall(ctx context.Context, q store.Querier, venueID, hallID string) (domain.Hall, error) {
	var h domain.Hall
	err := q.QueryRow(ctx,
		`select id, venue_id, name, sort_order from halls where id = $1 and venue_id = $2`, hallID, venueID,
	).Scan(&h.ID, &h.VenueID, &h.Name, &h.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Hall{}, fmt.Errorf("get hall %s: %w", hallID, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Hall{}, fmt.Errorf("get hall %s: %w", hallID, err)
	}
	return h, nil
}

const tableColumns = `id, hall_id, label, shape, x, y, w, h, rotation, seats`

func scanTable(row pgx.CollectableRow) (domain.Table, error) {
	var t domain.Table
	return t, row.Scan(&t.ID, &t.HallID, &t.Label, &t.Shape, &t.X, &t.Y, &t.W, &t.H, &t.Rotation, &t.Seats)
}

func (Repository) ListTables(ctx context.Context, q store.Querier, hallID string) ([]domain.Table, error) {
	rows, err := q.Query(ctx,
		`select `+tableColumns+` from tables where hall_id = $1 and archived_at is null order by created_at, label`, hallID)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	tables, err := pgx.CollectRows(rows, scanTable)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	return tables, nil
}

// HallNameTaken checks names case-insensitively; the unique index is exact.
func (Repository) HallNameTaken(ctx context.Context, q store.Querier, venueID, name, exceptID string) (bool, error) {
	var taken bool
	err := q.QueryRow(ctx,
		`select exists(select 1 from halls where venue_id = $1 and lower(name) = lower($2) and id::text <> $3)`,
		venueID, name, exceptID).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("hall name taken: %w", err)
	}
	return taken, nil
}

func (Repository) CreateHall(ctx context.Context, q store.Querier, venueID, name string, sortOrder int) (domain.Hall, error) {
	var h domain.Hall
	err := q.QueryRow(ctx,
		`insert into halls (venue_id, name, sort_order) values ($1, $2, $3)
		 returning id, venue_id, name, sort_order`, venueID, name, sortOrder,
	).Scan(&h.ID, &h.VenueID, &h.Name, &h.SortOrder)
	if err != nil {
		return domain.Hall{}, fmt.Errorf("create hall: %w", err)
	}
	return h, nil
}

func (Repository) UpdateHall(ctx context.Context, q store.Querier, h domain.Hall) error {
	if _, err := q.Exec(ctx, `update halls set name = $2, sort_order = $3 where id = $1`, h.ID, h.Name, h.SortOrder); err != nil {
		return fmt.Errorf("update hall: %w", err)
	}
	return nil
}

// LockHall takes the hall row so concurrent deletes and hookah starts queue up.
func (Repository) LockHall(ctx context.Context, q store.Querier, hallID string) error {
	if _, err := q.Exec(ctx, `select 1 from halls where id = $1 for update`, hallID); err != nil {
		return fmt.Errorf("lock hall: %w", err)
	}
	return nil
}

func (Repository) HallHasLiveHookah(ctx context.Context, q store.Querier, hallID string) (bool, error) {
	var busy bool
	err := q.QueryRow(ctx,
		`select exists(select 1 from hookahs where hall_id = $1 and state in ('active', 'expired'))`, hallID).Scan(&busy)
	if err != nil {
		return false, fmt.Errorf("hall busy: %w", err)
	}
	return busy, nil
}

// DeleteHall removes the hall with its hookah history, tech.md section 19.6.
// Explicit deletes come first because tables are referenced with RESTRICT.
func (Repository) DeleteHall(ctx context.Context, q store.Querier, hallID string) error {
	stmts := []string{
		`delete from hookahs where hall_id = $1`,
		`delete from hookah_transfers where from_table_id in (select id from tables where hall_id = $1)
		   or to_table_id in (select id from tables where hall_id = $1)`,
		`delete from halls where id = $1`,
	}
	for _, sql := range stmts {
		if _, err := q.Exec(ctx, sql, hallID); err != nil {
			return fmt.Errorf("delete hall: %w", err)
		}
	}
	return nil
}

// StoredTable is a table with its archive flag and venue, for mutations.
type StoredTable struct {
	domain.Table
	VenueID  string
	Archived bool
}

// GetTableForUpdate locks the table row for the rest of the transaction.
func (Repository) GetTableForUpdate(ctx context.Context, q store.Querier, venueID, tableID string) (StoredTable, error) {
	var t StoredTable
	err := q.QueryRow(ctx,
		`select t.id, t.hall_id, t.label, t.shape, t.x, t.y, t.w, t.h, t.rotation, t.seats, h.venue_id, t.archived_at is not null
		 from tables t join halls h on h.id = t.hall_id
		 where t.id = $1 and h.venue_id = $2
		 for update of t`, tableID, venueID,
	).Scan(&t.ID, &t.HallID, &t.Label, &t.Shape, &t.X, &t.Y, &t.W, &t.H, &t.Rotation, &t.Seats, &t.VenueID, &t.Archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredTable{}, fmt.Errorf("get table %s: %w", tableID, domain.ErrNotFound)
	}
	if err != nil {
		return StoredTable{}, fmt.Errorf("get table %s: %w", tableID, err)
	}
	return t, nil
}

func (Repository) LabelTaken(ctx context.Context, q store.Querier, hallID, label, exceptID string) (bool, error) {
	var taken bool
	err := q.QueryRow(ctx,
		`select exists(select 1 from tables where hall_id = $1 and label = $2 and archived_at is null and id::text <> $3)`,
		hallID, label, exceptID).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("label taken: %w", err)
	}
	return taken, nil
}

func (Repository) CreateTable(ctx context.Context, q store.Querier, hallID string, f domain.TableFields) (domain.Table, error) {
	rows, err := q.Query(ctx,
		`insert into tables (hall_id, label, shape, x, y, w, h, rotation, seats)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning `+tableColumns,
		hallID, f.Label, f.Shape, f.X, f.Y, f.W, f.H, f.Rotation, f.Seats)
	if err != nil {
		return domain.Table{}, fmt.Errorf("create table: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, scanTable)
	if err != nil {
		return domain.Table{}, fmt.Errorf("create table: %w", err)
	}
	return t, nil
}

func (Repository) UpdateTable(ctx context.Context, q store.Querier, id string, f domain.TableFields, archived bool) error {
	_, err := q.Exec(ctx,
		`update tables set label = $2, shape = $3, x = $4, y = $5, w = $6, h = $7, rotation = $8, seats = $9,
		   archived_at = case when $10 then coalesce(archived_at, now()) else null end
		 where id = $1`,
		id, f.Label, f.Shape, f.X, f.Y, f.W, f.H, f.Rotation, f.Seats, archived)
	if err != nil {
		return fmt.Errorf("update table: %w", err)
	}
	return nil
}

func (Repository) TableHasLiveHookah(ctx context.Context, q store.Querier, tableID string) (bool, error) {
	var busy bool
	err := q.QueryRow(ctx,
		`select exists(select 1 from hookahs where table_id = $1 and state in ('active', 'expired'))`, tableID).Scan(&busy)
	if err != nil {
		return false, fmt.Errorf("table busy: %w", err)
	}
	return busy, nil
}

// LockTables locks the active tables of a hall and returns them by id.
func (Repository) LockTables(ctx context.Context, q store.Querier, hallID string) (map[string]domain.Table, error) {
	rows, err := q.Query(ctx,
		`select `+tableColumns+` from tables where hall_id = $1 and archived_at is null for update`, hallID)
	if err != nil {
		return nil, fmt.Errorf("lock tables: %w", err)
	}
	list, err := pgx.CollectRows(rows, scanTable)
	if err != nil {
		return nil, fmt.Errorf("lock tables: %w", err)
	}
	out := make(map[string]domain.Table, len(list))
	for _, t := range list {
		out[t.ID] = t
	}
	return out, nil
}

func (Repository) SetPlacement(ctx context.Context, q store.Querier, p domain.Placement) error {
	_, err := q.Exec(ctx, `update tables set x = $2, y = $3, w = $4, h = $5, rotation = $6 where id = $1`,
		p.ID, p.X, p.Y, p.W, p.H, p.Rotation)
	if err != nil {
		return fmt.Errorf("set placement: %w", err)
	}
	return nil
}
