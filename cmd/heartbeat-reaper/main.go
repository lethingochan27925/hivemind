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

func reapStuckTasks(ctx context.Context, db *cockroach.Client, thresholdSeconds int) {
	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
		UPDATE tasks
		SET status = 'pending', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL
		WHERE status = 'investigating'
		  AND heartbeat_at < NOW() - INTERVAL '%d seconds'
		RETURNING id, claimed_by, transaction_id
	`, thresholdSeconds))
	if err != nil {
		log.Printf("reaper query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var taskID, workerID, txnID string
		if err := rows.Scan(&taskID, &workerID, &txnID); err != nil {
			continue
		}
		count++
		db.Pool.Exec(ctx, `
			INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, created_at)
			VALUES ($1, $2, $3, 'task_requeued', $4, now())
		`, taskID, txnID, workerID, "stuck task re-queued")
	}
	if count > 0 {
		fmt.Printf("Reaped %d stuck task(s)\n", count)
	}
}
