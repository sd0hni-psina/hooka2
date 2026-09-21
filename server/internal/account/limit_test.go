package account_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
)

const (
	ownerEmail = "owner@example.kz"
	ownerPass  = "hookah-pass-1"
)

// loginFrom posts /auth/login the way Caddy forwards it: the client address is
// the last X-Forwarded-For entry.
func loginFrom(e *apitest.Env, ip, email, password string) apitest.Response {
	return e.Do("POST", "/auth/login", "",
		apitest.With(map[string]any{"email": email, "password": password}, apitest.Device("device-login-1")),
		"X-Forwarded-For", ip)
}

func registerFrom(e *apitest.Env, ip, email string) apitest.Response {
	return e.Do("POST", "/auth/register", "",
		apitest.With(map[string]any{"email": email, "password": ownerPass, "display_name": "Арсен", "venue_name": "Кальянная"}, apitest.Device("device-reg-001")),
		"X-Forwarded-For", ip)
}

func requireLimited(t *testing.T, res apitest.Response, maxRetry float64) {
	t.Helper()
	require.Equal(t, 429, res.Status, string(res.Raw))
	require.Equal(t, "rate_limited", res.Code())
	retry, ok := res.Details()["retry_after_sec"].(float64)
	require.True(t, ok, "retry_after_sec is missing: %s", res.Raw)
	require.Greater(t, retry, 0.0)
	require.LessOrEqual(t, retry, maxRetry)
}

func TestLoginLimitsByIP(t *testing.T) {
	t.Run("ten attempts a minute from one ip, then 429 even for the right password", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "203.0.113.1", ownerEmail).Status)
		for i := range 10 {
			// Unknown emails, so the per email limit does not hide the ip one.
			res := loginFrom(e, "198.51.100.9", fmt.Sprintf("nobody%d@example.kz", i), ownerPass)
			require.Equal(t, "unauthorized", res.Code(), "attempt %d", i+1)
		}
		requireLimited(t, loginFrom(e, "198.51.100.9", ownerEmail, ownerPass), 60)
		require.Equal(t, 10, e.Count(`auth_attempts where kind = 'login' and ip = '198.51.100.9'`), "a limited attempt is not recorded")

		require.Equal(t, 200, loginFrom(e, "198.51.100.10", ownerEmail, ownerPass).Status, "another ip is not blocked")
	})

	t.Run("a spoofed first forwarded entry does not buy a fresh allowance", func(t *testing.T) {
		e := apitest.New(t, false)
		for i := range 10 {
			require.Equal(t, "unauthorized", loginFrom(e, fmt.Sprintf("10.0.0.%d, 198.51.100.9", i), fmt.Sprintf("nobody%d@example.kz", i), ownerPass).Code())
		}
		requireLimited(t, loginFrom(e, "10.9.9.9, 198.51.100.9", "nobody99@example.kz", ownerPass), 60)
	})

	t.Run("a parallel volley from one ip is answered exactly ten times", func(t *testing.T) {
		e := apitest.New(t, false)
		codes := make(chan string, 30)
		var wg sync.WaitGroup
		for i := range 30 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes <- loginFrom(e, "198.51.100.9", fmt.Sprintf("nobody%d@example.kz", i), ownerPass).Code()
			}()
		}
		wg.Wait()
		close(codes)
		got := map[string]int{}
		for c := range codes {
			got[c]++
		}
		require.Equal(t, map[string]int{"unauthorized": 10, "rate_limited": 20}, got)
		require.Equal(t, 10, e.Count(`auth_attempts where kind = 'login'`))
	})

	t.Run("a request that fails validation is not counted", func(t *testing.T) {
		e := apitest.New(t, false)
		for range 15 {
			res := loginFrom(e, "198.51.100.9", "not-an-email", ownerPass)
			require.Equal(t, 400, res.Status)
		}
		require.Zero(t, e.Count(`auth_attempts`))
	})
}

func TestLoginLimitsByEmail(t *testing.T) {
	t.Run("five wrong passwords lock the email for every ip, the right password included", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "203.0.113.1", ownerEmail).Status)
		for i := range 5 {
			require.Equal(t, "unauthorized", loginFrom(e, fmt.Sprintf("198.51.100.%d", i+10), ownerEmail, "wrong-pass-1").Code(), "attempt %d", i+1)
		}
		requireLimited(t, loginFrom(e, "198.51.100.50", ownerEmail, ownerPass), 900)

		require.Equal(t, 200, registerFrom(e, "203.0.113.2", "other@example.kz").Status)
		require.Equal(t, 200, loginFrom(e, "198.51.100.50", "other@example.kz", ownerPass).Status, "another account is not locked")
	})

	t.Run("the email matches without regard to case", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "203.0.113.1", ownerEmail).Status)
		for i, spelling := range []string{"owner@example.kz", "OWNER@example.kz", "Owner@Example.KZ", "owner@EXAMPLE.kz", "oWnEr@example.kz"} {
			require.Equal(t, "unauthorized", loginFrom(e, fmt.Sprintf("198.51.100.%d", i+10), spelling, "wrong-pass-1").Code())
		}
		requireLimited(t, loginFrom(e, "198.51.100.60", "owner@EXAMPLE.KZ", ownerPass), 900)
	})

	t.Run("an unknown email locks the same way, so the answer does not reveal accounts", func(t *testing.T) {
		e := apitest.New(t, false)
		for i := range 5 {
			require.Equal(t, "unauthorized", loginFrom(e, fmt.Sprintf("198.51.100.%d", i+10), "nobody@example.kz", ownerPass).Code())
		}
		requireLimited(t, loginFrom(e, "198.51.100.50", "nobody@example.kz", ownerPass), 900)
	})

	t.Run("right passwords are not failures", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "203.0.113.1", ownerEmail).Status)
		for i := range 8 {
			require.Equal(t, 200, loginFrom(e, fmt.Sprintf("198.51.100.%d", i+10), ownerEmail, ownerPass).Status, "login %d", i+1)
		}
		require.Equal(t, 8, e.Count(`auth_attempts where kind = 'login' and success`))
		require.Equal(t, "unauthorized", loginFrom(e, "198.51.100.30", ownerEmail, "wrong-pass-1").Code(), "one wrong password after eight good logins passes")
	})

	t.Run("wrong passwords older than fifteen minutes do not count", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "203.0.113.1", ownerEmail).Status)
		for i := range 5 {
			require.Equal(t, "unauthorized", loginFrom(e, fmt.Sprintf("198.51.100.%d", i+10), ownerEmail, "wrong-pass-1").Code())
		}
		e.Exec(`update auth_attempts set created_at = created_at - interval '16 minutes' where kind = 'login'`)
		require.Equal(t, 200, loginFrom(e, "198.51.100.50", ownerEmail, ownerPass).Status)
	})
}

func TestRegisterLimits(t *testing.T) {
	t.Run("five registrations an hour from one ip, then 429", func(t *testing.T) {
		e := apitest.New(t, false)
		for i := range 5 {
			require.Equal(t, 200, registerFrom(e, "198.51.100.9", fmt.Sprintf("owner%d@example.kz", i)).Status, "registration %d", i+1)
		}
		requireLimited(t, registerFrom(e, "198.51.100.9", "owner6@example.kz"), 3600)
		require.Equal(t, 5, e.Count(`venues`), "a limited request creates nothing")
		require.Equal(t, 5, e.Count(`auth_attempts where kind = 'register' and success`))

		require.Equal(t, 200, registerFrom(e, "198.51.100.10", "owner7@example.kz").Status, "another ip is not blocked")
	})

	t.Run("attempts on a taken email count too", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "198.51.100.9", ownerEmail).Status)
		for i := range 4 {
			res := registerFrom(e, "198.51.100.9", ownerEmail)
			require.Equal(t, 400, res.Status, "attempt %d", i+2)
			require.Equal(t, map[string]any{"email": "taken"}, res.Details()["fields"])
		}
		requireLimited(t, registerFrom(e, "198.51.100.9", "fresh@example.kz"), 3600)
		require.Equal(t, 1, e.Count(`auth_attempts where kind = 'register' and success`))
	})

	t.Run("a request that fails validation is not counted", func(t *testing.T) {
		e := apitest.New(t, false)
		for range 8 {
			res := e.Do("POST", "/auth/register", "", apitest.With(map[string]any{
				"email": "a@b.kz", "password": "short", "display_name": "А", "venue_name": "Б",
			}, apitest.Device("device-reg-001")), "X-Forwarded-For", "198.51.100.9")
			require.Equal(t, 400, res.Status)
		}
		require.Zero(t, e.Count(`auth_attempts`))
		require.Equal(t, 200, registerFrom(e, "198.51.100.9", ownerEmail).Status)
	})

	t.Run("logins do not spend the registration allowance", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, registerFrom(e, "198.51.100.9", ownerEmail).Status)
		for range 6 {
			loginFrom(e, "198.51.100.9", ownerEmail, ownerPass)
		}
		require.Equal(t, 200, registerFrom(e, "198.51.100.9", "second@example.kz").Status)
	})
}
