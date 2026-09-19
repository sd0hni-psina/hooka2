// Command migrate applies the embedded goose migrations to DATABASE_URL.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"hookah/server/internal/config"
	"hookah/server/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	url := cfg.DatabaseURL
	if len(os.Args) > 2 {
		url = os.Args[2]
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	ctx := context.Background()
	switch cmd {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "status":
		return goose.StatusContext(ctx, db, ".")
	}
	return fmt.Errorf("unknown command %q, want up or status", cmd)
}
