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
    Module      = "iam_irsa"
  }

  services = ["agent-worker", "scoring-api", "dispatcher", "reaper"]
}

# -- IRSA trust policy per service ---------------------------------------------
data "aws_iam_policy_document" "irsa_assume" {
  for_each = toset(local.services)

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [var.eks_oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(var.eks_oidc_provider_url, "https://", "")}:sub"
      values   = ["system:serviceaccount:${var.k8s_namespace}:${each.key}"]
    }
  }
}

# -- Agent Worker --------------------------------------------------------------
resource "aws_iam_role" "agent_worker" {
  name               = "${var.project}-${var.environment}-agent-worker"
  assume_role_policy = data.aws_iam_policy_document.irsa_assume["agent-worker"].json
  tags               = merge(local.common_tags, { Name = "${var.project}-${var.environment}-agent-worker" })
}

resource "aws_iam_role_policy" "agent_worker" {
  name = "agent-worker-policy"
  role = aws_iam_role.agent_worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "BedrockInvoke"
        Effect = "Allow"
        Action = ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"]
        Resource = [
          "arn:aws:bedrock:${local.region}::foundation-model/${var.bedrock_model_id}",
          "arn:aws:bedrock:${local.region}::foundation-model/${var.bedrock_embedding_model_id}"
        ]
      },
      {
        Sid      = "ReadWriteEvidence"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${var.evidence_bucket_arn}/*"
      },
      {
        Sid      = "ReadSSM"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = "arn:aws:ssm:${local.region}:${local.account_id}:parameter${local.ssm_prefix}/*"
      },
      {
        Sid      = "PutMetrics"
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = "HiveMind" }
        }
      }
    ]
  })
}

# -- Scoring API ---------------------------------------------------------------
resource "aws_iam_role" "scoring_api" {
  name               = "${var.project}-${var.environment}-scoring-api"
  assume_role_policy = data.aws_iam_policy_document.irsa_assume["scoring-api"].json
  tags               = merge(local.common_tags, { Name = "${var.project}-${var.environment}-scoring-api" })
}

resource "aws_iam_role_policy" "scoring_api" {
  name = "scoring-api-policy"
  role = aws_iam_role.scoring_api.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ReadSSM"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = "arn:aws:ssm:${local.region}:${local.account_id}:parameter${local.ssm_prefix}/*"
      },
      {
        Sid      = "PutMetrics"
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = "HiveMind" }
        }
      }
    ]
  })
}

# -- Dispatcher ----------------------------------------------------------------
resource "aws_iam_role" "dispatcher" {
  name               = "${var.project}-${var.environment}-dispatcher"
  assume_role_policy = data.aws_iam_policy_document.irsa_assume["dispatcher"].json
  tags               = merge(local.common_tags, { Name = "${var.project}-${var.environment}-dispatcher" })
}

resource "aws_iam_role_policy" "dispatcher" {
  name = "dispatcher-policy"
  role = aws_iam_role.dispatcher.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ReadWriteEvidence"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${var.evidence_bucket_arn}/*"
      },
      {
        Sid      = "ReadSSM"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = "arn:aws:ssm:${local.region}:${local.account_id}:parameter${local.ssm_prefix}/*"
      },
      {
        Sid      = "PutMetrics"
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = "HiveMind" }
        }
      }
    ]
  })
}

# -- Heartbeat Reaper ----------------------------------------------------------
resource "aws_iam_role" "reaper" {
  name               = "${var.project}-${var.environment}-reaper"
  assume_role_policy = data.aws_iam_policy_document.irsa_assume["reaper"].json
  tags               = merge(local.common_tags, { Name = "${var.project}-${var.environment}-reaper" })
}

resource "aws_iam_role_policy" "reaper" {
  name = "reaper-policy"
  role = aws_iam_role.reaper.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ReadSSM"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = "arn:aws:ssm:${local.region}:${local.account_id}:parameter${local.ssm_prefix}/*"
      },
      {
        Sid      = "PutMetrics"
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = "HiveMind" }
        }
      }
    ]
  })
}
