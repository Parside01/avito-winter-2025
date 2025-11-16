package e2e_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

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
