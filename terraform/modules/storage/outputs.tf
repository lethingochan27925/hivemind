output "evidence_bucket_name" {
  value = aws_s3_bucket.evidence.bucket
}

output "evidence_bucket_arn" {
  value = aws_s3_bucket.evidence.arn
}

output "lambda_artifacts_bucket_name" {
  value = aws_s3_bucket.lambda_artifacts.bucket
}

output "lambda_artifacts_bucket_arn" {
  value = aws_s3_bucket.lambda_artifacts.arn
}

output "dashboard_bucket_name" {
  value = aws_s3_bucket.dashboard.bucket
}

output "dashboard_url" {
  value = "https://${aws_cloudfront_distribution.dashboard.domain_name}"
}
