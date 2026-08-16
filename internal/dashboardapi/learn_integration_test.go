//go:build integration

package dashboardapi

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

func setupLearnTestDB(t *testing.T) *cockroach.Client {
	t.Helper()
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

func insertLearnTestTransaction(t *testing.T, db *cockroach.Client) string {
	t.Helper()
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

func insertLearnTestTask(t *testing.T, db *cockroach.Client, txnID string) string {
	t.Helper()
	ctx := context.Background()
	taskID := uuid.New().String()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO tasks (id, transaction_id, risk_score, status, created_at)
		VALUES ($1, $2, 0.5, 'done', now())
	`, taskID, txnID)
	if err != nil {
		t.Fatalf("inserting test task: %v", err)
	}
	return taskID
}

// TestHumanLessonStoredActionIsAccepted is the regression test for a real bug
// found by reading the schema against the code: 'human_lesson_stored' - the
// action internal/dashboardapi/learn.go's learnHumanLesson writes right after
// pinning a human-taught memory into case_memory - was never added to
// audit_log's action_ck (migrations/001_init.sql). Every such insert failed
// with a 23514 check_violation from the day the human-in-the-loop feature was
// written, silently, because the call site swallowed the error. Reproduced
// directly against a live CockroachDB container before writing
// migrations/003_human_lesson_audit_action.sql; this test is the permanent
// version of that manual reproduction, run for real in CI's go-integration
// job. It does not call learnHumanLesson itself (that needs live Bedrock
// access for the embedding call, which CI's CockroachDB-only job does not
// have) - it exercises the exact INSERT learn.go performs, which is the part
// migration 003 actually fixes.
func TestHumanLessonStoredActionIsAccepted(t *testing.T) {
	db := setupLearnTestDB(t)
	defer db.Close()
	ctx := context.Background()

	txnID := insertLearnTestTransaction(t, db)
	taskID := insertLearnTestTask(t, db, txnID)
	defer func() {
		db.Pool.Exec(ctx, `DELETE FROM audit_log WHERE task_id = $1`, taskID)
		db.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
		db.Pool.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, txnID)
	}()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, reviewer_id, created_at)
		VALUES ($1, $2, 'control-plane', 'human_lesson_stored', 'test lesson', 'ops', now())
	`, taskID, txnID)
	if err != nil {
		t.Fatalf("INSERT with action='human_lesson_stored' failed (action_ck missing this value - is migration 003 applied?): %v", err)
	}

	var action string
	if err := db.Pool.QueryRow(ctx,
		`SELECT action FROM audit_log WHERE task_id = $1`, taskID,
	).Scan(&action); err != nil {
		t.Fatalf("reading back the audit row: %v", err)
	}
	if action != "human_lesson_stored" {
		t.Errorf("action = %q, want %q", action, "human_lesson_stored")
	}
}
