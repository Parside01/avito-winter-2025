package db

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"time"
)

// Transactor allows you to run queries from repositories within a transaction
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Ping(ctx context.Context) error
}

type TxContextKey struct{}

type pgxTransactor struct {
	pool       *pgxpool.Pool
	maxRetries int
}

func NewPgxTransactor(pool *pgxpool.Pool, maxRetries int) Transactor {
	return &pgxTransactor{pool: pool, maxRetries: maxRetries}
}

func (t *pgxTransactor) Ping(ctx context.Context) error {
	return t.pool.Ping(ctx)
}

func (t *pgxTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		tx, err := t.pool.Begin(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to begin transaction")
		}

		func() {
			defer func() {
				if tx.Conn() != nil && !tx.Conn().IsClosed() {
					_ = tx.Rollback(ctx)
				}
			}()
		}()

		ctxWithTx := context.WithValue(ctx, TxContextKey{}, tx)

		if err = fn(ctxWithTx); err != nil {
			lastErr = errors.Wrap(err, "transaction function failed")

			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
				if commitErr := tx.Rollback(ctx); commitErr != nil {
					return errors.Wrap(commitErr, "failed to rollback after deadlock")
				}

				if attempt < t.maxRetries {
					time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
					continue
				}
			}

			return lastErr
		}

		if err = tx.Commit(ctx); err != nil {
			lastErr = errors.Wrap(err, "failed to commit transaction")

			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "40P01" && attempt < t.maxRetries {
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
				continue
			}
			return lastErr
		}
		return nil
	}

	return lastErr
}
func GetPgxExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
