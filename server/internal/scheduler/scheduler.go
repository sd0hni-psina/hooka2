package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Result counts the rows each tick query moved.
type Result struct {
	Due     int
	Missed  int
	Expired int
}

// Ticker runs one pass. The hookah service implements it; tests use a fake.
type Ticker interface {
	Tick(ctx context.Context, now time.Time) (Result, error)
}

type Scheduler struct {
	log    *slog.Logger
	tick   time.Duration
	ticker Ticker
	now    func() time.Time
}

func New(log *slog.Logger, tick time.Duration, ticker Ticker) *Scheduler {
	return &Scheduler{log: log, tick: tick, ticker: ticker, now: time.Now}
}

// Run ticks until ctx is cancelled. A failed tick is logged and the next one
// retries: every tick query is idempotent by state.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			res, err := s.ticker.Tick(ctx, s.now())
			if err != nil {
				s.log.Error("scheduler tick", "err", err)
				continue
			}
			s.log.Debug("scheduler tick", "due", res.Due, "missed", res.Missed, "expired", res.Expired)
		}
	}
}
