package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

type MockTeamRepository struct {
	mock.Mock
}

func (m *MockTeamRepository) Create(ctx context.Context, team *repository.Team) error {
	args := m.Called(ctx, team)
	return args.Error(0)
}

func (m *MockTeamRepository) Get(ctx context.Context, name string) (*repository.Team, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Team), args.Error(1)
}

func (m *MockTeamRepository) GetTeamMembers(ctx context.Context, name string) ([]*repository.User, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.User), args.Error(1)
}
