data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  account_id        = data.aws_caller_identity.current.account_id
  region            = data.aws_region.current.name
  ssm_prefix        = "/${var.project}/${var.environment}"
  metrics_namespace = "HiveMind"

  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "iam"
  }

  ssm_arn_pattern = "arn:aws:ssm:${local.region}:${local.account_id}:parameter${local.ssm_prefix}/*"

  # Log group ARN duoc build tu function_names thay vi tham chieu module lambda
  # -> tranh cycle iam <-> lambda. Cung ky thuat da dung de go cycle EKS<->IAM.
  log_group_arns = {
    for k, name in var.function_names :
    k => "arn:aws:logs:${local.region}:${local.account_id}:log-group:/aws/lambda/${name}:*"
  }

  worker_function_arn = "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.function_names["agent-worker"]}"

  # Titan Embed khong co o ap-southeast-1 -> ARN phai dung region rieng.
  # Bug cu trong iam_irsa: dung chung local.region cho ca 2 model -> IAM deny.
  bedrock_model_arns = [
    "arn:aws:bedrock:${var.bedrock_region}::foundation-model/${var.bedrock_model_id}",
    "arn:aws:bedrock:${var.bedrock_embedding_region}::foundation-model/${var.bedrock_embedding_model_id}",
  ]
}

# =============================================================================
# SSM Parameters -- nguon secrets duy nhat luc runtime
# =============================================================================
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
resource "aws_ssm_parameter" "scoring_python_endpoint" {
  name        = "${local.ssm_prefix}/scoring/python_endpoint"
  type        = "String"
  value       = "placeholder"
  tags        = local.common_tags
  lifecycle { ignore_changes = [value] }
}

# =============================================================================
# Lambda execution roles -- 1 role / service, least privilege
# =============================================================================
data "aws_iam_policy_document" "lambda_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  for_each = var.function_names

  name               = each.value
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
  tags               = merge(local.common_tags, { Name = each.value })
}

# -- Baseline: logs + SSM read + custom metrics (moi service deu can) ---------
resource "aws_iam_role_policy" "baseline" {
  for_each = var.function_names

  name = "${each.value}-baseline"
  role = aws_iam_role.lambda[each.key].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "WriteOwnLogs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = local.log_group_arns[each.key]
      },
      {
        Sid      = "ReadSSM"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:GetParameters", "ssm:GetParametersByPath"]
        Resource = local.ssm_arn_pattern
      },
      {
        Sid      = "PutMetrics"
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = local.metrics_namespace }
        }
      }
    ]
  })
}

# -- Agent Worker: Bedrock (2 region) + evidence S3 --------------------------
resource "aws_iam_role_policy" "agent_worker" {
  name = "${var.function_names["agent-worker"]}-policy"
  role = aws_iam_role.lambda["agent-worker"].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "BedrockInvoke"
        Effect   = "Allow"
        Action   = ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"]
        Resource = local.bedrock_model_arns
      },
      {
        Sid      = "WriteEvidence"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${var.evidence_bucket_arn}/*"
      }
    ]
  })
}

# -- Dispatcher: invoke worker (fleet size = f(pending tasks)) + evidence ----
resource "aws_iam_role_policy" "dispatcher" {
  name = "${var.function_names["dispatcher"]}-policy"
  role = aws_iam_role.lambda["dispatcher"].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "InvokeAgentWorker"
        Effect   = "Allow"
        Action   = ["lambda:InvokeFunction"]
        Resource = local.worker_function_arn
      },
      {
        Sid      = "WriteEvidence"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${var.evidence_bucket_arn}/*"
      }
    ]
  })
}

# scoring-api va reaper: chi can baseline. Khong them policy nao.
