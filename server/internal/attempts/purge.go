// Package attempts holds the upkeep of the tables that back the sign-in
// limits: join_attempts and auth_attempts.
package attempts

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

const (
	// retention is longer than every limit window (one hour at most), so a
	// purge never lifts a block early.
	retention = 24 * time.Hour
	every     = time.Hour
)

// Purger deletes attempts older than a day, tech.md section 19.16. It runs as a
// scheduler ticker but works once an hour.
type Purger struct {
	pool *pgxpool.Pool
	last time.Time
}

func NewPurger(pool *pgxpool.Pool) *Purger { return &Purger{pool: pool} }

// Tick purges when an hour has passed since the last pass. The time of the
// pass is kept before the delete, so a failing database is retried next hour,
// not logged every second. The age condition makes a repeated pass harmless.
func (p *Purger) Tick(ctx context.Context, now time.Time) (scheduler.Result, error) {
	if !p.last.IsZero() && now.Sub(p.last) < every {
		return scheduler.Result{}, nil
	}
	p.last = now
	cutoff := domain.DBTime(now.Add(-retention))
	var res scheduler.Result
	err := store.WithTx(ctx, p.pool, func(tx pgx.Tx) error {
		for _, table := range []string{"join_attempts", "auth_attempts"} {
			tag, err := tx.Exec(ctx, `delete from `+table+` where created_at < $1`, cutoff)
			if err != nil {
				return fmt.Errorf("purge %s: %w", table, err)
			}
			res.Purged += int(tag.RowsAffected())
		}
		return nil
	})
	return res, err
}
