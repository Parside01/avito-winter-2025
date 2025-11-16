package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/yakoovad/avito-winter-2025/internal/db"
)

type ReviewRepository interface {
	Assign(ctx context.Context, prID string, reviewerIDs []string) error
	Unassign(ctx context.Context, prID string, reviewerIDs string) error
	UnassignFromOpenPRs(ctx context.Context, userID string) error
}
type pgxReviewRepository struct {
	pool *pgxpool.Pool
}

func NewPgxReviewRepository(pool *pgxpool.Pool) ReviewRepository {
	return &pgxReviewRepository{pool: pool}
}

func (p *pgxReviewRepository) UnassignFromOpenPRs(ctx context.Context, userID string) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Delete(
		dm.From("review"),
		dm.Where(
			psql.Quote("user_id").EQ(psql.Arg(userID)).And(
				psql.Quote("pull_request_id").In(
					psql.Select(
						sm.Columns("id"),
						sm.From("pull_request"),
						sm.Where(psql.Quote("status").EQ(psql.Arg("OPEN"))),
					),
				),
			),
		),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	tag, err := e.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	// Strange if dont use this, objects dont delete
	if tag.Delete() {
		return nil
	}
	return nil
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
