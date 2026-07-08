data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.name
  ssm_prefix = "/${var.project}/${var.environment}"

  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "iam"
  }
}

# -- SSM Parameters ------------------------------------------------------------
resource "aws_ssm_parameter" "cockroachdb_conn" {
  name        = "${local.ssm_prefix}/cockroachdb/connection_string"
  description = "CockroachDB Cloud connection string"
  type        = "SecureString"
  value       = var.cockroachdb_connection_string
  tags        = local.common_tags
  lifecycle { ignore_changes = [value] }
}

resource "aws_ssm_parameter" "cockroachdb_mcp_endpoint" {
  name        = "${local.ssm_prefix}/cockroachdb/mcp_endpoint"
  description = "CockroachDB Managed MCP Server endpoint"
  type        = "SecureString"
  value       = var.cockroachdb_mcp_endpoint
  tags        = local.common_tags
  lifecycle { ignore_changes = [value] }
}

resource "aws_ssm_parameter" "bedrock_model_id" {
  name        = "${local.ssm_prefix}/bedrock/model_id"
  description = "Bedrock LLM model ID"
  type        = "String"
  value       = var.bedrock_model_id
  tags        = local.common_tags
}

resource "aws_ssm_parameter" "bedrock_embedding_model_id" {
  name        = "${local.ssm_prefix}/bedrock/embedding_model_id"
  description = "Bedrock embedding model ID"
  type        = "String"
  value       = var.bedrock_embedding_model_id
  tags        = local.common_tags
}

# -- EKS Cluster Role ----------------------------------------------------------
resource "aws_iam_role" "eks_cluster" {
  name = "${var.project}-${var.environment}-eks-cluster"
  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-eks-cluster" })

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "eks.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# -- EKS Node Role -------------------------------------------------------------
resource "aws_iam_role" "eks_nodes" {
  name = "${var.project}-${var.environment}-eks-nodes"
  tags = merge(local.common_tags, { Name = "${var.project}-${var.environment}-eks-nodes" })

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_nodes_worker" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "eks_nodes_cni" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "eks_nodes_ecr" {
  role       = aws_iam_role.eks_nodes.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}
