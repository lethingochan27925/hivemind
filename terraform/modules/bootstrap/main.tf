data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.region

  bucket_name = "${var.project}-tfstate-${local.account_id}"

  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "bootstrap"
  }
}

resource "aws_s3_bucket" "tfstate" {
  bucket = local.bucket_name
  tags   = merge(local.common_tags, { Name = local.bucket_name })

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# No DynamoDB lock table here: the root module's backend (versions.tf) uses
# S3 native locking (use_lockfile = true), not the older DynamoDB-table
# pattern. A DynamoDB table used to be declared in this module for that
# purpose and was never removed after the backend moved on - if you already
# applied this module for real, that table (<project>-tfstate-lock) is now
# unused and safe to delete by hand; it was never referenced by the actual
# backend configuration.
