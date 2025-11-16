package db

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
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
	l := logger.FromContext(ctx)
	for i := 0; i <= t.maxRetries; i++ {
		err := t.transaction(ctx, fn)
		if err != nil && t.isDeadlockError(err) {
			if i == t.maxRetries {
				l.Error("maximum transaction retries reached due to deadlock", zap.Int("max_retries", t.maxRetries))
				return err
			}
			l.Warn("deadlock detected, retrying transaction", zap.Int("attempt", i+1), zap.Int("max_retries", t.maxRetries))
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	// Successful execution
	return nil
}

func (t *pgxTransactor) transaction(ctx context.Context, fn func(ctx context.Context) error) error {
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

func (t *pgxTransactor) isDeadlockError(err error) bool {
	var modelErr *model.Error
	if errors.As(err, &modelErr) && modelErr.Cause != nil {
		err = modelErr.Cause
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "40P01"
	}
	return false
}

func GetPgxExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
