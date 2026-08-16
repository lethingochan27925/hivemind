output "tfstate_bucket_name" {
  description = "S3 bucket name for Terraform remote state"
  value       = aws_s3_bucket.tfstate.bucket
}

output "tfstate_region" {
  description = "AWS region where tfstate bucket is created"
  value       = local.region
}

output "backend_config_snippet" {
  description = "Copy snippet này vào versions.tf backend block của root module"
  value       = <<-EOT
    backend "s3" {
      bucket       = "${aws_s3_bucket.tfstate.bucket}"
      key          = "${var.project}/${var.environment}/terraform.tfstate"
      region       = "${local.region}"
      use_lockfile = true
      encrypt      = true
    }
  EOT
}
