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
	"sort"
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

func (p *PullRequestService) DeactivateTeamMembers(ctx context.Context, team string, users []string) *model.Error {
	l := logger.FromContext(ctx)

	l.Info("deactivating team members",
		zap.String("team_name", team),
		zap.Strings("user_ids", users),
	)

	// Sort users to have deterministic processing order
	sort.Strings(users)

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		membersRepo, err := p.teams.GetTeamMembers(txCtx, team)
		if err != nil {
			l.Error("failed to get team members", zap.String("team_name", team), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get team members", err)
		}

		members := make([]*model.TeamMember, 0, len(membersRepo))
		membersSet := make(map[string]struct{})
		for _, member := range membersRepo {
			membersSet[member.ID] = struct{}{}
			members = append(members, &model.TeamMember{
				UserID:   member.ID,
				Username: member.Username,
				IsActive: member.IsActive,
			})
		}

		inactiveUsers := make(map[string]struct{}, len(users))
		for _, userID := range users {
			inactiveUsers[userID] = struct{}{}
			if _, exists := membersSet[userID]; !exists {
				l.Warn("user not found in the team", zap.String("team_name", team), zap.String("user_id", userID))
				return model.NewError(model.ErrorCodeNotFound, "user not found in the team")
			}
		}

		candidates := make(map[string]struct{})
		for _, member := range members {
			if !member.IsActive {
				inactiveUsers[member.UserID] = struct{}{}
				continue
			}
			if _, toDeactivate := inactiveUsers[member.UserID]; !toDeactivate {
				continue
			}
			candidates[member.UserID] = struct{}{}
		}

		assignments, err := p.prs.GetReviewAssignments(txCtx, users)
		if err != nil {
			l.Error("failed to get reviewer PRs", zap.Strings("user_ids", users), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get reviewer PRs", err)
		}

		prSet := make(map[string]struct{})
		newReviewers := make(map[string][]string) // prID -> []newReviewerIDs
		for _, a := range assignments {
			prID := a.PRID

			if _, exists := prSet[prID]; exists {
				continue
			}
			prSet[prID] = struct{}{}

			if a.Status == model.PRStatusMerged {
				continue
			}

			// Check how many reviewers need to be reassigned, they are inactive
			oldReviewers := a.UserIDs
			reassignCount := 0
			for _, r := range oldReviewers {
				if _, inactive := inactiveUsers[r]; inactive {
					reassignCount++
				}
			}

			selectedReviewers := p.selectReviewers(a.AuthorID, candidates, reassignCount)
			l.Debug("new reviewers for pr selected",
				zap.String("pull_request_id", prID),
				zap.Strings("old_reviewers", oldReviewers),
				zap.Strings("new_reviewers", selectedReviewers))

			newReviewers[prID] = selectedReviewers
		}

		isActive := false
		for _, userID := range users {
			_, err = p.users.Patch(txCtx, &repository.UserPatch{
				ID:       userID,
				IsActive: &isActive,
			})
			if err != nil {
				l.Error("failed to patch user", zap.String("user_id", userID), zap.Error(err))
				return model.NewError(model.ErrorCodeUnspecified, "failed to update user", err)
			}

			err = p.reviews.UnassignFromOpenPRs(txCtx, userID)
			if err != nil {
				l.Error("failed to unassign user from open PRs",
					zap.String("user_id", userID),
					zap.Error(err),
				)
				return model.NewError(model.ErrorCodeUnspecified, "failed to unassign user from open PRs", err)
			}
		}

		for prID, reviewers := range newReviewers {
			for _, reviewerID := range reviewers {
				if reviewerID == "" {
					continue
				}
				err = p.reviews.Assign(txCtx, prID, []string{reviewerID})
				if err != nil {
					l.Error("failed to assign new reviewer",
						zap.String("pull_request_id", prID),
						zap.String("new_reviewer", reviewerID),
						zap.Error(err),
					)
					return model.NewError(model.ErrorCodeUnspecified, "failed to assign new reviewer", err)
				}
			}
		}

		return nil
	})

	var res *model.Error
	errors.As(err, &res)

	return res
}

func (p *PullRequestService) GetUserReview(ctx context.Context, userID string) (*model.UserReviews, *model.Error) {
	l := logger.FromContext(ctx)
	l.Info("getting user reviews", zap.String("user_id", userID))

	prs := make([]*model.PullRequestShort, 0)

	repoPRs, err := p.prs.GetReviewedPRs(ctx, userID)
	if err != nil {
		l.Error("failed to get user review PRs", zap.String("user_id", userID), zap.Error(err))
		return nil, model.NewError(model.ErrorCodeUnspecified, "failed to get user reviews")
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

func (p *PullRequestService) ReassignPullRequest(ctx context.Context, prID, userID string) (*model.PullRequest, *model.Error) {
	l := logger.FromContext(ctx)
	l.Info("reassigning pull request", zap.String("pull_request_id", prID), zap.String("user_id", userID))

	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		repoTeam, err := p.users.GetUserTeam(txCtx, userID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			l.Warn("user or team not found", zap.String("user_id", userID))
			return model.NewError(model.ErrorCodeNotFound, "user or team not found")
		case err != nil:
			l.Error("failed to get user team", zap.String("user_id", userID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get user team")
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
			return model.NewError(model.ErrorCodeNotFound, "PR not found")
		case err != nil:
			l.Error("failed to get PR", zap.String("pull_request_id", prID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get PR")
		}

		if repoPR.Status == model.PRStatusMerged {
			l.Warn("cannot reassign merged PR", zap.String("pull_request_id", prID))
			return model.NewError(model.ErrorCodePRMerged, "cannot reassign on merged PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil {
			l.Error("failed to get reviewers", zap.String("pull_request_id", prID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get reviewers")
		}

		if !slices.Contains(reviewers, userID) {
			l.Warn("reviewer not assigned to PR", zap.String("pull_request_id", prID), zap.String("user_id", userID))
			return model.NewError(model.ErrorCodeNotAssigned, "reviewer is not assigned to this PR")
		}

		newReviewer := p.selectReplacementReviewer(repoPR.AuthorID, reviewers, team)
		if newReviewer == "" {
			l.Warn("no replacement candidate found", zap.String("pull_request_id", prID))
			return model.NewError(model.ErrorCodeNoCandidate, "no active replacement candidate in team")
		}

		if err = p.reviews.Unassign(txCtx, prID, userID); err != nil {
			l.Error("failed to unassign old reviewer", zap.String("pull_request_id", prID), zap.String("user_id", userID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to unassign old reviewer")
		}

		if err = p.reviews.Assign(txCtx, prID, []string{newReviewer}); err != nil {
			l.Error("failed to assign new reviewer", zap.String("pull_request_id", prID), zap.String("new_reviewer", newReviewer), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to assign new reviewer")
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

	var res *model.Error
	errors.As(err, &res)

	if res != nil {
		l.Error("reassign PR operation failed", zap.String("pull_request_id", prID), zap.Error(res))
	}

	return pr, res
}

func (p *PullRequestService) MergePullRequest(ctx context.Context, prID string) (*model.PullRequest, *model.Error) {
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
			return model.NewError(model.ErrorCodeNotFound, "PR not found")
		case err != nil:
			l.Error("failed to patch PR", zap.String("pull_request_id", prID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil {
			l.Error("failed to get reviewers", zap.String("pull_request_id", prID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get reviewers")
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

	var res *model.Error
	errors.As(err, &res)

	return pr, res
}

func (p *PullRequestService) GetStats(ctx context.Context) (*model.Stats, *model.Error) {
	l := logger.FromContext(ctx)
	l.Info("getting statistics")

	stats, err := p.prs.GetStats(ctx)
	if err != nil {
		l.Error("failed to get statistics", zap.Error(err))
		return nil, model.NewError(model.ErrorCodeUnspecified, "failed to get statistics")
	}

	l.Debug("statistics retrieved successfully")
	return stats, nil
}

// CreatePullRequest Create a new pull request and assign two team members as reviewers
func (p *PullRequestService) CreatePullRequest(ctx context.Context, short *model.PullRequestShort) (*model.PullRequest, *model.Error) {
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
			return model.NewError(model.ErrorCodeNotFound, "author or PR not found")
		case err != nil:
			l.Error("failed to get author team", zap.String("author_id", short.AuthorID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to get author team")
		}

		team := make([]*model.User, 0, len(repoTeam))
		for i := range repoTeam {
			if repoTeam[i].ID == short.AuthorID && !repoTeam[i].IsActive {
				l.Warn("inactive user cannot create PR", zap.String("author_id", short.AuthorID))
				return model.NewError(model.ErrorCodeUserInactive, "inactive user cannot create PR")
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
			return model.NewError(model.ErrorCodePRExists, "PR id already exists")
		case err != nil:
			l.Error("failed to create PR", zap.String("pull_request_id", short.ID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to create PR")
		}

		activeUsers := make(map[string]struct{})
		for _, member := range team {
			if member.IsActive {
				activeUsers[member.ID] = struct{}{}
			}
		}

		reviewers := p.selectReviewers(short.AuthorID, activeUsers, 2)

		err = p.reviews.Assign(txCtx, repoPR.ID, reviewers)
		if err != nil {
			l.Error("failed to assign reviewers", zap.String("pull_request_id", repoPR.ID), zap.Error(err))
			return model.NewError(model.ErrorCodeUnspecified, "failed to assign PR")
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
		pr.ID = repoPR.ID

		return nil
	})

	var res *model.Error
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

func (p *PullRequestService) selectReviewers(author string, candidates map[string]struct{}, max int) []string {
	reviewers := make([]string, 0, max)
	for candidate, _ := range candidates {
		if candidate == author {
			continue
		}

		reviewers = append(reviewers, candidate)

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
