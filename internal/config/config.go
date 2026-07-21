// config.go: doc va tap trung toan bo cau hinh cua he thong tu file .env va bien moi truong.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string

	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegionBedrock   string
	AWSRegionEmbed     string

	TitanModelID  string
	ClaudeModelID string
	EmbedDim      int

	MCPEndpoint  string
	MCPAPIKey    string
	MCPClusterID string
	MCPDatabase  string
	MCPTimeout   time.Duration

	WorkerPollInterval time.Duration
	HeartbeatThreshold time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		DatabaseURL: mustGetEnv("DATABASE_URL"),

		AWSAccessKeyID:     mustGetEnv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: mustGetEnv("AWS_SECRET_ACCESS_KEY"),
		AWSRegionBedrock:   getEnvDefault("AWS_REGION_BEDROCK", "ap-southeast-1"),
		AWSRegionEmbed:     getEnvDefault("AWS_REGION_EMBED", "us-east-1"),

		TitanModelID:  mustGetEnv("TITAN_MODEL_ID"),
		ClaudeModelID: mustGetEnv("CLAUDE_MODEL_ID"),

		MCPEndpoint:  getEnvDefault("COCKROACHDB_MCP_ENDPOINT", "https://cockroachlabs.cloud/mcp"),
		MCPAPIKey:    mustGetEnv("COCKROACHDB_MCP_API_KEY"),
		MCPClusterID: mustGetEnv("COCKROACHDB_CLUSTER_ID"),
		MCPDatabase:  getEnvDefault("COCKROACHDB_DATABASE", "hivemind"),
	}

	embedDim, err := strconv.Atoi(getEnvDefault("EMBED_DIM", "1024"))
	if err != nil {
		return nil, fmt.Errorf("invalid EMBED_DIM: %w", err)
	}
	cfg.EmbedDim = embedDim

	mcpTimeoutSec, err := strconv.Atoi(getEnvDefault("MCP_TIMEOUT_SECONDS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid MCP_TIMEOUT_SECONDS: %w", err)
	}
	cfg.MCPTimeout = time.Duration(mcpTimeoutSec) * time.Second

	pollIntervalSec, err := strconv.Atoi(getEnvDefault("WORKER_POLL_INTERVAL_SECONDS", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_POLL_INTERVAL_SECONDS: %w", err)
	}
	cfg.WorkerPollInterval = time.Duration(pollIntervalSec) * time.Second

	heartbeatSec, err := strconv.Atoi(getEnvDefault("REAPER_STUCK_THRESHOLD_SECONDS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid REAPER_STUCK_THRESHOLD_SECONDS: %w", err)
	}
	cfg.HeartbeatThreshold = time.Duration(heartbeatSec) * time.Second

	return cfg, nil
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

func getEnvDefault(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
