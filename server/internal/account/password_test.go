package account_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"hookah/server/internal/apitest"
)

func TestLoginUpgradesAWeakHash(t *testing.T) {
	const password = "hookah-pass-1"
	hashOf := func(e *apitest.Env, email string) string {
		var h string
		require.NoError(t, e.Pool.QueryRow(context.Background(), `select password_hash from accounts where email = $1`, email).Scan(&h))
		return h
	}
	costOf := func(h string) int {
		c, err := bcrypt.Cost([]byte(h))
		require.NoError(t, err)
		return c
	}
	login := func(e *apitest.Env, pw string) apitest.Response {
		return e.Do("POST", "/auth/login", "", apitest.With(map[string]any{"email": "owner@example.kz", "password": pw}, apitest.Device("device-owner-9")))
	}
	weaken := func(e *apitest.Env) string {
		weak, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		require.NoError(t, err)
		e.Exec(`update accounts set password_hash = $1 where email = 'owner@example.kz'`, string(weak))
		return string(weak)
	}

	t.Run("a correct login replaces a hash below the current cost", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, register(e, "owner@example.kz", "device-owner-1").Status)
		require.Equal(t, apitest.PasswordCost, costOf(hashOf(e, "owner@example.kz")), "register hashes at the configured cost")

		weak := weaken(e)
		require.Equal(t, bcrypt.MinCost, costOf(weak))
		require.Equal(t, 200, login(e, password).Status)
		upgraded := hashOf(e, "owner@example.kz")
		require.NotEqual(t, weak, upgraded)
		require.Equal(t, apitest.PasswordCost, costOf(upgraded))

		// The new hash still opens the account, and a second login leaves it alone.
		require.Equal(t, 200, login(e, password).Status)
		require.Equal(t, upgraded, hashOf(e, "owner@example.kz"))
	})

	t.Run("a wrong password never touches the hash", func(t *testing.T) {
		e := apitest.New(t, false)
		require.Equal(t, 200, register(e, "owner@example.kz", "device-owner-1").Status)
		weak := weaken(e)
		require.Equal(t, 401, login(e, "wrong-pass-123").Status)
		require.Equal(t, weak, hashOf(e, "owner@example.kz"))
	})
}
