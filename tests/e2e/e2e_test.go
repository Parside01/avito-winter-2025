package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	faker "github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yakoovad/avito-winter-2025/internal/model"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
)

type metric struct {
	Count int64
	Total int64
}

var httpMetrics = struct {
	mu   sync.Mutex
	data map[string]*metric
}{
	data: make(map[string]*metric),
}

var sliSuccess = struct {
	mu   sync.Mutex
	data map[string]*metric
}{
	data: make(map[string]*metric),
}

func recordMetric(endpoint string, durationMs int64) {
	httpMetrics.mu.Lock()
	defer httpMetrics.mu.Unlock()
	m, ok := httpMetrics.data[endpoint]
	if !ok {
		m = &metric{}
		httpMetrics.data[endpoint] = m
	}
	m.Count++
	m.Total += durationMs
}

func recordMetricSuccess(endpoint string, success bool) {
	sliSuccess.mu.Lock()
	defer sliSuccess.mu.Unlock()

	m, ok := sliSuccess.data[endpoint]
	if !ok {
		m = &metric{}
		sliSuccess.data[endpoint] = m
	}
	if success {
		m.Count++
	}
	m.Total++
}
func printMetrics(t *testing.T) {
	t.Log("--- METRICS ---")
	for ep, m := range sliSuccess.data {
		if m.Total == 0 {
			continue
		}
		avg := float64(httpMetrics.data[ep].Total) / float64(httpMetrics.data[ep].Count)
		successRate := (float64(m.Count) / float64(m.Total)) * 100.0
		t.Logf("endpoint=%s success=%d total=%d success_rate=%.2f%%, avg_time=%.2fms", ep, m.Count, m.Total, successRate, avg)
	}
}

func TestEndToEnd(t *testing.T) {
	t.Helper()

	_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx := context.Background()

	composePath := filepath.Join("../../", "docker-compose.yml")

	stack, err := compose.NewDockerComposeWith(
		compose.WithStackFiles(composePath),
		compose.WithLogger(log.Default()),
		compose.StackIdentifier("load_test_team_deactivate"),
	)

	if err != nil {
		t.Fatalf("failed to create compose stack: %v", err)
	}

	t.Cleanup(func() {
		printMetrics(t)

		err = stack.Down(
			context.Background(),
			compose.RemoveOrphans(true),
			compose.RemoveVolumes(true),
			compose.RemoveImagesLocal,
		)
		if err != nil {
			t.Fatalf("failed to tear down compose stack: %v", err)
		}
	})

	stack.WaitForService("postgres", wait.ForHealthCheck())
	stack.WaitForService("app", wait.ForHealthCheck())

	if err = stack.Up(ctx); err != nil {
		t.Fatalf("failed to start compose stack: %v", err)
	}

	app, err := stack.ServiceContainer(ctx, "app")
	assert.NoError(t, err)
	assert.NotNil(t, app)

	adminToken := generateToken(t, ctx, app, "admin")
	userToken := generateToken(t, ctx, app, "user")

	defer func() {
		goLogs, _ := app.Logs(ctx)
		assert.NoError(t, err)
		var logBuf bytes.Buffer
		_, _ = logBuf.ReadFrom(goLogs)
		t.Logf("app logs:\n%s", logBuf.String())
	}()

	port, err := app.MappedPort(ctx, "8080/tcp")
	assert.NoError(t, err)
	assert.NotEmpty(t, port)

	baseUrl := fmt.Sprintf("http://localhost:%s", port.Port())

	allTeams, teamUsers, allPRs := generateTestData(t, baseUrl, adminToken)
	assert.NotEmpty(t, allTeams)
	assert.NotEmpty(t, teamUsers)
	assert.NotEmpty(t, allPRs)

	t.Run("e2e test", func(t *testing.T) {
		t.Parallel()
		for _, teamName := range allTeams {
			t.Run(fmt.Sprintf("e2e for team %s", teamName), func(t *testing.T) {
				runForTeam(t, baseUrl, adminToken, userToken, teamName, allPRs)
			})
		}
	})
}

func generateToken(t *testing.T, ctx context.Context, app testcontainers.Container, tokenType string) string {
	_, reader, err := app.Exec(ctx, []string{"./main", "generate-api-key", "--type", tokenType, "--only-token", "true"})
	assert.NoError(t, err)

	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	assert.NoError(t, err)

	token := buf.String()
	token = strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return -1
		}
		if r < ' ' {
			return -1
		}
		return r
	}, token)
	return token
}

type TeamAddRequest struct {
	TeamName string              `json:"team_name"`
	Members  []*model.TeamMember `json:"members"`
}

type PRCreateRequest struct {
	PRID   string `json:"pull_request_id"`
	Name   string `json:"pull_request_name"`
	Author string `json:"author_id"`
}

func generateTestData(t *testing.T, baseURL string, token string) (allTeams []string, teamUsers map[string][]string, all []*model.PullRequest) {
	t.Helper()

	teamUsers = make(map[string][]string)
	all = make([]*model.PullRequest, 0)

	numTeams := 20
	userCount := 0

	for i := 0; i < numTeams; i++ {
		teamName := fmt.Sprintf("team-%d", i+1)

		teamSize := rand.Intn(15) + 10

		members := make([]*model.TeamMember, 0, teamSize)
		userIDs := make([]string, 0, teamSize)

		for j := 0; j < teamSize; j++ {
			userID := fmt.Sprintf("u-%d-%d", i+1, j+1)
			username := faker.Name()
			members = append(members, &model.TeamMember{
				UserID:   userID,
				Username: username,
				IsActive: true,
			})
			userIDs = append(userIDs, userID)
			userCount++
		}

		body := TeamAddRequest{
			TeamName: teamName,
			Members:  members,
		}

		resp := doJSONRequest(t,
			http.MethodPost,
			baseURL+"/team/add",
			token,
			body,
		)
		_ = resp.Body.Close()

		allTeams = append(allTeams, teamName)
		teamUsers[teamName] = userIDs
	}

	for team, users := range teamUsers {
		for _, userID := range users {
			count := rand.Intn(19) + 1

			for k := 0; k < count; k++ {
				prID := fmt.Sprintf("pr-%s-%s-%d", team, userID, k+1)
				prName := faker.Sentence()

				body := PRCreateRequest{
					PRID:   prID,
					Name:   prName,
					Author: userID,
				}

				resp := doJSONRequest(t,
					http.MethodPost,
					baseURL+"/pullRequest/create",
					token,
					body,
				)
				if resp.StatusCode == http.StatusCreated {
					var created struct {
						PR *model.PullRequest `json:"pr"`
					}
					err := json.NewDecoder(resp.Body).Decode(&created)
					_ = resp.Body.Close()
					assert.NoError(t, err)
					assert.NotNil(t, created)
					assert.NotNil(t, created.PR)

					all = append(all, created.PR)
				}
				_ = resp.Body.Close()
			}
		}
	}

	return allTeams, teamUsers, all
}

func doJSONRequest(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", token)

	start := time.Now()

	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	recordMetric(req.URL.Path, elapsed)

	if err != nil {
		recordMetricSuccess(url, false)
		return nil
	}

	if resp.StatusCode >= 400 {
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)

		recordMetricSuccess(req.URL.Path, false)
		return resp
	}
	recordMetricSuccess(req.URL.Path, resp.StatusCode < 400)
	return resp
}

func runForTeam(t *testing.T, baseUrl, adminToken, userToken, teamName string, allPRs []*model.PullRequest) {
	t.Run("TestGetNonExistentTeam", func(t *testing.T) {
		testGetNonExistentTeam(t, baseUrl, adminToken)
	})
	var team []string
	t.Run("TestGetExistingTeam", func(t *testing.T) {
		team = testGetExistingTeam(t, baseUrl, adminToken, userToken, teamName)
	})
	var reviewers map[string][]*model.PullRequestShort
	t.Run("TestGetAllUserReviews", func(t *testing.T) {
		reviewers = testGetAllUserReviews(t, baseUrl, adminToken, userToken, team)
	})
	opened := make([]*model.PullRequestShort, 0)
	merged := make([]*model.PullRequestShort, 0)
	t.Run("TestMergePRs", func(t *testing.T) {
		opened, merged = testMergePRs(t, baseUrl, adminToken, userToken, reviewers)
	})
	t.Run("TestReassignReviewer", func(t *testing.T) {
		testReassignReviewer(t, baseUrl, adminToken, userToken, opened, merged, allPRs)
	})
	t.Run("TestDeactivateReviewersAndReassign", func(t *testing.T) {
		testDeactivateReviewersAndReassign(t, baseUrl, adminToken, opened, allPRs)
	})
	t.Run("TestDeactivateTeamMembers", func(t *testing.T) {
		testDeactivateTeamMembers(t, baseUrl, adminToken, teamName)
	})
}

func testGetNonExistentTeam(t *testing.T, baseUrl, adminToken string) {
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name=non-existent-team", adminToken, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()
}

func testGetExistingTeam(t *testing.T, baseUrl, adminToken, userToken, teamName string) []string {
	var teamUsers []string
	var teamResp *model.Team
	// Admin token success
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, adminToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.NoError(t, json.Unmarshal(bodyBytes, &teamResp))

	for _, member := range teamResp.Members {
		teamUsers = append(teamUsers, member.UserID)
	}
	assert.NotEmpty(t, teamUsers, "team should have users")

	// User token success
	resp = doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, userToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	return teamUsers
}

func testGetAllUserReviews(t *testing.T, baseURL, adminToken, userToken string, teamUsers []string) map[string][]*model.PullRequestShort {
	allReviews := make(map[string][]*model.PullRequestShort)
	for _, userID := range teamUsers {
		var userReview struct {
			UserID       string                    `json:"user_id"`
			PullRequests []*model.PullRequestShort `json:"pull_requests"`
		}
		// Admin token success
		url := fmt.Sprintf("%s/users/getReview?user_id=%s", baseURL, userID)
		resp := doJSONRequest(t, http.MethodGet, url, adminToken, nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.NoError(t, json.Unmarshal(bodyBytes, &userReview))
		allReviews[userID] = userReview.PullRequests

		// User token success
		resp = doJSONRequest(t, http.MethodGet, url, userToken, nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}
	return allReviews
}

func testMergePRs(t *testing.T, baseUrl, adminToken, userToken string, allReviews map[string][]*model.PullRequestShort) ([]*model.PullRequestShort, []*model.PullRequestShort) {
	var opened []*model.PullRequestShort
	var merged []*model.PullRequestShort
	for _, reviews := range allReviews {
		for _, pr := range reviews {
			switch pr.Status {
			case model.PRStatusOpen:
				opened = append(opened, pr)
			case model.PRStatusMerged:
				merged = append(merged, pr)
			}
		}
	}

	if len(opened) == 0 {
		t.Skip("no open PRs to merge")
	}

	prsToMerge := opened[:len(opened)/2]
	for _, pr := range prsToMerge {
		mergePayload := map[string]string{"pull_request_id": pr.ID}

		// User token fail
		resp := doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/merge", userToken, mergePayload)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		_ = resp.Body.Close()

		// Admin token succeed
		resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/merge", adminToken, mergePayload)
		// In this case team can be small or all reviewers are deactivated
		assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, resp.StatusCode)
		_ = resp.Body.Close()
		merged = append(merged, pr)
	}

	opened = opened[len(opened)/2:]
	return opened, merged
}

func testReassignReviewer(t *testing.T, baseUrl, adminToken, userToken string, opened []*model.PullRequestShort, merged []*model.PullRequestShort, all []*model.PullRequest) {
	if len(opened) == 0 || len(all) == 0 {
		t.Skip("no open PRs to reassign")
	}

	var prToReassign *model.PullRequest
	for _, pr := range all {
		if pr.Status == "MERGED" {
			continue
		}
		if len(pr.Reviewers) < 2 {
			continue
		}
		prToReassign = pr
		break
	}

	if prToReassign == nil {
		t.Skip()
	}

	if len(prToReassign.Reviewers) == 0 {
		t.Skip("no reviewers to reassign")
	}

	for _, reviewer := range prToReassign.Reviewers {
		if reviewer == prToReassign.AuthorID {
			continue
		}
	}

	oldReviewer := prToReassign.Reviewers[0]
	reassignPayload := map[string]string{"pull_request_id": prToReassign.ID, "old_user_id": oldReviewer}

	// User token should fail
	resp := doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", userToken, reassignPayload)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()

	// Admin token should succeed
	resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignPayload)
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, resp.StatusCode)
	_ = resp.Body.Close()

	if len(merged) == 0 {
		t.Skip()
	}

	mergedPRShort := merged[0]

	var mergedPR *model.PullRequest
	for _, pr := range all {
		if pr.ID == mergedPRShort.ID {
			mergedPR = pr
			break
		}
	}
	if mergedPR == nil {
		t.Skip()
	}

	if len(mergedPR.Reviewers) > 0 {
		reassignRequest := map[string]string{
			"pull_request_id": mergedPR.ID,
			"old_user_id":     mergedPR.Reviewers[0],
		}
		resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignRequest)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		_ = resp.Body.Close()
	}
}

func testDeactivateReviewersAndReassign(t *testing.T, baseUrl, adminToken string, opened []*model.PullRequestShort, all []*model.PullRequest) {
	var pullRequest *model.PullRequest
	found := false
	for _, prShort := range opened {
		var pr *model.PullRequest
		for _, p := range all {
			if p.ID == prShort.ID {
				pr = p
				break
			}
		}

		if pr != nil && len(pr.Reviewers) > 1 {
			pullRequest = pr
			found = true
			break
		}
	}

	if !found {
		t.Skip()
	}
	for _, reviewer := range pullRequest.Reviewers {
		req := map[string]interface{}{"user_id": reviewer, "is_active": false}
		resp := doJSONRequest(t, http.MethodPost, baseUrl+"/users/setIsActive", adminToken, req)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}

	reassignRequest := map[string]string{"pull_request_id": pullRequest.ID, "old_user_id": pullRequest.Reviewers[0]}
	resp := doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignRequest)

	// In this case pr can be merged and user is inactive
	assert.Contains(t, []int{http.StatusNotFound, http.StatusConflict}, resp.StatusCode)
	_ = resp.Body.Close()
}

func testDeactivateTeamMembers(t *testing.T, baseUrl, adminToken, teamName string) {
	var activeUsers []string
	teamResp := &model.Team{}
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, adminToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	err := json.NewDecoder(resp.Body).Decode(teamResp)
	_ = resp.Body.Close()
	assert.NoError(t, err)

	for _, member := range teamResp.Members {
		if member.IsActive {
			activeUsers = append(activeUsers, member.UserID)
		}
	}

	if len(activeUsers) > 0 {
		usersToDeactivate := activeUsers[:int(math.Ceil(float64(len(activeUsers))/2.0))]
		if len(usersToDeactivate) > 0 {
			deactivatePayload := map[string]interface{}{"team_name": teamName, "users": usersToDeactivate}
			deactivateResponse := doJSONRequest(t, http.MethodPost, baseUrl+"/team/deactivateMembers", adminToken, deactivatePayload)
			assert.Equal(t, http.StatusOK, deactivateResponse.StatusCode)
			_ = deactivateResponse.Body.Close()
		}
	}
}
