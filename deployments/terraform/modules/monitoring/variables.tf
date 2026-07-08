variable "project"              { type = string }
variable "environment"          { type = string }
variable "alert_email"          { type = string }
variable "eks_cluster_name"     { type = string }

variable "billing_threshold_usd" {
  type    = number
  default = 50
}
