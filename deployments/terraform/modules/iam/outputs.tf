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
