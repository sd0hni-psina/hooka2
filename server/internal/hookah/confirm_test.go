package hookah_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/hookah"
)

type liveHookah struct {
	env    *apitest.Env
	hookah domain.Hookah
	ws     *apitest.WS
}

// withDueTask starts a hookah and fires its first task.
func withDueTask(t *testing.T) liveHookah {
	e := apitest.New(t, true)
	e.Exec(`delete from hookahs`)
	ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[0])
	h := decodeHookah(t, start(e, e.TokenFor(e.Fx.Masters[0]), e.Fx.TableIDs["1"], key()))
	ws.Until("hookah.started")
	_, err := e.Hookah.Tick(context.Background(), h.StartedAt.Add(5*time.Minute))
	require.NoError(t, err)
	ws.Until("service.due")
	return liveHookah{env: e, hookah: h, ws: ws}
}

func decodeEvent(t *testing.T, res apitest.Response) domain.ServiceEvent {
	t.Helper()
	var ev domain.ServiceEvent
	res.Decode(t, &ev)
	return ev
}

func TestClaim(t *testing.T) {
	t.Run("the first master takes the task and the second sees the name", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		eventID := l.hookah.Events[0].ID

		first := e.Do("POST", "/service-events/"+eventID+"/claim", e.TokenFor(e.Fx.Masters[0]), nil)
		require.Equal(t, 200, first.Status, string(first.Raw))
		require.Equal(t, "Ерлан", *decodeEvent(t, first).ClaimedByName)

		m := l.ws.Until("service.claimed")
		var data hookah.ClaimedData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, eventID, data.EventID)
		require.Equal(t, "Ерлан", data.ClaimedByName)

		second := e.Do("POST", "/service-events/"+eventID+"/claim", e.TokenFor(e.Fx.Masters[1]), nil)
		require.Equal(t, 200, second.Status, "claiming a taken task is not an error")
		require.Equal(t, e.Fx.Masters[0].StaffID, *decodeEvent(t, second).ClaimedBy, "the first master keeps it")
	})

	t.Run("a task that has not fired cannot be claimed", func(t *testing.T) {
		l := withDueTask(t)
		res := l.env.Do("POST", "/service-events/"+l.hookah.Events[1].ID+"/claim", l.env.TokenFor(l.env.Fx.Masters[0]), nil)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "event_not_actionable", res.Code())
	})

	t.Run("an event of another venue is not_found", func(t *testing.T) {
		l := withDueTask(t)
		res := l.env.Do("POST", "/service-events/00000000-0000-4000-8000-000000000000/claim", l.env.TokenFor(l.env.Fx.Masters[0]), nil)
		require.Equal(t, 404, res.Status)
	})
}

func TestConfirm(t *testing.T) {
	confirm := func(e *apitest.Env, token, eventID string) apitest.Response {
		return e.Do("POST", "/service-events/"+eventID+"/confirm", token, nil)
	}

	t.Run("confirmation closes the task and does not move any deadline", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		res := confirm(e, e.TokenFor(e.Fx.Masters[0]), l.hookah.Events[0].ID)
		require.Equal(t, 200, res.Status, string(res.Raw))
		ev := decodeEvent(t, res)
		require.Equal(t, domain.EventConfirmed, ev.State)
		require.False(t, ev.ConfirmedEarly)
		require.Equal(t, "Ерлан", *ev.ConfirmedByName)

		var state hookah.HallState
		e.Do("GET", "/halls/"+e.Fx.HallIDs[0]+"/state", e.TokenFor(e.Fx.Masters[0]), nil).Decode(t, &state)
		require.Len(t, state.Hookahs, 1)
		after := state.Hookahs[0]
		require.True(t, l.hookah.ExpiresAt.Equal(after.ExpiresAt))
		for i, before := range l.hookah.Events {
			require.True(t, before.DueAt.Equal(after.Events[i].DueAt), "event %d moved", i+1)
		}

		m := l.ws.Until("service.confirmed")
		var data hookah.ConfirmedData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, domain.StatusGreen, data.Status, "the table goes back to green")
		require.False(t, data.ConfirmedEarly)
	})

	t.Run("two masters confirming at once both see success and the database keeps one", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		tokens := []string{e.TokenFor(e.Fx.Masters[0]), e.TokenFor(e.Fx.Masters[1])}
		var wg sync.WaitGroup
		codes := make(chan int, 6)
		for i := range 6 {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- confirm(e, tokens[i%2], l.hookah.Events[0].ID).Status }()
		}
		wg.Wait()
		close(codes)
		for c := range codes {
			require.Equal(t, 200, c)
		}
		require.Equal(t, 1, e.Count(`service_events where id = $1 and confirmed_by is not null`, l.hookah.Events[0].ID))
	})

	t.Run("the nearest pending task of a kind confirms early", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		shake := l.hookah.Events[1]
		require.Equal(t, domain.KindShake, shake.Kind)

		res := confirm(e, e.TokenFor(e.Fx.Masters[0]), shake.ID)
		require.Equal(t, 200, res.Status, string(res.Raw))
		ev := decodeEvent(t, res)
		require.Equal(t, domain.EventConfirmed, ev.State)
		require.True(t, ev.ConfirmedEarly)

		later := l.hookah.Events[3]
		require.Equal(t, domain.KindShake, later.Kind)
		res = confirm(e, e.TokenFor(e.Fx.Masters[0]), later.ID)
		require.Equal(t, 200, res.Status, "it is now the nearest pending shake")

		expire := l.hookah.Events[6]
		require.Equal(t, domain.KindExpire, expire.Kind)
		require.Equal(t, 200, confirm(e, e.TokenFor(e.Fx.Masters[0]), expire.ID).Status)
	})

	t.Run("a later pending task of the same kind is not actionable", func(t *testing.T) {
		l := withDueTask(t)
		res := confirm(l.env, l.env.TokenFor(l.env.Fx.Masters[0]), l.hookah.Events[3].ID)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "event_not_actionable", res.Code())
	})

	t.Run("confirming twice is idempotent", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		require.Equal(t, 200, confirm(e, e.TokenFor(e.Fx.Masters[0]), l.hookah.Events[0].ID).Status)
		res := confirm(e, e.TokenFor(e.Fx.Masters[1]), l.hookah.Events[0].ID)
		require.Equal(t, 200, res.Status)
		require.Equal(t, e.Fx.Masters[0].StaffID, *decodeEvent(t, res).ConfirmedBy)
	})

	t.Run("a missed task cannot be confirmed", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		_, err := e.Hookah.Tick(context.Background(), l.hookah.StartedAt.Add(16*time.Minute))
		require.NoError(t, err)
		res := confirm(e, e.TokenFor(e.Fx.Masters[0]), l.hookah.Events[0].ID)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "event_not_actionable", res.Code())
	})

	t.Run("a finished hookah rejects confirmation", func(t *testing.T) {
		l := withDueTask(t)
		e := l.env
		require.Equal(t, 200, e.Do("POST", "/hookahs/"+l.hookah.ID+"/finish", e.TokenFor(e.Fx.Masters[0]), map[string]any{"reason": "guest_left"}).Status)
		res := confirm(e, e.TokenFor(e.Fx.Masters[0]), l.hookah.Events[0].ID)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "hookah_finished", res.Code())
	})
}
