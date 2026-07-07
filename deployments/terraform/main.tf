# 1. Networking -- khong depend vao gi
module "networking" {
  source      = "./modules/networking"
  project     = var.project
  environment = var.environment
  vpc_cidr    = var.vpc_cidr
  az_count    = var.az_count
}

# 2. Storage -- khong depend vao gi
module "storage" {
  source      = "./modules/storage"
  project     = var.project
  environment = var.environment
}

# 3. ECR -- khong depend vao gi
module "ecr" {
  source      = "./modules/ecr"
  project     = var.project
  environment = var.environment
}

# 4. IAM base -- chi tao EKS roles + SSM, khong can OIDC
module "iam" {
  source      = "./modules/iam"
  project     = var.project
  environment = var.environment

  cockroachdb_connection_string = var.cockroachdb_connection_string
  cockroachdb_mcp_endpoint      = var.cockroachdb_mcp_endpoint
  bedrock_model_id              = var.bedrock_model_id
  bedrock_embedding_model_id    = var.bedrock_embedding_model_id
}

# 5. EKS -- can IAM base roles
module "eks" {
  source      = "./modules/eks"
  project     = var.project
  environment = var.environment

  kubernetes_version      = var.kubernetes_version
  private_subnet_ids      = module.networking.private_subnet_ids
  eks_control_plane_sg_id = module.networking.eks_control_plane_sg_id
  eks_cluster_role_arn    = module.iam.eks_cluster_role_arn
  eks_nodes_role_arn      = module.iam.eks_nodes_role_arn
  node_instance_type      = var.node_instance_type
  node_desired_size       = var.node_desired_size
  node_min_size           = var.node_min_size
  node_max_size           = var.node_max_size
}

# 6. IAM IRSA -- can OIDC tu EKS
module "iam_irsa" {
  source      = "./modules/iam_irsa"
  project     = var.project
  environment = var.environment

  eks_oidc_provider_arn      = module.eks.oidc_provider_arn
  eks_oidc_provider_url      = module.eks.oidc_provider_url
  k8s_namespace              = var.k8s_namespace
  evidence_bucket_arn        = module.storage.evidence_bucket_arn
  bedrock_model_id           = var.bedrock_model_id
  bedrock_embedding_model_id = var.bedrock_embedding_model_id
}

# 7. Monitoring -- can EKS cluster name
module "monitoring" {
  source      = "./modules/monitoring"
  project     = var.project
  environment = var.environment

  alert_email           = var.alert_email
  billing_threshold_usd = var.billing_threshold_usd
  eks_cluster_name      = module.eks.cluster_name
}
