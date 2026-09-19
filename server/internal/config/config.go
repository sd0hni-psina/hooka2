package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	SchedulerTick   time.Duration
	PushTick        time.Duration
	PushProvider    string
	ExpoAccessToken string
	LogLevel        slog.Level
	CORSOrigins     []string
}

func (c Config) IsProd() bool { return c.AppEnv == "prod" }

// Load reads env with dev defaults. In prod the secrets have no defaults,
// so a forgotten variable stops the process instead of running insecurely.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	env := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	appEnv := env("APP_ENV", "dev")
	prod := appEnv == "prod"
	devDefault := func(v string) string {
		if prod {
			return ""
		}
		return v
	}

	var errs []error
	dur := func(key, def string) time.Duration {
		d, err := time.ParseDuration(env(key, def))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s: invalid duration", key))
		}
		return d
	}

	cfg := Config{
		AppEnv:          appEnv,
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", devDefault("postgres://hookah:hookah@localhost:5432/hookah?sslmode=disable")),
		JWTSecret:       env("JWT_SECRET", devDefault("dev-secret-change-me")),
		AccessTTL:       dur("ACCESS_TTL", "30m"),
		RefreshTTL:      dur("REFRESH_TTL", "2160h"),
		SchedulerTick:   dur("SCHEDULER_TICK", "1s"),
		PushTick:        dur("PUSH_TICK", "5s"),
		PushProvider:    env("PUSH_PROVIDER", "fake"),
		ExpoAccessToken: env("EXPO_ACCESS_TOKEN", ""),
	}
	if o := env("CORS_ORIGINS", ""); o != "" {
		for _, s := range strings.Split(o, ",") {
			cfg.CORSOrigins = append(cfg.CORSOrigins, strings.TrimSpace(s))
		}
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, errors.New("LOG_LEVEL: invalid level"))
	}

	switch appEnv {
	case "dev", "test", "prod":
	default:
		errs = append(errs, errors.New("APP_ENV: must be dev, test or prod"))
	}
	switch cfg.PushProvider {
	case "fake", "expo":
	default:
		errs = append(errs, errors.New("PUSH_PROVIDER: must be fake or expo"))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL: required"))
	}
	if cfg.JWTSecret == "" {
		errs = append(errs, errors.New("JWT_SECRET: required"))
	}
	if prod && cfg.JWTSecret == "dev-secret-change-me" {
		errs = append(errs, errors.New("JWT_SECRET: dev secret in prod"))
	}
	if cfg.PushProvider == "expo" && prod && cfg.ExpoAccessToken == "" {
		errs = append(errs, errors.New("EXPO_ACCESS_TOKEN: required for expo provider in prod"))
	}
	return cfg, errors.Join(errs...)
}
