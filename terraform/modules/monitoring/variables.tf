variable "project" { type = string }
variable "environment" { type = string }
variable "alert_email" { type = string }

variable "function_names" {
  description = "Map service key -> Lambda function name"
  type        = map(string)
}

variable "metrics_namespace" {
  description = "CloudWatch namespace cho custom metrics"
  type        = string
}

variable "billing_threshold_usd" {
  type    = number
  default = 50
}

variable "error_threshold" {
  description = "So error trong 1 period truoc khi bao dong"
  type        = number
  default     = 5
}

variable "bedrock_model_id" {
  description = "Claude model ID (AWS/Bedrock InvocationThrottles ModelId dimension)"
  type        = string
}

variable "bedrock_embedding_model_id" {
  description = "Titan embedding model ID (AWS/Bedrock InvocationThrottles ModelId dimension)"
  type        = string
}
