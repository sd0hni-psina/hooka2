// Package storetest gives each integration test its own freshly migrated
// database cloned from a template, so packages can run in parallel.
package storetest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"hookah/server/internal/store"
	"hookah/server/migrations"
)

const (
	templateDB   = "hookah_template"
	testMaxConns = 10
)

// Open returns a pool on a new database with all migrations applied. The test
// is skipped when TEST_DATABASE_URL is empty.
func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer func() {
		if err := admin.Close(ctx); err != nil {
			t.Logf("close admin: %v", err)
		}
	}()

	name := "hookah_t_" + randomSuffix(t)
	if err := cloneTemplate(ctx, admin, base, name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	pool, err := store.NewPool(ctx, withDB(t, base, name), testMaxConns)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		conn, err := pgx.Connect(context.Background(), base)
		if err != nil {
			t.Logf("drop %s: %v", name, err)
			return
		}
		defer func() {
			if err := conn.Close(context.Background()); err != nil {
				t.Logf("close admin: %v", err)
			}
		}()
		if _, err := conn.Exec(context.Background(), `drop database if exists `+name+` with (force)`); err != nil {
			t.Logf("drop %s: %v", name, err)
		}
	})
	return pool
}

// cloneTemplate migrates the template and clones it under an advisory lock:
// CREATE DATABASE ... TEMPLATE fails while another binary is migrating it.
func cloneTemplate(ctx context.Context, admin *pgx.Conn, base, name string) error {
	if _, err := admin.Exec(ctx, `select pg_advisory_lock(727274)`); err != nil {
		return err
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), `select pg_advisory_unlock(727274)`); err != nil {
			fmt.Fprintln(os.Stderr, "storetest: unlock:", err)
		}
	}()

	var exists bool
	if err := admin.QueryRow(ctx, `select exists(select 1 from pg_database where datname = $1)`, templateDB).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := admin.Exec(ctx, `create database `+templateDB); err != nil {
			return err
		}
	}

	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	u.Path = "/" + templateDB
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		return err
	}
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return errors.Join(err, db.Close())
	}
	if err := db.Close(); err != nil {
		return err
	}
	_, err = admin.Exec(ctx, fmt.Sprintf(`create database %s template %s`, name, templateDB))
	return err
}

func withDB(t testing.TB, base, name string) string {
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func randomSuffix(t testing.TB) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random: %v", err)
	}
	return hex.EncodeToString(b)
}
