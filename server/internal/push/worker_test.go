package push_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
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

func ptr(s string) *string { return &s }

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

	t.Run("transport error marks the delivery failed", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("ExponentPushToken[abc]"))
		e.fake.Next = append(e.fake.Next, func([]push.Message) ([]push.Result, error) {
			return nil, errors.New("expo 500")
		})
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, errText := e.status(t, id)
		require.Equal(t, "failed", status)
		require.Contains(t, *errText, "expo 500")
	})

	t.Run("malformed token fails the fake loudly", func(t *testing.T) {
		e := setup(t)
		id := e.queue(t, e.fx.Masters[0].SessionID, ptr("not-a-token"))
		_, err := e.worker.RunOnce(context.Background())
		require.NoError(t, err)
		status, errText := e.status(t, id)
		require.Equal(t, "failed", status)
		require.Contains(t, *errText, "bad token")
	})
}
