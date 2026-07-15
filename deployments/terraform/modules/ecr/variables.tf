variable "project" {
  description = "Project name — used as prefix for all resource names"
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

variable "repositories" {
  description = "Danh sach repo can tao — nhan tu root de dong bo voi Lambda/IAM"
  type        = list(string)
}

variable "image_retention_count" {
  description = "So image giu lai moi repo"
  type        = number
  default     = 10
}
