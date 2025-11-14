package repository

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/yakoovad/avito-winter-2025/internal/db"
)

type Team struct {
	Name string `db:"name"`
}

type TeamRepository interface {
	Create(ctx context.Context, team *Team) error
	Get(ctx context.Context, name string) (*Team, error)
	GetTeamMembers(ctx context.Context, name string) ([]*User, error)
}

type pgxTeamRepository struct {
	pool *pgxpool.Pool
}

func NewPgxTeamRepository(pool *pgxpool.Pool) TeamRepository {
	return &pgxTeamRepository{pool: pool}
}

func (p *pgxTeamRepository) Create(ctx context.Context, team *Team) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Insert(
		im.Into("team", "name"),
		im.Values(psql.Arg(team.Name)),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	_, err = e.Exec(ctx, sql, args...)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyExists
	}

	return err
}

func (p *pgxTeamRepository) Get(ctx context.Context, name string) (*Team, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("name"),
		sm.From("team"),
		sm.Where(psql.Quote("name").EQ(psql.Arg(name))),
	)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	team := &Team{}
	if err = e.QueryRow(ctx, sql, args...).Scan(&team.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return team, nil
}

func (p *pgxTeamRepository) GetTeamMembers(ctx context.Context, name string) ([]*User, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("*"),
		sm.From("users").As("u"),
		sm.Where(psql.Quote("u", "team_name").EQ(psql.Arg(name))),
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

	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*User, error) {
		user := &User{}
		if err = row.Scan(&user.ID, &user.Username, &user.IsActive, &user.TeamName); err != nil {
			return nil, err
		}
		return user, nil
	})
	if err != nil {
		return nil, err
	}

	return users, err
}
