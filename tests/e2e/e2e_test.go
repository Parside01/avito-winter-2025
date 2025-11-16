package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, reader, err := app.Exec(ctx, []string{"./main", "generate-api-key", "--type", "admin", "--only-token", "true"})
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

	assert.NotEmpty(t, token)

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

	allTeams, teamUsers, userPRs := prepareTestData(t, baseUrl, token)
	assert.NotEmpty(t, allTeams)
	assert.NotEmpty(t, teamUsers)
	assert.NotEmpty(t, userPRs)

	t.Run("e2e test", func(t *testing.T) {
		t.Parallel()
		for _, teamName := range allTeams {
			t.Run(fmt.Sprintf("e2e for team %s", teamName), func(t *testing.T) {
				t.Parallel()
				runForTeam(t, baseUrl, token, token, teamName)
			})
		}
	})
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

func prepareTestData(t *testing.T, baseURL string, token string) (allTeams []string, teamUsers map[string][]string, userPRs map[string][]string) {
	t.Helper()

	teamUsers = make(map[string][]string)
	userPRs = make(map[string][]string)

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

		doJSONRequest(t,
			http.MethodPost,
			baseURL+"/team/add",
			token,
			body,
		)

		allTeams = append(allTeams, teamName)
		teamUsers[teamName] = userIDs
	}

	for teamName, users := range teamUsers {
		for _, userID := range users {
			prCount := rand.Intn(19) + 1

			for k := 0; k < prCount; k++ {
				prID := fmt.Sprintf("pr-%s-%s-%d", teamName, userID, k+1)
				prName := faker.Sentence()

				prBody := PRCreateRequest{
					PRID:   prID,
					Name:   prName,
					Author: userID,
				}

				doJSONRequest(t,
					http.MethodPost,
					baseURL+"/pullRequest/create",
					token,
					prBody,
				)
				userPRs[userID] = append(userPRs[userID], prID)
			}
		}
	}

	return allTeams, teamUsers, userPRs
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
		t.Errorf("request failed: %v", err)
		recordMetricSuccess(url, false)
		return nil
	}

	if resp.StatusCode >= 400 {
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			t.Logf("request to %s finished with conflict: %s, body: %s", url, resp.Status, b.String())
		} else {
			t.Errorf("request to %s failed: %s, body: %s", url, resp.Status, b.String())
		}
		recordMetricSuccess(req.URL.Path, false)
		return resp
	}
	recordMetricSuccess(req.URL.Path, resp.StatusCode < 400)
	return resp
}

func runForTeam(t *testing.T, baseUrl, adminToken, userToken, teamName string) {
	t.Run("TestGetNonExistentTeam", func(t *testing.T) {
		testGetNonExistentTeam(t, baseUrl, adminToken)
	})
	var teamUsers []string
	t.Run("TestGetExistingTeam", func(t *testing.T) {
		teamUsers = testGetExistingTeam(t, baseUrl, adminToken, userToken, teamName)
	})
	var allReviews map[string][]*model.PullRequestShort
	t.Run("TestGetAllUserReviews", func(t *testing.T) {
		allReviews = testGetAllUserReviews(t, baseUrl, adminToken, userToken, teamUsers)
	})
	var openPRs []*model.PullRequestShort
	var mergedPRs []*model.PullRequestShort
	t.Run("TestMergePRs", func(t *testing.T) {
		openPRs, mergedPRs = testMergePRs(t, baseUrl, adminToken, userToken, allReviews)
	})
	t.Run("TestReassignReviewer", func(t *testing.T) {
		testReassignReviewer(t, baseUrl, adminToken, userToken, openPRs, mergedPRs)
	})
	t.Run("TestDeactivateReviewersAndReassign", func(t *testing.T) {
		testDeactivateReviewersAndReassign(t, baseUrl, adminToken, openPRs)
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
	// Admin token
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, adminToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, json.Unmarshal(bodyBytes, &teamResp))

	for _, member := range teamResp.Members {
		teamUsers = append(teamUsers, member.UserID)
	}
	require.NotEmpty(t, teamUsers, "team should have users")

	// User token
	resp = doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, userToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	return teamUsers
}

func testGetAllUserReviews(t *testing.T, baseUrl, adminToken, userToken string, teamUsers []string) map[string][]*model.PullRequestShort {
	allReviews := make(map[string][]*model.PullRequestShort)
	for _, userID := range teamUsers {
		var userReview struct {
			UserID       string                    `json:"user_id"`
			PullRequests []*model.PullRequestShort `json:"pull_requests"`
		}
		// Admin token
		url := fmt.Sprintf("%s/users/getReview?user_id=%s", baseUrl, userID)
		resp := doJSONRequest(t, http.MethodGet, url, adminToken, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, json.Unmarshal(bodyBytes, &userReview))
		allReviews[userID] = userReview.PullRequests

		// User token
		resp = doJSONRequest(t, http.MethodGet, url, userToken, nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}
	return allReviews
}

func testMergePRs(t *testing.T, baseUrl, adminToken, userToken string, allReviews map[string][]*model.PullRequestShort) ([]*model.PullRequestShort, []*model.PullRequestShort) {
	var openPRs []*model.PullRequestShort
	var mergedPRs []*model.PullRequestShort
	for _, reviews := range allReviews {
		for _, pr := range reviews {
			if pr.Status == "OPEN" {
				openPRs = append(openPRs, pr)
			} else if pr.Status == "MERGED" {
				mergedPRs = append(mergedPRs, pr)
			}
		}
	}

	if len(openPRs) == 0 {
		t.Skip("no open PRs to merge")
	}
	prsToMerge := openPRs[:len(openPRs)/2]
	for _, pr := range prsToMerge {
		mergePayload := map[string]string{"pull_request_id": pr.ID}

		// User token should fail
		resp := doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/merge", userToken, mergePayload)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		_ = resp.Body.Close()

		// Admin token should succeed
		resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/merge", adminToken, mergePayload)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
		mergedPRs = append(mergedPRs, pr)
	}
	// update openPRs
	openPRs = openPRs[len(openPRs)/2:]
	return openPRs, mergedPRs
}

func testReassignReviewer(t *testing.T, baseUrl, adminToken, userToken string, openPRs []*model.PullRequestShort, mergedPRs []*model.PullRequestShort) {
	if len(openPRs) == 0 {
		t.Skip("no open PRs with reviewers to reassign")
	}

	var prToReassign *model.PullRequest
	// get full PR to have reviewers
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/pullRequest/get?id="+openPRs[0].ID, adminToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	err := json.NewDecoder(resp.Body).Decode(&prToReassign)
	_ = resp.Body.Close()
	require.NoError(t, err)

	if len(prToReassign.Reviewers) == 0 {
		t.Skip("no reviewers to reassign")
	}

	oldReviewer := prToReassign.Reviewers[0]
	reassignPayload := map[string]string{"pull_request_id": prToReassign.ID, "old_user_id": oldReviewer}

	// User token should fail
	resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", userToken, reassignPayload)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()

	// Admin token should succeed
	resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignPayload)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	if len(mergedPRs) > 0 {
		mergedPR := mergedPRs[0]

		var fullMergedPR *model.PullRequest
		resp = doJSONRequest(t, http.MethodGet, baseUrl+"/pullRequest/get?id="+mergedPR.ID, adminToken, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		err = json.NewDecoder(resp.Body).Decode(&fullMergedPR)
		_ = resp.Body.Close()
		require.NoError(t, err)

		if len(fullMergedPR.Reviewers) > 0 {
			reassignMergedPayload := map[string]string{"pull_request_id": fullMergedPR.ID, "old_user_id": fullMergedPR.Reviewers[0]}
			resp = doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignMergedPayload)
			assert.Equal(t, http.StatusConflict, resp.StatusCode)
			var errResp *model.Error
			err = json.NewDecoder(resp.Body).Decode(&errResp)
			require.NoError(t, err)
			assert.Equal(t, model.ErrorCodePRMerged, errResp.Code)
			_ = resp.Body.Close()
		}
	}
}

func testDeactivateReviewersAndReassign(t *testing.T, baseUrl, adminToken string, openPRs []*model.PullRequestShort) {
	var prWithMultipleReviewers *model.PullRequest
	found := false
	for _, pr := range openPRs {
		var fullPR *model.PullRequest
		resp := doJSONRequest(t, http.MethodGet, baseUrl+"/pullRequest/get?id="+pr.ID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			continue
		}
		err := json.NewDecoder(resp.Body).Decode(&fullPR)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}

		if len(fullPR.Reviewers) > 1 {
			prWithMultipleReviewers = fullPR
			found = true
			break
		}
	}

	if found {
		for _, reviewerID := range prWithMultipleReviewers.Reviewers {
			payload := map[string]interface{}{"user_id": reviewerID, "is_active": false}
			resp := doJSONRequest(t, http.MethodPost, baseUrl+"/users/setIsActive", adminToken, payload)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			_ = resp.Body.Close()
		}

		reassignPayload := map[string]string{"pull_request_id": prWithMultipleReviewers.ID, "old_user_id": prWithMultipleReviewers.Reviewers[0]}
		resp := doJSONRequest(t, http.MethodPost, baseUrl+"/pullRequest/reassign", adminToken, reassignPayload)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		var errResp *model.Error
		err := json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		assert.Equal(t, model.ErrorCodeNoCandidate, errResp.Code)
		_ = resp.Body.Close()
	}
}

func testDeactivateTeamMembers(t *testing.T, baseUrl, adminToken, teamName string) {
	var activeUsers []string
	teamResp := &model.Team{}
	resp := doJSONRequest(t, http.MethodGet, baseUrl+"/team/get?team_name="+teamName, adminToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	err := json.NewDecoder(resp.Body).Decode(teamResp)
	_ = resp.Body.Close()
	require.NoError(t, err)

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
