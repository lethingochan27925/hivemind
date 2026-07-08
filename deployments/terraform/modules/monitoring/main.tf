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

# -- EKS Node CPU Alarm -------------------------------------------------------
resource "aws_cloudwatch_metric_alarm" "eks_node_cpu" {
  alarm_name          = "${var.project}-${var.environment}-eks-node-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "node_cpu_utilization"
  namespace           = "ContainerInsights"
  period              = 60
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "EKS node CPU > 80%"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  tags                = local.common_tags

  dimensions = { ClusterName = var.eks_cluster_name }
}

# -- EKS Node Memory Alarm ----------------------------------------------------
resource "aws_cloudwatch_metric_alarm" "eks_node_memory" {
  alarm_name          = "${var.project}-${var.environment}-eks-node-memory"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "node_memory_utilization"
  namespace           = "ContainerInsights"
  period              = 60
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "EKS node memory > 80%"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  tags                = local.common_tags

  dimensions = { ClusterName = var.eks_cluster_name }
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
        width  = 8
        height = 6
        properties = {
          title   = "EKS Node CPU Utilization"
          view    = "timeSeries"
          region  = data.aws_region.current.name
          period  = 60
          metrics = [["ContainerInsights", "node_cpu_utilization", "ClusterName", var.eks_cluster_name, { stat = "Average" }]]
        }
      },
      {
        type   = "metric"
        x      = 8
        y      = 0
        width  = 8
        height = 6
        properties = {
          title   = "EKS Node Memory Utilization"
          view    = "timeSeries"
          region  = data.aws_region.current.name
          period  = 60
          metrics = [["ContainerInsights", "node_memory_utilization", "ClusterName", var.eks_cluster_name, { stat = "Average" }]]
        }
      },
      {
        type   = "metric"
        x      = 16
        y      = 0
        width  = 8
        height = 6
        properties = {
          title   = "Pod Restart Count"
          view    = "timeSeries"
          region  = data.aws_region.current.name
          period  = 60
          metrics = [["ContainerInsights", "pod_number_of_container_restarts", "ClusterName", var.eks_cluster_name, { stat = "Sum" }]]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "HiveMind - Agent Verdicts"
          view    = "timeSeries"
          region  = data.aws_region.current.name
          period  = 60
          metrics = [
            ["HiveMind", "AgentVerdicts", "Environment", var.environment, { stat = "Sum", label = "Total Verdicts" }],
            ["HiveMind", "AgentErrors", "Environment", var.environment, { stat = "Sum", label = "Errors", color = "#d62728" }]
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
          title   = "HiveMind - Investigation Latency"
          view    = "timeSeries"
          region  = data.aws_region.current.name
          period  = 60
          metrics = [
            ["HiveMind", "InvestigationDuration", "Environment", var.environment, { stat = "p50", label = "p50" }],
            ["HiveMind", "InvestigationDuration", "Environment", var.environment, { stat = "p99", label = "p99" }]
          ]
        }
      }
    ]
  })
}
