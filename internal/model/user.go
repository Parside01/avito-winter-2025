package model

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
	TeamName string `json:"team_name"`
}

type UserReviews struct {
	UserID       string              `json:"user_id"`
	PullRequests []*PullRequestShort `json:"pull_requests"`
}
