package repository

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"time"
)

type PullRequest struct {
	ID                string         `db:"id"`
	Name              string         `db:"name"`
	AuthorID          string         `db:"author_id"`
	Status            model.PRStatus `db:"status"`
	NeedMoreReviewers bool           `db:"need_more_reviewers"`
	CreatedAt         *time.Time     `db:"created_at"`
	MergedAt          *time.Time     `db:"merged_at"`
}

type PullRequestPatch struct {
	ID                string          `db:"id"`
	Name              *string         `db:"name"`
	AuthorID          *string         `db:"author_id"`
	Status            *model.PRStatus `db:"status"`
	NeedMoreReviewers *bool           `db:"need_more_reviewers"`
}

type PullRequestRepository interface {
	Create(ctx context.Context, pr *PullRequest) error
	Patch(ctx context.Context, pr *PullRequestPatch) (*PullRequest, error)
	Get(ctx context.Context, prID string) (*PullRequest, error)
	GetReviewers(ctx context.Context, prID string) ([]string, error)
	GetReviewPRs(ctx context.Context, userID string) ([]*PullRequest, error)
}

type pgxPullRequestRepository struct {
	pool *pgxpool.Pool
}

func NewPgxPullRequestRepository(pool *pgxpool.Pool) PullRequestRepository {
	return &pgxPullRequestRepository{pool: pool}
}

func (p *pgxPullRequestRepository) GetReviewPRs(ctx context.Context, userID string) ([]*PullRequest, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("pull_request_id", "pull_request.name", "pull_request.author_id", "pull_request.status"),
		sm.From("review"),
		sm.LeftJoin("pull_request").On(psql.Quote("review", "pull_request_id").EQ(psql.Quote("pull_request", "id"))),
		sm.Where(
			psql.Quote("user_id").EQ(psql.Arg(userID)),
		),
		sm.ForShare("review"),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := e.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*PullRequest, error) {
		pr := &PullRequest{}
		if err = row.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		return pr, nil
	})
	if err != nil {
		return nil, err
	}

	return prs, nil
}

func (p *pgxPullRequestRepository) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("user_id"),
		sm.From("review"),
		sm.Where(psql.Quote("pull_request_id").EQ(psql.Arg(prID))),
		sm.ForShare("review"),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := e.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviewers, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err = row.Scan(&id)
		return id, err
	})

	if len(reviewers) == 0 {
		return nil, ErrNotFound
	}

	return reviewers, nil
}

func (p *pgxPullRequestRepository) Get(ctx context.Context, prID string) (*PullRequest, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("id", "name", "author_id", "status", "need_more_reviewers", "created_at", "merged_at"),
		sm.From("pull_request"),
		sm.Where(psql.Quote("id").EQ(psql.Arg(prID))),
		sm.ForShare("pull_request"),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	pr := &PullRequest{}
	if err = e.QueryRow(ctx, sql, args...).Scan(
		&pr.ID,
		&pr.Name,
		&pr.AuthorID,
		&pr.Status,
		&pr.NeedMoreReviewers,
		&pr.CreatedAt,
		&pr.MergedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return pr, nil
}

// Create Insert a pull request into database and set pr.ID
func (p *pgxPullRequestRepository) Create(ctx context.Context, pr *PullRequest) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Insert(
		im.Into("pull_request", "id", "name", "author_id", "status", "need_more_reviewers"),
		im.Values(psql.Arg(pr.ID), psql.Arg(pr.Name), psql.Arg(pr.AuthorID), psql.Arg(pr.Status), psql.Arg(pr.NeedMoreReviewers)),
		im.Returning("id", "name", "author_id", "status", "need_more_reviewers", "created_at", "merged_at"),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	err = e.QueryRow(ctx, sql, args...).Scan(
		&pr.ID,
		&pr.Name,
		&pr.AuthorID,
		&pr.Status,
		&pr.NeedMoreReviewers,
		&pr.CreatedAt,
		&pr.MergedAt,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrAlreadyExists
		case "23503": // In this case author_id does not exist
			return ErrNotFound
		}
	}
	return err
}

func (p *pgxPullRequestRepository) Patch(ctx context.Context, patch *PullRequestPatch) (*PullRequest, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	sets := make([]bob.Mod[*dialect.UpdateQuery], 0, 3)
	if patch.Name != nil {
		sets = append(sets, um.SetCol("name").ToArg(*patch.Name))
	}
	if patch.Status != nil {
		sets = append(sets, um.SetCol("status").ToArg(*patch.Status))
	}
	if patch.NeedMoreReviewers != nil {
		sets = append(sets, um.SetCol("need_more_reviewers").ToArg(*patch.NeedMoreReviewers))
	}

	q := psql.Update(
		um.Table("pull_request"),
		um.Where(psql.Quote("id").EQ(psql.Arg(patch.ID))),
		um.Returning("id", "name", "status", "author_id", "need_more_reviewers", "created_at", "merged_at"),
	)

	q.Apply(sets...)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	pr := &PullRequest{}
	if err = e.QueryRow(ctx, sql, args...).Scan(
		&pr.ID,
		&pr.Name,
		&pr.Status,
		&pr.AuthorID,
		&pr.NeedMoreReviewers,
		&pr.CreatedAt,
		&pr.MergedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return pr, nil
}
