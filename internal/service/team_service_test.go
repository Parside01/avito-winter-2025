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

func TestTeamService_GetTeam(t *testing.T) {
	tests := []struct {
		name          string
		teamName      string
		setupMocks    func(*MockTransactor, *MockTeamRepository)
		expectedError bool
		errorCode     model.ErrorCode
		expectedTeam  *model.Team
	}{
		{
			name:     "success",
			teamName: "backend",
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				tr.On("Get", mock.Anything, "backend").Return(&repository.Team{Name: "backend"}, nil)
				tr.On("GetTeamMembers", mock.Anything, "backend").Return([]*repository.User{
					{ID: "user1", Username: "john", IsActive: true},
					{ID: "user2", Username: "jane", IsActive: false},
				}, nil)
			},
			expectedError: false,
			expectedTeam: &model.Team{
				Name: "backend",
				Members: []*model.TeamMember{
					{UserID: "user1", Username: "john", IsActive: true},
					{UserID: "user2", Username: "jane", IsActive: false},
				},
			},
		},
		{
			name:     "team not found",
			teamName: "backend",
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				tr.On("Get", mock.Anything, "backend").Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     model.ErrorCodeNotFound,
		},
		{
			name:     "get team failure",
			teamName: "backend",
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				tr.On("Get", mock.Anything, "backend").Return(nil, errors.New("db error"))
			},
			expectedError: true,
			errorCode:     model.ErrorCodeUnspecified,
		},
		{
			name:     "get members failure",
			teamName: "backend",
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)

				tr.On("Get", mock.Anything, "backend").Return(&repository.Team{Name: "backend"}, nil)
				tr.On("GetTeamMembers", mock.Anything, "backend").Return(nil, errors.New("db error"))
			},
			expectedError: true,
			errorCode:     model.ErrorCodeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockTeamRepo := new(MockTeamRepository)

			tt.setupMocks(mockTx, mockTeamRepo)

			service := NewTeamService(mockTx).
				WithTeamRepo(mockTeamRepo)

			got, err := service.GetTeam(context.Background(), tt.teamName)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
				assert.Nil(t, got)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.expectedTeam, got)
				assert.NotNil(t, got)
			}

			mockTeamRepo.AssertExpectations(t)
		})
	}
}

func TestTeamService_AddTeam(t *testing.T) {
	tests := []struct {
		name          string
		team          *model.Team
		setupMocks    func(*MockTransactor, *MockTeamRepository, *MockUserRepository)
		expectedError bool
		errorCode     model.ErrorCode
	}{
		{
			name: "success",
			team: &model.Team{
				Name: "backend",
				Members: []*model.TeamMember{
					{UserID: "user1", Username: "john", IsActive: true},
					{UserID: "user2", Username: "jane", IsActive: true},
				},
			},
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository, ur *MockUserRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				tr.On("Create", mock.Anything, mock.MatchedBy(func(t *repository.Team) bool {
					return t.Name == "backend"
				})).Return(nil)

				ur.On("Upsert", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "team already exists",
			team: &model.Team{
				Name:    "existing-team",
				Members: []*model.TeamMember{},
			},
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository, ur *MockUserRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)
				tr.On("Create", mock.Anything, mock.Anything).Return(repository.ErrAlreadyExists)
			},
			expectedError: true,
			errorCode:     model.ErrorCodeTeamExists,
		},
		{
			name: "user upsert failure",
			team: &model.Team{
				Name: "new-team",
				Members: []*model.TeamMember{
					{UserID: "user1", Username: "john", IsActive: true},
				},
			},
			setupMocks: func(mockTx *MockTransactor, tr *MockTeamRepository, ur *MockUserRepository) {
				mockTx.On("WithinTransaction", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						fn := args.Get(1).(func(context.Context) error)
						_ = fn(args.Get(0).(context.Context))
					}).Return(nil)

				tr.On("Create", mock.Anything, mock.Anything).Return(nil)
				ur.On("Upsert", mock.Anything, mock.Anything).Return(errors.New("db error"))
			},
			expectedError: true,
			errorCode:     model.ErrorCodeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockTeamRepo := new(MockTeamRepository)
			mockUserRepo := new(MockUserRepository)

			tt.setupMocks(mockTx, mockTeamRepo, mockUserRepo)

			service := NewTeamService(mockTx).
				WithTeamRepo(mockTeamRepo).
				WithUserRepo(mockUserRepo)

			err := service.AddTeam(context.Background(), tt.team)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
			} else {
				assert.Nil(t, err)
			}

			mockTeamRepo.AssertExpectations(t)
			mockUserRepo.AssertExpectations(t)
		})
	}
}
