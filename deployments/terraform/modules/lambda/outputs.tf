output "function_names" {
  description = "Map service key -> function name"
  value       = { for k, v in aws_lambda_function.functions : k => v.function_name }
}

output "function_arns" {
  description = "Map service key -> function ARN"
  value       = { for k, v in aws_lambda_function.functions : k => v.arn }
}

output "function_urls" {
  description = "Map service key -> Function URL"
  value       = { for k, v in aws_lambda_function_url.endpoints : k => v.function_url }
}

output "log_group_names" {
  value = { for k, v in aws_cloudwatch_log_group.functions : k => v.name }
}
