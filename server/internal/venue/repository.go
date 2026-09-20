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

// Create inserts a venue unless the join code is taken; ok is false then and
// the caller retries with another code.
func (Repository) Create(ctx context.Context, q store.Querier, ownerID, name, code string, plan domain.Plan) (domain.Venue, bool, error) {
	var v domain.Venue
	var stored string
	err := q.QueryRow(ctx,
		`insert into venues (owner_id, name, join_code, service_plan) values ($1, $2, $3, $4)
		 on conflict (join_code) do nothing
		 returning id, name, timezone, join_code`, ownerID, name, code, plan,
	).Scan(&v.ID, &v.Name, &v.Timezone, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Venue{}, false, nil
	}
	if err != nil {
		return domain.Venue{}, false, fmt.Errorf("create venue: %w", err)
	}
	v.JoinCode = &stored
	return v, true, nil
}

func (Repository) GetByCode(ctx context.Context, q store.Querier, code string) (domain.Venue, bool, error) {
	var v domain.Venue
	var stored string
	err := q.QueryRow(ctx,
		`select id, name, timezone, join_code from venues where join_code = $1`, code,
	).Scan(&v.ID, &v.Name, &v.Timezone, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Venue{}, false, nil
	}
	if err != nil {
		return domain.Venue{}, false, fmt.Errorf("venue by code: %w", err)
	}
	v.JoinCode = &stored
	return v, true, nil
}

// SetCode changes the join code unless another venue holds it.
func (Repository) SetCode(ctx context.Context, q store.Querier, venueID, code string) (bool, error) {
	tag, err := q.Exec(ctx,
		`update venues set join_code = $2, updated_at = now()
		 where id = $1 and not exists (select 1 from venues where join_code = $2)`, venueID, code)
	if store.IsUniqueViolation(err, "venues_join_code_key") {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("set join code: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (Repository) SetPlan(ctx context.Context, q store.Querier, venueID string, plan domain.Plan) error {
	if _, err := q.Exec(ctx,
		`update venues set service_plan = $2, updated_at = now() where id = $1`, venueID, plan); err != nil {
		return fmt.Errorf("set service plan: %w", err)
	}
	return nil
}
