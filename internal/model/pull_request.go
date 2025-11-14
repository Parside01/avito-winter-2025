package model

import "time"

type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

type PullRequest struct {
	ID        string     `json:"pull_request_id" validate:"required"`
	Name      string     `json:"pull_request_name" validate:"required"`
	AuthorID  string     `json:"author_id" validate:"required"`
	Status    PRStatus   `json:"status" validate:"required"`
	Reviewers []string   `json:"assigned_reviewers" validate:"required"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	MergedAt  *time.Time `json:"mergedAt,omitempty"`
}

type PullRequestShort struct {
	ID       string   `json:"pull_request_id" validate:"required"`
	Name     string   `json:"pull_request_name" validate:"required"`
	AuthorID string   `json:"author_id" validate:"required"`
	Status   PRStatus `json:"status" validate:"required"`
}
