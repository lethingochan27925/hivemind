variable "project" {
  description = "Project name — prefix cho tất cả resources"
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
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "CIDR block cho VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Số lượng Availability Zones"
  type        = number
  default     = 2
}

variable "kubernetes_version" {
  description = "EKS Kubernetes version"
  type        = string
  default     = "1.30"
}

variable "node_instance_type" {
  description = "EC2 instance type cho EKS nodes"
  type        = string
  default     = "t3.medium"
}

variable "node_desired_size" {
  type    = number
  default = 2
}

variable "node_min_size" {
  type    = number
  default = 1
}

variable "node_max_size" {
  type    = number
  default = 4
}

variable "k8s_namespace" {
  description = "Kubernetes namespace cho HiveMind services"
  type        = string
  default     = "hivemind"
}

variable "cockroachdb_connection_string" {
  description = "CockroachDB connection string"
  type        = string
  sensitive   = true
}

variable "cockroachdb_mcp_endpoint" {
  description = "CockroachDB MCP Server endpoint"
  type        = string
  sensitive   = true
}

variable "bedrock_model_id" {
  description = "Bedrock LLM model ID"
  type        = string
  default     = "anthropic.claude-haiku-4-5-20251001-v1:0"
}

variable "bedrock_embedding_model_id" {
  description = "Bedrock embedding model ID"
  type        = string
  default     = "amazon.titan-embed-text-v2:0"
}

variable "dispatcher_schedule" {
  description = "EventBridge schedule cho dispatcher"
  type        = string
  default     = "rate(1 minute)"
}

variable "heartbeat_stale_threshold_sec" {
  description = "Giây không heartbeat → task stale"
  type        = number
  default     = 90
}

variable "scoring_api_url" {
  description = "Internal URL của Scoring API service trong K8s"
  type        = string
  default     = "http://scoring-api.hivemind.svc.cluster.local:8000/score"
}

variable "alert_email" {
  description = "Email nhận billing alert và alarm notifications"
  type        = string
}

variable "billing_threshold_usd" {
  description = "USD threshold cho billing alarm"
  type        = number
  default     = 50
}
