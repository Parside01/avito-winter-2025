package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/yakoovad/avito-winter-2025/internal/db"
)

type ReviewRepository interface {
	Assign(ctx context.Context, prID string, reviewerIDs []string) error
	Unassign(ctx context.Context, prID string, reviewerIDs string) error
}
type pgxReviewRepository struct {
	pool *pgxpool.Pool
}

func NewPgxReviewRepository(pool *pgxpool.Pool) ReviewRepository {
	return &pgxReviewRepository{pool: pool}
}

func (p *pgxReviewRepository) Assign(ctx context.Context, prID string, reviewerIDs []string) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Insert(
		im.Into("review", "user_id", "pull_request_id"),
	)

	for _, reviewerID := range reviewerIDs {
		q.Apply(im.Values(psql.Arg(reviewerID), psql.Arg(prID)))
	}

	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	if _, err = e.Exec(ctx, sql, args...); err != nil {
		return err
	}

	return nil
}

func (p *pgxReviewRepository) Unassign(ctx context.Context, prID string, reviewerID string) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Delete(
		dm.From("review"),
		dm.Where(
			psql.Quote("pull_request_id").EQ(psql.Arg(prID)).
				And(psql.Quote("user_id").In(psql.Arg(reviewerID))),
		))

	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	commandTag, err := e.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
