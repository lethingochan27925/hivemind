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
