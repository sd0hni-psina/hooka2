package attempts_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/attempts"
	"hookah/server/internal/store/storetest"
)

var now = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "select count(*) from "+table).Scan(&n))
	return n
}

// plant writes one row per table, age ago from now.
func plant(t *testing.T, pool *pgxpool.Pool, age time.Duration) {
	t.Helper()
	at := now.Add(-age)
	_, err := pool.Exec(context.Background(),
		`insert into join_attempts (device_id, code, success, created_at) values ('device-x', '123456', false, $1)`, at)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		`insert into auth_attempts (kind, email, success, created_at) values ('login', 'a@b.kz', false, $1)`, at)
	require.NoError(t, err)
}

func TestPurgerDeletesAttemptsOlderThanADay(t *testing.T) {
	pool := storetest.Open(t)
	p := attempts.NewPurger(pool)
	plant(t, pool, 25*time.Hour)
	plant(t, pool, 24*time.Hour+time.Minute)
	plant(t, pool, 23*time.Hour+59*time.Minute)
	plant(t, pool, time.Minute)

	res, err := p.Tick(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 4, res.Purged, "two rows of each table")
	require.Equal(t, 2, count(t, pool, "join_attempts"))
	require.Equal(t, 2, count(t, pool, "auth_attempts"))
}

func TestPurgerRunsOnceAnHour(t *testing.T) {
	pool := storetest.Open(t)
	p := attempts.NewPurger(pool)

	first, err := p.Tick(context.Background(), now)
	require.NoError(t, err)
	require.Zero(t, first.Purged)

	// The scheduler ticks every second, the purge must not follow it.
	plant(t, pool, 30*time.Hour)
	early, err := p.Tick(context.Background(), now.Add(59*time.Minute+59*time.Second))
	require.NoError(t, err)
	require.Zero(t, early.Purged)
	require.Equal(t, 1, count(t, pool, "join_attempts"))

	late, err := p.Tick(context.Background(), now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 2, late.Purged)
	require.Zero(t, count(t, pool, "join_attempts"))
}

func TestPurgerIsIdempotent(t *testing.T) {
	pool := storetest.Open(t)
	plant(t, pool, 48*time.Hour)

	// Two processes, or one after a restart: the age condition makes the second
	// pass find nothing.
	first, err := attempts.NewPurger(pool).Tick(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 2, first.Purged)
	again, err := attempts.NewPurger(pool).Tick(context.Background(), now)
	require.NoError(t, err)
	require.Zero(t, again.Purged)
}

func TestPurgerKeepsEveryKindOfRecentRow(t *testing.T) {
	pool := storetest.Open(t)
	_, err := pool.Exec(context.Background(),
		`insert into auth_attempts (kind, ip, success, created_at) values ('register', '203.0.113.1', true, $1), ('login', '203.0.113.2', false, $1)`,
		now.Add(-time.Hour))
	require.NoError(t, err)
	res, err := attempts.NewPurger(pool).Tick(context.Background(), now)
	require.NoError(t, err)
	require.Zero(t, res.Purged)
	require.Equal(t, 2, count(t, pool, "auth_attempts"))
}
