package service

import (
	"context"
	"errors"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
)

type UserService struct {
	tx db.Transactor

	users   repository.UserRepository
	teams   repository.TeamRepository
	reviews repository.ReviewRepository
}

func NewUserService(tx db.Transactor) *UserService {
	return &UserService{
		tx: tx,
	}
}

func (u *UserService) SetUserIsActive(ctx context.Context, userID string, isActive bool) (*model.User, *Error) {
	l := logger.FromContext(ctx)

	l.Info("setting user active status", zap.String("user_id", userID), zap.Bool("is_active", isActive))

	user, err := u.users.Patch(ctx, &repository.UserPatch{
		ID:       userID,
		IsActive: &isActive,
	})
	if errors.Is(err, repository.ErrNotFound) {
		l.Warn("user not found", zap.String("user_id", userID))
		return nil, NewError(ErrorCodeNotFound, "user not found")
	}
	if err != nil {
		l.Error("failed to patch user", zap.String("user_id", userID), zap.Error(err))
		return nil, NewError(ErrorCodeUnspecified, "failed to update user")
	}

	l.Debug("user active status updated successfully", zap.String("user_id", userID), zap.Bool("is_active", isActive))

	return &model.User{
		ID:       user.ID,
		Username: user.Username,
		IsActive: user.IsActive,
		TeamName: user.TeamName,
	}, nil
}

func (u *UserService) WithUserRepo(userRepo repository.UserRepository) *UserService {
	u.users = userRepo
	return u
}

func (u *UserService) WithTeamRepo(teamRepo repository.TeamRepository) *UserService {
	u.teams = teamRepo
	return u
}

func (u *UserService) WithReviewRepo(reviewRepo repository.ReviewRepository) *UserService {
	u.reviews = reviewRepo
	return u
}
