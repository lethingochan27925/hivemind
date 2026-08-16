# =============================================================================
# Core
# =============================================================================
variable "project" {
  description = "Project name — prefix cho tat ca resources"
  type        = string
  default     = "hivemind"
}

variable "environment" {
  description = "Deployment environment (dev | demo | prod)"
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "demo", "prod"], var.environment)
    error_message = "environment must be one of: dev, demo, prod"
  }
}

variable "aws_region" {
  description = "AWS region cho toan bo stack"
  type        = string
  default     = "ap-southeast-1"
}

# =============================================================================
# Services -- single source of truth cho ten service
# Doi o day = doi ECR repo + IAM role + Lambda function name cung luc
# =============================================================================
variable "services" {
  description = "Danh sach service chay tren Lambda"
  type        = list(string)
  default     = ["agent-worker", "scoring-api", "scoring-python", "dispatcher", "reaper", "salience-decay", "dashboard-api"]
}

variable "image_tag" {
  description = "Tag image tren ECR ma Lambda se chay (init.sh build & push truoc khi apply)"
  type        = string
  default     = "latest"
}

# =============================================================================
# Bedrock
# Titan Embed KHONG co o ap-southeast-1 -> phai tach region rieng.
# Day cung la ly do agent_loop.py tao 2 boto3 client.
# =============================================================================
variable "bedrock_model_id" {
  description = "Bedrock LLM model ID cho reasoning"
  type        = string
  default     = "anthropic.claude-3-haiku-20240307-v1:0"
}

variable "bedrock_embedding_model_id" {
  description = "Bedrock embedding model ID"
  type        = string
  default     = "amazon.titan-embed-text-v2:0"
}

variable "bedrock_region" {
  description = "Region goi LLM. null = dung aws_region"
  type        = string
  default     = null
}

variable "bedrock_embedding_region" {
  description = "Region goi embedding model (Titan v2 khong co o ap-southeast-1)"
  type        = string
  default     = "us-east-1"
}

# =============================================================================
# Lambda runtime
# =============================================================================
variable "lambda_log_retention_days" {
  description = "So ngay giu CloudWatch logs"
  type        = number
  default     = 7
}

variable "agent_worker_timeout_seconds" {
  description = "Timeout worker. PHAI nho hon reaper_stuck_threshold_seconds de tranh reap task con song"
  type        = number
  default     = 20
}

variable "agent_worker_memory_mb" {
  type    = number
  default = 512
}

variable "agent_worker_reserved_concurrency" {
  description = "Chan connection storm len CockroachDB Serverless. -1 = khong gioi han"
  type        = number
  default     = 20
}

# dispatcher/reaper/salience-decay chay dinh ky qua EventBridge. Voi
# reserved_concurrency=1 hardcode, neu 1 chu ky chay lau hon khoang cach lich
# (DB cham, tai cao) thi lan invoke ke tiep bi throttle ngay luc he thong can
# no nhat. 2 cho mot khoang dem: chu ky truoc chua xong van khong chan chu ky
# sau. Van gioi han (khong -1) vi day la job dang "mot ban ghi tai mot thoi
# diem", khong phai fleet co the scale ngang vo han.
variable "dispatcher_reserved_concurrency" {
  description = "Concurrency toi da cua dispatcher"
  type        = number
  default     = 2
}

variable "reaper_reserved_concurrency" {
  description = "Concurrency toi da cua heartbeat-reaper"
  type        = number
  default     = 2
}

variable "salience_decay_reserved_concurrency" {
  description = "Concurrency toi da cua salience-decay"
  type        = number
  default     = 2
}

variable "scoring_api_timeout_seconds" {
  type    = number
  default = 60
}

variable "scoring_api_memory_mb" {
  description = "XGBoost + sklearn can nhieu RAM luc load pickle"
  type        = number
  default     = 2048
}

variable "dispatcher_timeout_seconds" {
  type    = number
  default = 60
}

variable "dispatcher_memory_mb" {
  type    = number
  default = 512
}

variable "reaper_timeout_seconds" {
  type    = number
  default = 30
}

variable "reaper_memory_mb" {
  type    = number
  default = 256
}

# =============================================================================
# Schedules (EventBridge)
# =============================================================================
variable "dispatcher_schedule_expression" {
  type    = string
  default = "rate(1 minute)"
}

variable "reaper_schedule_expression" {
  type    = string
  default = "rate(1 minute)"
}

variable "schedules_enabled" {
  description = "false = tat het cron, khong ton tien khi khong demo"
  type        = bool
  default     = true
}

# =============================================================================
# Agent behaviour -- khong hardcode trong Python nua, doc tu env
# =============================================================================
variable "dispatcher_batch_size" {
  type    = number
  default = 100
}

variable "dispatcher_max_worker_invokes" {
  description = "So worker toi da dispatcher tu invoke moi vong. Fleet size = f(so task pending)"
  type        = number
  default     = 20
}

variable "reaper_stuck_threshold_seconds" {
  description = "Task 'investigating' qua nguong nay -> re-queue"
  type        = number
  default     = 60

  validation {
    condition     = var.reaper_stuck_threshold_seconds >= 60
    error_message = "reaper_stuck_threshold_seconds phai >= 60 va > agent_worker_timeout_seconds"
  }
}

variable "memory_top_k" {
  description = "So case recall tu episodic memory moi lan investigate"
  type        = number
  default     = 3
}

variable "risk_low_threshold" {
  description = "risk_score < nguong nay -> auto approve. PaySim bimodal -> 0.001"
  type        = number
  default     = 0.001
}

variable "risk_high_threshold" {
  description = "risk_score > nguong nay -> auto block"
  type        = number
  default     = 0.999
}

variable "chaos_kill_rate" {
  description = "Xac suat worker tu chet sau khi claim, TRUOC khi commit verdict. Chi bat luc quay demo"
  type        = number
  default     = 0

  validation {
    condition     = var.chaos_kill_rate >= 0 && var.chaos_kill_rate <= 1
    error_message = "chaos_kill_rate must be between 0 and 1"
  }
}

# =============================================================================
# Scoring API
# =============================================================================
variable "scoring_api_url_auth_type" {
  description = "AWS_IAM = phai ky SigV4 (an toan). NONE = public"
  type        = string
  default     = "AWS_IAM"

  validation {
    condition     = contains(["AWS_IAM", "NONE"], var.scoring_api_url_auth_type)
    error_message = "scoring_api_url_auth_type must be AWS_IAM or NONE"
  }
}

# =============================================================================
# Monitoring
# =============================================================================
variable "alert_email" {
  description = "Email nhan billing alert va alarm notifications"
  type        = string
}

variable "billing_threshold_usd" {
  type    = number
  default = 50
}

# =============================================================================
# Secrets -- truyen qua -var tu .env, KHONG bo vao tfvars
# =============================================================================
variable "cockroachdb_connection_string" {
  description = "CockroachDB connection string"
  type        = string
  sensitive   = true
}

variable "cockroachdb_mcp_endpoint" {
  description = "CockroachDB Managed MCP Server endpoint"
  type        = string
  sensitive   = true
}

variable "scoring_python_timeout_seconds" {
  type    = number
  default = 60
}

variable "scoring_python_memory_mb" {
  type    = number
  default = 3008
}

variable "salience_decay_timeout_seconds" {
  type    = number
  default = 60
}

variable "salience_decay_memory_mb" {
  type    = number
  default = 256
}

# Chay it hon dispatcher/reaper - day la memory management, khong phai fault recovery
variable "salience_decay_schedule_expression" {
  type    = string
  default = "rate(6 hours)"
}

variable "review_api_timeout_seconds" {
  type    = number
  default = 30
}

variable "review_api_memory_mb" {
  type    = number
  default = 256
}

variable "review_api_url_auth_type" {
  type    = string
  default = "NONE"

  validation {
    condition     = contains(["AWS_IAM", "NONE"], var.review_api_url_auth_type)
    error_message = "review_api_url_auth_type must be AWS_IAM or NONE"
  }
}

variable "github_repo" {
  type    = string
  default = "lethingochan27925/hivemind"
}

variable "agent_worker_schedule_expression" {
  type    = string
  default = "rate(1 minute)"
}

variable "cockroachdb_mcp_api_key" {
  description = "API key cua CockroachDB Managed MCP Server"
  type        = string
  sensitive   = true
}

# controlAllowed() (internal/dashboardapi/control.go) reads CONTROL_TOKEN
# straight from the Lambda environment and requires a matching X-Control-Token
# header on every mutating /control/* and /reviews/* request. dashboard-api's
# Function URL is public (auth = NONE, main.tf), so leaving this unset is not
# a neutral default - it means every rollback/region/memory/review mutation on
# the deployed console is open to the internet. Empty default only so a fresh
# `terraform apply` does not hard-fail before the user has picked a value;
# docs/DEPLOYMENT.md and .env.example both call out that a real deployment
# must set this.
variable "control_token" {
  description = "Shared secret required in X-Control-Token for mutating dashboard-api endpoints. REQUIRED for any deployment reachable outside a private demo - see docs/SECURITY.md."
  type        = string
  sensitive   = true
  default     = ""
}

# Optional: raises the Pipeline page's GitHub Actions proxy (/control/pipeline,
# internal/dashboardapi/pipeline.go) from GitHub's unauthenticated rate limit
# (60 req/hour, shared by every viewer through this one Lambda) to the
# authenticated one (5,000 req/hour). A fine-grained PAT with only
# "Actions: Read" on this one repository is enough - see
# https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api.
# Left empty, the proxy still works unauthenticated; it just shares the
# smaller budget.
variable "github_token" {
  description = "Optional GitHub PAT (Actions:Read) to raise the Pipeline page's GitHub API rate limit from 60/hour to 5,000/hour"
  type        = string
  sensitive   = true
  default     = ""
}

variable "cockroachdb_cluster_id" {
  description = "Cluster ID cua CockroachDB Cloud"
  type        = string
}
