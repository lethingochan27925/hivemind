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
  description = "AWS region"
  type        = string
  default     = "ap-southeast-1"
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "az_count" {
  type    = number
  default = 2
}

variable "kubernetes_version" {
  type    = string
  default = "1.30"
}

variable "node_instance_type" {
  type    = string
  default = "t3.medium"
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
  type    = string
  default = "hivemind"
}

variable "bedrock_model_id" {
  type    = string
  default = "anthropic.claude-haiku-4-5-20251001-v1:0"
}

variable "bedrock_embedding_model_id" {
  type    = string
  default = "amazon.titan-embed-text-v2:0"
}

variable "alert_email" {
  description = "Email nhan billing alert va alarm notifications"
  type        = string
}

variable "billing_threshold_usd" {
  type    = number
  default = 50
}

# Secrets -- truyen qua .env, khong trong tfvars
variable "cockroachdb_connection_string" {
  description = "CockroachDB connection string — truyen qua -var flag tu .env"
  type        = string
  sensitive   = true
}

variable "cockroachdb_mcp_endpoint" {
  description = "CockroachDB MCP endpoint — truyen qua -var flag tu .env"
  type        = string
  sensitive   = true
}
