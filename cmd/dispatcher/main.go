// main.go: dispatcher entrypoint - tao task cho transaction medium-tier chua co task.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/stream"
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

	fmt.Println("Dispatcher started")

	for {
		count, err := stream.InsertMediumTasks(ctx, db)
		if err != nil {
			log.Printf("dispatcher error: %v", err)
		} else if count > 0 {
			fmt.Printf("Dispatched %d new tasks\n", count)
		}
		time.Sleep(5 * time.Second)
	}
}
