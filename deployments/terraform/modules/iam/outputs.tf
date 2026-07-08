output "eks_cluster_role_arn" { value = aws_iam_role.eks_cluster.arn }
output "eks_nodes_role_arn"   { value = aws_iam_role.eks_nodes.arn }

output "ssm_prefix" {
  description = "SSM parameter prefix dung trong tat ca services"
  value       = local.ssm_prefix
}
