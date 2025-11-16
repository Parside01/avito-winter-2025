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

type ReviewAssignment struct {
	PRID     string
	AuthorID string
	Status   model.PRStatus
	UserIDs  []string
}

type PullRequestRepository interface {
	Create(ctx context.Context, pr *PullRequest) error
	Patch(ctx context.Context, pr *PullRequestPatch) (*PullRequest, error)
	Get(ctx context.Context, prID string) (*PullRequest, error)
	GetReviewers(ctx context.Context, prID string) ([]string, error)
	GetReviewedPRs(ctx context.Context, userID string) ([]*PullRequest, error)
	GetStats(ctx context.Context) (*model.Stats, error)
	GetReviewAssignments(ctx context.Context, users []string) ([]*ReviewAssignment, error)
}

type pgxPullRequestRepository struct {
	pool *pgxpool.Pool
}

func NewPgxPullRequestRepository(pool *pgxpool.Pool) PullRequestRepository {
	return &pgxPullRequestRepository{pool: pool}
}

// GetReviewAssignments returns pull requests assigned to users (merged + open!)
func (p *pgxPullRequestRepository) GetReviewAssignments(ctx context.Context, users []string) ([]*ReviewAssignment, error) {
	if len(users) == 0 {
		return []*ReviewAssignment{}, nil
	}
	e := db.GetPgxExecutorFromContext(ctx, p.pool)
	inArgs := make([]any, 0, len(users)+1)
	for _, u := range users {
		inArgs = append(inArgs, u)
	}

	q := psql.Select(
		sm.Columns(
			psql.Quote("pr", "id"),
			psql.Quote("pr", "author_id"),
			psql.Quote("pr", "status"),
			psql.F("ARRAY_AGG", psql.Quote("r", "user_id")),
		),
		sm.From("review").As("r"),
		sm.LeftJoin("pull_request").As("pr").
			On(psql.Quote("r", "pull_request_id").
				EQ(psql.Quote("pr", "id"))),
		sm.Where(
			psql.Quote("r", "user_id").In(psql.Arg(inArgs...)),
		),
		sm.GroupBy(
			psql.Quote("pr", "id")),
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

	reviews, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*ReviewAssignment, error) {
		ra := &ReviewAssignment{}
		err = row.Scan(&ra.PRID, &ra.AuthorID, &ra.Status, &ra.UserIDs)
		return ra, err
	})
	if err != nil {
		return nil, err
	}

	return reviews, nil
}

func (p *pgxPullRequestRepository) GetReviewedPRs(ctx context.Context, userID string) ([]*PullRequest, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("pull_request_id", "pull_request.name", "pull_request.author_id", "pull_request.status"),
		sm.From("review"),
		sm.LeftJoin("pull_request").On(psql.Quote("review", "pull_request_id").EQ(psql.Quote("pull_request", "id"))),
		sm.Where(
			psql.Quote("user_id").EQ(psql.Arg(userID)),
		),
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

// GetStats Я полностью осознаю, что этот метод очень тяжелый и его надо распилить + везде использовал bob, а тут чистый sql😶‍🌫️
// Но как это красиво сделать с bob я пока не придумал, а времени уже мало.... Просто пришел к тому что надо слать батчями хотя бы...
func (p *pgxPullRequestRepository) GetStats(ctx context.Context) (*model.Stats, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)
	batch := &pgx.Batch{}

	const (
		totalPRsQuery = `
		SELECT COUNT(*) 
		FROM pull_request 
		WHERE status = 'OPEN';`

		totalAssignmentsQuery = `
		SELECT COUNT(r.user_id)
		FROM review r
		JOIN pull_request pr ON r.pull_request_id = pr.id
		WHERE pr.status = 'OPEN';`

		userAssignmentsQuery = `
		WITH open_reviews AS (SELECT r.user_id, COUNT(r.pull_request_id) AS assignments_count
		  FROM review r
				   JOIN pull_request pr ON r.pull_request_id = pr.id
		  WHERE pr.status = 'OPEN'
		  GROUP BY r.user_id
		)
		SELECT
			u.id AS user_id,
			u.username,
			COALESCE(open_reviews.assignments_count, 0) AS assignments_count
		FROM users u
		LEFT JOIN open_reviews ON u.id = open_reviews.user_id
		WHERE u.is_active = TRUE`

		reviewsRPQuery = `
		SELECT
			pr.id AS pull_request_id,
			pr.name AS pull_request_name,
			COUNT(r.user_id) AS reviewers_count
		FROM pull_request pr
		LEFT JOIN review r ON pr.id = r.pull_request_id
		WHERE pr.status = 'OPEN'
		GROUP BY pr.id, pr.name`
	)

	batch.Queue(totalPRsQuery)
	batch.Queue(totalAssignmentsQuery)
	batch.Queue(userAssignmentsQuery)
	batch.Queue(reviewsRPQuery)

	br := e.SendBatch(ctx, batch)
	defer func() {
		_ = br.Close()
	}()

	var totalPRs int
	var totalAssignments int
	var userAssignments []*model.UserAssignments
	var reviewsPRs []*model.ReviewsPR

	if err := br.QueryRow().Scan(&totalPRs); err != nil {
		return nil, err
	}

	if err := br.QueryRow().Scan(&totalAssignments); err != nil {
		return nil, err
	}

	rowsUserStats, err := br.Query()
	if err != nil {
		return nil, err
	}
	userAssignments, err = pgx.CollectRows(rowsUserStats, func(row pgx.CollectableRow) (*model.UserAssignments, error) {
		us := &model.UserAssignments{}
		err = row.Scan(&us.UserID, &us.Username, &us.AssignmentsCount)
		return us, err
	})
	if err != nil {
		return nil, err
	}

	rowsPRStats, err := br.Query()
	if err != nil {
		return nil, err
	}
	reviewsPRs, err = pgx.CollectRows(rowsPRStats, func(row pgx.CollectableRow) (*model.ReviewsPR, error) {
		ps := &model.ReviewsPR{}
		err = row.Scan(&ps.PullRequestID, &ps.PullRequestName, &ps.ReviewersCount)
		return ps, err
	})
	if err != nil {
		return nil, err
	}

	return &model.Stats{
		TotalPRsCount:    totalPRs,
		TotalAssignments: totalAssignments,
		UserAssignments:  userAssignments,
		ReviewsPR:        reviewsPRs,
	}, nil
}
