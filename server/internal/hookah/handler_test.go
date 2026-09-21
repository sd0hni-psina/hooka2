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

func key() string {
	return newUUID()
}

var uuidCounter struct {
	sync.Mutex
	n int
}

// newUUID builds a valid v4-shaped key without pulling in a uuid library.
func newUUID() string {
	uuidCounter.Lock()
	defer uuidCounter.Unlock()
	uuidCounter.n++
	const hex = "0123456789abcdef"
	out := []byte("00000000-0000-4000-8000-000000000000")
	n := uuidCounter.n
	for i := len(out) - 1; i >= 0 && n > 0; i-- {
		if out[i] == '-' {
			continue
		}
		out[i] = hex[n%16]
		n /= 16
	}
	return string(out)
}

func start(e *apitest.Env, token, tableID, idemKey string) apitest.Response {
	return e.Do("POST", "/tables/"+tableID+"/hookah", token, nil, "Idempotency-Key", idemKey)
}

// parallelStarts fires one start per table id at once with a shared key and
// returns every response.
func parallelStarts(e *apitest.Env, token, idemKey string, tables ...string) []apitest.Response {
	out := make([]apitest.Response, len(tables))
	var wg sync.WaitGroup
	for i, table := range tables {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = start(e, token, table, idemKey)
		}()
	}
	wg.Wait()
	return out
}

func decodeHookah(t *testing.T, res apitest.Response) domain.Hookah {
	t.Helper()
	var h domain.Hookah
	res.Decode(t, &h)
	return h
}

func TestStartHookah(t *testing.T) {
	t.Run("tap starts a hookah and materializes the whole plan at once", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		table := e.Fx.TableIDs["1"]

		res := start(e, token, table, key())
		require.Equal(t, 200, res.Status, string(res.Raw))
		h := decodeHookah(t, res)
		require.Equal(t, domain.HookahActive, h.State)
		require.Equal(t, domain.StatusGreen, h.Status)
		require.Equal(t, table, h.TableID)
		require.Equal(t, "Ерлан", h.StartedByName)
		require.WithinDuration(t, time.Now(), h.StartedAt, 5*time.Second)
		require.Equal(t, h.StartedAt.Add(80*time.Minute), h.ExpiresAt)

		want := domain.MaterializePlan(domain.DefaultPlan(), h.StartedAt)
		require.Len(t, h.Events, len(want))
		require.Equal(t, 7, len(want))
		for i, e := range want {
			require.Equal(t, e.Seq, h.Events[i].Seq)
			require.Equal(t, e.Kind, h.Events[i].Kind)
			require.True(t, e.DueAt.Equal(h.Events[i].DueAt))
			require.Equal(t, domain.EventPending, h.Events[i].State)
		}
		require.Equal(t, len(want), e.Count(`service_events where hookah_id = $1`, h.ID))
	})

	t.Run("second tap on a busy table is table_occupied", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		table := e.Fx.TableIDs["1"]
		require.Equal(t, 200, start(e, token, table, key()).Status)

		res := start(e, token, table, key())
		require.Equal(t, 409, res.Status)
		require.Equal(t, "table_occupied", res.Code())
		require.Equal(t, 1, e.Count(`hookahs where table_id = $1`, table))
	})

	t.Run("retry with the same idempotency key returns the same hookah", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		k := key()
		first := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], k))
		second := start(e, token, e.Fx.TableIDs["1"], k)
		require.Equal(t, 200, second.Status)
		require.Equal(t, first.ID, decodeHookah(t, second).ID)
		require.Equal(t, 1, e.Count(`hookahs where idempotency_key = $1`, k))
		require.Equal(t, 7, e.Count(`service_events where hookah_id = $1`, first.ID))

		other := start(e, token, e.Fx.TableIDs["2"], k)
		require.Equal(t, 200, other.Status)
		require.Equal(t, first.ID, decodeHookah(t, other).ID, "the key wins over the table")
	})

	t.Run("two masters tapping one table at once create one hookah", func(t *testing.T) {
		e := apitest.New(t, true)
		table := e.Fx.TableIDs["2"]
		tokens := []string{e.TokenFor(e.Fx.Masters[0]), e.TokenFor(e.Fx.Masters[1])}
		codes := make(chan int, 8)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes <- start(e, tokens[i%2], table, key()).Status
			}()
		}
		wg.Wait()
		close(codes)
		ok := 0
		for c := range codes {
			if c == 200 {
				ok++
			} else {
				require.Equal(t, 409, c)
			}
		}
		require.Equal(t, 1, ok)
		require.Equal(t, 1, e.Count(`hookahs where table_id = $1`, table))
	})

	t.Run("parallel retries with one key on one table all return the same hookah", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		// Several rounds on free tables (the seed occupies 3 and 15): the race is timing dependent.
		for _, label := range []string{"4", "5", "6", "7", "8"} {
			table, k := e.Fx.TableIDs[label], key()
			results := parallelStarts(e, token, k, table, table, table, table, table, table, table, table)
			ids := map[string]bool{}
			for _, r := range results {
				require.Equal(t, 200, r.Status, "table %s: %s", label, r.Raw)
				ids[decodeHookah(t, r).ID] = true
			}
			require.Len(t, ids, 1, "table %s", label)
			require.Equal(t, 1, e.Count(`hookahs where idempotency_key = $1`, k))
			require.Equal(t, 7, e.Count(`service_events where hookah_id in (select id from hookahs where idempotency_key = $1)`, k))
		}
	})

	t.Run("parallel retries with one key aimed at two tables never fail with 500", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		for _, pair := range [][2]string{{"4", "5"}, {"6", "7"}, {"8", "9"}} {
			a, b, k := e.Fx.TableIDs[pair[0]], e.Fx.TableIDs[pair[1]], key()
			tables := make([]string, 16)
			for i := range tables {
				tables[i] = []string{a, b}[i%2]
			}
			results := parallelStarts(e, token, k, tables...)
			ids := map[string]bool{}
			for _, r := range results {
				require.Equal(t, 200, r.Status, "tables %v: %s", pair, r.Raw)
				ids[decodeHookah(t, r).ID] = true
			}
			require.Len(t, ids, 1)
			require.Equal(t, 1, e.Count(`hookahs where idempotency_key = $1`, k))
		}
	})

	t.Run("a repeated key and a fresh key racing for one table leave one hookah and one refusal", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		table, k := e.Fx.TableIDs["4"], key()
		results := parallelStarts(e, token, k, table, table, table, table)
		other := start(e, token, table, key())
		require.Equal(t, 409, other.Status)
		require.Equal(t, "table_occupied", other.Code())
		for _, r := range results {
			require.Equal(t, 200, r.Status, string(r.Raw))
		}
		require.Equal(t, 1, e.Count(`hookahs where table_id = $1`, table))
	})

	t.Run("bad or missing idempotency key is validation_failed", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		for _, k := range []string{"", "not-a-uuid"} {
			res := start(e, token, e.Fx.TableIDs["1"], k)
			require.Equal(t, 400, res.Status)
			require.Contains(t, res.Details()["fields"], "idempotency_key")
		}
		require.Equal(t, 2, e.Count(`hookahs`), "only the two seeded hookahs exist")
	})

	t.Run("archived and foreign tables are not_found", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		e.Exec(`update tables set archived_at = now() where id = $1`, e.Fx.TableIDs["1"])
		require.Equal(t, 404, start(e, token, e.Fx.TableIDs["1"], key()).Status)
		require.Equal(t, 404, start(e, token, "00000000-0000-4000-8000-000000000000", key()).Status)
		require.Equal(t, 404, start(e, token, "15", key()).Status)
	})

	t.Run("the second device sees the new hookah over the socket", func(t *testing.T) {
		e := apitest.New(t, true)
		ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[0])
		h := decodeHookah(t, start(e, e.TokenFor(e.Fx.Masters[0]), e.Fx.TableIDs["1"], key()))

		m := ws.Until("hookah.started")
		var data hookah.StartedData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, h.ID, data.Hookah.ID)
		require.Len(t, data.Events, 7)
		require.Equal(t, domain.StatusGreen, data.Status)
		require.Equal(t, e.Fx.HallIDs[0], *m.HallID)
	})
}

func TestScheduler(t *testing.T) {
	setup := func(t *testing.T) (*apitest.Env, domain.Hookah, *apitest.WS) {
		e := apitest.New(t, true)
		// The seeded hookahs would answer the same ticks; this test is about one.
		e.Exec(`delete from hookahs`)
		ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[0])
		h := decodeHookah(t, start(e, e.TokenFor(e.Fx.Masters[0]), e.Fx.TableIDs["1"], key()))
		ws.Until("hookah.started")
		return e, h, ws
	}
	state := func(e *apitest.Env, eventID string) string {
		var s string
		require.NoError(t, e.Pool.QueryRow(context.Background(), `select state from service_events where id = $1`, eventID).Scan(&s))
		return s
	}

	t.Run("the five minute task turns due and yellow for everyone", func(t *testing.T) {
		e, h, ws := setup(t)
		res, err := e.Hookah.Tick(context.Background(), h.StartedAt.Add(5*time.Minute))
		require.NoError(t, err)
		require.Equal(t, 1, res.Due)

		m := ws.Until("service.due")
		var data hookah.DueData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, h.Events[0].ID, data.Event.ID)
		require.Equal(t, domain.KindCheck, data.Event.Kind)
		require.Equal(t, "1", data.TableLabel)
		require.Equal(t, domain.StatusYellow, data.Status)
		require.Equal(t, "due", state(e, h.Events[0].ID))
	})

	t.Run("a repeated tick has no effect", func(t *testing.T) {
		e, h, _ := setup(t)
		// At 16 minutes the check of minute 5 is already past missed_after_sec,
		// so one tick both fires it and closes it.
		at := h.StartedAt.Add(16 * time.Minute)
		first, err := e.Hookah.Tick(context.Background(), at)
		require.NoError(t, err)
		require.Equal(t, 2, first.Due, "check at 5 and shake at 15 minutes")
		require.Equal(t, 1, first.Missed, "the check of minute 5")

		second, err := e.Hookah.Tick(context.Background(), at)
		require.NoError(t, err)
		require.Equal(t, 0, second.Due)
		require.Equal(t, 0, second.Missed)
		require.Equal(t, 1, e.Count(`service_events where hookah_id = $1 and state = 'due'`, h.ID))
		require.Equal(t, 1, e.Count(`service_events where hookah_id = $1 and state = 'missed'`, h.ID))
		require.Equal(t, 5, e.Count(`service_events where hookah_id = $1 and state = 'pending'`, h.ID))
	})

	t.Run("an unconfirmed task turns missed once and stays missed", func(t *testing.T) {
		e, h, ws := setup(t)
		_, err := e.Hookah.Tick(context.Background(), h.StartedAt.Add(5*time.Minute))
		require.NoError(t, err)
		ws.Until("service.due")

		at := h.StartedAt.Add(5*time.Minute + 601*time.Second)
		res, err := e.Hookah.Tick(context.Background(), at)
		require.NoError(t, err)
		require.Equal(t, 1, res.Missed)
		require.Equal(t, "missed", state(e, h.Events[0].ID))

		again, err := e.Hookah.Tick(context.Background(), at)
		require.NoError(t, err)
		require.Equal(t, 0, again.Missed)

		m := ws.Until("service.missed")
		var data hookah.MissedData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, h.Events[0].ID, data.EventID)
	})

	t.Run("at eighty minutes the hookah expires and the table turns red", func(t *testing.T) {
		e, h, ws := setup(t)
		res, err := e.Hookah.Tick(context.Background(), h.ExpiresAt)
		require.NoError(t, err)
		require.Equal(t, 1, res.Expired)

		m := ws.Until("hookah.expired")
		var data hookah.ExpiredData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, h.ID, data.HookahID)
		require.Equal(t, domain.StatusRed, data.Status)
		require.Equal(t, 1, e.Count(`hookahs where id = $1 and state = 'expired'`, h.ID))

		again, err := e.Hookah.Tick(context.Background(), h.ExpiresAt)
		require.NoError(t, err)
		require.Equal(t, 0, again.Expired)
	})

	t.Run("a finished hookah stops producing due events", func(t *testing.T) {
		e, h, _ := setup(t)
		require.Equal(t, 200, e.Do("POST", "/hookahs/"+h.ID+"/finish", e.TokenFor(e.Fx.Masters[0]), map[string]any{"reason": "guest_left"}).Status)
		res, err := e.Hookah.Tick(context.Background(), h.ExpiresAt)
		require.NoError(t, err)
		require.Equal(t, 0, res.Due)
		require.Equal(t, 0, res.Expired)
	})
}

func TestTransfer(t *testing.T) {
	transfer := func(e *apitest.Env, token, hookahID, toTableID string) apitest.Response {
		return e.Do("POST", "/hookahs/"+hookahID+"/transfer", token, map[string]any{"to_table_id": toTableID})
	}

	t.Run("transfer keeps the counter and every future deadline", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		before := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		_, err := e.Hookah.Tick(context.Background(), before.StartedAt.Add(20*time.Minute))
		require.NoError(t, err)

		res := transfer(e, token, before.ID, e.Fx.TableIDs["2"])
		require.Equal(t, 200, res.Status, string(res.Raw))
		after := decodeHookah(t, res)
		require.Equal(t, e.Fx.TableIDs["2"], after.TableID)
		require.True(t, before.StartedAt.Equal(after.StartedAt), "the counter keeps running from the same moment")
		require.True(t, before.ExpiresAt.Equal(after.ExpiresAt))
		require.Len(t, after.Events, len(before.Events))
		for i := range before.Events {
			require.True(t, before.Events[i].DueAt.Equal(after.Events[i].DueAt), "event %d moved", i+1)
		}
		require.Equal(t, 0, e.Count(`hookahs where table_id = $1 and state in ('active','expired')`, e.Fx.TableIDs["1"]), "the old table is free")
		require.Equal(t, 1, e.Count(`hookah_transfers where hookah_id = $1`, before.ID))
	})

	t.Run("transfer to a busy table is table_occupied", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		first := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		require.Equal(t, 200, start(e, token, e.Fx.TableIDs["2"], key()).Status)

		res := transfer(e, token, first.ID, e.Fx.TableIDs["2"])
		require.Equal(t, 409, res.Status)
		require.Equal(t, "table_occupied", res.Code())
		require.Equal(t, 1, e.Count(`hookahs where id = $1 and table_id = $2`, first.ID, e.Fx.TableIDs["1"]))
	})

	t.Run("transfer onto the same table changes nothing", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		res := transfer(e, token, h.ID, e.Fx.TableIDs["1"])
		require.Equal(t, 200, res.Status)
		require.Equal(t, 0, e.Count(`hookah_transfers where hookah_id = $1`, h.ID))
	})

	t.Run("transfer to another hall moves the hookah between halls", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[1])

		res := transfer(e, token, h.ID, e.Fx.TableIDs["11"])
		require.Equal(t, 200, res.Status)
		moved := decodeHookah(t, res)
		require.Equal(t, e.Fx.HallIDs[1], moved.HallID)

		m := ws.Until("hookah.transferred")
		require.Equal(t, e.Fx.HallIDs[1], *m.HallID)
		var data hookah.TransferredData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, e.Fx.TableIDs["1"], data.FromTableID)
		require.Equal(t, e.Fx.TableIDs["11"], data.ToTableID)
		require.Equal(t, "Ерлан", data.MovedByName)
		require.Len(t, data.Hookah.Events, 7, "the destination hall learns the whole hookah")
	})

	t.Run("finished hookah cannot be transferred", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		require.Equal(t, 200, e.Do("POST", "/hookahs/"+h.ID+"/finish", token, map[string]any{"reason": "guest_left"}).Status)
		res := transfer(e, token, h.ID, e.Fx.TableIDs["2"])
		require.Equal(t, 409, res.Status)
		require.Equal(t, "hookah_finished", res.Code())
	})

	t.Run("unknown or archived destination is not_found", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		e.Exec(`update tables set archived_at = now() where id = $1`, e.Fx.TableIDs["2"])
		require.Equal(t, 404, transfer(e, token, h.ID, e.Fx.TableIDs["2"]).Status)
		require.Equal(t, 404, transfer(e, token, h.ID, "00000000-0000-4000-8000-000000000000").Status)
		require.Equal(t, 404, transfer(e, token, "00000000-0000-4000-8000-000000000000", e.Fx.TableIDs["4"]).Status)
	})
}

func TestFinish(t *testing.T) {
	finish := func(e *apitest.Env, token, hookahID string, reason any) apitest.Response {
		return e.Do("POST", "/hookahs/"+hookahID+"/finish", token, map[string]any{"reason": reason})
	}

	t.Run("finish frees the table and cancels future tasks", func(t *testing.T) {
		e := apitest.New(t, true)
		e.Exec(`delete from hookahs`)
		token := e.TokenFor(e.Fx.Masters[0])
		ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		_, err := e.Hookah.Tick(context.Background(), h.StartedAt.Add(5*time.Minute))
		require.NoError(t, err)

		res := finish(e, token, h.ID, "guest_left")
		require.Equal(t, 200, res.Status, string(res.Raw))
		done := decodeHookah(t, res)
		require.Equal(t, domain.HookahFinished, done.State)
		require.Equal(t, domain.StatusFree, done.Status)
		require.NotNil(t, done.FinishedAt)
		require.Equal(t, 6, e.Count(`service_events where hookah_id = $1 and state = 'cancelled'`, h.ID))
		require.Equal(t, 1, e.Count(`service_events where hookah_id = $1 and state = 'due'`, h.ID), "the fired task stays for history")

		m := ws.Until("hookah.finished")
		var data hookah.FinishedData
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, h.ID, data.HookahID)
		require.Equal(t, domain.FinishGuestLeft, data.Reason)

		require.Equal(t, 200, start(e, token, e.Fx.TableIDs["1"], key()).Status, "the table is free again")
	})

	t.Run("second finish is hookah_finished", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		require.Equal(t, 200, finish(e, token, h.ID, "shift_end").Status)
		res := finish(e, token, h.ID, "shift_end")
		require.Equal(t, 409, res.Status)
		require.Equal(t, "hookah_finished", res.Code())
	})

	t.Run("unknown reason is validation_failed", func(t *testing.T) {
		e := apitest.New(t, true)
		token := e.TokenFor(e.Fx.Masters[0])
		h := decodeHookah(t, start(e, token, e.Fx.TableIDs["1"], key()))
		res := finish(e, token, h.ID, "because")
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"reason": "invalid"}, res.Details()["fields"])
	})
}

func TestHallStateEndpoint(t *testing.T) {
	e := apitest.New(t, true)
	token := e.TokenFor(e.Fx.Masters[0])
	res := e.Do("GET", "/halls/"+e.Fx.HallIDs[1]+"/state", token, nil)
	require.Equal(t, 200, res.Status)
	var state hookah.HallState
	res.Decode(t, &state)
	require.Equal(t, "Второй зал", state.Hall.Name)
	require.Len(t, state.Tables, 8)
	require.Len(t, state.Hookahs, 1)
	require.Equal(t, e.Fx.BusyHookahID, state.Hookahs[0].ID)
	require.Equal(t, domain.DefaultPlan(), state.Plan)

	require.Equal(t, 404, e.Do("GET", "/halls/00000000-0000-4000-8000-000000000000/state", token, nil).Status)
	require.Equal(t, 401, e.Do("GET", "/halls/"+e.Fx.HallIDs[1]+"/state", "", nil).Status)
}
