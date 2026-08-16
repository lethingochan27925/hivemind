locals {
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "monitoring"
  }
}

data "aws_region" "current" {}
data "aws_caller_identity" "current" {}

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
# AWS only publishes the AWS/Billing "EstimatedCharges" metric in us-east-1,
# regardless of which region the billed resources run in. This alarm is NOT
# the native aws_cloudwatch_metric_alarm resource - it shells out to the AWS
# CLI instead, and that took three attempts to get right, in order:
#
#   1. `provider = aws.billing` (a provider alias pinned to us-east-1) and
#      later `region = "us-east-1"` (AWS provider v6's per-resource argument)
#      both failed apply with "Invalid region ap-southeast-1 specified. Only
#      us-east-1 is supported." even though `terraform plan` showed the
#      resource's resolved region as us-east-1 correctly in both cases.
#   2. Switching to a raw `aws cloudwatch put-metric-alarm --region us-east-1`
#      call via null_resource + local-exec hit the *exact same* error message
#      - which is what exposed that this was never actually about which
#      regional endpoint received the request. A bare put-metric-alarm call
#      with no alarm_actions, typed by hand, succeeded immediately.
#   3. The one difference: `alarm_actions` pointed at aws_sns_topic.alerts,
#      which lives in ap-southeast-1 (the default provider region, correct
#      for the Lambda/Bedrock alarms below that share it). AWS requires a
#      billing alarm's SNS target to live in us-east-1 too, and reports that
#      mismatch with the identical "Invalid region {region} specified. Only
#      us-east-1 is supported." wording used for the alarm's own region - the
#      two failure modes are indistinguishable from the error text alone.
#      (docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/gs_monitor_estimated_charges_with_cloudwatch.html:
#      "you must set your Region to US East (N. Virginia)" before creating a
#      billing alarm - which in practice means everything it points at, too.)
#
# So: its own SNS topic in us-east-1, created the same way, not the shared
# ap-southeast-1 one above.
locals {
  billing_sns_topic_name = "${var.project}-${var.environment}-billing-alerts"
  billing_sns_topic_arn  = "arn:aws:sns:us-east-1:${data.aws_caller_identity.current.account_id}:${local.billing_sns_topic_name}"
}

resource "null_resource" "billing_alarm" {
  triggers = {
    alarm_name     = "${var.project}-${var.environment}-billing"
    threshold      = var.billing_threshold_usd
    sns_topic_name = local.billing_sns_topic_name
    sns_topic_arn  = local.billing_sns_topic_arn
    alert_email    = var.alert_email
    project        = var.project
    env            = var.environment
  }

  # Explicit bash (not local-exec's default /bin/sh, which on this project's
  # dev machines resolves to dash) and an explicit AWS_REGION via Terraform's
  # own `environment` argument, in addition to every command's own --region -
  # belt and suspenders once a wrong-region error had already been chased
  # down two blind alleys once this session.
  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    environment = {
      AWS_REGION = "us-east-1"
    }
    command = <<-EOT
      set -euo pipefail
      aws sns create-topic --region us-east-1 --name "${self.triggers.sns_topic_name}" \
        --tags "Key=Project,Value=${self.triggers.project}" "Key=Environment,Value=${self.triggers.env}" "Key=ManagedBy,Value=terraform" "Key=Module,Value=monitoring" \
        >/dev/null
      aws sns subscribe --region us-east-1 --topic-arn "${self.triggers.sns_topic_arn}" \
        --protocol email --notification-endpoint "${self.triggers.alert_email}" \
        >/dev/null
      aws cloudwatch put-metric-alarm --region us-east-1 \
        --alarm-name "${self.triggers.alarm_name}" \
        --alarm-description "${format("AWS spend exceeded \\$%s", self.triggers.threshold)}" \
        --namespace "AWS/Billing" \
        --metric-name "EstimatedCharges" \
        --dimensions "Name=Currency,Value=USD" \
        --statistic Maximum \
        --period 86400 \
        --evaluation-periods 1 \
        --threshold "${self.triggers.threshold}" \
        --comparison-operator GreaterThanThreshold \
        --treat-missing-data missing \
        --alarm-actions "${self.triggers.sns_topic_arn}" \
        --tags "Key=Project,Value=${self.triggers.project}" "Key=Environment,Value=${self.triggers.env}" "Key=ManagedBy,Value=terraform" "Key=Module,Value=monitoring"
    EOT
  }

  provisioner "local-exec" {
    when        = destroy
    interpreter = ["/bin/bash", "-c"]
    environment = {
      AWS_REGION = "us-east-1"
    }
    command = <<-EOT
      set -euo pipefail
      aws cloudwatch delete-alarms --region us-east-1 --alarm-names "${self.triggers.alarm_name}" || true
      topic_arn="arn:aws:sns:us-east-1:$(aws sts get-caller-identity --query Account --output text):${self.triggers.sns_topic_name}"
      aws sns delete-topic --region us-east-1 --topic-arn "$topic_arn" || true
    EOT
  }
}

# -- Bedrock throttling --------------------------------------------------------
# AWS/Bedrock publishes InvocationThrottles (SampleCount) per ModelId - the
# signal that the fleet's own retry/fallback logic (adaptive retry in
# pkg/bedrock, ruleBasedFallback in internal/agent/reasoning.go) is masking
# from every other alarm here: a throttled call never shows up as a Lambda
# Error, it shows up as a fallback verdict and a slower response, silently.
# Without this, sustained throttling had no CloudWatch signal at all - see
# docs.aws.amazon.com/bedrock/latest/userguide/monitoring-runtime-metrics.html.
resource "aws_cloudwatch_metric_alarm" "bedrock_throttles" {
  for_each = {
    claude = var.bedrock_model_id
    titan  = var.bedrock_embedding_model_id
  }

  alarm_name          = "${var.project}-${var.environment}-bedrock-throttles-${each.key}"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "InvocationThrottles"
  namespace           = "AWS/Bedrock"
  period              = 300
  statistic           = "Sum"
  threshold           = 0
  alarm_description   = "Bedrock is throttling ${each.key} (${each.value}) - the fleet is falling back to rule-based verdicts"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  treat_missing_data  = "notBreaching"
  tags                = local.common_tags

  dimensions = { ModelId = each.value }
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
          region = data.aws_region.current.region
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
          region = data.aws_region.current.region
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
          region = data.aws_region.current.region
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
          region = data.aws_region.current.region
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
          region = data.aws_region.current.region
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
          region = data.aws_region.current.region
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
