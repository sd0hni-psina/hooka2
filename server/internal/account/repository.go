package account

import (
	"context"
	"errors"
	"fmt"

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
