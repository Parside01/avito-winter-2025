package load_test

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

type metric struct {
	Count int64
	Total int64 // суммарная задержка в мс
}

var httpMetrics = struct {
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

func printMetrics(t *testing.T) {
	t.Log("--- http metrics (avg ms) ---")
	for ep, m := range httpMetrics.data {
		if m.Count == 0 {
			continue
		}
		avg := float64(m.Total) / float64(m.Count)
		t.Logf("endpoint=%s count=%d avg=%.1fms", ep, m.Count, avg)
	}
}

type DeactivateMembersRequest struct {
	Team    string   `json:"team_name"`
	Members []string `json:"users"`
}

func TestLoad_TeamDeactivateMembers(t *testing.T) {
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

	allTeams, teamUsers := prepareTestData(t, "http://localhost:8080", token)
	assert.NotEmpty(t, allTeams)
	assert.NotEmpty(t, teamUsers)

	t.Run("deactivate members load test", func(t *testing.T) {
		runDeactivateMembersLoadTest(
			t,
			"http://localhost:8080",
			token,
			allTeams,
			teamUsers,
			20,
			200,
		)
	})
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
				count := rnd.Intn(maxUsers) + 1

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

func prepareTestData(t *testing.T, baseURL string, token string) (allTeams []string, teamUsers map[string][]string) {
	t.Helper()

	teamUsers = make(map[string][]string)

	numTeams := 70
	userCount := 0

	for i := 0; i < numTeams; i++ {
		teamName := fmt.Sprintf("team-%d", i+1)

		teamSize := rand.Intn(15) + 5

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
			}
		}
	}

	return allTeams, teamUsers
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
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode >= 400 {
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("request to %s failed: %s, body: %s", url, resp.Status, b.String())
	}
	t.Logf("Success %s request to %s, time: %d ms", method, url, time.Since(start).Milliseconds())
	return resp
}
