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

variable "eks_oidc_provider_arn" {
  description = "EKS OIDC provider ARN"
  type        = string
}

variable "eks_oidc_provider_url" {
  description = "EKS OIDC provider URL"
  type        = string
}

variable "k8s_namespace" {
  description = "Kubernetes namespace chua HiveMind services"
  type        = string
  default     = "hivemind"
}

variable "evidence_bucket_arn" {
  description = "ARN cua S3 evidence bucket"
  type        = string
}

variable "bedrock_model_id" {
  description = "Bedrock LLM model ID"
  type        = string
}

variable "bedrock_embedding_model_id" {
  description = "Bedrock embedding model ID"
  type        = string
}
