package staff_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/seed"
)

const (
	attackerIP = "198.51.100.9"
	wrongCode  = "987654"
)

// joinFrom posts /join the way Caddy forwards it: the client address is the
// last X-Forwarded-For entry.
func joinFrom(e *apitest.Env, forwarded, code, device string) apitest.Response {
	return e.Do("POST", "/join", "",
		apitest.With(map[string]any{"code": code, "display_name": "Нурлан"}, apitest.Device(device)),
		"X-Forwarded-For", forwarded)
}

func TestJoinLimitsByIP(t *testing.T) {
	t.Run("a hundred guesses from rotating devices and one ip stop at 429 with retry_after_sec", func(t *testing.T) {
		e := apitest.New(t, true)
		var wrong, limited int
		for i := range 100 {
			// The client picks device_id and any leading X-Forwarded-For entry,
			// only the address the proxy appended counts.
			res := joinFrom(e, fmt.Sprintf("10.0.0.%d, %s", i, attackerIP), wrongCode, fmt.Sprintf("device-guess-%03d", i))
			switch res.Code() {
			case "invalid_join_code":
				wrong++
			case "rate_limited":
				limited++
				retry := res.Details()["retry_after_sec"].(float64)
				require.Greater(t, retry, 0.0)
				require.LessOrEqual(t, retry, 60.0)
				require.Equal(t, 429, res.Status)
			default:
				t.Fatalf("attempt %d: unexpected %d %s", i, res.Status, res.Raw)
			}
		}
		require.Equal(t, 30, wrong, "thirty attempts a minute per ip")
		require.Equal(t, 70, limited)
		require.Equal(t, 30, e.Count(`join_attempts where ip = $1`, attackerIP), "limited attempts are not recorded")
	})

	t.Run("the right code from a limited ip is limited too", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 30 {
			require.Equal(t, "invalid_join_code", joinFrom(e, attackerIP, wrongCode, fmt.Sprintf("device-guess-%03d", i)).Code())
		}
		res := joinFrom(e, attackerIP, seed.JoinCode, "device-lucky-01")
		require.Equal(t, 429, res.Status)
		require.Equal(t, "rate_limited", res.Code())
	})

	t.Run("another ip is not blocked by it", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 40 {
			joinFrom(e, attackerIP, wrongCode, fmt.Sprintf("device-guess-%03d", i))
		}
		res := joinFrom(e, "203.0.113.20", seed.JoinCode, "device-master-01")
		require.Equal(t, 200, res.Status, string(res.Raw))
	})

	t.Run("masters behind one wifi share an ip and still get in", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 12 {
			res := joinFrom(e, "203.0.113.30", seed.JoinCode, fmt.Sprintf("device-wifi-%02d", i))
			require.Equal(t, 200, res.Status, "master %d: %s", i, res.Raw)
		}
	})

	t.Run("a parallel volley from one ip records exactly the limit", func(t *testing.T) {
		e := apitest.New(t, true)
		codes := make(chan string, 40)
		var wg sync.WaitGroup
		for i := range 40 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes <- joinFrom(e, attackerIP, wrongCode, fmt.Sprintf("device-volley-%03d", i)).Code()
			}()
		}
		wg.Wait()
		close(codes)
		got := map[string]int{}
		for c := range codes {
			got[c]++
		}
		require.Equal(t, map[string]int{"invalid_join_code": 30, "rate_limited": 10}, got)
		require.Equal(t, 30, e.Count(`join_attempts where ip = $1`, attackerIP))
	})

	t.Run("attempts store the address and old rows without one never match", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 40 {
			e.Exec(`insert into join_attempts (device_id, code, success) values ($1, $2, false)`, fmt.Sprintf("device-old-%03d", i), wrongCode)
		}
		require.Equal(t, 200, joinFrom(e, "203.0.113.40", seed.JoinCode, "device-new-0001").Status)
		require.Equal(t, 1, e.Count(`join_attempts where ip = '203.0.113.40' and success`))
	})
}

func TestJoinServiceBudget(t *testing.T) {
	failures := func(e *apitest.Env, n int) {
		e.Exec(`insert into join_attempts (device_id, code, success, ip, created_at)
		        select 'device-bot-' || g, $1, false, ('10.1.' || (g / 250) || '.' || (g % 250))::inet, now() - (g % 3000) * interval '1 second'
		        from generate_series(1, $2) g`, wrongCode, n)
	}

	t.Run("299 wrong codes in the last hour leave room", func(t *testing.T) {
		e := apitest.New(t, true)
		failures(e, 299)
		res := joinFrom(e, "203.0.113.50", seed.JoinCode, "device-master-01")
		require.Equal(t, 200, res.Status, string(res.Raw))
	})

	t.Run("300 wrong codes exhaust the budget for every ip, the right code included", func(t *testing.T) {
		e := apitest.New(t, true)
		failures(e, 300)
		res := joinFrom(e, "203.0.113.51", seed.JoinCode, "device-master-01")
		require.Equal(t, 429, res.Status)
		require.Equal(t, "rate_limited", res.Code())
		retry := res.Details()["retry_after_sec"].(float64)
		require.Greater(t, retry, 0.0)
		require.LessOrEqual(t, retry, 3600.0)
		require.Zero(t, e.Count(`staff where display_name = 'Нурлан'`), "a limited join creates nobody")
	})

	t.Run("wrong codes older than an hour do not count", func(t *testing.T) {
		e := apitest.New(t, true)
		failures(e, 300)
		e.Exec(`update join_attempts set created_at = created_at - interval '2 hours'`)
		require.Equal(t, 200, joinFrom(e, "203.0.113.52", seed.JoinCode, "device-master-01").Status)
	})

	t.Run("right codes do not spend the budget", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 12 {
			require.Equal(t, 200, joinFrom(e, fmt.Sprintf("203.0.113.%d", 100+i), seed.JoinCode, fmt.Sprintf("device-ok-%02d", i)).Status)
		}
		require.Zero(t, e.Count(`join_attempts where not success`))
	})
}
