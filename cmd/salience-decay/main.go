// main.go: salience decay entrypoint - giam salience theo thoi gian, archive case cu.
// Chay doc lap voi heartbeat-reaper vi day la memory management, khong phai fault recovery.
// Tren Lambda that: moi invoke la mot chu ky decay, EventBridge lo viec lap lai.
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
	"github.com/lethingochan27925/hivemind/internal/memory"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const localCycleInterval = 6 * time.Hour

type CycleResult struct {
	ActiveBefore int `json:"active_before"`
	ActiveAfter  int `json:"active_after"`
	Archived     int `json:"archived"`
}

type DecayJob struct {
	db *cockroach.Client
}

func NewDecayJob(ctx context.Context) (*DecayJob, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return &DecayJob{db: db}, nil
}

func (j *DecayJob) RunOnce(ctx context.Context) (CycleResult, error) {
	before, err := j.countActiveCases(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("counting active cases before decay: %w", err)
	}

	if err := memory.DecaySalience(ctx, j.db); err != nil {
		return CycleResult{}, fmt.Errorf("decaying salience: %w", err)
	}

	after, err := j.countActiveCases(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("counting active cases after decay: %w", err)
	}

	return CycleResult{
		ActiveBefore: before,
		ActiveAfter:  after,
		Archived:     before - after,
	}, nil
}

func (j *DecayJob) countActiveCases(ctx context.Context) (int, error) {
	var count int
	err := j.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM case_memory WHERE archived = false
	`).Scan(&count)
	return count, err
}

func main() {
	ctx := context.Background()

	job, err := NewDecayJob(ctx)
	if err != nil {
		log.Fatalf("initializing salience decay job: %v", err)
	}
	defer job.db.Close()

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(func(ctx context.Context) (CycleResult, error) {
			return job.RunOnce(ctx)
		})
		return
	}

	log.Printf("Salience Decay job started (local mode, cycle=%s)", localCycleInterval)
	for {
		res, err := job.RunOnce(ctx)
		if err != nil {
			log.Printf("decay cycle error: %v", err)
		} else {
			log.Printf("decay complete | active_before=%d active_after=%d archived=%d",
				res.ActiveBefore, res.ActiveAfter, res.Archived)
		}
		time.Sleep(localCycleInterval)
	}
}
