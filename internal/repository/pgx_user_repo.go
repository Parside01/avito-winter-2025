package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/yakoovad/avito-winter-2025/internal/db"
)

type User struct {
	ID       string `db:"id"`
	Username string `db:"username"`
	IsActive bool   `db:"is_active"`
	TeamName string `db:"team_name"`
}

type UserPatch struct {
	ID       string  `db:"id"`
	Username *string `db:"username"`
	IsActive *bool   `db:"is_active"`
	TeamName *string `db:"team_name"`
}

type UserRepository interface {
	Get(ctx context.Context, userID string) (*User, error)
	GetUserTeam(ctx context.Context, userID string) ([]*User, error)
	Upsert(ctx context.Context, user *User) error
	Patch(ctx context.Context, patch *UserPatch) (*User, error)
}

type pgxUserRepository struct {
	pool *pgxpool.Pool
}

func NewPgxUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgxUserRepository{pool: pool}
}

func (p *pgxUserRepository) GetUserTeam(ctx context.Context, userID string) ([]*User, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.From("users"),
		sm.Columns("id", "username", "is_active", "team_name"),
		sm.Where(psql.Quote("team_name").EQ(
			psql.Select(
				sm.Columns("team_name"),
				sm.From("users"),
				sm.Where(psql.Quote("id").EQ(psql.Arg(userID))),
			),
		)),
		sm.ForShare("users"),
	)

	query, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := e.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*User, error) {
		user := &User{}
		if err = row.Scan(&user.ID, &user.Username, &user.IsActive, &user.TeamName); err != nil {
			return nil, err
		}
		return user, nil
	})
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, ErrNotFound
	}

	return members, nil
}

func (p *pgxUserRepository) Patch(ctx context.Context, patch *UserPatch) (*User, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	sets := make([]bob.Mod[*dialect.UpdateQuery], 0, 3)

	if patch.Username != nil {
		sets = append(sets, um.SetCol("username").ToArg(*patch.Username))
	}
	if patch.IsActive != nil {
		sets = append(sets, um.SetCol("is_active").ToArg(*patch.IsActive))
	}
	if patch.TeamName != nil {
		sets = append(sets, um.SetCol("team_name").ToArg(*patch.TeamName))
	}

	q := psql.Update(
		um.Table("users"),
		um.Where(psql.Quote("id").EQ(psql.Arg(patch.ID))),
		um.Returning("id", "username", "is_active", "team_name"),
	)

	q.Apply(sets...)

	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	u := &User{}
	if err = e.QueryRow(ctx, sql, args...).Scan(
		&u.ID,
		&u.Username,
		&u.IsActive,
		&u.TeamName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return u, nil
}

func (p *pgxUserRepository) Upsert(ctx context.Context, user *User) error {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Insert(
		im.Into("users", "id", "username", "is_active", "team_name"),
		im.Values(psql.Arg(user.ID), psql.Arg(user.Username), psql.Arg(user.IsActive), psql.Arg(user.TeamName)),
		im.OnConflict(psql.Quote("id")).DoUpdate(
			im.SetCol("username").ToArg(user.Username),
			im.SetCol("is_active").ToArg(user.IsActive),
			im.SetCol("team_name").ToArg(user.TeamName),
		),
	)
	sql, args, err := q.Build(ctx)
	if err != nil {
		return err
	}

	if _, err = e.Exec(ctx, sql, args...); err != nil {
		return err
	}

	return nil
}

func (p *pgxUserRepository) Get(ctx context.Context, userID string) (*User, error) {
	e := db.GetPgxExecutorFromContext(ctx, p.pool)

	q := psql.Select(
		sm.Columns("id", "username", "is_active", "team_name"),
		sm.From("users"),
		sm.Where(psql.Quote("id").EQ(psql.Arg(userID))),
	)
	sql, args, err := q.Build(ctx)
	if err != nil {
		return nil, err
	}

	u := &User{}
	if err = e.QueryRow(ctx, sql, args...).Scan(
		&u.ID,
		&u.Username,
		&u.IsActive,
		&u.TeamName,
	); err != nil {
		return nil, err
	}
	return u, nil
}
