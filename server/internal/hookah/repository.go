package hookah

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

type Repository struct{}

func NewRepository() *Repository { return &Repository{} }

// Row is a hookah with the plan snapshot it runs on.
type Row struct {
	domain.Hookah
	Plan domain.Plan
}

const hookahColumns = `h.id, h.table_id, h.hall_id, h.state, h.started_at, h.expires_at, h.finished_at,
	h.started_by, st.display_name, h.plan_snapshot`

func scanRow(row pgx.CollectableRow) (Row, error) {
	var r Row
	err := row.Scan(&r.ID, &r.TableID, &r.HallID, &r.State, &r.StartedAt, &r.ExpiresAt, &r.FinishedAt,
		&r.StartedBy, &r.StartedByName, &r.Plan)
	return r, err
}

func (Repository) ListLive(ctx context.Context, q store.Querier, hallID string) ([]Row, error) {
	rows, err := q.Query(ctx,
		`select `+hookahColumns+` from hookahs h join staff st on st.id = h.started_by
		 where h.hall_id = $1 and h.state in ('active', 'expired') order by h.started_at`, hallID)
	if err != nil {
		return nil, fmt.Errorf("list live hookahs: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanRow)
	if err != nil {
		return nil, fmt.Errorf("list live hookahs: %w", err)
	}
	return out, nil
}

const eventColumns = `e.id, e.hookah_id, e.seq, e.kind, e.title, e.due_at, e.state,
	e.claimed_by, cl.display_name, e.claimed_at, e.confirmed_by, cf.display_name, e.confirmed_at, e.confirmed_early`

const eventJoins = ` from service_events e
	left join staff cl on cl.id = e.claimed_by
	left join staff cf on cf.id = e.confirmed_by`

func scanEvent(row pgx.CollectableRow) (domain.ServiceEvent, error) {
	var e domain.ServiceEvent
	err := row.Scan(&e.ID, &e.HookahID, &e.Seq, &e.Kind, &e.Title, &e.DueAt, &e.State,
		&e.ClaimedBy, &e.ClaimedByName, &e.ClaimedAt, &e.ConfirmedBy, &e.ConfirmedByName, &e.ConfirmedAt, &e.ConfirmedEarly)
	return e, err
}

// ListEvents returns events of the given hookahs ordered by hookah and seq.
func (Repository) ListEvents(ctx context.Context, q store.Querier, hookahIDs []string) ([]domain.ServiceEvent, error) {
	rows, err := q.Query(ctx,
		`select `+eventColumns+eventJoins+` where e.hookah_id = any($1::uuid[]) order by e.hookah_id, e.seq`, hookahIDs)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return out, nil
}
