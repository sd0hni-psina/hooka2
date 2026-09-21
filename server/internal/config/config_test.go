package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	t.Run("dev starts with defaults only", func(t *testing.T) {
		cfg, err := load(envOf(nil))
		require.NoError(t, err)
		require.Equal(t, "dev", cfg.AppEnv)
		require.Equal(t, 30*time.Minute, cfg.AccessTTL)
		require.Equal(t, "fake", cfg.PushProvider)
		require.NotEmpty(t, cfg.JWTSecret)
	})

	t.Run("prod fails when secrets are empty", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"APP_ENV": "prod"}))
		require.ErrorContains(t, err, "DATABASE_URL")
		require.ErrorContains(t, err, "JWT_SECRET")
	})

	t.Run("prod rejects the dev secret", func(t *testing.T) {
		_, err := load(envOf(map[string]string{
			"APP_ENV": "prod", "DATABASE_URL": "postgres://x", "JWT_SECRET": "dev-secret-change-me",
		}))
		require.ErrorContains(t, err, "dev secret")
	})

	t.Run("prod rejects a secret shorter than 32 characters", func(t *testing.T) {
		env := map[string]string{"APP_ENV": "prod", "DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 31)}
		_, err := load(envOf(env))
		require.ErrorContains(t, err, "at least 32")

		env["JWT_SECRET"] = strings.Repeat("s", 32)
		_, err = load(envOf(env))
		require.NoError(t, err)
	})

	t.Run("dev keeps a short secret", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"JWT_SECRET": "short"}))
		require.NoError(t, err)
	})

	t.Run("proxy trust is off by default and parses a boolean", func(t *testing.T) {
		cfg, err := load(envOf(nil))
		require.NoError(t, err)
		require.False(t, cfg.TrustProxy)

		cfg, err = load(envOf(map[string]string{"TRUST_PROXY": "true"}))
		require.NoError(t, err)
		require.True(t, cfg.TrustProxy)

		_, err = load(envOf(map[string]string{"TRUST_PROXY": "maybe"}))
		require.ErrorContains(t, err, "TRUST_PROXY")
	})

	t.Run("pool size defaults to 10 and stays between 2 and 100", func(t *testing.T) {
		cfg, err := load(envOf(nil))
		require.NoError(t, err)
		require.Equal(t, 10, cfg.DBMaxConns)

		for in, want := range map[string]int{"2": 2, "100": 100, "40": 40} {
			cfg, err = load(envOf(map[string]string{"DB_MAX_CONNS": in}))
			require.NoError(t, err, in)
			require.Equal(t, want, cfg.DBMaxConns)
		}
		for _, bad := range []string{"1", "101", "0", "-4", "many"} {
			_, err = load(envOf(map[string]string{"DB_MAX_CONNS": bad}))
			require.ErrorContains(t, err, "DB_MAX_CONNS", bad)
		}
	})

	t.Run("unknown push provider is rejected", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"PUSH_PROVIDER": "fcm"}))
		require.ErrorContains(t, err, "PUSH_PROVIDER")
	})

	t.Run("broken duration is rejected", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"SCHEDULER_TICK": "soon"}))
		require.ErrorContains(t, err, "SCHEDULER_TICK")
	})
}
