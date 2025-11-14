package service

import (
	"context"
	"github.com/pkg/errors"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
)

type TeamService struct {
	tx db.Transactor

	users   repository.UserRepository
	teams   repository.TeamRepository
	reviews repository.ReviewRepository
}

func NewTeamService(tx db.Transactor) *TeamService {
	return &TeamService{
		tx: tx,
	}
}

func (t *TeamService) AddTeam(ctx context.Context, team *model.Team) *Error {
	l := logger.FromContext(ctx)
	l.Info("adding team", zap.String("team_name", team.Name), zap.Any("team", team))

	err := t.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		err := t.teams.Create(txCtx, &repository.Team{
			Name: team.Name,
		})
		if errors.Is(err, repository.ErrAlreadyExists) {
			l.Warn("team already exists", zap.String("team_name", team.Name))
			return NewError(ErrorCodeTeamExists, "team_name already exists")
		}
		if err != nil {
			l.Error("failed to create team", zap.String("team_name", team.Name), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to create team")
		}

		for _, user := range team.Members {
			if err = t.users.Upsert(txCtx, &repository.User{
				ID:       user.UserID,
				Username: user.Username,
				IsActive: user.IsActive,
				TeamName: team.Name,
			}); err != nil {
				l.Error("failed to upsert team member",
					zap.String("team_name", team.Name),
					zap.String("user_id", user.UserID),
					zap.Error(err))
				return NewError(ErrorCodeUnspecified, "failed to upsert team member")
			}
		}

		l.Debug("team added successfully", zap.String("team_name", team.Name))

		return nil
	})

	var res *Error
	errors.As(err, &res)

	return res
}

func (t *TeamService) GetTeam(ctx context.Context, name string) (*model.Team, *Error) {
	l := logger.FromContext(ctx)
	l.Debug("getting team", zap.String("team_name", name))

	teamRepo, err := t.teams.Get(ctx, name)
	if errors.Is(err, repository.ErrNotFound) {
		l.Warn("team not found", zap.String("team_name", name))
		return nil, NewError(ErrorCodeNotFound, "team not found")
	}
	if err != nil {
		l.Error("failed to get team", zap.String("team_name", name), zap.Error(err))
		return nil, NewError(ErrorCodeUnspecified, "failed to get team")
	}

	membersRepo, err := t.teams.GetTeamMembers(ctx, name)
	if err != nil {
		l.Error("failed to get team members", zap.String("team_name", name), zap.Error(err))
		return nil, NewError(ErrorCodeUnspecified, "failed to get team members")
	}

	members := make([]*model.TeamMember, 0, len(membersRepo))
	for _, member := range membersRepo {
		members = append(members, &model.TeamMember{
			UserID:   member.ID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}

	l.Debug("team retrieved successfully", zap.String("team_name", name))

	return &model.Team{
		Name:    teamRepo.Name,
		Members: members,
	}, nil
}

func (t *TeamService) WithUserRepo(r repository.UserRepository) *TeamService {
	t.users = r
	return t
}

func (t *TeamService) WithTeamRepo(r repository.TeamRepository) *TeamService {
	t.teams = r
	return t
}

func (t *TeamService) WithReviewRepo(r repository.ReviewRepository) *TeamService {
	t.reviews = r
	return t
}
