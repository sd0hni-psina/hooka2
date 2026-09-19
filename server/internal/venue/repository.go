package venue

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

func (Repository) Get(ctx context.Context, q store.Querier, id string) (domain.Venue, error) {
	var v domain.Venue
	var code string
	err := q.QueryRow(ctx,
		`select id, name, timezone, join_code from venues where id = $1`, id,
	).Scan(&v.ID, &v.Name, &v.Timezone, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Venue{}, fmt.Errorf("get venue %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Venue{}, fmt.Errorf("get venue %s: %w", id, err)
	}
	v.JoinCode = &code
	return v, nil
}

func (Repository) GetPlan(ctx context.Context, q store.Querier, venueID string) (domain.Plan, error) {
	var p domain.Plan
	err := q.QueryRow(ctx, `select service_plan from venues where id = $1`, venueID).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Plan{}, fmt.Errorf("get plan %s: %w", venueID, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Plan{}, fmt.Errorf("get plan %s: %w", venueID, err)
	}
	return p, nil
}
