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

	l.Info("setting user active status",
		zap.String("user_id", userID),
		zap.Bool("is_active", isActive))

	var result *model.User

	err := u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		user, err := u.users.Patch(txCtx, &repository.UserPatch{
			ID:       userID,
			IsActive: &isActive,
		})
		if errors.Is(err, repository.ErrNotFound) {
			l.Warn("user not found", zap.String("user_id", userID))
			return NewError(ErrorCodeNotFound, "user not found")
		}
		if err != nil {
			l.Error("failed to patch user", zap.String("user_id", userID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to update user")
		}

		if !isActive {
			if err = u.reviews.UnassignFromOpenPRs(txCtx, userID); err != nil {
				l.Error("failed to unassign user from open PRs",
					zap.String("user_id", userID),
					zap.Error(err),
				)
				return NewError(ErrorCodeUnspecified, "failed to unassign user from open PRs")
			}
		}

		result = &model.User{
			ID:       user.ID,
			Username: user.Username,
			IsActive: user.IsActive,
			TeamName: user.TeamName,
		}

		l.Debug("user active status updated successfully",
			zap.String("user_id", userID),
			zap.Bool("is_active", isActive),
		)

		return nil
	})

	var se *Error
	errors.As(err, &se)
	return result, se
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
