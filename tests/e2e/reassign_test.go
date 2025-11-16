package e2e_test

import (
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

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
