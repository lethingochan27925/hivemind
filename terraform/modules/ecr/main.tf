# Repo "dashboard" da bo: dashboard la static site -> S3 + CloudFront
# (module storage), khong can container image.
# Repo "reaper" duoc them: reaper chay nhu 1 Lambda rieng.

locals {
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "ecr"
  }
}

resource "aws_ecr_repository" "services" {
  for_each = toset(var.repositories)

  name                 = "${var.project}/${var.environment}/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = merge(local.common_tags, { Name = "${var.project}/${var.environment}/${each.key}" })
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each   = aws_ecr_repository.services
  repository = each.value.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last ${var.image_retention_count} images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = var.image_retention_count
      }
      action = { type = "expire" }
    }]
  })
}
