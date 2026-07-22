//go:build integration

package memory

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

func setupTestDB(t *testing.T) *cockroach.Client {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	db, err := cockroach.NewClient(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	return db
}

func insertTestTransaction(t *testing.T, db *cockroach.Client) string {
	ctx := context.Background()
	txnID := uuid.New().String()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO transactions (
			id, step, type, amount,
			name_orig, old_balance_orig, new_balance_orig,
			name_dest, old_balance_dest, new_balance_dest,
			error_balance_orig, error_balance_dest,
			risk_score, risk_tier, is_fraud_label, arrived_at
		) VALUES ($1, 1, 'TRANSFER', 100, 'TEST_ORIG', 100, 0, 'TEST_DEST', 0, 0, 0, 100, 0.5, 'medium', false, now())
	`, txnID)
	if err != nil {
		t.Fatalf("inserting test transaction: %v", err)
	}
	return txnID
}

func insertTestTask(t *testing.T, db *cockroach.Client, txnID string) string {
	ctx := context.Background()
	taskID := uuid.New().String()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO tasks (id, transaction_id, risk_score, status, created_at)
		VALUES ($1, $2, 0.5, 'pending', now())
	`, taskID, txnID)
	if err != nil {
		t.Fatalf("inserting test task: %v", err)
	}
	return taskID
}

func cleanupTestData(t *testing.T, db *cockroach.Client, taskID, txnID string) {
	ctx := context.Background()
	db.Pool.Exec(ctx, `DELETE FROM audit_log WHERE task_id = $1`, taskID)
	db.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	db.Pool.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, txnID)
}

// claimSpecificTask lap qua ClaimNextTask nhieu lan cho toi khi lay dung
// task can test, vi DB co the co san nhieu task pending khac tu truoc.
// An toan vi moi lan claim se doi status task do sang investigating,
// khong lam nhieu them lan sau.
func claimSpecificTask(t *testing.T, db *cockroach.Client, workerID, wantTaskID string, maxAttempts int) *Task {
	ctx := context.Background()
	for i := 0; i < maxAttempts; i++ {
		task, err := ClaimNextTask(ctx, db, workerID)
		if err != nil {
			t.Fatalf("ClaimNextTask failed: %v", err)
		}
		if task == nil {
			return nil
		}
		if task.ID == wantTaskID {
			return task
		}
		// Task khac, không phai cai minh can - danh dau lai la done de khong
		// vuong lan sau (day la task rac tu truoc, khong anh huong test that).
		CompleteTask(ctx, db, task.ID, "done", "legit", "cleared_by_test", 1.0)
	}
	return nil
}

func TestClaimNextTask_SingleClaim(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	txnID := insertTestTransaction(t, db)
	taskID := insertTestTask(t, db, txnID)
	defer cleanupTestData(t, db, taskID, txnID)

	task := claimSpecificTask(t, db, "test-worker", taskID, 200)
	if task == nil {
		t.Fatal("expected to claim the test task, but did not find it among pending tasks")
	}

	ctx := context.Background()
	var status string
	var claimedBy *string
	err := db.Pool.QueryRow(ctx, `SELECT status, claimed_by FROM tasks WHERE id = $1`, taskID).Scan(&status, &claimedBy)
	if err != nil {
		t.Fatalf("querying task status: %v", err)
	}
	if status != "investigating" {
		t.Errorf("status after claim = %q, want %q", status, "investigating")
	}
	if claimedBy == nil || *claimedBy != "test-worker" {
		t.Errorf("claimed_by = %v, want %q", claimedBy, "test-worker")
	}
}

func TestClaimNextTask_NoConcurrentDoubleClaim(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	txnID := insertTestTransaction(t, db)
	taskID := insertTestTask(t, db, txnID)
	defer cleanupTestData(t, db, taskID, txnID)

	// Xoa het task pending khac de dam bao test nay chi con dung 1 task
	// muc tieu trong hang doi, tranh nhieu tu du lieu cu trong DB.
	_, err := db.Pool.Exec(ctx, `
		UPDATE tasks SET status = 'done'
		WHERE status = 'pending' AND id != $1
	`, taskID)
	if err != nil {
		t.Fatalf("clearing other pending tasks: %v", err)
	}

	const numWorkers = 10
	claimed := make(chan string, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			workerID := uuid.New().String()
			task, err := ClaimNextTask(ctx, db, workerID)
			if err != nil || task == nil {
				claimed <- ""
				return
			}
			claimed <- task.ID
		}()
	}

	successCount := 0
	for i := 0; i < numWorkers; i++ {
		id := <-claimed
		if id == taskID {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 worker to claim the task, got %d successful claims", successCount)
	}
}

func TestCompleteTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	txnID := insertTestTransaction(t, db)
	taskID := insertTestTask(t, db, txnID)
	defer cleanupTestData(t, db, taskID, txnID)

	task := claimSpecificTask(t, db, "test-worker", taskID, 200)
	if task == nil {
		t.Fatal("expected to claim the test task, but did not find it among pending tasks")
	}

	ctx := context.Background()
	err := CompleteTask(ctx, db, taskID, "done", "legit", "reasoned", 0.9)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	var status, verdict string
	var confidence float64
	err = db.Pool.QueryRow(ctx, `SELECT status, verdict, confidence FROM tasks WHERE id = $1`, taskID).Scan(&status, &verdict, &confidence)
	if err != nil {
		t.Fatalf("querying completed task: %v", err)
	}
	if status != "done" {
		t.Errorf("status = %q, want %q", status, "done")
	}
	if verdict != "legit" {
		t.Errorf("verdict = %q, want %q", verdict, "legit")
	}
	if confidence != 0.9 {
		t.Errorf("confidence = %v, want %v", confidence, 0.9)
	}
}
