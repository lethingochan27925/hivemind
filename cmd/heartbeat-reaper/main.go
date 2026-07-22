// main.go: heartbeat reaper entrypoint - re-queue task bi stuck ve pending.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type reapedTask struct {
	taskID   string
	workerID string
	txnID    string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	ctx := context.Background()
	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Heartbeat Reaper started")

	thresholdSeconds := int(cfg.HeartbeatThreshold.Seconds())

	for {
		reapStuckTasks(ctx, db, thresholdSeconds)
		time.Sleep(10 * time.Second)
	}
}

// reapStuckTasks dung CTE de lay claimed_by TRUOC khi UPDATE set no ve NULL,
// vi RETURNING chi tra ve gia tri SAU khi update, se luon la NULL neu doc truc tiep.
func reapStuckTasks(ctx context.Context, db *cockroach.Client, thresholdSeconds int) {
	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
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
	`, thresholdSeconds))
	if err != nil {
		log.Printf("reaper query error: %v", err)
		return
	}

	var reaped []reapedTask
	for rows.Next() {
		var t reapedTask
		var workerID *string
		if err := rows.Scan(&t.taskID, &workerID, &t.txnID); err != nil {
			log.Printf("reaper scan error: %v", err)
			continue
		}
		if workerID != nil {
			t.workerID = *workerID
		} else {
			t.workerID = "unknown"
		}
		reaped = append(reaped, t)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		log.Printf("reaper rows iteration error: %v", err)
		return
	}

	for _, t := range reaped {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, created_at)
			VALUES ($1, $2, $3, 'task_requeued', $4, now())
		`, t.taskID, t.txnID, t.workerID, "stuck task re-queued")
		if err != nil {
			log.Printf("failed to write audit log for task %s: %v", t.taskID, err)
		}
	}

	if len(reaped) > 0 {
		fmt.Printf("Reaped %d stuck task(s)\n", len(reaped))
	}
}
