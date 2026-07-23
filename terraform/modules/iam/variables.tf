variable "project" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Deployment environment (dev | demo | prod)"
  type        = string

  validation {
    condition     = contains(["dev", "demo", "prod"], var.environment)
    error_message = "environment must be one of: dev, demo, prod"
  }
}

variable "function_names" {
  description = "Map service key -> Lambda function name. Nhan tu root, khong tu build."
  type        = map(string)
}

variable "evidence_bucket_arn" {
  description = "ARN bucket S3 luu evidence"
  type        = string
}

variable "log_retention_days" {
  description = "Chi de tham chieu — log group do module lambda tao"
  type        = number
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
}

variable "bedrock_embedding_model_id" {
  description = "Bedrock embedding model ID"
  type        = string
}

variable "bedrock_region" {
  description = "Region goi LLM"
  type        = string
}

variable "bedrock_embedding_region" {
  description = "Region goi embedding model — thuong khac bedrock_region"
  type        = string
}
