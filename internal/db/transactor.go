package db

import "context"

// Transactor allows you to run queries from repositories within a transaction
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
