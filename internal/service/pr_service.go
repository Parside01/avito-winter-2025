package service

import (
	"context"
	"errors"
	"github.com/yakoovad/avito-winter-2025/internal/db"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"github.com/yakoovad/avito-winter-2025/internal/repository"
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
	return &PullRequestService{tx: tx}
}

func (p *PullRequestService) GetUserReview(ctx context.Context, userID string) (*model.UserReviews, *Error) {
	prs := make([]*model.PullRequestShort, 0)

	repoPRs, err := p.prs.GetReviewPRs(ctx, userID)
	if err != nil {
		return nil, NewServiceError(ErrorCodeUnspecified, "failed to get user reviews")
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

	return res, nil
}

func (p *PullRequestService) ReassignPullRequest(ctx context.Context, prID, userID string) (*model.PullRequest, *Error) {
	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		repoTeam, err := p.users.GetUserTeam(txCtx, userID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return NewServiceError(ErrorCodeNotFound, "user or PR not found")
		case err != nil:
			return NewServiceError(ErrorCodeUnspecified, "failed to get user team")
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
			return NewServiceError(ErrorCodeNotFound, "PR not found")
		case err != nil:
			return NewServiceError(ErrorCodeUnspecified, "failed to get PR")
		}

		if repoPR.Status == model.PRStatusMerged {
			return NewServiceError(ErrorCodePRMerged, "cannot reassign on merged PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil { // Here not process ErrNotFound we have already checked above
			return NewServiceError(ErrorCodeUnspecified, "failed to get reviewers")
		}

		if !slices.Contains(reviewers, userID) {
			return NewServiceError(ErrorCodeNotAssigned, "reviewer is not assigned to this PR")
		}

		newReviewer := p.selectReplacementReviewer(repoPR.AuthorID, reviewers, team)
		if newReviewer == "" {
			return NewServiceError(ErrorCodeNoCandidate, "no active replacement candidate in team")
		}

		if err = p.reviews.Unassign(txCtx, prID, userID); err != nil {
			return NewServiceError(ErrorCodeUnspecified, "failed to unassign old reviewer")
		}

		if err = p.reviews.Assign(txCtx, prID, []string{newReviewer}); err != nil {
			return NewServiceError(ErrorCodeUnspecified, "failed to assign new reviewer")
		}

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

func (p *PullRequestService) MergePullRequest(ctx context.Context, prID string) (*model.PullRequest, *Error) {
	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		status := model.PRStatusMerged
		repoPR, err := p.prs.Patch(txCtx, &repository.PullRequestPatch{
			ID:     prID,
			Status: &status,
		})
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return NewServiceError(ErrorCodeNotFound, "PR not found")
		case err != nil:
			return NewServiceError(ErrorCodeUnspecified, "failed to get PR")
		}

		reviewers, err := p.prs.GetReviewers(txCtx, prID)
		if err != nil { // Here not process ErrNotFound we have already checked above
			return NewServiceError(ErrorCodeUnspecified, "failed to get reviewers")
		}

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
	pr := &model.PullRequest{}

	err := p.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		repoTeam, err := p.users.GetUserTeam(ctx, short.AuthorID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return NewServiceError(ErrorCodeNotFound, "author or PR not found")
		case err != nil:
			return NewServiceError(ErrorCodeUnspecified, "failed to get author team")
		}

		team := make([]*model.User, 0, len(repoTeam))
		for i := range repoTeam {
			// TODO: Возможно, вынести на уровень middleware.
			if repoTeam[i].ID == short.AuthorID && !repoTeam[i].IsActive {
				return NewServiceError(ErrorCodeUserInactive, "inactive user cannot create PR")
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
		switch { // Here not process ErrNotFound we have already checked above
		case errors.Is(err, repository.ErrAlreadyExists):
			return NewServiceError(ErrorCodePRExists, "PR id already exists")
		case err != nil:
			return NewServiceError(ErrorCodeUnspecified, "failed to create PR")
		}

		reviewers := p.selectReviewers(short.AuthorID, team, 2)

		err = p.reviews.Assign(txCtx, repoPR.ID, reviewers)
		if err != nil {
			return NewServiceError(ErrorCodeUnspecified, "failed to assign PR")
		}

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
