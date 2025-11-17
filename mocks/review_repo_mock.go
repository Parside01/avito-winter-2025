package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"
)

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

func (m *MockReviewRepository) UnassignFromOpenPRs(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
