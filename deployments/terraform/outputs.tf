output "ecr_urls" { value = module.ecr.repository_urls }
output "evidence_bucket" { value = module.storage.evidence_bucket_name }
output "dashboard_url" { value = module.storage.dashboard_url }
output "cloudwatch_url" { value = module.monitoring.dashboard_url }
output "ssm_prefix" { value = module.iam.ssm_prefix }

output "lambda_function_names" {
  description = "Ten function -- dung cho aws lambda invoke / logs tail"
  value       = module.lambda.function_names
}

output "lambda_function_arns" {
  value = module.lambda.function_arns
}

output "scoring_api_url" {
  description = "Function URL cua scoring-api"
  value       = try(module.lambda.function_urls["scoring-api"], null)
}

output "next_steps" {
  value = <<-EOT
    Infrastructure ready (Lambda edition).

    1. Xem fleet:
       aws lambda list-functions --query "Functions[?starts_with(FunctionName, '${var.project}-${var.environment}')].FunctionName"

    2. Invoke thu 1 worker:
       aws lambda invoke --function-name ${module.lambda.function_names["agent-worker"]} /dev/stdout

    3. Tail log:
       aws logs tail /aws/lambda/${module.lambda.function_names["agent-worker"]} --follow

    4. Bom transaction vao DB tu may local:
       python scripts/demo-stream.py --mode replay --limit 500

    5. Demo chaos (AWS tu giet agent, reaper cuu):
       terraform apply -var="agent_worker_timeout_seconds=3"
  EOT
}
