output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.main.id
}

output "vpc_cidr" {
  description = "VPC CIDR block"
  value       = aws_vpc.main.cidr_block
}

output "public_subnet_ids" {
  description = "List public subnet IDs"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "List private subnet IDs"
  value       = aws_subnet.private[*].id
}

output "eks_nodes_sg_id" {
  description = "Security group ID cho EKS nodes"
  value       = aws_security_group.eks_nodes.id
}

output "eks_control_plane_sg_id" {
  description = "Security group ID cho EKS control plane"
  value       = aws_security_group.eks_control_plane.id
}

output "lambda_sg_id" {
  description = "Security group ID cho Lambda functions"
  value       = aws_security_group.lambda.id
}
