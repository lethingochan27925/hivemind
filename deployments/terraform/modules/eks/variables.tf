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

variable "kubernetes_version" {
  description = "EKS Kubernetes version"
  type        = string
  default     = "1.30"
}

variable "private_subnet_ids" {
  description = "Private subnet IDs cho EKS nodes"
  type        = list(string)
}

variable "eks_control_plane_sg_id" {
  description = "Security group ID cho EKS control plane"
  type        = string
}

variable "eks_cluster_role_arn" {
  description = "IAM role ARN cho EKS cluster"
  type        = string
}

variable "eks_nodes_role_arn" {
  description = "IAM role ARN cho EKS node group"
  type        = string
}

variable "node_instance_type" {
  description = "EC2 instance type cho EKS nodes"
  type        = string
  default     = "t3.medium"
}

variable "node_desired_size" {
  description = "Desired số lượng nodes"
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Min số lượng nodes"
  type        = number
  default     = 1
}

variable "node_max_size" {
  description = "Max số lượng nodes"
  type        = number
  default     = 4
}
