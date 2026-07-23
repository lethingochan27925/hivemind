terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.50"
    }
  }

  # tls  -- da bo: chi dung cho EKS OIDC thumbprint
  # archive -- da bo: Lambda dung container image tu ECR, khong dung zip

  backend "s3" {
    bucket       = "hivemind-tfstate-375916766707"
    key          = "hivemind/dev/terraform.tfstate"
    region       = "ap-southeast-1"
    use_lockfile = true
    encrypt      = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
