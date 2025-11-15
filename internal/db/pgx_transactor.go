package db

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

// Transactor allows you to run queries from repositories within a transaction
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Ping(ctx context.Context) error
}

type TxContextKey struct{}

type pgxTransactor struct {
	pool *pgxpool.Pool
}

func NewPgxTransactor(pool *pgxpool.Pool) Transactor {
	return &pgxTransactor{pool: pool}
}

func (t *pgxTransactor) Ping(ctx context.Context) error {
	return t.pool.Ping(ctx)
}

func (t *pgxTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		if tx.Conn() != nil && !tx.Conn().IsClosed() {
			_ = tx.Rollback(ctx)
		}
	}()

	ctxWithTx := context.WithValue(ctx, TxContextKey{}, tx)

	if err = fn(ctxWithTx); err != nil {
		// The transaction will be rolled back in the deferred function
		return errors.Wrap(err, "transaction function failed")
	}

	if err = tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func GetPgxExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
