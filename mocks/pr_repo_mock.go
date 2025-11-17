package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

type MockPullRequestRepository struct {
	mock.Mock
}

func (m *MockPullRequestRepository) GetReviewedPRs(ctx context.Context, userID string) ([]*repository.PullRequest, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.PullRequest), args.Error(1)
}

func (m *MockPullRequestRepository) GetStats(ctx context.Context) (*model.Stats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Stats), args.Error(1)
}

func (m *MockPullRequestRepository) GetReviewAssignments(ctx context.Context, users []string) ([]*repository.ReviewAssignment, error) {
	args := m.Called(ctx, users)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.ReviewAssignment), args.Error(1)
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
