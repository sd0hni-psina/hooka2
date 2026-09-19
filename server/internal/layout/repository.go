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
