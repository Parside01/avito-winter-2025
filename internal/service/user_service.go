package service

import (
	"context"
	"errors"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

type UserService struct {
	tx db.Transactor

	users   repository.UserRepository
	teams   repository.TeamRepository
	reviews repository.ReviewRepository
}

func NewUserService(tx db.Transactor) *UserService {
	return &UserService{tx: tx}
}

func (u *UserService) SetUserIsActive(ctx context.Context, userID string, isActive bool) (*model.User, *Error) {
	user, err := u.users.Patch(ctx, &repository.UserPatch{
		ID:       userID,
		IsActive: &isActive,
	})
	if errors.Is(err, repository.ErrNotFound) {
		return nil, NewServiceError(ErrorCodeNotFound, "user not found")
	}
	if err != nil {
		return nil, NewServiceError(ErrorCodeUnspecified, "failed to update user")
	}
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
