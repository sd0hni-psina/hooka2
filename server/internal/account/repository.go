package account

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"

	"hookah/server/internal/store"
)

type Account struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
}

type Repository struct{}

func NewRepository() *Repository { return &Repository{} }

// Create inserts an account; ok is false when the email is taken.
func (Repository) Create(ctx context.Context, q store.Querier, email, hash, name string) (string, bool, error) {
	var id string
	err := q.QueryRow(ctx,
		`insert into accounts (email, password_hash, display_name) values ($1, $2, $3)
		 on conflict (email) do nothing returning id`, email, hash, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("create account: %w", err)
	}
	return id, true, nil
}

// Kinds of auth_attempts rows, tech.md section 19.16.
const (
	kindLogin    = "login"
	kindRegister = "register"
)

// Reserve records an attempt as a failure before its password is looked at, so
// a parallel request already sees it. An invalid ip and an empty email are
// stored as null.
func (Repository) Reserve(ctx context.Context, q store.Querier, kind string, ip netip.Addr, email string, now time.Time) (int64, error) {
	var ipArg, emailArg any
	if ip.IsValid() {
		ipArg = ip
	}
	if email != "" {
		emailArg = email
	}
	var id int64
	if err := q.QueryRow(ctx,
		`insert into auth_attempts (kind, ip, email, created_at) values ($1, $2, $3, $4) returning id`,
		kind, ipArg, emailArg, now).Scan(&id); err != nil {
		return 0, fmt.Errorf("reserve %s attempt: %w", kind, err)
	}
	return id, nil
}

// Succeed turns a reserved attempt into a success.
func (Repository) Succeed(ctx context.Context, q store.Querier, id int64) error {
	if _, err := q.Exec(ctx, `update auth_attempts set success = true where id = $1`, id); err != nil {
		return fmt.Errorf("mark attempt %d successful: %w", id, err)
	}
	return nil
}

// RecentByIP lists every attempt of a kind from one address.
func (Repository) RecentByIP(ctx context.Context, q store.Querier, kind string, ip netip.Addr, since time.Time) ([]time.Time, error) {
	rows, err := q.Query(ctx,
		`select created_at from auth_attempts where kind = $1 and ip = $2 and created_at > $3`, kind, ip, since)
	if err != nil {
		return nil, fmt.Errorf("recent %s attempts by ip: %w", kind, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return nil, fmt.Errorf("recent %s attempts by ip: %w", kind, err)
	}
	return out, nil
}

// RecentFailedLogins lists the wrong passwords tried for one email. The
// column is citext, so the spelling of the address does not matter.
func (Repository) RecentFailedLogins(ctx context.Context, q store.Querier, email string, since time.Time) ([]time.Time, error) {
	rows, err := q.Query(ctx,
		`select created_at from auth_attempts where kind = 'login' and email = $1 and not success and created_at > $2`, email, since)
	if err != nil {
		return nil, fmt.Errorf("recent failed logins: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return nil, fmt.Errorf("recent failed logins: %w", err)
	}
	return out, nil
}

// UpgradeHash swaps the stored hash for a stronger one of the same password.
// The condition on the old hash keeps a hash that changed meanwhile.
func (Repository) UpgradeHash(ctx context.Context, q store.Querier, id, oldHash, newHash string) error {
	if _, err := q.Exec(ctx,
		`update accounts set password_hash = $3 where id = $1 and password_hash = $2`, id, oldHash, newHash); err != nil {
		return fmt.Errorf("upgrade password hash: %w", err)
	}
	return nil
}

func (Repository) GetByEmail(ctx context.Context, q store.Querier, email string) (Account, bool, error) {
	var a Account
	err := q.QueryRow(ctx,
		`select id, email, password_hash, display_name from accounts where email = $1`, email,
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, fmt.Errorf("account by email: %w", err)
	}
	return a, true, nil
}
