package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Upsert(ctx context.Context, user *repository.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserTeam(ctx context.Context, userID string) ([]*repository.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.User), args.Error(1)
}

func (m *MockUserRepository) Get(ctx context.Context, userID string) (*repository.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*repository.User), args.Error(1)
}

func (m *MockUserRepository) Patch(ctx context.Context, patch *repository.UserPatch) (*repository.User, error) {
	args := m.Called(ctx, patch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.User), args.Error(1)
}
