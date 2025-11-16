package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
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
	pool *pgxpool.Pool
}

func NewPgxTransactor(pool *pgxpool.Pool) Transactor {
	return &pgxTransactor{pool: pool}
}

func (t *pgxTransactor) Ping(ctx context.Context) error {
	return t.pool.Ping(ctx)
}

func (t *pgxTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	const maxRetries = 3

	l := logger.FromContext(ctx)

	for i := 0; i < maxRetries; i++ {
		l.Info("starting transaction attempt",
			zap.Int("attempt", i+1),
			zap.Int("max_attempts", maxRetries),
		)
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

			ctxWithTx := context.WithValue(ctx, TxContextKey{}, tx)

			if err = fn(ctxWithTx); err != nil {
				err = errors.Wrap(err, "transaction function failed")
				return
			}

			if err = tx.Commit(ctx); err != nil {
				err = errors.Wrap(err, "failed to commit transaction")
				return
			}

			l.Info("transaction succeeded",
				zap.Int("attempt", i+1),
			)
		}()

		if err == nil {
			return nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
			l.Warn("deadlock detected, retrying transaction",
				zap.Int("attempt", i+1),
				zap.Int("max_attempts", maxRetries),
			)
			// Deadlock detected -> retry
			if i < maxRetries-1 {
				continue
			}
			return errors.Wrap(err, fmt.Sprintf("transaction failed after %d retries due to deadlock", maxRetries))
		}

		return err
	}
	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func GetPgxExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(TxContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
