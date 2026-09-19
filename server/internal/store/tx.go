package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx runs fn in a transaction and commits when fn returns nil.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// A panic must not leak the connection with an open transaction.
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
				panic(fmt.Sprintf("%v (rollback tx: %v)", p, rbErr))
			}
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback tx: %w", rbErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// IsUniqueViolation reports whether err violates the named unique index.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
