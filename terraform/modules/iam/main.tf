data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  account_id        = data.aws_caller_identity.current.account_id
  region            = data.aws_region.current.region
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

  worker_function_arn         = "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.function_names["agent-worker"]}"
  scoring_python_function_arn = "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.function_names["scoring-python"]}"

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

# MCP can CA API key lan cluster ID luc runtime. Truoc day chi co endpoint duoc
# dua len SSM, nen tren Lambda hai gia tri kia luon rong: agent gui header
# "Bearer " khong co token va MCP Server tra 401 - loi da lam hong 33% task.
resource "aws_ssm_parameter" "cockroachdb_mcp_api_key" {
  name        = "${local.ssm_prefix}/cockroachdb/mcp_api_key"
  description = "CockroachDB Managed MCP Server API key"
  type        = "SecureString"
  value       = var.cockroachdb_mcp_api_key
  tags        = local.common_tags
  lifecycle { ignore_changes = [value] }
}

resource "aws_ssm_parameter" "cockroachdb_cluster_id" {
  name        = "${local.ssm_prefix}/cockroachdb/cluster_id"
  description = "CockroachDB Cloud cluster ID"
  type        = "String"
  value       = var.cockroachdb_cluster_id
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
  name  = "${local.ssm_prefix}/scoring/python_endpoint"
  type  = "String"
  value = "placeholder"
  tags  = local.common_tags
  lifecycle { ignore_changes = [value] }
}

# =============================================================================
# Dead-letter queue for async invocations
# =============================================================================
# EventBridge and the dispatcher's fan-out (InvocationTypeEvent) both invoke
# Lambda asynchronously. An async invoke that exhausts its retries used to
# just vanish - no DLQ, no destination, nothing in any log to say a task's
# worker invocation failed outright rather than the worker itself returning
# an error. One shared queue for every function: cheap, and destinations
# don't need per-function isolation the way execution roles do.
resource "aws_sqs_queue" "async_dlq" {
  name                      = "${var.project}-${var.environment}-lambda-dlq"
  message_retention_seconds = 1209600 # 14 days (max) - a failure sits here until someone looks, not until it expires unnoticed
  tags                      = local.common_tags
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
      },
      {
        # Lambda's own service role (not this function's) delivers the
        # failure record after retries are exhausted, but AWS requires the
        # FUNCTION's execution role to hold this permission on the
        # destination - see "Configure a destination" in the Lambda docs.
        Sid      = "WriteAsyncDLQ"
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = aws_sqs_queue.async_dlq.arn
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
        Sid    = "InvokeAgentWorker"
        Effect = "Allow"
        Action = ["lambda:InvokeFunction"]
        # Function URL va EventBridge goi qua alias "live" -> quyen phai phu
        # ca ARN qualified, khong chi ARN unqualified.
        Resource = [
          local.worker_function_arn,
          "${local.worker_function_arn}:*",
        ]
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

# -- Dashboard API: doc trang thai alarm de ve trang Infrastructure ----------
resource "aws_iam_role_policy" "dashboard_api" {
  name = "${var.function_names["dashboard-api"]}-policy"
  role = aws_iam_role.lambda["dashboard-api"].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # Vong lap hoc tu con nguoi: control plane embed phan quyet cua
        # reviewer de nhung vao case_memory. Chi model embedding - khong Claude.
        # Don gia token lay song tu AWS Pricing API (internal/pricing).
        # Pricing API chi chap nhan Resource "*".
        Sid      = "ReadLivePricing"
        Effect   = "Allow"
        Action   = ["pricing:GetProducts"]
        Resource = "*"
      },
      {
        Sid      = "EmbedHumanLessons"
        Effect   = "Allow"
        Action   = ["bedrock:InvokeModel"]
        Resource = local.bedrock_model_arns
      },
      {
        Sid      = "ReadAlarmState"
        Effect   = "Allow"
        Action   = ["cloudwatch:DescribeAlarms", "cloudwatch:GetMetricData"]
        Resource = "*"
      },
      {
        Sid      = "ControlSchedules"
        Effect   = "Allow"
        Action   = ["events:EnableRule", "events:DisableRule", "events:DescribeRule"]
        Resource = "arn:aws:events:${local.region}:${local.account_id}:rule/${var.project}-${var.environment}-*-schedule"
      },
      {
        Sid    = "CostExplorerRead"
        Effect = "Allow"
        Action = [
          "ce:GetCostAndUsage",
          "ce:GetCostForecast",
        ]
        Resource = "*"
      },
      {
        Sid    = "ManageAliases"
        Effect = "Allow"
        Action = [
          "lambda:GetAlias",
          "lambda:UpdateAlias",
          "lambda:ListVersionsByFunction",
        ]
        Resource = "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.project}-${var.environment}-*"
      },
      {
        Sid    = "InvokeFleet"
        Effect = "Allow"
        Action = ["lambda:InvokeFunction"]
        Resource = flatten([
          for svc in ["dispatcher", "agent-worker", "reaper", "salience-decay"] : [
            "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.function_names[svc]}",
            "arn:aws:lambda:${local.region}:${local.account_id}:function:${var.function_names[svc]}:*",
          ]
        ])
      },
      {
        Sid      = "ReadLambdaConfig"
        Effect   = "Allow"
        Action   = ["lambda:GetFunctionConfiguration", "lambda:ListFunctions"]
        Resource = "*"
      },
      {
        Sid      = "ReadResources"
        Effect   = "Allow"
        Action   = ["tag:GetResources"]
        Resource = "*"
      }
    ]
  })
}

# -- Scoring API: goi scoring-python qua Function URL dung auth AWS_IAM -------
# Endpoint khong public, caller phai ky SigV4. Tu 10/2025 AWS doi CA HAI quyen
# cho moi lan goi Function URL. Qualifier "*" de phu ARN cua alias "live".
resource "aws_iam_role_policy" "scoring_api" {
  name = "${var.function_names["scoring-api"]}-policy"
  role = aws_iam_role.lambda["scoring-api"].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "InvokeScoringPython"
        Effect = "Allow"
        Action = ["lambda:InvokeFunctionUrl", "lambda:InvokeFunction"]
        Resource = [
          local.scoring_python_function_arn,
          "${local.scoring_python_function_arn}:*",
        ]
      }
    ]
  })
}
