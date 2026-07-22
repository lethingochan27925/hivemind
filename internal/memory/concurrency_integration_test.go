//go:build integration

package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// TestFleetConcurrency_NoTaskClaimedTwice mo phong dung kich ban fleet that:
// N worker chay song song, M task trong hang doi, xac nhan khong co task nao
// bi claim boi 2 worker cung luc - day la bang chung ky thuat cho SKIP LOCKED.
func TestFleetConcurrency_NoTaskClaimedTwice(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	const numTasks = 50
	const numWorkers = 20

	taskIDs := make([]string, numTasks)
	txnIDs := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		txnID := insertTestTransaction(t, db)
		taskID := insertTestTask(t, db, txnID)
		taskIDs[i] = taskID
		txnIDs[i] = txnID
	}
	defer func() {
		for i := 0; i < numTasks; i++ {
			cleanupTestData(t, db, taskIDs[i], txnIDs[i])
		}
	}()

	claimedBy := make(map[string][]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	claimsPerWorker := (numTasks / numWorkers) + 2

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			workerID := uuid.New().String()

			for c := 0; c < claimsPerWorker; c++ {
				task, err := ClaimNextTask(ctx, db, workerID)
				if err != nil {
					t.Errorf("worker %d claim error: %v", workerNum, err)
					return
				}
				if task == nil {
					return
				}

				isOurTask := false
				for _, id := range taskIDs {
					if id == task.ID {
						isOurTask = true
						break
					}
				}
				if !isOurTask {
					continue
				}

				mu.Lock()
				claimedBy[task.ID] = append(claimedBy[task.ID], workerID)
				mu.Unlock()
			}
		}(w)
	}

	wg.Wait()

	doubleClaimCount := 0
	for taskID, workers := range claimedBy {
		if len(workers) > 1 {
			doubleClaimCount++
			t.Errorf("task %s claimed by %d workers: %v", taskID, len(workers), workers)
		}
	}

	if doubleClaimCount > 0 {
		t.Fatalf("SKIP LOCKED failed: %d task(s) claimed by more than one worker", doubleClaimCount)
	}

	if len(claimedBy) == 0 {
		t.Fatal("no tasks were claimed at all - test setup issue")
	}

	t.Logf("Fleet concurrency test passed: %d/%d tasks claimed, 0 double-claims, %d workers competing",
		len(claimedBy), numTasks, numWorkers)
}
