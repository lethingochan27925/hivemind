// working.go: thao tac bang tasks (working memory) - claim, update status, resume.
package memory

import (
	"context"
	"fmt"

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

// ClaimNextTask lay 1 task pending va lock no bang SKIP LOCKED de nhieu
// worker chay song song khong bao gio claim trung 1 task.
func ClaimNextTask(ctx context.Context, db *cockroach.Client, workerID string) (*Task, error) {
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

func FailTask(ctx context.Context, db *cockroach.Client, taskID string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE tasks SET status = 'failed', completed_at = NOW() WHERE id = $1
	`, taskID)
	if err != nil {
		return fmt.Errorf("failing task: %w", err)
	}
	return nil
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
