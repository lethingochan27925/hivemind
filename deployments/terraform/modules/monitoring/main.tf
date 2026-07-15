locals {
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "monitoring"
  }
}

data "aws_region" "current" {}

# -- SNS Topic ----------------------------------------------------------------
resource "aws_sns_topic" "alerts" {
  name = "${var.project}-${var.environment}-alerts"
  tags = local.common_tags
}

resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# -- Billing Alarm ------------------------------------------------------------
resource "aws_cloudwatch_metric_alarm" "billing" {
  alarm_name          = "${var.project}-${var.environment}-billing"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "EstimatedCharges"
  namespace           = "AWS/Billing"
  period              = 86400
  statistic           = "Maximum"
  threshold           = var.billing_threshold_usd
  alarm_description   = "AWS spend exceeded $${var.billing_threshold_usd}"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  tags                = local.common_tags

  dimensions = { Currency = "USD" }
}

# -- Lambda Errors ------------------------------------------------------------
resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = var.function_names

  alarm_name          = "${each.value}-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = var.error_threshold
  alarm_description   = "${each.value} errors > ${var.error_threshold} / 5min"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  treat_missing_data  = "notBreaching"
  tags                = local.common_tags

  dimensions = { FunctionName = each.value }
}

# -- Lambda Throttles ---------------------------------------------------------
# Worker bi throttle = reserved_concurrency qua thap so voi tai.
resource "aws_cloudwatch_metric_alarm" "lambda_throttles" {
  for_each = var.function_names

  alarm_name          = "${each.value}-throttles"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Throttles"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = 0
  alarm_description   = "${each.value} bi throttle -- kiem tra reserved concurrency"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  treat_missing_data  = "notBreaching"
  tags                = local.common_tags

  dimensions = { FunctionName = each.value }
}

# -- CloudWatch Dashboard -----------------------------------------------------
resource "aws_cloudwatch_dashboard" "hivemind" {
  dashboard_name = "${var.project}-${var.environment}"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Fleet Invocations"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            for k, name in var.function_names :
            ["AWS/Lambda", "Invocations", "FunctionName", name, { stat = "Sum", label = k }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Fleet Errors"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            for k, name in var.function_names :
            ["AWS/Lambda", "Errors", "FunctionName", name, { stat = "Sum", label = k }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Agent Worker Duration"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            ["AWS/Lambda", "Duration", "FunctionName", var.function_names["agent-worker"], { stat = "p50", label = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", var.function_names["agent-worker"], { stat = "p99", label = "p99" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "Agent Worker Concurrency"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            ["AWS/Lambda", "ConcurrentExecutions", "FunctionName", var.function_names["agent-worker"], { stat = "Maximum" }],
            ["AWS/Lambda", "Throttles", "FunctionName", var.function_names["agent-worker"], { stat = "Sum", color = "#d62728" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 12
        height = 6
        properties = {
          title  = "HiveMind - Agent Verdicts"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            [var.metrics_namespace, "AgentVerdicts", "Environment", var.environment, { stat = "Sum", label = "Verdicts" }],
            [var.metrics_namespace, "MemoryHits", "Environment", var.environment, { stat = "Sum", label = "Memory recalls" }],
            [var.metrics_namespace, "TasksRequeued", "Environment", var.environment, { stat = "Sum", label = "Re-queued", color = "#ff7f0e" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 12
        width  = 12
        height = 6
        properties = {
          title  = "HiveMind - Investigation Latency"
          view   = "timeSeries"
          region = data.aws_region.current.name
          period = 60
          metrics = [
            [var.metrics_namespace, "InvestigationDuration", "Environment", var.environment, { stat = "p50", label = "p50" }],
            [var.metrics_namespace, "InvestigationDuration", "Environment", var.environment, { stat = "p99", label = "p99" }]
          ]
        }
      }
    ]
  })
}
