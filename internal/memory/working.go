// working.go: thao tac bang tasks (working memory) - claim, update status, resume.
package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type Task struct {
	ID            string
	TransactionID string
	RiskScore     float64
	Step          *string
	Scratchpad    []byte
}

// claimRetryAttempts/claimRetryDelay work around a real CockroachDB bug
// (cockroachdb/cockroach#167582): SKIP LOCKED's scanner skips any key with an
// intent, including one from a transaction that already committed but whose
// intent hasn't been asynchronously resolved yet. A task inserted moments
// ago (dispatcher just wrote it, or a test just inserted its own fixture)
// can therefore make ClaimNextTask see zero pending rows even though one
// genuinely exists - the intent clears on its own within milliseconds, so a
// couple of short retries is enough without meaningfully slowing down the
// real "queue is empty" case.
const claimRetryAttempts = 3
const claimRetryDelay = 25 * time.Millisecond

// ClaimNextTask lay 1 task pending va lock no bang SKIP LOCKED de nhieu
// worker chay song song khong bao gio claim trung 1 task.
func ClaimNextTask(ctx context.Context, db *cockroach.Client, workerID string) (*Task, error) {
	for attempt := 1; attempt <= claimRetryAttempts; attempt++ {
		task, err := claimNextTaskOnce(ctx, db, workerID)
		if err != nil || task != nil || attempt == claimRetryAttempts {
			return task, err
		}
		time.Sleep(claimRetryDelay)
	}
	return nil, nil
}

func claimNextTaskOnce(ctx context.Context, db *cockroach.Client, workerID string) (*Task, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var task Task
	err = tx.QueryRow(ctx, `
		SELECT id, transaction_id, risk_score, step, scratchpad
		FROM tasks
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&task.ID, &task.TransactionID, &task.RiskScore, &task.Step, &task.Scratchpad)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claiming task: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE tasks
		SET status = 'investigating',
			claimed_by = $1,
			claimed_at = NOW(),
			heartbeat_at = NOW()
		WHERE id = $2
	`, workerID, task.ID)
	if err != nil {
		return nil, fmt.Errorf("marking task claimed: %w", err)
	}

	// task_claimed is a real value in audit_log's action CHECK constraint but
	// nothing ever wrote it - every concurrency/evidence query that counts
	// "distinct agents that claimed work" (scripts/capture-evidence.sh,
	// evidence/README.md's fleet-distinct-agents.json) filters on this exact
	// action and silently read 0 forever. Written inside the same transaction
	// as the claim itself so the two can never disagree: either both land, or
	// neither does.
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (task_id, transaction_id, agent_id, action, created_at)
		VALUES ($1, $2, $3, 'task_claimed', now())
	`, task.ID, task.TransactionID, workerID)
	if err != nil {
		return nil, fmt.Errorf("recording claim audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing claim: %w", err)
	}

	return &task, nil
}

func CompleteTask(ctx context.Context, db *cockroach.Client, taskID, status, verdict, step string, confidence float64) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE tasks
		SET status = $1,
			verdict = $2,
			confidence = $3,
			completed_at = NOW(),
			heartbeat_at = NOW(),
			step = $4
		WHERE id = $5
	`, status, verdict, confidence, step, taskID)
	if err != nil {
		return fmt.Errorf("completing task: %w", err)
	}
	return nil
}

// FailTask danh dau task that bai VA ghi ly do vao audit_log.
// Ly do la tham so bat buoc: mot task chet khong de lai dau vet pha vo dung
// cam ket "moi quyet dinh cua agent truy vet duoc" cua he thong.
func FailTask(ctx context.Context, db *cockroach.Client, task *Task, agentID, reason string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE tasks SET status = 'failed', completed_at = NOW() WHERE id = $1
	`, task.ID)
	if err != nil {
		return fmt.Errorf("failing task: %w", err)
	}

	return WriteAuditLog(ctx, db, AuditEntry{
		TaskID:        task.ID,
		TransactionID: task.TransactionID,
		AgentID:       agentID,
		Action:        "task_failed",
		Reasoning:     &reason,
	})
}

// SaveScratchpad ghi checkpoint (step + scratchpad) de agent khac co the
// resume dung cho neu task nay bi crash va duoc re-queue.
func SaveScratchpad(ctx context.Context, db *cockroach.Client, taskID, step string, scratchpad []byte) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE tasks
		SET step = $1, scratchpad = $2, heartbeat_at = NOW()
		WHERE id = $3
	`, step, scratchpad, taskID)
	if err != nil {
		return fmt.Errorf("saving scratchpad: %w", err)
	}
	return nil
}
