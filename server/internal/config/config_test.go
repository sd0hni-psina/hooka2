package config

import (
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

	t.Run("unknown push provider is rejected", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"PUSH_PROVIDER": "fcm"}))
		require.ErrorContains(t, err, "PUSH_PROVIDER")
	})

	t.Run("broken duration is rejected", func(t *testing.T) {
		_, err := load(envOf(map[string]string{"SCHEDULER_TICK": "soon"}))
		require.ErrorContains(t, err, "SCHEDULER_TICK")
	})
}
