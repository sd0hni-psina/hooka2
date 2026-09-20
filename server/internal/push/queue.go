package push

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/domain"
	"hookah/server/internal/scheduler"
	"hookah/server/internal/store"
)

// Presence tells whether a device session is watching the venue over WS.
// The realtime hub implements it.
type Presence interface {
	SessionOnline(venueID, sessionID string) bool
}

// Queue turns due service events into push_deliveries rows, tech.md
// section 19.9. It runs inside the scheduler tick.
type Queue struct {
	pool     *pgxpool.Pool
	presence Presence
}

func NewQueue(pool *pgxpool.Pool, presence Presence) *Queue {
	return &Queue{pool: pool, presence: presence}
}

const queueLimit = 200

type candidate struct {
	eventID  string
	venueID  string
	attempts int
}

// Tick queues the first push of freshly due events and the repeats of the
// ones still waiting. The unique index on (event, session, attempt) keeps a
// repeated tick from sending twice.
func (q *Queue) Tick(ctx context.Context, now time.Time) (scheduler.Result, error) {
	now = domain.DBTime(now)
	var res scheduler.Result
	err := store.WithTx(ctx, q.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select e.id, h.venue_id, e.push_attempts
			from service_events e
			join hookahs h on h.id = e.hookah_id
			where e.state = 'due'
			  and e.push_attempts < ((h.plan_snapshot->>'push_max_attempts')::int)
			  and (e.last_push_at is null
			       or e.last_push_at + (((h.plan_snapshot->>'push_repeat_sec')::int) * interval '1 second') <= $1)
			order by e.due_at
			for update of e skip locked
			limit $2`, now, queueLimit)
		if err != nil {
			return fmt.Errorf("push candidates: %w", err)
		}
		candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
			var c candidate
			return c, row.Scan(&c.eventID, &c.venueID, &c.attempts)
		})
		if err != nil {
			return fmt.Errorf("push candidates: %w", err)
		}

		for _, c := range candidates {
			sessions, err := q.sessions(ctx, tx, c.venueID)
			if err != nil {
				return err
			}
			queued := 0
			for _, sessionID := range sessions {
				// An open app already got service.due over WS, tech.md section 10.
				if q.presence.SessionOnline(c.venueID, sessionID) {
					continue
				}
				tag, err := tx.Exec(ctx,
					`insert into push_deliveries (service_event_id, session_id, attempt, status)
					 values ($1, $2, $3, 'queued') on conflict do nothing`,
					c.eventID, sessionID, c.attempts+1)
				if err != nil {
					return fmt.Errorf("queue push: %w", err)
				}
				queued += int(tag.RowsAffected())
			}
			// Without a device to notify only the clock moves on, so the
			// attempt is still available once somebody closes the app.
			if _, err := tx.Exec(ctx, `
				update service_events
				set push_attempts = push_attempts + case when $2 > 0 then 1 else 0 end, last_push_at = $3
				where id = $1`, c.eventID, queued, now); err != nil {
				return fmt.Errorf("count push attempt: %w", err)
			}
			res.Queued += queued
		}
		return nil
	})
	return res, err
}

func (Queue) sessions(ctx context.Context, q store.Querier, venueID string) ([]string, error) {
	rows, err := q.Query(ctx, `
		select s.id from staff_sessions s
		join staff st on st.id = s.staff_id
		where st.venue_id = $1 and st.blocked_at is null
		  and s.revoked_at is null and s.expo_push_token is not null
		order by s.created_at`, venueID)
	if err != nil {
		return nil, fmt.Errorf("push sessions: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("push sessions: %w", err)
	}
	return out, nil
}
