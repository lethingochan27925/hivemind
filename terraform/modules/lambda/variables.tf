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
  description = "Map service key -> Lambda function name"
  type        = map(string)
}

variable "image_uris" {
  description = "Map service key -> ECR image URI. Image PHAI duoc push truoc khi apply."
  type        = map(string)
}

variable "role_arns" {
  description = "Map service key -> IAM execution role ARN"
  type        = map(string)
}

variable "common_env" {
  description = "Env vars ap cho moi function"
  type        = map(string)
}

variable "function_config" {
  description = "Cau hinh rieng tung function"
  type = map(object({
    timeout_seconds      = number
    memory_mb            = number
    reserved_concurrency = number
    environment          = map(string)
  }))
}

variable "schedules" {
  description = "Map service key -> EventBridge schedule. Service khong co trong map = khong co cron."
  type = map(object({
    schedule_expression = string
    enabled             = bool
  }))
  default = {}
}

variable "function_url_services" {
  description = "Service can Function URL (HTTP endpoint)"
  type        = list(string)
  default     = []
}

variable "function_url_auth_type" {
  description = "AWS_IAM | NONE"
  type        = string
  default     = "AWS_IAM"
}

variable "log_retention_days" {
  description = "So ngay giu CloudWatch logs"
  type        = number
  default     = 7
}

variable "canary_services" {
  type    = list(string)
  default = []
}
