terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # tls  -- da bo: chi dung cho EKS OIDC thumbprint
  # archive -- da bo: Lambda dung container image tu ECR, khong dung zip
  # null -- khong khai bao tuong minh, Terraform tu suy ra tu
  #   resource "null_resource" "billing_alarm" (modules/monitoring/main.tf),
  #   giong cach tls duoc suy ra tu data "tls_certificate" ma khong can khai
  #   trong required_providers.

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

# No aws.billing provider alias here. The billing alarm needs the AWS/Billing
# "EstimatedCharges" metric, which only ever publishes in us-east-1 - but
# reaching it turned out to need working around a provider gap rather than
# just pointing a provider block at the right region. See the long comment on
# null_resource.billing_alarm in modules/monitoring/main.tf for what was
# tried and why.
