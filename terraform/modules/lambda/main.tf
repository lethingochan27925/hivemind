locals {
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "lambda"
  }
}

# =============================================================================
# Log groups -- tao truoc, co retention.
# Neu de Lambda tu tao thi retention = never expire -> ton tien am tham.
# =============================================================================
resource "aws_cloudwatch_log_group" "functions" {
  for_each = var.function_names

  name              = "/aws/lambda/${each.value}"
  retention_in_days = var.log_retention_days
  tags              = merge(local.common_tags, { Name = each.value })
}

# =============================================================================
# Functions
# =============================================================================
resource "aws_lambda_function" "functions" {
  for_each = var.function_names

  function_name = each.value
  role          = var.role_arns[each.key]
  package_type  = "Image"
  image_uri     = var.image_uris[each.key]

  timeout                        = var.function_config[each.key].timeout_seconds
  memory_size                    = var.function_config[each.key].memory_mb
  reserved_concurrent_executions = var.function_config[each.key].reserved_concurrency
  publish                        = true

  environment {
    variables = merge(var.common_env, var.function_config[each.key].environment)
  }

  tags = merge(local.common_tags, { Name = each.value })

  depends_on = [aws_cloudwatch_log_group.functions]
}

resource "aws_lambda_alias" "live" {
  for_each = var.function_names

  name             = "live"
  function_name    = aws_lambda_function.functions[each.key].function_name
  function_version = aws_lambda_function.functions[each.key].version

  dynamic "routing_config" {
    for_each = contains(var.canary_services, each.key) ? [1] : []
    content {
      additional_version_weights = {}
    }
  }

  lifecycle {
    ignore_changes = [function_version, routing_config]
  }
}

# =============================================================================
# EventBridge schedules -- thay cho K8s CronJob
# =============================================================================
resource "aws_cloudwatch_event_rule" "schedules" {
  for_each = var.schedules

  name                = "${var.function_names[each.key]}-schedule"
  description         = "Trigger ${var.function_names[each.key]}"
  schedule_expression = each.value.schedule_expression
  state               = each.value.enabled ? "ENABLED" : "DISABLED"
  tags                = local.common_tags
}

resource "aws_cloudwatch_event_target" "schedules" {
  for_each = var.schedules

  rule      = aws_cloudwatch_event_rule.schedules[each.key].name
  target_id = each.key
  arn       = aws_lambda_alias.live[each.key].arn
}

resource "aws_lambda_permission" "eventbridge" {
  for_each = var.schedules

  statement_id  = "AllowInvokeFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_alias.live[each.key].function_name
  qualifier     = aws_lambda_alias.live[each.key].name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedules[each.key].arn
}

# =============================================================================
# Function URLs
# =============================================================================
resource "aws_lambda_function_url" "endpoints" {
  for_each = toset(var.function_url_services)

  function_name      = aws_lambda_function.functions[each.key].function_name
  qualifier          = aws_lambda_alias.live[each.key].name
  authorization_type = var.function_url_auth_type
}
