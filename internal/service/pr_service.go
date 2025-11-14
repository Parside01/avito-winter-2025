package service

import (
	"context"
	"errors"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
	"github.com/yakoovad/avito-winter-2025/pkg/logger"
	"go.uber.org/zap"
	"slices"
)

type PullRequestService struct {
	tx db.Transactor

	users   repository.UserRepository
	teams   repository.TeamRepository
	prs     repository.PullRequestRepository
	reviews repository.ReviewRepository
}

func NewPullRequestService(tx db.Transactor) *PullRequestService {
	return &PullRequestService{
		tx: tx,
	}
}

func (p *PullRequestService) GetUserReview(ctx context.Context, userID string) (*model.UserReviews, *Error) {
	l := logger.FromContext(ctx)
	l.Info("getting user reviews", zap.String("user_id", userID))

	prs := make([]*model.PullRequestShort, 0)

	repoPRs, err := p.prs.GetReviewPRs(ctx, userID)
	if err != nil {
		l.Error("failed to get user review PRs", zap.String("user_id", userID), zap.Error(err))
		return nil, NewError(ErrorCodeUnspecified, "failed to get user reviews")
	}

	for _, pr := range repoPRs {
		prs = append(prs, &model.PullRequestShort{
			ID:       pr.ID,
			AuthorID: pr.AuthorID,
			Name:     pr.Name,
			Status:   pr.Status,
		})
	}

	res := &model.UserReviews{
		UserID:       userID,
		PullRequests: prs,
	}

	l.Debug("user reviews retrieved successfully")
	return res, nil
}

func (p *PullRequestService) ReassignPullRequest(ctx context.Context, prID, userID string) (*model.PullRequest, *Error) {
	l := logger.FromContext(ctx)
	l.Info("reassigning pull request", zap.String("pull_request_id", prID), zap.String("user_id", userID))

	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		repoTeam, err := p.users.GetUserTeam(txCtx, userID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			l.Warn("user or team not found", zap.String("user_id", userID))
			return NewError(ErrorCodeNotFound, "user or team not found")
		case err != nil:
			l.Error("failed to get user team", zap.String("user_id", userID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get user team")
		}

		team := make([]*model.User, 0, len(repoTeam))
		for i := range repoTeam {
			team = append(team, &model.User{
				ID:       repoTeam[i].ID,
				Username: repoTeam[i].Username,
				IsActive: repoTeam[i].IsActive,
				TeamName: repoTeam[i].TeamName,
			})
		}

		repoPR, err := p.prs.Get(txCtx, prID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			l.Warn("PR not found", zap.String("pull_request_id", prID))
			return NewError(ErrorCodeNotFound, "PR not found")
		case err != nil:
			l.Error("failed to get PR", zap.String("pull_request_id", prID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get PR")
		}

		if repoPR.Status == model.PRStatusMerged {
			l.Warn("cannot reassign merged PR", zap.String("pull_request_id", prID))
			return NewError(ErrorCodePRMerged, "cannot reassign on merged PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil {
			l.Error("failed to get reviewers", zap.String("pull_request_id", prID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get reviewers")
		}

		if !slices.Contains(reviewers, userID) {
			l.Warn("reviewer not assigned to PR", zap.String("pull_request_id", prID), zap.String("user_id", userID))
			return NewError(ErrorCodeNotAssigned, "reviewer is not assigned to this PR")
		}

		newReviewer := p.selectReplacementReviewer(repoPR.AuthorID, reviewers, team)
		if newReviewer == "" {
			l.Warn("no replacement candidate found", zap.String("pull_request_id", prID))
			return NewError(ErrorCodeNoCandidate, "no active replacement candidate in team")
		}

		if err = p.reviews.Unassign(txCtx, prID, userID); err != nil {
			l.Error("failed to unassign old reviewer", zap.String("pull_request_id", prID), zap.String("user_id", userID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to unassign old reviewer")
		}

		if err = p.reviews.Assign(txCtx, prID, []string{newReviewer}); err != nil {
			l.Error("failed to assign new reviewer", zap.String("pull_request_id", prID), zap.String("new_reviewer", newReviewer), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to assign new reviewer")
		}

		l.Debug("reviewer reassigned successfully",
			zap.String("pull_request_id", prID),
			zap.String("old_reviewer", userID),
			zap.String("new_reviewer", newReviewer))

		pr.CreatedAt = repoPR.CreatedAt
		pr.MergedAt = repoPR.MergedAt
		pr.Name = repoPR.Name
		pr.Status = repoPR.Status
		pr.AuthorID = repoPR.AuthorID
		pr.Reviewers = reviewers

		return nil
	})

	var res *Error
	errors.As(err, &res)

	if res != nil {
		l.Error("reassign PR operation failed", zap.String("pull_request_id", prID), zap.Error(res))
	}

	return pr, res
}

func (p *PullRequestService) MergePullRequest(ctx context.Context, prID string) (*model.PullRequest, *Error) {
	l := logger.FromContext(ctx)
	l.Info("merging pull request", zap.String("pull_request_id", prID))

	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		status := model.PRStatusMerged
		repoPR, err := p.prs.Patch(txCtx, &repository.PullRequestPatch{
			ID:     prID,
			Status: &status,
		})
		switch {
		case errors.Is(err, repository.ErrNotFound):
			l.Warn("PR not found", zap.String("pull_request_id", prID))
			return NewError(ErrorCodeNotFound, "PR not found")
		case err != nil:
			l.Error("failed to patch PR", zap.String("pull_request_id", prID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil {
			l.Error("failed to get reviewers", zap.String("pull_request_id", prID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get reviewers")
		}

		l.Debug("PR merged successfully", zap.String("pull_request_id", prID))

		pr.CreatedAt = repoPR.CreatedAt
		pr.MergedAt = repoPR.MergedAt
		pr.Name = repoPR.Name
		pr.Status = repoPR.Status
		pr.AuthorID = repoPR.AuthorID
		pr.Reviewers = reviewers

		return nil
	})

	var res *Error
	errors.As(err, &res)

	return pr, res
}

// CreatePullRequest Create a new pull request and assign two team members as reviewers
func (p *PullRequestService) CreatePullRequest(ctx context.Context, short *model.PullRequestShort) (*model.PullRequest, *Error) {
	l := logger.FromContext(ctx)
	l.Info("creating pull request",
		zap.String("pull_request_id", short.ID),
		zap.String("pr_name", short.Name),
		zap.String("author_id", short.AuthorID))

	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		repoTeam, err := p.users.GetUserTeam(ctx, short.AuthorID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			l.Warn("author not found", zap.String("author_id", short.AuthorID))
			return NewError(ErrorCodeNotFound, "author or PR not found")
		case err != nil:
			l.Error("failed to get author team", zap.String("author_id", short.AuthorID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to get author team")
		}

		team := make([]*model.User, 0, len(repoTeam))
		for i := range repoTeam {
			if repoTeam[i].ID == short.AuthorID && !repoTeam[i].IsActive {
				l.Warn("inactive user cannot create PR", zap.String("author_id", short.AuthorID))
				return NewError(ErrorCodeUserInactive, "inactive user cannot create PR")
			}

			team = append(team, &model.User{
				ID:       repoTeam[i].ID,
				Username: repoTeam[i].Username,
				IsActive: repoTeam[i].IsActive,
				TeamName: repoTeam[i].TeamName,
			})
		}

		repoPR := &repository.PullRequest{
			ID:                short.ID,
			AuthorID:          short.AuthorID,
			Name:              short.Name,
			NeedMoreReviewers: false,
			Status:            model.PRStatusOpen,
		}
		err = p.prs.Create(txCtx, repoPR)
		switch {
		case errors.Is(err, repository.ErrAlreadyExists):
			l.Warn("PR already exists", zap.String("pull_request_id", short.ID))
			return NewError(ErrorCodePRExists, "PR id already exists")
		case err != nil:
			l.Error("failed to create PR", zap.String("pull_request_id", short.ID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to create PR")
		}

		reviewers := p.selectReviewers(short.AuthorID, team, 2)

		err = p.reviews.Assign(txCtx, repoPR.ID, reviewers)
		if err != nil {
			l.Error("failed to assign reviewers", zap.String("pull_request_id", repoPR.ID), zap.Error(err))
			return NewError(ErrorCodeUnspecified, "failed to assign PR")
		}

		l.Info("PR created successfully",
			zap.String("pull_request_id", repoPR.ID),
			zap.Strings("reviewers", reviewers))

		pr.CreatedAt = repoPR.CreatedAt
		pr.MergedAt = repoPR.MergedAt
		pr.Name = repoPR.Name
		pr.Status = repoPR.Status
		pr.AuthorID = repoPR.AuthorID
		pr.Reviewers = reviewers

		return nil
	})

	var res *Error
	errors.As(err, &res)

	return pr, res
}

func (p *PullRequestService) selectReplacementReviewer(authorID string, reviewers []string, team []*model.User) string {
	for _, member := range team {
		if member.ID == authorID {
			continue
		}

		if !member.IsActive {
			continue
		}

		if slices.Contains(reviewers, member.ID) {
			continue
		}

		return member.ID
	}

	return ""
}

// selectReviewers Selects up to `max` active reviewers from team and returns their IDs
func (p *PullRequestService) selectReviewers(author string, team []*model.User, max int) []string {
	reviewers := make([]string, 0, max)
	for _, member := range team {
		if member.ID == author || !member.IsActive {
			continue
		}

		reviewers = append(reviewers, member.ID)

		if len(reviewers) == max {
			break
		}
	}
	return reviewers
}

func (p *PullRequestService) WithUserRepo(r repository.UserRepository) *PullRequestService {
	p.users = r
	return p
}

func (p *PullRequestService) WithTeamRepo(r repository.TeamRepository) *PullRequestService {
	p.teams = r
	return p
}

func (p *PullRequestService) WithPullRequestRepo(r repository.PullRequestRepository) *PullRequestService {
	p.prs = r
	return p
}

func (p *PullRequestService) WithReviewRepo(r repository.ReviewRepository) *PullRequestService {
	p.reviews = r
	return p
}
