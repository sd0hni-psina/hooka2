package push_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/push"
)

// presenceSet answers who has the app open.
type presenceSet map[string]bool

func (p presenceSet) SessionOnline(_ string, sessionID string) bool { return p[sessionID] }

type queueEnv struct {
	env     *apitest.Env
	queue   *push.Queue
	eventID string
	dueAt   time.Time
	online  presenceSet
}

func setupQueue(t *testing.T) queueEnv {
	e := apitest.New(t, true)
	online := presenceSet{}
	q := push.NewQueue(e.Pool, online)
	// Every seeded session can take a push.
	e.Exec(`update staff_sessions set expo_push_token = 'ExponentPushToken[' || device_id || ']'`)

	var eventID string
	var dueAt time.Time
	require.NoError(t, e.Pool.QueryRow(context.Background(),
		`select id, due_at from service_events where hookah_id = $1 and state = 'due'`, e.Fx.BusyHookahID).Scan(&eventID, &dueAt))
	return queueEnv{env: e, queue: q, eventID: eventID, dueAt: dueAt, online: online}
}

func (q queueEnv) deliveries() int {
	return q.env.Count(`push_deliveries where service_event_id = $1`, q.eventID)
}

func TestQueue(t *testing.T) {
	t.Run("a due task queues one push per session and never twice", func(t *testing.T) {
		q := setupQueue(t)
		now := time.Now()
		res, err := q.queue.Tick(context.Background(), now)
		require.NoError(t, err)
		require.Equal(t, 4, res.Queued, "owner and three masters")
		require.Equal(t, 4, q.deliveries())
		require.Equal(t, 4, q.env.Count(`push_deliveries where service_event_id = $1 and attempt = 1 and status = 'queued'`, q.eventID))

		again, err := q.queue.Tick(context.Background(), now)
		require.NoError(t, err)
		require.Zero(t, again.Queued, "the same tick queues nothing new")
		require.Equal(t, 4, q.deliveries())
	})

	t.Run("an open app gets no push", func(t *testing.T) {
		q := setupQueue(t)
		q.online[q.env.Fx.Masters[0].SessionID] = true
		res, err := q.queue.Tick(context.Background(), time.Now())
		require.NoError(t, err)
		require.Equal(t, 3, res.Queued)
		require.Equal(t, 0, q.env.Count(`push_deliveries where service_event_id = $1 and session_id = $2`, q.eventID, q.env.Fx.Masters[0].SessionID))
	})

	t.Run("an unconfirmed task repeats after push_repeat_sec up to the limit", func(t *testing.T) {
		q := setupQueue(t)
		now := time.Now()
		_, err := q.queue.Tick(context.Background(), now)
		require.NoError(t, err)

		tooEarly, err := q.queue.Tick(context.Background(), now.Add(119*time.Second))
		require.NoError(t, err)
		require.Zero(t, tooEarly.Queued)

		second, err := q.queue.Tick(context.Background(), now.Add(120*time.Second))
		require.NoError(t, err)
		require.Equal(t, 4, second.Queued)
		third, err := q.queue.Tick(context.Background(), now.Add(240*time.Second))
		require.NoError(t, err)
		require.Equal(t, 4, third.Queued)

		fourth, err := q.queue.Tick(context.Background(), now.Add(360*time.Second))
		require.NoError(t, err)
		require.Zero(t, fourth.Queued, "push_max_attempts is three")
		require.Equal(t, 12, q.deliveries())
	})

	t.Run("a confirmed task stops the repeats", func(t *testing.T) {
		q := setupQueue(t)
		now := time.Now()
		_, err := q.queue.Tick(context.Background(), now)
		require.NoError(t, err)
		res := q.env.Do("POST", "/service-events/"+q.eventID+"/confirm", q.env.TokenFor(q.env.Fx.Masters[0]), nil)
		require.Equal(t, 200, res.Status)

		next, err := q.queue.Tick(context.Background(), now.Add(120*time.Second))
		require.NoError(t, err)
		require.Zero(t, next.Queued)
		require.Equal(t, 4, q.deliveries())
	})

	t.Run("a missed task stops the repeats", func(t *testing.T) {
		q := setupQueue(t)
		now := time.Now()
		_, err := q.queue.Tick(context.Background(), now)
		require.NoError(t, err)
		q.env.Exec(`update service_events set state = 'missed' where id = $1`, q.eventID)

		next, err := q.queue.Tick(context.Background(), now.Add(120*time.Second))
		require.NoError(t, err)
		require.Zero(t, next.Queued)
	})

	t.Run("a finished hookah queues nothing although its task stays due", func(t *testing.T) {
		q := setupQueue(t)
		res := q.env.Do("POST", "/hookahs/"+q.env.Fx.BusyHookahID+"/finish", q.env.TokenFor(q.env.Fx.Masters[0]), map[string]any{"reason": "guest_left"})
		require.Equal(t, 200, res.Status, string(res.Raw))
		require.Equal(t, 1, q.env.Count(`service_events where id = $1 and state = 'due'`, q.eventID), "history keeps the due task")

		now := time.Now()
		for _, at := range []time.Time{now, now, now.Add(120 * time.Second)} {
			got, err := q.queue.Tick(context.Background(), at)
			require.NoError(t, err)
			require.Zero(t, got.Queued)
		}
		require.Zero(t, q.deliveries())
	})

	t.Run("sessions without a token or of blocked staff are skipped", func(t *testing.T) {
		q := setupQueue(t)
		q.env.Exec(`update staff_sessions set expo_push_token = null where id = $1`, q.env.Fx.Masters[0].SessionID)
		q.env.Exec(`update staff set blocked_at = now() where id = $1`, q.env.Fx.Masters[1].StaffID)
		q.env.Exec(`update staff_sessions set revoked_at = now() where id = $1`, q.env.Fx.Owner.SessionID)

		res, err := q.queue.Tick(context.Background(), time.Now())
		require.NoError(t, err)
		require.Equal(t, 1, res.Queued)
	})

	t.Run("a push of a due task carries the contract payload", func(t *testing.T) {
		q := setupQueue(t)
		_, err := q.queue.Tick(context.Background(), time.Now())
		require.NoError(t, err)

		fake := push.NewFake(discardLogger())
		worker := push.NewWorker(q.env.Pool, fake, discardLogger(), time.Second)
		n, err := worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.Equal(t, 4, n)

		sent := fake.Sent()
		require.Len(t, sent, 4)
		require.Equal(t, "Стол 15", sent[0].Title)
		require.Equal(t, "Стряхните угли", sent[0].Body)
		require.Equal(t, "service_action", sent[0].CategoryID)
		require.Equal(t, "high", sent[0].Priority)
		require.Equal(t, q.eventID, sent[0].Data.EventID)
		require.Equal(t, string(domain.KindShake), sent[0].Data.Kind)
		require.True(t, q.dueAt.Equal(sent[0].Data.DueAt))
		require.Equal(t, 4, q.env.Count(`push_deliveries where service_event_id = $1 and status = 'sent'`, q.eventID))
	})
}
