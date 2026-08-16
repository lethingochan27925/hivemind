package config

import (
	"strings"
	"testing"
)

// requiredEnvVars are exactly the ones Load() cannot proceed without and has
// no SSM fallback available for in this test (isRunningOnLambda() is false,
// so getFromSSM is never reached).
var requiredEnvVars = []string{
	"DATABASE_URL", "TITAN_MODEL_ID", "CLAUDE_MODEL_ID", "COCKROACHDB_MCP_ENDPOINT",
}

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "") // isRunningOnLambda() must be false
	for _, k := range requiredEnvVars {
		t.Setenv(k, "")
	}
	for _, k := range []string{
		"AWS_REGION_BEDROCK", "AWS_REGION_EMBED", "COCKROACHDB_DATABASE",
		"EMBED_DIM", "MCP_TIMEOUT_SECONDS", "WORKER_POLL_INTERVAL_SECONDS",
		"REAPER_STUCK_THRESHOLD_SECONDS",
	} {
		t.Setenv(k, "")
	}
}

// TestLoadMissingRequiredEnvReturnsError is the regression test for the bug
// this file exists to catch: mustGetEnv used to panic on a missing required
// variable instead of returning an error like every other validation failure
// in Load() — a cold start with one missing SSM parameter would crash the
// Lambda runtime instead of failing with a readable error.
func TestLoadMissingRequiredEnvReturnsError(t *testing.T) {
	for _, missing := range requiredEnvVars {
		t.Run(missing, func(t *testing.T) {
			clearEnv(t)
			for _, k := range requiredEnvVars {
				if k != missing {
					t.Setenv(k, "x")
				}
			}

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Load() panicked instead of returning an error: %v", r)
				}
			}()

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() with %s unset: expected an error, got nil", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Errorf("Load() error %q does not name the missing variable %s", err.Error(), missing)
			}
		})
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearEnv(t)
	for _, k := range requiredEnvVars {
		t.Setenv(k, "x")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with all required vars set: unexpected error: %v", err)
	}
	if cfg.AWSRegionBedrock != "ap-southeast-1" {
		t.Errorf("AWSRegionBedrock default = %q, want ap-southeast-1", cfg.AWSRegionBedrock)
	}
	if cfg.AWSRegionEmbed != "us-east-1" {
		t.Errorf("AWSRegionEmbed default = %q, want us-east-1", cfg.AWSRegionEmbed)
	}
	if cfg.EmbedDim != 1024 {
		t.Errorf("EmbedDim default = %d, want 1024", cfg.EmbedDim)
	}
	if cfg.MCPDatabase != "hivemind" {
		t.Errorf("MCPDatabase default = %q, want hivemind", cfg.MCPDatabase)
	}
}

func TestLoadInvalidEmbedDim(t *testing.T) {
	clearEnv(t)
	for _, k := range requiredEnvVars {
		t.Setenv(k, "x")
	}
	t.Setenv("EMBED_DIM", "not-a-number")

	if _, err := Load(); err == nil {
		t.Error("Load() with EMBED_DIM=not-a-number: expected an error, got nil")
	}
}
