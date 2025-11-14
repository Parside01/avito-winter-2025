package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
)

func TestPullRequestService_GetUserReview(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		setupMocks    func(*MockPullRequestRepository)
		expectedError bool
		errorCode     ErrorCode
		expectedPRs   int
	}{
		{
			name:   "success: user has review PRs",
			userID: "u1",
			setupMocks: func(pr *MockPullRequestRepository) {
				pr.On("GetReviewPRs", mock.Anything, "u1").Return([]*repository.PullRequest{
					{
						ID:       "pr-1001",
						AuthorID: "john",
						Name:     "ref: function",
						Status:   model.PRStatusOpen,
					},
					{
						ID:       "pr-1002",
						AuthorID: "joe",
						Name:     "fix: problem",
						Status:   model.PRStatusOpen,
					},
				}, nil)
			},
			expectedError: false,
			expectedPRs:   2,
		},
		{
			name:   "success: no PRs for user",
			userID: "u2",
			setupMocks: func(pr *MockPullRequestRepository) {
				pr.On("GetReviewPRs", mock.Anything, "u2").Return([]*repository.PullRequest{}, nil)
			},
			expectedError: false,
			expectedPRs:   0,
		},
		{
			name:   "failure repository error",
			userID: "u3",
			setupMocks: func(pr *MockPullRequestRepository) {
				pr.On("GetReviewPRs", mock.Anything, "u3").Return(nil, errors.New("db error"))
			},
			expectedError: true,
			errorCode:     ErrorCodeUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockPRRepo := new(MockPullRequestRepository)

			tt.setupMocks(mockPRRepo)

			service := NewPullRequestService(mockTx).
				WithPullRequestRepo(mockPRRepo)

			got, err := service.GetUserReview(context.Background(), tt.userID)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
				assert.Nil(t, got)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.userID, got.UserID)
				assert.Len(t, got.PullRequests, tt.expectedPRs)
			}

			mockPRRepo.AssertExpectations(t)
		})
	}
}

func TestPullRequestService_CreatePullRequest(t *testing.T) {
	tests := []struct {
		name          string
		prShort       *model.PullRequestShort
		setupMocks    func(*MockUserRepository, *MockPullRequestRepository, *MockReviewRepository)
		expectedError bool
		errorCode     ErrorCode
	}{
		{
			name: "success: create PR with 2 reviewers",
			prShort: &model.PullRequestShort{
				ID:       "pr-1001",
				AuthorID: "u1",
				Name:     "feat: feature",
				Status:   model.PRStatusOpen,
			},
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u1").Return([]*repository.User{
					{ID: "u1", Username: "author", IsActive: true, TeamName: "backend"},
					{ID: "u2", Username: "reviewer1", IsActive: true, TeamName: "backend"},
					{ID: "u3", Username: "reviewer2", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Create", mock.Anything, mock.MatchedBy(func(p *repository.PullRequest) bool {
					return p.ID == "pr-1001" && p.AuthorID == "u1"
				})).Return(nil)

				rr.On("Assign", mock.Anything, "pr-1001", []string{"u2", "u3"}).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "failure: inactive author",
			prShort: &model.PullRequestShort{
				ID:       "pr-1002",
				AuthorID: "u1",
				Name:     "feat: feature",
				Status:   model.PRStatusOpen,
			},
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u1").Return([]*repository.User{
					{ID: "u1", Username: "author", IsActive: false, TeamName: "backend"},
				}, nil)
			},
			expectedError: true,
			errorCode:     ErrorCodeUserInactive,
		},
		{
			name: "failure: author not found",
			prShort: &model.PullRequestShort{
				ID:       "pr-1003",
				AuthorID: "unknown",
				Name:     "feat: feature",
				Status:   model.PRStatusOpen,
			},
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "unknown").Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     ErrorCodeNotFound,
		},
		{
			name: "failure: PR already exists",
			prShort: &model.PullRequestShort{
				ID:       "pr-1001",
				AuthorID: "u1",
				Name:     "Duplicated",
				Status:   model.PRStatusOpen,
			},
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u1").Return([]*repository.User{
					{ID: "u1", Username: "author", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Create", mock.Anything, mock.Anything).Return(repository.ErrAlreadyExists)
			},
			expectedError: true,
			errorCode:     ErrorCodePRExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockUserRepo := new(MockUserRepository)
			mockPRRepo := new(MockPullRequestRepository)
			mockReviewRepo := new(MockReviewRepository)

			tt.setupMocks(mockUserRepo, mockPRRepo, mockReviewRepo)

			service := NewPullRequestService(mockTx).
				WithUserRepo(mockUserRepo).
				WithPullRequestRepo(mockPRRepo).
				WithReviewRepo(mockReviewRepo)

			got, err := service.CreatePullRequest(context.Background(), tt.prShort)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.prShort.ID, got.ID)
				assert.Equal(t, tt.prShort.AuthorID, got.AuthorID)
			}

			mockTx.AssertExpectations(t)
			mockUserRepo.AssertExpectations(t)
			mockPRRepo.AssertExpectations(t)
			mockReviewRepo.AssertExpectations(t)
		})
	}
}

func TestPullRequestService_ReassignPullRequest(t *testing.T) {
	tests := []struct {
		name          string
		prID          string
		userID        string
		setupMocks    func(*MockUserRepository, *MockPullRequestRepository, *MockReviewRepository)
		expectedError bool
		errorCode     ErrorCode
	}{
		{
			name:   "success: reassign to new reviewer",
			prID:   "pr-1001",
			userID: "u2",
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u2").Return([]*repository.User{
					{ID: "u1", Username: "author", IsActive: true, TeamName: "backend"},
					{ID: "u2", Username: "old_reviewer", IsActive: true, TeamName: "backend"},
					{ID: "u3", Username: "new_reviewer", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Get", mock.Anything, "pr-1001").Return(&repository.PullRequest{
					ID:       "pr-1001",
					AuthorID: "u1",
					Name:     "feat: feature",
					Status:   model.PRStatusOpen,
				}, nil)

				pr.On("GetReviewers", mock.Anything, "pr-1001").Return([]string{"u2"}, nil)

				rr.On("Unassign", mock.Anything, "pr-1001", "u2").Return(nil)
				rr.On("Assign", mock.Anything, "pr-1001", []string{"u3"}).Return(nil)
			},
			expectedError: false,
		},
		{
			name:   "failure: PR not found",
			prID:   "unknown",
			userID: "u2",
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u2").Return([]*repository.User{
					{ID: "u2", Username: "reviewer", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Get", mock.Anything, "unknown").Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     ErrorCodeNotFound,
		},
		{
			name:   "failure: PR already merged",
			prID:   "pr-1001",
			userID: "u2",
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u2").Return([]*repository.User{
					{ID: "u2", Username: "reviewer", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Get", mock.Anything, "pr-1001").Return(&repository.PullRequest{
					ID:       "pr-1001",
					AuthorID: "u1",
					Name:     "feat: feature",
					Status:   model.PRStatusMerged,
				}, nil)
			},
			expectedError: true,
			errorCode:     ErrorCodePRMerged,
		},
		{
			name:   "failure: reviewer not assigned",
			prID:   "pr-1001",
			userID: "u5",
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u5").Return([]*repository.User{
					{ID: "u5", Username: "other", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Get", mock.Anything, "pr-1001").Return(&repository.PullRequest{
					ID:       "pr-1001",
					AuthorID: "u1",
					Name:     "feat: feature",
					Status:   model.PRStatusOpen,
				}, nil)

				pr.On("GetReviewers", mock.Anything, "pr-1001").Return([]string{"u2", "u3"}, nil)
			},
			expectedError: true,
			errorCode:     ErrorCodeNotAssigned,
		},
		{
			name:   "failure: no replacement candidate",
			prID:   "pr-1001",
			userID: "u2",
			setupMocks: func(ur *MockUserRepository, pr *MockPullRequestRepository, rr *MockReviewRepository) {
				ur.On("GetUserTeam", mock.Anything, "u2").Return([]*repository.User{
					{ID: "u1", Username: "author", IsActive: true, TeamName: "backend"},
					{ID: "u2", Username: "reviewer", IsActive: true, TeamName: "backend"},
				}, nil)

				pr.On("Get", mock.Anything, "pr-1001").Return(&repository.PullRequest{
					ID:       "pr-1001",
					AuthorID: "u1",
					Name:     "feat: feature",
					Status:   model.PRStatusOpen,
				}, nil)

				pr.On("GetReviewers", mock.Anything, "pr-1001").Return([]string{"u2"}, nil)
			},
			expectedError: true,
			errorCode:     ErrorCodeNoCandidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockUserRepo := new(MockUserRepository)
			mockPRRepo := new(MockPullRequestRepository)
			mockReviewRepo := new(MockReviewRepository)

			tt.setupMocks(mockUserRepo, mockPRRepo, mockReviewRepo)

			service := NewPullRequestService(mockTx).
				WithUserRepo(mockUserRepo).
				WithPullRequestRepo(mockPRRepo).
				WithReviewRepo(mockReviewRepo)

			got, err := service.ReassignPullRequest(context.Background(), tt.prID, tt.userID)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, got)
			}

			mockTx.AssertExpectations(t)
			mockUserRepo.AssertExpectations(t)
			mockPRRepo.AssertExpectations(t)
			mockReviewRepo.AssertExpectations(t)
		})
	}
}

func TestPullRequestService_MergePullRequest(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		prID          string
		setupMocks    func(*MockPullRequestRepository)
		expectedError bool
		errorCode     ErrorCode
	}{
		{
			name: "success: - merge PR",
			prID: "pr-1001",
			setupMocks: func(pr *MockPullRequestRepository) {
				pr.On("Patch", mock.Anything, mock.MatchedBy(func(p *repository.PullRequestPatch) bool {
					return p.ID == "pr-1001" && *p.Status == model.PRStatusMerged
				})).Return(&repository.PullRequest{
					ID:       "pr-1001",
					AuthorID: "u1",
					Name:     "feat: feature",
					Status:   model.PRStatusMerged,
					MergedAt: &now,
				}, nil)

				pr.On("GetReviewers", mock.Anything, "pr-1001").Return([]string{"u2", "u3"}, nil)
			},
			expectedError: false,
		},
		{
			name: "failure: PR not found",
			prID: "unknown",
			setupMocks: func(pr *MockPullRequestRepository) {
				pr.On("Patch", mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
			},
			expectedError: true,
			errorCode:     ErrorCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTx := new(MockTransactor)
			mockPRRepo := new(MockPullRequestRepository)

			tt.setupMocks(mockPRRepo)

			service := NewPullRequestService(mockTx).
				WithPullRequestRepo(mockPRRepo)

			got, err := service.MergePullRequest(context.Background(), tt.prID)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, tt.errorCode, err.Code)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, model.PRStatusMerged, got.Status)
			}

			mockTx.AssertExpectations(t)
			mockPRRepo.AssertExpectations(t)
		})
	}
}
