package push_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/push"
	"hookah/server/internal/store/storetest"
	"hookah/server/seed"
)

type env struct {
	pool    *pgxpool.Pool
	fx      seed.Fixture
	eventID string
	fake    *push.Fake
	worker  *push.Worker
}

func setup(t *testing.T) env {
	pool := storetest.Open(t)
	ctx := context.Background()
	fx, err := seed.Seed(ctx, pool)
	require.NoError(t, err)
	var eventID string
	require.NoError(t, pool.QueryRow(ctx,
		`select id from service_events where hookah_id = $1 and state = 'due'`, fx.BusyHookahID).Scan(&eventID))
	log := discardLogger()
	fake := push.NewFake(log)
	return env{pool: pool, fx: fx, eventID: eventID, fake: fake, worker: push.NewWorker(pool, fake, log, time.Second)}
}

func (e env) queue(t *testing.T, sessionID string, token *string) string {
	ctx := context.Background()
	_, err := e.pool.Exec(ctx, `update staff_sessions set expo_push_token = $2 where id = $1`, sessionID, token)
	require.NoError(t, err)
	var id string
	require.NoError(t, e.pool.QueryRow(ctx,
		`insert into push_deliveries (service_event_id, session_id, attempt, status) values ($1, $2, 1, 'queued') returning id`,
		e.eventID, sessionID).Scan(&id))
	return id
}

func (e env) status(t *testing.T, id string) (string, *string) {
	var status string
	var errText *string
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`select status, error from push_deliveries where id = $1`, id).Scan(&status, &errText))
	return status, errText
}

func (e env) retries(t *testing.T, id string) (int, *time.Time) {
	var retries int
	var retryAt *time.Time
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`select retries, retry_at from push_deliveries where id = $1`, id).Scan(&retries, &retryAt))
	return retries, retryAt
}

func ptr(s string) *string { return &s }

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func (e env) count(t *testing.T, tail string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(), "select count(*) from "+tail, args...).Scan(&n))
	return n
}

// closeEvent turns the seeded due task into the state a master or the
// scheduler leaves it in.
func (e env) closeEvent(t *testing.T, how string) {
	t.Helper()
	switch how {
	case "confirmed":
		e.exec(t, `update service_events set state = 'confirmed', confirmed_at = now(), confirmed_by = $2 where id = $1`, e.eventID, e.fx.Masters[0].StaffID)
	case "missed":
		e.exec(t, `update service_events set state = 'missed' where id = $1`, e.eventID)
	case "finished hookah":
		e.exec(t, `update hookahs set state = 'finished', finished_at = now() where id = $1`, e.fx.BusyHookahID)
	default:
		t.Fatalf("unknown way to close an event: %s", how)
	}
}

func TestWorkerCancelsPushesOfClosedEvents(t *testing.T) {
	for _, how := range []string{"confirmed", "missed", "finished hookah"} {
		t.Run("a queued push of a "+how+" task is cancelled, not sent", func(t *testing.T) {
			e := setup(t)
			id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
			e.closeEvent(t, how)

			_, err := e.worker.RunOnce(context.Background())
			require.NoError(t, err)
			require.Empty(t, e.fake.Sent())
			status, _ := e.status(t, id)
			require.Equal(t, "cancelled", status)

			n, err := e.worker.RunOnce(context.Background())
			require.NoError(t, err)
			require.Zero(t, n, "a cancelled row is final")
		})
	}

	t.Run("a transport retry of a task confirmed meanwhile is cancelled", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		e.fake.Next = append(e.fake.Next, func([]push.Message) ([]push.Result, error) { return nil, errors.New("expo 500") })
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, _ := e.status(t, id)
		require.Equal(t, "queued", status)
		require.Len(t, e.fake.Sent(), 1)

		e.closeEvent(t, "confirmed")
		e.exec(t, `update push_deliveries set retry_at = now() - interval '1 second' where id = $1`, id)
		_, err = e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, _ = e.status(t, id)
		require.Equal(t, "cancelled", status)
		require.Len(t, e.fake.Sent(), 1, "no second send after the confirmation")
		_, retryAt := e.retries(t, id)
		require.Nil(t, retryAt)
	})

	t.Run("an open task in the same batch is still sent", func(t *testing.T) {
		e := setup(t)
		closed := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		e.exec(t, `update service_events set state = 'confirmed', confirmed_at = now() where id = $1`, e.eventID)
		var otherEvent string
		require.NoError(t, e.pool.QueryRow(context.Background(),
			`select id from service_events where hookah_id = $1 and state = 'pending' order by seq limit 1`, e.fx.BusyHookahID).Scan(&otherEvent))
		e.exec(t, `update service_events set state = 'due', fired_at = now() where id = $1`, otherEvent)
		e.exec(t, `update staff_sessions set expo_push_token = 'ExponentPushToken[def]' where id = $1`, e.fx.Masters[1].SessionID)
		var open string
		require.NoError(t, e.pool.QueryRow(context.Background(),
			`insert into push_deliveries (service_event_id, session_id, attempt, status) values ($1, $2, 1, 'queued') returning id`,
			otherEvent, e.fx.Masters[1].SessionID).Scan(&open))

		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, e.fake.Sent(), 1)
		require.Equal(t, otherEvent, e.fake.Sent()[0].Data.EventID)
		status, _ := e.status(t, closed)
		require.Equal(t, "cancelled", status)
		status, _ = e.status(t, open)
		require.Equal(t, "sent", status)
	})
}

func TestWorker(t *testing.T) {
	t.Run("fake sender marks the queued delivery sent with a valid payload", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))

		n, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, n)
		status, _ := e.status(t, id)
		require.Equal(t, "sent", status)

		sent := e.fake.Sent()
		require.Len(t, sent, 1)
		require.Equal(t, "Стол 15", sent[0].Title)
		require.Equal(t, "Стряхните угли", sent[0].Body)
		require.Equal(t, e.eventID, sent[0].Data.EventID)
		require.Equal(t, e.fx.TableIDs["15"], sent[0].Data.TableID)
		require.Equal(t, "shake", sent[0].Data.Kind)
	})

	t.Run("second run sends nothing again", func(t *testing.T) {
		e := setup(t)
		e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		n, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.Zero(t, n)
		require.Len(t, e.fake.Sent(), 1)
	})

	t.Run("session without a token is marked invalid_token", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[1].SessionID, nil)
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, _ := e.status(t, id)
		require.Equal(t, "invalid_token", status)
		require.Empty(t, e.fake.Sent())
	})

	t.Run("transport error waits and gives up after three tries", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		fail := func([]push.Message) ([]push.Result, error) { return nil, errors.New("expo 500") }

		for attempt := 1; attempt <= 2; attempt++ {
			e.fake.Next = append(e.fake.Next, fail)
			_, err := e.worker.RunOnce(context.Background())
			require.NoError(t, err)
			status, errText := e.status(t, id)
			require.Equal(t, "queued", status, "attempt %d waits for the next try", attempt)
			require.Contains(t, *errText, "expo 500")
			retries, retryAt := e.retries(t, id)
			require.Equal(t, attempt, retries)
			require.NotNil(t, retryAt)
			// The worker skips rows whose wait has not passed yet.
			e.exec(t, `update push_deliveries set retry_at = now() - interval '1 second' where id = $1`, id)
		}

		e.fake.Next = append(e.fake.Next, fail)
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, _ := e.status(t, id)
		require.Equal(t, "failed", status)
	})

	t.Run("a delivery waiting for its retry is skipped", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		e.fake.Next = append(e.fake.Next, func([]push.Message) ([]push.Result, error) { return nil, errors.New("expo 500") })
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)

		n, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.Zero(t, n)
		require.Len(t, e.fake.Sent(), 1)
		_ = id
	})

	t.Run("malformed token fails the fake loudly", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("not-a-token"))
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, errText := e.status(t, id)
		require.Equal(t, "queued", status, "a bad payload is a transport failure of the batch")
		require.Contains(t, *errText, "bad token")
	})

	t.Run("expo rejecting the device clears its token", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[gone]"))
		e.fake.Next = append(e.fake.Next, func(msgs []push.Message) ([]push.Result, error) {
			return []push.Result{{Status: push.StatusInvalidToken, Error: "DeviceNotRegistered"}}, nil
		})
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, _ := e.status(t, id)
		require.Equal(t, "invalid_token", status)
		require.Equal(t, 0, e.count(t, `staff_sessions where id = $1 and expo_push_token is not null`, e.fx.Masters[0].SessionID))
	})
}
