// main.go: salience decay entrypoint - giam salience theo thoi gian, archive case cu.
// Chay doc lap voi heartbeat-reaper vi day la memory management, khong phai fault recovery.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/memory"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const decayInterval = 6 * time.Hour

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

	fmt.Println("Salience Decay job started")

	for {
		runDecay(ctx, db)
		time.Sleep(decayInterval)
	}
}

func runDecay(ctx context.Context, db *cockroach.Client) {
	before, err := countActiveCases(ctx, db)
	if err != nil {
		log.Printf("counting active cases before decay: %v", err)
	}

	if err := memory.DecaySalience(ctx, db); err != nil {
		log.Printf("decay salience error: %v", err)
		return
	}

	after, err := countActiveCases(ctx, db)
	if err != nil {
		log.Printf("counting active cases after decay: %v", err)
		return
	}

	archived := before - after
	fmt.Printf("Decay cycle complete | active_before=%d active_after=%d archived_this_cycle=%d\n",
		before, after, archived)
}

func countActiveCases(ctx context.Context, db *cockroach.Client) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM case_memory WHERE archived = false
	`).Scan(&count)
	return count, err
}
