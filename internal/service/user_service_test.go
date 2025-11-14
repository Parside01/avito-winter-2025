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
		setupMocks    func(*MockUserRepository)
		expectedError bool
		errorCode     ErrorCode
		expectedUser  *model.User
	}{
		{
			name:     "success activate",
			userID:   "user1",
			isActive: true,
			setupMocks: func(ur *MockUserRepository) {
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
			setupMocks: func(ur *MockUserRepository) {
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
			setupMocks: func(ur *MockUserRepository) {
				ur.On("Patch", mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     ErrorCodeNotFound,
		},
		{
			name:     "patch failed",
			userID:   "user1",
			isActive: true,
			setupMocks: func(ur *MockUserRepository) {
				ur.On("Patch", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
			},
			expectedError: true,
			errorCode:     ErrorCodeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockUserRepo := new(MockUserRepository)

			tt.setupMocks(mockUserRepo)

			service := NewUserService(mockTx).
				WithUserRepo(mockUserRepo)

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
