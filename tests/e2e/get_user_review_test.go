package e2e_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

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
