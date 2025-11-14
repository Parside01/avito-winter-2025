package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxContextKey struct{}

type pgxTransactor struct {
	pool *pgxpool.Pool
}

func NewPgxTransactor(pool *pgxpool.Pool) Transactor {
	return &pgxTransactor{pool: pool}
}

func (t *pgxTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if tx.Conn() != nil && !tx.Conn().IsClosed() {
			_ = tx.Rollback(ctx)
		}
	}()

	ctxWithTx := context.WithValue(ctx, TxContextKey{}, tx)

	if err = fn(ctxWithTx); err != nil {
		// The transaction will be rolled back in the deferred function
		return fmt.Errorf("transaction function failed: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func GetPgxExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
