output "role_arns" {
  description = "Map service key -> IAM role ARN cho Lambda execution"
  value       = { for k, v in aws_iam_role.lambda : k => v.arn }
}

output "ssm_prefix" {
  description = "SSM parameter prefix — code doc secrets tu day luc runtime"
  value       = local.ssm_prefix
}

output "metrics_namespace" {
  description = "CloudWatch namespace cho custom metrics"
  value       = local.metrics_namespace
}

output "async_dlq_arn" {
  description = "SQS queue ARN cho failed async Lambda invocations (EventBridge, dispatcher fan-out)"
  value       = aws_sqs_queue.async_dlq.arn
}
