// Command seed resets DATABASE_URL and fills it with the development fixture.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"hookah/server/internal/config"
	"hookah/server/internal/store"
	"hookah/server/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProd() {
		return errors.New("refusing to reset a prod database")
	}
	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := seed.Reset(ctx, pool); err != nil {
		return err
	}
	fx, err := seed.Seed(ctx, pool)
	if err != nil {
		return err
	}
	fmt.Printf("venue %s, join code %s\n", fx.VenueID, seed.JoinCode)
	fmt.Printf("owner %s / %s\n", seed.OwnerEmail, seed.OwnerPassword)
	fmt.Printf("halls %v, tables %d, masters %d\n", fx.HallIDs, len(fx.TableIDs), len(fx.Masters))
	return nil
}
