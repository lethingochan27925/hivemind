variable "project" {
  type = string
}

variable "environment" {
  type = string
}

variable "github_repo" {
  description = "Format: owner/repo"
  type        = string
}

variable "tfstate_bucket" {
  type = string
}

variable "common_tags" {
  type    = map(string)
  default = {}
}
