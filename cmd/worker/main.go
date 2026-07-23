// main.go: worker entrypoint - khoi tao client va chay agent.
// Tren Lambda that: chay qua Lambda Runtime API, moi invoke xu ly dung 1 task.
// Local dev: chay vong lap Run() lien tuc, giu nguyen workflow cu.
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lethingochan27925/hivemind/internal/agent"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/pkg/bedrock"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

var worker *agent.Worker

func initWorker(ctx context.Context) (*agent.Worker, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	bedrockClient, err := bedrock.NewClient(ctx,
		cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey,
		cfg.AWSRegionBedrock, cfg.AWSRegionEmbed,
		cfg.ClaudeModelID, cfg.TitanModelID, cfg.EmbedDim,
	)
	if err != nil {
		return nil, err
	}

	mcpClient := mcp.NewClient(mcp.Config{
		Endpoint:  cfg.MCPEndpoint,
		APIKey:    cfg.MCPAPIKey,
		ClusterID: cfg.MCPClusterID,
		Database:  cfg.MCPDatabase,
		Timeout:   cfg.MCPTimeout,
	})

	return agent.NewWorker(cfg, db, bedrockClient, mcpClient), nil
}

func handleRequest(ctx context.Context) (string, error) {
	if worker == nil {
		var err error
		worker, err = initWorker(ctx)
		if err != nil {
			return "", err
		}
	}

	processed, err := worker.RunOnce(ctx)
	if err != nil {
		return "", err
	}
	if !processed {
		return "no pending tasks", nil
	}
	return "task processed", nil
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handleRequest)
		return
	}

	ctx := context.Background()
	w, err := initWorker(ctx)
	if err != nil {
		log.Fatalf("initializing worker: %v", err)
	}
	if err := w.Run(ctx); err != nil {
		log.Fatalf("worker stopped: %v", err)
	}
}
