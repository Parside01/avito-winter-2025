package model

type Stats struct {
	TotalPRsCount    int                `json:"prs_count"`
	TotalAssignments int                `json:"assignments_count"`
	UserAssignments  []*UserAssignments `json:"user_assignments"`
	ReviewsPR        []*ReviewsPR       `json:"pr_reviews"`
}

type UserAssignments struct {
	UserID           string `json:"user_id"`
	Username         string `json:"username"`
	AssignmentsCount int    `json:"assignments_count"`
}

type ReviewsPR struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	ReviewersCount  int    `json:"reviewers_count"`
}
