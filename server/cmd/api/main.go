package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"hookah/server/internal/auth"
	"hookah/server/internal/config"
	"hookah/server/internal/hookah"
	"hookah/server/internal/httpx"
	"hookah/server/internal/layout"
	"hookah/server/internal/push"
	"hookah/server/internal/realtime"
	"hookah/server/internal/scheduler"
	"hookah/server/internal/staff"
	"hookah/server/internal/store"
	"hookah/server/internal/venue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("config: %v", err))
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	if err := run(cfg, log); err != nil {
		log.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var sender push.Sender
	switch cfg.PushProvider {
	case "fake":
		sender = push.NewFake(log)
	default:
		return fmt.Errorf("push provider %q is not available yet", cfg.PushProvider)
	}

	venueRepo := venue.NewRepository()
	layoutRepo := layout.NewRepository()
	staffRepo := staff.NewRepository()
	hookahRepo := hookah.NewRepository()

	staffSvc := staff.NewService(pool, staffRepo, venueRepo, layoutRepo)
	hookahSvc := hookah.NewService(pool, hookahRepo, layoutRepo, venueRepo, staffRepo)

	tokens := auth.NewTokens(cfg.JWTSecret, cfg.AccessTTL)
	guard := auth.NewGuard(tokens, staffSvc, httpx.WriteError)
	hub := realtime.NewHub(log)

	router := httpx.NewRouter(guard.Middleware)
	router.Raw("GET /healthz", httpx.Health(pool))
	staff.NewHandler(staffSvc).Register(router)
	realtime.NewHandler(hub, guard, hookah.Snapshots{Svc: hookahSvc}, cfg.CORSOrigins, log).Register(router)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpx.Chain(router, httpx.RequestID, httpx.Recover(log), httpx.Logger(log), httpx.CORS(cfg.CORSOrigins)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	var wg sync.WaitGroup
	bg, cancelBG := context.WithCancel(ctx)
	defer cancelBG()
	wg.Add(2)
	go func() { defer wg.Done(); scheduler.New(log, cfg.SchedulerTick).Run(bg) }()
	go func() { defer wg.Done(); push.NewWorker(pool, sender, log, cfg.PushTick).Run(bg) }()

	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv, "push", cfg.PushProvider)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		cancelBG()
		wg.Wait()
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	cancelBG()
	wg.Wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
