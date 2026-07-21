// main.go: worker entrypoint - khoi tao toan bo client va chay vong lap agent.
package main

import (
	"context"
	"log"

	"github.com/lethingochan27925/hivemind/internal/agent"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/pkg/bedrock"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	bedrockClient, err := bedrock.NewClient(ctx,
		cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey,
		cfg.AWSRegionBedrock, cfg.AWSRegionEmbed,
		cfg.ClaudeModelID, cfg.TitanModelID, cfg.EmbedDim,
	)
	if err != nil {
		log.Fatalf("initializing bedrock client: %v", err)
	}

	mcpClient := mcp.NewClient(mcp.Config{
		Endpoint:  cfg.MCPEndpoint,
		APIKey:    cfg.MCPAPIKey,
		ClusterID: cfg.MCPClusterID,
		Database:  cfg.MCPDatabase,
		Timeout:   cfg.MCPTimeout,
	})

	worker := agent.NewWorker(cfg, db, bedrockClient, mcpClient)
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("worker stopped: %v", err)
	}
}
