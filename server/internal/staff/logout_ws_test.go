package staff_test

import (
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/seed"
)

func TestLogoutClosesTheSocketOfThatSession(t *testing.T) {
	for _, tc := range []struct{ name, method, path string }{
		{"POST /auth/logout", "POST", "/auth/logout"},
		{"DELETE /sessions/current", "DELETE", "/sessions/current"},
	} {
		t.Run(tc.name+" closes it with 1008 and spares other sessions of the same master", func(t *testing.T) {
			e := apitest.New(t, true)
			master := e.Fx.Masters[0]

			// Two devices of one master: the seeded session and a fresh join.
			var second joinBody
			res := join(e, seed.JoinCode, master.Name, "device-second-1")
			require.Equal(t, 200, res.Status, string(res.Raw))
			res.Decode(t, &second)

			keep, _ := e.Subscribe(e.TokenFor(master), e.Fx.HallIDs[0])
			leaving, _ := e.Subscribe(second.Tokens.AccessToken, e.Fx.HallIDs[0])

			require.Equal(t, 204, e.Do(tc.method, tc.path, second.Tokens.AccessToken, nil).Status)
			require.Equal(t, websocket.StatusPolicyViolation, leaving.CloseStatus(), "the signed out device is cut off at once")

			// The seeded session still receives what the hall publishes.
			start := e.Do("POST", "/tables/"+e.Fx.TableIDs["1"]+"/hookah", e.TokenFor(e.Fx.Masters[1]), nil,
				"Idempotency-Key", "00000000-0000-4000-8000-0000000000aa")
			require.Equal(t, 200, start.Status, string(start.Raw))
			require.Equal(t, "hookah.started", keep.Until("hookah.started").Type)
		})
	}

	t.Run("a device that signed out cannot keep listening on a token that is still unexpired", func(t *testing.T) {
		e := apitest.New(t, true)
		var b joinBody
		join(e, seed.JoinCode, "Нурлан", "device-solo-001").Decode(t, &b)
		ws, _ := e.Subscribe(b.Tokens.AccessToken, e.Fx.HallIDs[0])
		require.Equal(t, 204, e.Do("POST", "/auth/logout", b.Tokens.AccessToken, nil).Status)
		require.Equal(t, websocket.StatusPolicyViolation, ws.CloseStatus())
		require.Equal(t, 401, e.Do("GET", "/me", b.Tokens.AccessToken, nil).Status)
	})
}
