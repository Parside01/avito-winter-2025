package service

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

type MockTransactor struct {
	mock.Mock
}

func (m *MockTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

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

type MockPullRequestRepository struct {
	mock.Mock
}

func (m *MockPullRequestRepository) Create(ctx context.Context, pr *repository.PullRequest) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockPullRequestRepository) Patch(ctx context.Context, pr *repository.PullRequestPatch) (*repository.PullRequest, error) {
	args := m.Called(ctx, pr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PullRequest), args.Error(1)
}

func (m *MockPullRequestRepository) Get(ctx context.Context, prID string) (*repository.PullRequest, error) {
	args := m.Called(ctx, prID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PullRequest), args.Error(1)
}

func (m *MockPullRequestRepository) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	args := m.Called(ctx, prID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockPullRequestRepository) GetReviewPRs(ctx context.Context, userID string) ([]*repository.PullRequest, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.PullRequest), args.Error(1)
}

type MockReviewRepository struct {
	mock.Mock
}

func (m *MockReviewRepository) Assign(ctx context.Context, prID string, reviewerIDs []string) error {
	args := m.Called(ctx, prID, reviewerIDs)
	return args.Error(0)
}

func (m *MockReviewRepository) Unassign(ctx context.Context, prID string, reviewerIDs string) error {
	args := m.Called(ctx, prID, reviewerIDs)
	return args.Error(0)
}
