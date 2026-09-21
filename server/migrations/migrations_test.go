package migrations_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"hookah/server/migrations"
)

// The hardening migration runs on a database that already holds venues, a
// session and join attempts, the way it will on the first real server.
func TestHardeningKeepsExistingData(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close(ctx)) })

	raw := make([]byte, 6)
	_, err = rand.Read(raw)
	require.NoError(t, err)
	name := "hookah_mig_" + hex.EncodeToString(raw)
	_, err = admin.Exec(ctx, `create database `+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(ctx, `drop database if exists `+name+` with (force)`)
		require.NoError(t, err)
	})

	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	require.NoError(t, goose.SetDialect("postgres"))

	// Version 3 is the schema of slices 0 to 5: four digit codes.
	require.NoError(t, goose.UpToContext(ctx, db, ".", 3))
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.ExecContext(ctx, q, args...)
		require.NoError(t, err)
	}
	exec(`insert into accounts (id, email, password_hash, display_name)
	      values ('00000000-0000-0000-0000-00000000000a', 'o@x.dev', 'h', 'Owner')`)
	for i, code := range []string{"1234", "0007", "9999"} {
		exec(`insert into venues (owner_id, name, join_code, service_plan)
		      values ('00000000-0000-0000-0000-00000000000a', $1, $2, '{}')`, "Venue "+string(rune('A'+i)), code)
	}
	exec(`insert into join_attempts (device_id, code, success) values ('old-device', '9876', false)`)

	require.NoError(t, goose.UpContext(ctx, db, "."))

	t.Run("existing codes keep four digits and gain two", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, `select name, join_code from venues order by name`)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()
		want := map[string]string{"Venue A": "1234", "Venue B": "0007", "Venue C": "9999"}
		got := 0
		for rows.Next() {
			var venue, code string
			require.NoError(t, rows.Scan(&venue, &code))
			require.Regexp(t, `^\d{6}$`, code, venue)
			require.Equal(t, want[venue], code[:4], venue)
			got++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, len(want), got)
	})

	t.Run("old join attempts survive without an address", func(t *testing.T) {
		var ip sql.NullString
		var code string
		require.NoError(t, db.QueryRowContext(ctx, `select rtrim(code), ip::text from join_attempts`).Scan(&code, &ip))
		require.Equal(t, "9876", code)
		require.False(t, ip.Valid)
	})

	t.Run("push status accepts cancelled and still rejects junk", func(t *testing.T) {
		var def string
		require.NoError(t, db.QueryRowContext(ctx,
			`select pg_get_constraintdef(oid) from pg_constraint where conname = 'push_deliveries_status_check'`).Scan(&def))
		require.Contains(t, def, "cancelled")
		require.NotContains(t, def, "junk")
	})

	t.Run("auth attempts table rejects an unknown kind", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `insert into auth_attempts (kind) values ('join')`)
		require.Error(t, err)
		_, err = db.ExecContext(ctx, `insert into auth_attempts (kind, email) values ('login', 'A@X.dev')`)
		require.NoError(t, err)
	})

	t.Run("a second run has nothing to do", func(t *testing.T) {
		require.NoError(t, goose.UpContext(ctx, db, "."))
	})
}
