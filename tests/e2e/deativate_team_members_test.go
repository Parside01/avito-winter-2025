package e2e_test

import (
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

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
