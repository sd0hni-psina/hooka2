package staff_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/staff"
	"hookah/server/seed"
)

func join(e *apitest.Env, code, name, device string) apitest.Response {
	return e.Do("POST", "/join", "", apitest.With(map[string]any{"code": code, "display_name": name}, apitest.Device(device)))
}

type joinBody struct {
	Tokens domain.Tokens `json:"tokens"`
	Venue  domain.Venue  `json:"venue"`
	Halls  []domain.Hall `json:"halls"`
}

func TestGetMe(t *testing.T) {
	e := apitest.New(t, true)

	t.Run("master sees self, venue without code and both halls in order", func(t *testing.T) {
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Masters[0]), nil)
		require.Equal(t, 200, res.Status)
		var me staff.Me
		res.Decode(t, &me)
		require.Equal(t, e.Fx.Masters[0].StaffID, me.Staff.ID)
		require.Equal(t, "Ерлан", me.Staff.DisplayName)
		require.Equal(t, domain.RoleMaster, me.Staff.Role)
		require.Nil(t, me.Venue.JoinCode)
		require.Len(t, me.Halls, 2)
		require.Equal(t, "Основной зал", me.Halls[0].Name)
	})

	t.Run("owner sees the join code", func(t *testing.T) {
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Owner), nil)
		require.Equal(t, seed.JoinCode, res.Body["venue"].(map[string]any)["join_code"])
	})

	t.Run("timestamps are served in utc", func(t *testing.T) {
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Owner), nil)
		require.Regexp(t, `Z$`, res.Body["staff"].(map[string]any)["created_at"])
	})

	t.Run("missing token is unauthorized", func(t *testing.T) {
		res := e.Do("GET", "/me", "", nil)
		require.Equal(t, 401, res.Status)
		require.Equal(t, "unauthorized", res.Code())
	})

	t.Run("revoked session is unauthorized", func(t *testing.T) {
		e.Exec(`update staff_sessions set revoked_at = now() where id = $1`, e.Fx.Masters[1].SessionID)
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Masters[1]), nil)
		require.Equal(t, 401, res.Status)
	})

	t.Run("blocked master is forbidden", func(t *testing.T) {
		e.Exec(`update staff set blocked_at = now() where id = $1`, e.Fx.Masters[2].StaffID)
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Masters[2]), nil)
		require.Equal(t, 403, res.Status)
		require.Equal(t, "forbidden", res.Code())
	})
}

func TestJoin(t *testing.T) {
	t.Run("master joins by code and lands in hall choice with a working token", func(t *testing.T) {
		e := apitest.New(t, true)
		res := join(e, seed.JoinCode, "Нурлан", "device-aaa-1")
		require.Equal(t, 200, res.Status, string(res.Raw))
		var b joinBody
		res.Decode(t, &b)
		require.Len(t, b.Halls, 2)
		require.Nil(t, b.Venue.JoinCode)
		require.NotEmpty(t, b.Tokens.RefreshToken)
		require.WithinDuration(t, time.Now().Add(apitest.AccessTTL), b.Tokens.AccessExpiresAt, time.Minute)

		me := e.Do("GET", "/me", b.Tokens.AccessToken, nil)
		require.Equal(t, 200, me.Status)
		require.Equal(t, "Нурлан", me.Body["staff"].(map[string]any)["display_name"])
		require.Equal(t, "master", me.Body["staff"].(map[string]any)["role"])
	})

	t.Run("second device with the same name opens a second session of the same master", func(t *testing.T) {
		e := apitest.New(t, true)
		require.Equal(t, 200, join(e, seed.JoinCode, "Ерлан", "device-phone-1").Status)
		require.Equal(t, 200, join(e, seed.JoinCode, "ерлан", "device-phone-2").Status)
		require.Equal(t, 1, e.Count(`staff where venue_id = $1 and lower(display_name) = 'ерлан'`, e.Fx.VenueID))
		// Seed session plus two new devices.
		require.Equal(t, 3, e.Count(`staff_sessions where staff_id = $1`, e.Fx.Masters[0].StaffID))
	})

	t.Run("repeated join from one device is idempotent", func(t *testing.T) {
		e := apitest.New(t, true)
		first := join(e, seed.JoinCode, "Нурлан", "device-same-1")
		second := join(e, seed.JoinCode, "Нурлан", "device-same-1")
		require.Equal(t, 200, first.Status)
		require.Equal(t, 200, second.Status)
		require.Equal(t, 1, e.Count(`staff where display_name = 'Нурлан'`))
		require.Equal(t, 1, e.Count(`staff_sessions where device_id = 'device-same-1'`))
	})

	t.Run("parallel joins with one name create one master", func(t *testing.T) {
		e := apitest.New(t, true)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := join(e, seed.JoinCode, "Дамир", "device-race-"+string(rune('a'+i))+"xxxxx")
				require.Equal(t, 200, res.Status, string(res.Raw))
			}()
		}
		wg.Wait()
		require.Equal(t, 1, e.Count(`staff where display_name = 'Дамир'`))
	})

	t.Run("wrong code is invalid_join_code", func(t *testing.T) {
		e := apitest.New(t, true)
		res := join(e, "9876", "Нурлан", "device-bad-1")
		require.Equal(t, 404, res.Status)
		require.Equal(t, "invalid_join_code", res.Code())
	})

	t.Run("sixth attempt within a minute is rate limited even with the right code", func(t *testing.T) {
		e := apitest.New(t, true)
		for i := range 4 {
			require.Equal(t, "invalid_join_code", join(e, "9876", "Нурлан", "device-brute-1").Code(), "attempt %d", i+1)
		}
		require.Equal(t, 200, join(e, seed.JoinCode, "Нурлан", "device-brute-1").Status, "fifth attempt passes")
		res := join(e, seed.JoinCode, "Нурлан", "device-brute-1")
		require.Equal(t, 429, res.Status)
		require.Equal(t, "rate_limited", res.Code())
		retry := res.Details()["retry_after_sec"].(float64)
		require.Greater(t, retry, 0.0)
		require.LessOrEqual(t, retry, 60.0)
		require.Equal(t, 5, e.Count(`join_attempts where device_id = 'device-brute-1'`), "limited attempts are not recorded")
		require.Equal(t, 200, join(e, seed.JoinCode, "Нурлан", "device-other-1").Status, "other devices are not affected")
	})

	t.Run("blocked master cannot join again by code", func(t *testing.T) {
		e := apitest.New(t, true)
		e.Exec(`update staff set blocked_at = now() where id = $1`, e.Fx.Masters[0].StaffID)
		res := join(e, seed.JoinCode, "Ерлан", "device-blocked-1")
		require.Equal(t, 403, res.Status)
		require.Equal(t, "forbidden", res.Code())
	})

	t.Run("bad input lists fields", func(t *testing.T) {
		e := apitest.New(t, true)
		res := e.Do("POST", "/join", "", map[string]any{"code": "12", "display_name": "", "device_id": "x", "platform": "web"})
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"code": "invalid", "display_name": "required", "device_id": "too_short", "platform": "invalid"}, res.Details()["fields"])
	})
}

func TestRefresh(t *testing.T) {
	joined := func(e *apitest.Env) joinBody {
		var b joinBody
		join(e, seed.JoinCode, "Нурлан", "device-refresh-1").Decode(t, &b)
		return b
	}
	refresh := func(e *apitest.Env, token string) apitest.Response {
		return e.Do("POST", "/auth/refresh", "", map[string]any{"refresh_token": token})
	}

	t.Run("refresh rotates the pair and the old refresh stops working", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		res := refresh(e, b.Tokens.RefreshToken)
		require.Equal(t, 200, res.Status)
		var next struct {
			Tokens domain.Tokens `json:"tokens"`
		}
		res.Decode(t, &next)
		require.NotEqual(t, b.Tokens.RefreshToken, next.Tokens.RefreshToken)
		require.Equal(t, 200, e.Do("GET", "/me", next.Tokens.AccessToken, nil).Status)

		old := refresh(e, b.Tokens.RefreshToken)
		require.Equal(t, 401, old.Status)
		require.Equal(t, "unauthorized", old.Code())
	})

	t.Run("parallel refreshes with one token let exactly one win", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		var wg sync.WaitGroup
		codes := make(chan int, 6)
		for range 6 {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- refresh(e, b.Tokens.RefreshToken).Status }()
		}
		wg.Wait()
		close(codes)
		wins := 0
		for c := range codes {
			if c == 200 {
				wins++
			} else {
				require.Equal(t, 401, c)
			}
		}
		require.Equal(t, 1, wins)
	})

	t.Run("refresh older than the ttl is unauthorized", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		e.Exec(`update staff_sessions set last_seen_at = now() - interval '91 days' where device_id = 'device-refresh-1'`)
		require.Equal(t, 401, refresh(e, b.Tokens.RefreshToken).Status)
	})

	t.Run("blocked master cannot refresh", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		e.Exec(`update staff set blocked_at = now() where display_name = 'Нурлан'`)
		require.Equal(t, 403, refresh(e, b.Tokens.RefreshToken).Status)
	})

	t.Run("logout revokes the session", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		require.Equal(t, 204, e.Do("POST", "/auth/logout", b.Tokens.AccessToken, nil).Status)
		require.Equal(t, 401, e.Do("GET", "/me", b.Tokens.AccessToken, nil).Status)
		require.Equal(t, 401, refresh(e, b.Tokens.RefreshToken).Status)
	})

	t.Run("delete current session works like logout", func(t *testing.T) {
		e := apitest.New(t, true)
		b := joined(e)
		require.Equal(t, 204, e.Do("DELETE", "/sessions/current", b.Tokens.AccessToken, nil).Status)
		require.Equal(t, 401, e.Do("GET", "/me", b.Tokens.AccessToken, nil).Status)
	})
}

func TestPushToken(t *testing.T) {
	e := apitest.New(t, true)
	tok := e.TokenFor(e.Fx.Masters[0])

	res := e.Do("POST", "/sessions/push-token", tok, map[string]any{"expo_push_token": "ExponentPushToken[abc123]"})
	require.Equal(t, 204, res.Status)
	require.Equal(t, 1, e.Count(`staff_sessions where id = $1 and expo_push_token = 'ExponentPushToken[abc123]'`, e.Fx.Masters[0].SessionID))

	bad := e.Do("POST", "/sessions/push-token", tok, map[string]any{"expo_push_token": "fcm-token"})
	require.Equal(t, 400, bad.Status)
	require.Equal(t, map[string]any{"expo_push_token": "invalid"}, bad.Details()["fields"])
}
