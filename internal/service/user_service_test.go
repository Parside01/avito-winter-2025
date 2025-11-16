package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"testing"
)

func TestUserService_SetUserIsActive(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		isActive      bool
		setupMocks    func(*MockTransactor, *MockUserRepository, *MockReviewRepository)
		expectedError bool
		errorCode     model.ErrorCode
		expectedUser  *model.User
	}{
		{
			name:     "success activate",
			userID:   "user1",
			isActive: true,
			setupMocks: func(mockTx *MockTransactor, ur *MockUserRepository, rw *MockReviewRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)

				isActive := true
				ur.On("Patch", mock.Anything, &repository.UserPatch{
					ID:       "user1",
					IsActive: &isActive,
				}).Return(&repository.User{
					ID:       "user1",
					Username: "john",
					IsActive: true,
					TeamName: "backend",
				}, nil)

				rw.On("UnassignFromOpenPRs", mock.Anything, "user1").Return(nil)
			},
			expectedError: false,
			expectedUser: &model.User{
				ID:       "user1",
				Username: "john",
				IsActive: true,
				TeamName: "backend",
			},
		},
		{
			name:     "success deactivate",
			userID:   "user1",
			isActive: false,
			setupMocks: func(mockTx *MockTransactor, ur *MockUserRepository, rw *MockReviewRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)

				isActive := false
				ur.On("Patch", mock.Anything, &repository.UserPatch{
					ID:       "user1",
					IsActive: &isActive,
				}).Return(&repository.User{
					ID:       "user1",
					Username: "john",
					IsActive: false,
					TeamName: "backend",
				}, nil)

				rw.On("UnassignFromOpenPRs", mock.Anything, "user1").Return(nil)
			},
			expectedError: false,
			expectedUser: &model.User{
				ID:       "user1",
				Username: "john",
				IsActive: false,
				TeamName: "backend",
			},
		},
		{
			name:     "user not found",
			userID:   "unknown",
			isActive: true,
			setupMocks: func(mockTx *MockTransactor, ur *MockUserRepository, rw *MockReviewRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				ur.On("Patch", mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     model.ErrorCodeNotFound,
		},
		{
			name:     "patch failed",
			userID:   "user1",
			isActive: true,
			setupMocks: func(mockTx *MockTransactor, ur *MockUserRepository, rw *MockReviewRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				ur.On("Patch", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
			},
			expectedError: true,
			errorCode:     model.ErrorCodeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockUserRepo := new(MockUserRepository)
			mockReviewRepo := new(MockReviewRepository)

			tt.setupMocks(mockTx, mockUserRepo, mockReviewRepo)

			service := NewUserService(mockTx).
				WithUserRepo(mockUserRepo).WithReviewRepo(mockReviewRepo)

			got, err := service.SetUserIsActive(context.Background(), tt.userID, tt.isActive)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
				assert.Nil(t, got)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}
