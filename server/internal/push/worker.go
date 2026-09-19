package push

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/store"
)

const batchSize = 100

// Worker sends queued push_deliveries through a Sender.
type Worker struct {
	pool   *pgxpool.Pool
	sender Sender
	log    *slog.Logger
	tick   time.Duration
}

func NewWorker(pool *pgxpool.Pool, sender Sender, log *slog.Logger, tick time.Duration) *Worker {
	return &Worker{pool: pool, sender: sender, log: log, tick: tick}
}

func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := w.RunOnce(ctx)
			if err != nil {
				w.log.Error("push worker", "err", err)
				continue
			}
			if n > 0 {
				w.log.Info("push worker", "processed", n)
			}
		}
	}
}

type delivery struct {
	id    string
	token *string
	msg   Message
}

// RunOnce sends one batch. Rows stay locked with SKIP LOCKED for the whole
// send, so a second worker never picks the same delivery.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	var n int
	err := store.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select d.id, s.expo_push_token, t.label, e.title, e.id, h.id, h.table_id, h.hall_id, e.kind, e.due_at
			from push_deliveries d
			join staff_sessions s on s.id = d.session_id
			join service_events e on e.id = d.service_event_id
			join hookahs h on h.id = e.hookah_id
			join tables t on t.id = h.table_id
			where d.status = 'queued'
			order by d.created_at
			for update of d skip locked
			limit $1`, batchSize)
		if err != nil {
			return fmt.Errorf("select queued: %w", err)
		}
		batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (delivery, error) {
			var d delivery
			var label string
			err := row.Scan(&d.id, &d.token, &label, &d.msg.Body, &d.msg.Data.EventID, &d.msg.Data.HookahID,
				&d.msg.Data.TableID, &d.msg.Data.HallID, &d.msg.Data.Kind, &d.msg.Data.DueAt)
			d.msg.Title = "Стол " + label
			d.msg.Sound, d.msg.Priority, d.msg.CategoryID = "default", "high", "service_action"
			return d, err
		})
		if err != nil {
			return fmt.Errorf("scan queued: %w", err)
		}
		n = len(batch)
		if n == 0 {
			return nil
		}

		var send []delivery
		for _, d := range batch {
			if d.token == nil {
				if err := mark(ctx, tx, d.id, Result{Status: StatusInvalidToken, Error: "session has no push token"}); err != nil {
					return err
				}
				continue
			}
			d.msg.To = *d.token
			send = append(send, d)
		}
		if len(send) == 0 {
			return nil
		}
		msgs := make([]Message, len(send))
		for i, d := range send {
			msgs[i] = d.msg
		}
		results, sendErr := w.sender.Send(ctx, msgs)
		for i, d := range send {
			res := Result{Status: StatusFailed}
			switch {
			case sendErr != nil:
				res.Error = sendErr.Error()
			case i < len(results):
				res = results[i]
			default:
				res.Error = "sender returned fewer results than messages"
			}
			if err := mark(ctx, tx, d.id, res); err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

func mark(ctx context.Context, tx pgx.Tx, id string, r Result) error {
	var receipt, errText *string
	if r.Receipt != "" {
		receipt = &r.Receipt
	}
	if r.Error != "" {
		errText = &r.Error
	}
	if _, err := tx.Exec(ctx,
		`update push_deliveries set status = $2, provider_receipt = $3, error = $4 where id = $1`,
		id, r.Status, receipt, errText); err != nil {
		return fmt.Errorf("mark delivery %s: %w", id, err)
	}
	return nil
}
