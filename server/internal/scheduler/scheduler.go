package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Result counts what one tick changed.
type Result struct {
	Due     int
	Missed  int
	Expired int
	Queued  int
}

func (r *Result) add(o Result) {
	r.Due += o.Due
	r.Missed += o.Missed
	r.Expired += o.Expired
	r.Queued += o.Queued
}

// Ticker runs one pass. The hookah service implements it; tests use a fake.
type Ticker interface {
	Tick(ctx context.Context, now time.Time) (Result, error)
}

type Scheduler struct {
	log     *slog.Logger
	tick    time.Duration
	tickers []Ticker
	now     func() time.Time
}

// New runs the tickers in order on every tick: state changes first, pushes
// after, so a task that just turned due is queued in the same pass.
func New(log *slog.Logger, tick time.Duration, tickers ...Ticker) *Scheduler {
	return &Scheduler{log: log, tick: tick, tickers: tickers, now: time.Now}
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
			now := s.now()
			var res Result
			failed := false
			for _, ticker := range s.tickers {
				one, err := ticker.Tick(ctx, now)
				if err != nil {
					s.log.Error("scheduler tick", "err", err)
					failed = true
					break
				}
				res.add(one)
			}
			if failed {
				continue
			}
			s.log.Debug("scheduler tick", "due", res.Due, "missed", res.Missed, "expired", res.Expired, "queued", res.Queued)
		}
	}
}
