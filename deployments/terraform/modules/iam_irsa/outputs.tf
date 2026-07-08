output "agent_worker_role_arn" { value = aws_iam_role.agent_worker.arn }
output "scoring_api_role_arn"  { value = aws_iam_role.scoring_api.arn }
output "dispatcher_role_arn"   { value = aws_iam_role.dispatcher.arn }
output "reaper_role_arn"       { value = aws_iam_role.reaper.arn }
