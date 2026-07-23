// config.go: doc va tap trung toan bo cau hinh cua he thong.
// Local dev: doc tu .env. Tren Lambda that: fallback sang SSM Parameter Store
// vi Lambda khong co file .env, chi co gia tri da duoc Terraform ghi vao SSM.
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
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

// ssmParamMap anh xa ten bien env sang duong dan SSM tuong ung,
// dung dung prefix da tao trong module iam cua Terraform.
var ssmParamMap = map[string]string{
	"DATABASE_URL":             "/cockroachdb/connection_string",
	"COCKROACHDB_MCP_ENDPOINT": "/cockroachdb/mcp_endpoint",
	"TITAN_MODEL_ID":           "/bedrock/embedding_model_id",
	"CLAUDE_MODEL_ID":          "/bedrock/model_id",
}

var ssmClient *ssm.Client

func isRunningOnLambda() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

func getSSMClient() (*ssm.Client, error) {
	if ssmClient != nil {
		return ssmClient, nil
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for SSM: %w", err)
	}
	ssmClient = ssm.NewFromConfig(cfg)
	return ssmClient, nil
}

func getFromSSM(key string) (string, error) {
	suffix, ok := ssmParamMap[key]
	if !ok {
		return "", fmt.Errorf("no SSM mapping for env var %s", key)
	}

	prefix := os.Getenv("SSM_PREFIX")
	if prefix == "" {
		prefix = "/hivemind/dev"
	}

	client, err := getSSMClient()
	if err != nil {
		return "", err
	}

	withDecryption := true
	out, err := client.GetParameter(context.Background(), &ssm.GetParameterInput{
		Name:           strPtr(prefix + suffix),
		WithDecryption: &withDecryption,
	})
	if err != nil {
		return "", fmt.Errorf("reading SSM parameter %s%s: %w", prefix, suffix, err)
	}

	return *out.Parameter.Value, nil
}

func strPtr(s string) *string { return &s }

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{
		DatabaseURL: mustGetEnv("DATABASE_URL"),

		AWSAccessKeyID:     getEnvDefault("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnvDefault("AWS_SECRET_ACCESS_KEY", ""),
		AWSRegionBedrock:   getEnvDefault("AWS_REGION_BEDROCK", "ap-southeast-1"),
		AWSRegionEmbed:     getEnvDefault("AWS_REGION_EMBED", "us-east-1"),

		TitanModelID:  mustGetEnv("TITAN_MODEL_ID"),
		ClaudeModelID: mustGetEnv("CLAUDE_MODEL_ID"),

		MCPEndpoint:  mustGetEnv("COCKROACHDB_MCP_ENDPOINT"),
		MCPAPIKey:    getEnvDefault("COCKROACHDB_MCP_API_KEY", ""),
		MCPClusterID: getEnvDefault("COCKROACHDB_CLUSTER_ID", ""),
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

// mustGetEnv doc tu bien moi truong truoc. Neu khong co va dang chay tren
// Lambda that, thu doc tu SSM. Chi panic neu ca hai deu khong co gia tri.
func mustGetEnv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	if isRunningOnLambda() {
		if val, err := getFromSSM(key); err == nil && val != "" {
			return val
		}
	}

	panic(fmt.Sprintf("required environment variable %s is not set (checked env and SSM)", key))
}

func getEnvDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if isRunningOnLambda() {
		if val, err := getFromSSM(key); err == nil && val != "" {
			return val
		}
	}
	return fallback
}
