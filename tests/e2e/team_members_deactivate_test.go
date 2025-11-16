package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	faker "github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
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

const (
	minWorkerDelay = 10 * time.Millisecond
	maxWorkerDelay = 100 * time.Millisecond
)

type metric struct {
	Count int64
	Total int64
}

var httpMetrics = struct {
	data map[string]*metric
}{
	data: make(map[string]*metric),
}

var sliSuccess = struct {
	data map[string]*metric
}{
	data: make(map[string]*metric),
}

func recordMetric(endpoint string, durationMs int64) {
	m, ok := httpMetrics.data[endpoint]
	if !ok {
		m = &metric{}
		httpMetrics.data[endpoint] = m
	}
	m.Count++
	m.Total += durationMs
}

func recordMetricSuccess(endpoint string, success bool) {
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
	t.Log("--- http metrics (avg ms) ---")
	for ep, m := range httpMetrics.data {
		if m.Count == 0 {
			continue
		}
		avg := float64(m.Total) / float64(m.Count)
		t.Logf("endpoint=%s count=%d avg=%.1fms", ep, m.Count, avg)
	}

	t.Log("--- SLI success rate ---")
	for ep, m := range sliSuccess.data {
		if m.Total == 0 {
			continue
		}
		successRate := (float64(m.Count) / float64(m.Total)) * 100.0
		t.Logf("endpoint=%s success=%d total=%d success_rate=%.2f%%", ep, m.Count, m.Total, successRate)
	}
}

type DeactivateMembersRequest struct {
	Team    string   `json:"team_name"`
	Members []string `json:"users"`
}

type ReassignPullRequestRequest struct {
	ID     string `json:"pull_request_id"`
	UserID string `json:"old_user_id"`
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

	const (
		numWorkers        = 5
		requestsPerWorker = 200
	)

	t.Run("deactivate members load test", func(t *testing.T) {
		t.Parallel()
		runDeactivateMembersLoadTest(
			t,
			baseUrl,
			token,
			allTeams,
			teamUsers,
			numWorkers,
			requestsPerWorker,
		)
	})

	t.Run("get team load test", func(t *testing.T) {
		t.Parallel()
		runGetTeamLoadTest(
			t,
			baseUrl,
			token,
			allTeams,
			numWorkers,
			requestsPerWorker,
		)
	})

	t.Run("get user review load test", func(t *testing.T) {
		t.Parallel()
		runGetUserReviewLoadTest(
			t,
			baseUrl,
			token,
			teamUsers,
			numWorkers,
			requestsPerWorker,
		)
	})

	t.Run("reassign pull request load test", func(t *testing.T) {
		t.Parallel()
		runReassignPRLoadTest(
			t,
			baseUrl,
			token,
			userPRs,
			numWorkers,
			requestsPerWorker,
		)
	})
}

func runReassignPRLoadTest(
	t *testing.T,
	baseURL string,
	token string,
	userPRs map[string][]string,
	numWorkers int,
	requestsPerWorker int,
) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	type prInfo struct {
		UserID string
		PRID   string
	}
	var allPRs []prInfo
	for userID, prs := range userPRs {
		for _, prID := range prs {
			allPRs = append(allPRs, prInfo{UserID: userID, PRID: prID})
		}
	}

	if len(allPRs) == 0 {
		t.Log("No pull requests to reassign, skipping test.")
		return
	}

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for i := 0; i < requestsPerWorker; i++ {
				prToReassign := allPRs[rnd.Intn(len(allPRs))]

				reqBody := ReassignPullRequestRequest{
					ID:     prToReassign.PRID,
					UserID: prToReassign.UserID,
				}

				doJSONRequest(
					t,
					http.MethodPost,
					baseURL+"/pullRequest/reassign",
					token,
					reqBody,
				)

				delay := minWorkerDelay + time.Duration(rnd.Int63n(int64(maxWorkerDelay-minWorkerDelay)))
				time.Sleep(delay)
			}
		}(w)
	}

	wg.Wait()
}

func runGetUserReviewLoadTest(
	t *testing.T,
	baseURL string,
	token string,
	teamUsers map[string][]string,
	numWorkers int,
	requestsPerWorker int,
) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	var allUsers []string
	for _, users := range teamUsers {
		allUsers = append(allUsers, users...)
	}

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for i := 0; i < requestsPerWorker; i++ {
				userID := allUsers[rnd.Intn(len(allUsers))]
				url := fmt.Sprintf("%s/users/getReview?user_id=%s", baseURL, userID)
				doJSONRequest(t, http.MethodGet, url, token, nil)

				delay := minWorkerDelay + time.Duration(rnd.Int63n(int64(maxWorkerDelay-minWorkerDelay)))
				time.Sleep(delay)
			}
		}(w)
	}

	wg.Wait()
}

func runGetTeamLoadTest(
	t *testing.T,
	baseURL string,
	token string,
	allTeams []string,
	numWorkers int,
	requestsPerWorker int,
) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for i := 0; i < requestsPerWorker; i++ {
				teamName := allTeams[rnd.Intn(len(allTeams))]
				url := fmt.Sprintf("%s/team/get?team_name=%s", baseURL, teamName)
				doJSONRequest(t, http.MethodGet, url, token, nil)

				delay := minWorkerDelay + time.Duration(rnd.Int63n(int64(maxWorkerDelay-minWorkerDelay)))
				time.Sleep(delay)
			}
		}(w)
	}

	wg.Wait()
}

func runDeactivateMembersLoadTest(
	t *testing.T,
	baseURL string,
	token string,
	allTeams []string,
	teamUsers map[string][]string,
	numWorkers int,
	requestsPerWorker int,
) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()

			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for i := 0; i < requestsPerWorker; i++ {
				teamIdx := rnd.Intn(len(allTeams))
				teamName := allTeams[teamIdx]

				users := teamUsers[teamName]
				if len(users) == 0 {
					continue
				}

				maxUsers := len(users)
				count := rnd.Intn(maxUsers/4) + 1

				tmp := make([]string, len(users))
				copy(tmp, users)
				rnd.Shuffle(len(tmp), func(i, j int) {
					tmp[i], tmp[j] = tmp[j], tmp[i]
				})

				selected := tmp[:count]

				reqBody := DeactivateMembersRequest{
					Team:    teamName,
					Members: selected,
				}

				doJSONRequest(
					t,
					http.MethodPost,
					baseURL+"/team/deactivateMembers",
					token,
					reqBody,
				)

				delay := minWorkerDelay + time.Duration(rnd.Int63n(int64(maxWorkerDelay-minWorkerDelay)))
				time.Sleep(delay)
			}
		}(w)
	}

	wg.Wait()
}

type teamMemberPayload struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type teamAddPayload struct {
	TeamName string              `json:"team_name"`
	Members  []teamMemberPayload `json:"members"`
}

type prCreatePayload struct {
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

		members := make([]teamMemberPayload, 0, teamSize)
		userIDs := make([]string, 0, teamSize)

		for j := 0; j < teamSize; j++ {
			userID := fmt.Sprintf("u-%d-%d", i+1, j+1)
			username := faker.Name()
			members = append(members, teamMemberPayload{
				UserID:   userID,
				Username: username,
				IsActive: true,
			})
			userIDs = append(userIDs, userID)
			userCount++
		}

		body := teamAddPayload{
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

				prBody := prCreatePayload{
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
	recordMetric(url, elapsed)

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
		recordMetricSuccess(url, false)
		return nil
	}
	recordMetricSuccess(url, resp.StatusCode < 400)
	t.Logf("%s request to %s, time: %d ms", method, url, time.Since(start).Milliseconds())
	return resp
}
