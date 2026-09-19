package venue_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/seed"
)

func TestRotateCode(t *testing.T) {
	e := apitest.New(t, true)
	masterToken := e.TokenFor(e.Fx.Masters[0])

	t.Run("master cannot rotate", func(t *testing.T) {
		res := e.Do("POST", "/venues/current/rotate-code", masterToken, nil)
		require.Equal(t, 403, res.Status)
		require.Equal(t, "forbidden", res.Code())
	})

	t.Run("owner rotates, old code stops working, sessions survive", func(t *testing.T) {
		res := e.Do("POST", "/venues/current/rotate-code", e.TokenFor(e.Fx.Owner), nil)
		require.Equal(t, 200, res.Status)
		code := res.Body["join_code"].(string)
		require.Regexp(t, `^\d{4}$`, code)
		require.NotEqual(t, seed.JoinCode, code)

		join := func(c string) apitest.Response {
			return e.Do("POST", "/join", "", apitest.With(map[string]any{"code": c, "display_name": "Нурлан"}, apitest.Device("device-rotate-1")))
		}
		require.Equal(t, "invalid_join_code", join(seed.JoinCode).Code())
		require.Equal(t, 200, join(code).Status)
		require.Equal(t, 200, e.Do("GET", "/me", masterToken, nil).Status, "rotation does not log masters out")
	})
}
