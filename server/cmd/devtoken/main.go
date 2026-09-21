// Command devtoken prints an access token for a seeded staff member, so the
// app can call the API before the join flow exists. Dev only.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"hookah/server/internal/auth"
	"hookah/server/internal/config"
	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "devtoken:", err)
		os.Exit(1)
	}
}

func run() error {
	name := flag.String("name", "Арсен", "display_name of the seeded staff member")
	ttl := flag.Duration("ttl", 24*time.Hour, "token lifetime")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AppEnv != "dev" {
		return errors.New("devtoken runs only with APP_ENV=dev")
	}
	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	var c auth.Claims
	err = pool.QueryRow(ctx,
		`select st.id, st.venue_id, st.role, s.id from staff st
		 join staff_sessions s on s.staff_id = st.id and s.revoked_at is null
		 where st.display_name = $1 order by s.created_at limit 1`, *name,
	).Scan(&c.StaffID, &c.VenueID, &c.Role, &c.SessionID)
	if err != nil {
		return fmt.Errorf("find staff %q with a live session: %w", *name, err)
	}
	if !c.Role.Valid() {
		return fmt.Errorf("staff %q: %w", *name, domain.ErrValidation)
	}
	token, _, err := auth.NewTokens(cfg.JWTSecret, *ttl).Issue(c, time.Now())
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}
