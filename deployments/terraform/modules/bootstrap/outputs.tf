output "tfstate_bucket_name" {
  description = "S3 bucket name for Terraform remote state"
  value       = aws_s3_bucket.tfstate.bucket
}

output "tfstate_lock_table_name" {
  description = "DynamoDB table name for state locking"
  value       = aws_dynamodb_table.tfstate_lock.name
}

output "tfstate_region" {
  description = "AWS region where tfstate bucket is created"
  value       = local.region
}

output "backend_config_snippet" {
  description = "Copy snippet này vào versions.tf backend block của root module"
  value = <<-EOT
    backend "s3" {
      bucket         = "${aws_s3_bucket.tfstate.bucket}"
      key            = "${var.project}/${var.environment}/terraform.tfstate"
      region         = "${local.region}"
      dynamodb_table = "${aws_dynamodb_table.tfstate_lock.name}"
      encrypt        = true
    }
  EOT
}
