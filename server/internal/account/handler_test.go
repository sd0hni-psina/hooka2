package account_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
)

type authBody struct {
	Tokens domain.Tokens `json:"tokens"`
	Venue  domain.Venue  `json:"venue"`
}

func register(e *apitest.Env, email, device string) apitest.Response {
	return e.Do("POST", "/auth/register", "", apitest.With(map[string]any{
		"email": email, "password": "hookah-pass-1", "display_name": "Арсен", "venue_name": "Кальянная на Абая",
	}, apitest.Device(device)))
}

func TestRegister(t *testing.T) {
	t.Run("owner registers and gets a venue with the default plan", func(t *testing.T) {
		e := apitest.New(t, false)
		res := register(e, "owner@example.kz", "device-owner-1")
		require.Equal(t, 200, res.Status, string(res.Raw))
		var b authBody
		res.Decode(t, &b)
		require.NotNil(t, b.Venue.JoinCode)
		require.Regexp(t, regexp.MustCompile(`^\d{4}$`), *b.Venue.JoinCode)
		require.Equal(t, "Asia/Atyrau", b.Venue.Timezone)

		var plan domain.Plan
		require.NoError(t, e.Pool.QueryRow(context.Background(), `select service_plan from venues where id = $1`, b.Venue.ID).Scan(&plan))
		require.Equal(t, domain.DefaultPlan(), plan)

		me := e.Do("GET", "/me", b.Tokens.AccessToken, nil)
		require.Equal(t, 200, me.Status)
		staff := me.Body["staff"].(map[string]any)
		require.Equal(t, "owner", staff["role"])
		require.Equal(t, *b.Venue.JoinCode, me.Body["venue"].(map[string]any)["join_code"])
	})

	t.Run("taken email is validation_failed with taken reason", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, register(e, "owner@example.kz", "device-owner-1").Status)
		res := register(e, "OWNER@example.kz", "device-owner-2")
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"email": "taken"}, res.Details()["fields"])
		require.Equal(t, 1, e.Count(`venues`))
	})

	t.Run("short password lists the field", func(t *testing.T) {
		e := apitest.New(t, false)
		res := e.Do("POST", "/auth/register", "", apitest.With(map[string]any{
			"email": "a@b.kz", "password": "short", "display_name": "А", "venue_name": "Б",
		}, apitest.Device("device-owner-1")))
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"password": "too_short"}, res.Details()["fields"])
	})
}

func TestLogin(t *testing.T) {
	login := func(e *apitest.Env, email, password, device string) apitest.Response {
		return e.Do("POST", "/auth/login", "", apitest.With(map[string]any{"email": email, "password": password}, apitest.Device(device)))
	}

	t.Run("owner logs in on a second device and keeps one session per device", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, register(e, "owner@example.kz", "device-owner-1").Status)
		res := login(e, "owner@example.kz", "hookah-pass-1", "device-owner-2")
		require.Equal(t, 200, res.Status)
		var b authBody
		res.Decode(t, &b)
		require.NotNil(t, b.Venue.JoinCode)
		require.Equal(t, 200, login(e, "owner@example.kz", "hookah-pass-1", "device-owner-2").Status)
		require.Equal(t, 2, e.Count(`staff_sessions`))
	})

	t.Run("wrong password and unknown email are both unauthorized", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, register(e, "owner@example.kz", "device-owner-1").Status)
		require.Equal(t, "unauthorized", login(e, "owner@example.kz", "wrong-pass-1", "device-owner-1").Code())
		require.Equal(t, "unauthorized", login(e, "nobody@example.kz", "hookah-pass-1", "device-owner-1").Code())
	})
}
