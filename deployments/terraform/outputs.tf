output "vpc_id"           { value = module.networking.vpc_id }
output "eks_cluster_name" { value = module.eks.cluster_name }
output "eks_endpoint"     { value = module.eks.cluster_endpoint }
output "ecr_urls"         { value = module.ecr.repository_urls }
output "evidence_bucket"  { value = module.storage.evidence_bucket_name }
output "dashboard_url"    { value = module.storage.dashboard_url }
output "cloudwatch_url"   { value = module.monitoring.dashboard_url }
output "ssm_prefix"       { value = module.iam.ssm_prefix }

output "irsa_role_arns" {
  description = "Annotate cac ARN nay vao K8s ServiceAccounts"
  value = {
    agent_worker = module.iam_irsa.agent_worker_role_arn
    scoring_api  = module.iam_irsa.scoring_api_role_arn
    dispatcher   = module.iam_irsa.dispatcher_role_arn
    reaper       = module.iam_irsa.reaper_role_arn
  }
}

output "next_steps" {
  value = <<-EOT
    Infrastructure ready. Next steps:

    1. Update kubeconfig:
       aws eks update-kubeconfig --name ${module.eks.cluster_name} --region ${var.aws_region}

    2. Build va push images len ECR:
       agent-worker : ${module.ecr.repository_urls["agent-worker"]}
       scoring-api  : ${module.ecr.repository_urls["scoring-api"]}
       dispatcher   : ${module.ecr.repository_urls["dispatcher"]}
       dashboard    : ${module.ecr.repository_urls["dashboard"]}

    3. Apply K8s manifests:
       kubectl apply -f deployments/k8s/
  EOT
}
