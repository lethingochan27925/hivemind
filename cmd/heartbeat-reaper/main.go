// main.go: heartbeat reaper entrypoint - re-queue task bi stuck ve pending.
// Tren Lambda that: moi invoke la mot chu ky quet, EventBridge lo viec lap lai.
// Local dev: vong lap tay de giu duoc workflow chay ngoai AWS.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const localCycleInterval = 10 * time.Second

type reapedTask struct {
	taskID   string
	workerID string
	txnID    string
}

type CycleResult struct {
	ReapedTasks int `json:"reaped_tasks"`
}

type Reaper struct {
	db               *cockroach.Client
	thresholdSeconds int
}

func NewReaper(ctx context.Context) (*Reaper, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return &Reaper{
		db:               db,
		thresholdSeconds: int(cfg.HeartbeatThreshold.Seconds()),
	}, nil
}

func (r *Reaper) RunOnce(ctx context.Context) (CycleResult, error) {
	reaped, err := r.reapStuckTasks(ctx)
	if err != nil {
		return CycleResult{}, err
	}

	// Audit log la bang chung compliance, khong duoc lam hong ca chu ky reap
	// chi vi mot dong ghi that bai.
	for _, t := range reaped {
		if err := r.recordRequeue(ctx, t); err != nil {
			log.Printf("failed to write audit log for task %s: %v", t.taskID, err)
		}
	}

	return CycleResult{ReapedTasks: len(reaped)}, nil
}

// reapStuckTasks dung CTE de lay claimed_by TRUOC khi UPDATE set no ve NULL,
// vi RETURNING chi tra ve gia tri SAU khi update, se luon la NULL neu doc truc tiep.
// thresholdSeconds la int cua Go (tu config), khong phai input nguoi dung.
func (r *Reaper) reapStuckTasks(ctx context.Context) ([]reapedTask, error) {
	rows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		WITH stuck AS (
			SELECT id, claimed_by, transaction_id
			FROM tasks
			WHERE status = 'investigating'
			  AND heartbeat_at < NOW() - INTERVAL '%d seconds'
		)
		UPDATE tasks
		SET status = 'pending', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL
		FROM stuck
		WHERE tasks.id = stuck.id
		RETURNING tasks.id, stuck.claimed_by, tasks.transaction_id
	`, r.thresholdSeconds))
	if err != nil {
		return nil, fmt.Errorf("querying stuck tasks: %w", err)
	}
	defer rows.Close()

	var reaped []reapedTask
	for rows.Next() {
		var t reapedTask
		var workerID *string
		if err := rows.Scan(&t.taskID, &workerID, &t.txnID); err != nil {
			return nil, fmt.Errorf("scanning stuck task: %w", err)
		}
		t.workerID = "unknown"
		if workerID != nil {
			t.workerID = *workerID
		}
		reaped = append(reaped, t)
	}

	// Next() tra false da tu dong dong Rows, nen doc Err() o day la hop le.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stuck tasks: %w", err)
	}

	return reaped, nil
}

func (r *Reaper) recordRequeue(ctx context.Context, t reapedTask) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, created_at)
		VALUES ($1, $2, $3, 'task_requeued', $4, now())
	`, t.taskID, t.txnID, t.workerID, "stuck task re-queued")
	return err
}

func main() {
	ctx := context.Background()

	r, err := NewReaper(ctx)
	if err != nil {
		log.Fatalf("initializing reaper: %v", err)
	}
	defer r.db.Close()

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(func(ctx context.Context) (CycleResult, error) {
			return r.RunOnce(ctx)
		})
		return
	}

	log.Printf("Heartbeat Reaper started (local mode, cycle=%s, threshold=%ds)",
		localCycleInterval, r.thresholdSeconds)
	for {
		res, err := r.RunOnce(ctx)
		if err != nil {
			log.Printf("reap cycle error: %v", err)
		} else if res.ReapedTasks > 0 {
			log.Printf("reaped %d stuck task(s)", res.ReapedTasks)
		}
		time.Sleep(localCycleInterval)
	}
}
