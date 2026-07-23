# =============================================================================
# HiveMind infrastructure -- Lambda edition
#
# Agent = Lambda: khong o dia, khong RAM giua cac invocation, chet sau 20s.
# Toan bo state nam trong CockroachDB. Do la cai demo can chung minh.
#
# Dependency graph phang, khong cycle:
#   storage ─┐
#   ecr    ──┼─> iam ──> lambda ──> monitoring
# =============================================================================

data "aws_caller_identity" "current" {}

locals {
  name_prefix = "${var.project}-${var.environment}"

  # Single source of truth: moi module deu derive tu day.
  function_names = { for s in var.services : s => "${local.name_prefix}-${s}" }

  image_uris = {
    for s in var.services : s => "${module.ecr.repository_urls[s]}:${var.image_tag}"
  }

  bedrock_region = coalesce(var.bedrock_region, var.aws_region)
}

# -- 1. Storage: evidence bucket + dashboard (S3 + CloudFront) ----------------
module "storage" {
  source      = "./modules/storage"
  project     = var.project
  environment = var.environment
}

# -- 2. ECR: 1 repo cho moi Lambda container image ----------------------------
module "ecr" {
  source       = "./modules/ecr"
  project      = var.project
  environment  = var.environment
  repositories = var.services
}

# -- 3. IAM: SSM params + 1 execution role cho moi Lambda ---------------------
# Gop iam + iam_irsa lai lam mot: cycle EKS<->OIDC khong con ton tai.
module "iam" {
  source      = "./modules/iam"
  project     = var.project
  environment = var.environment

  function_names      = local.function_names
  evidence_bucket_arn = module.storage.evidence_bucket_arn
  log_retention_days  = var.lambda_log_retention_days

  cockroachdb_connection_string = var.cockroachdb_connection_string
  cockroachdb_mcp_endpoint      = var.cockroachdb_mcp_endpoint

  bedrock_model_id           = var.bedrock_model_id
  bedrock_embedding_model_id = var.bedrock_embedding_model_id
  bedrock_region             = local.bedrock_region
  bedrock_embedding_region   = var.bedrock_embedding_region
}


module "lambda" {
  source      = "./modules/lambda"
  project     = var.project
  environment = var.environment

  function_names     = local.function_names
  image_uris         = local.image_uris
  role_arns          = module.iam.role_arns
  log_retention_days = var.lambda_log_retention_days

  common_env = {
    PROJECT                  = var.project
    ENVIRONMENT              = var.environment
    SSM_PREFIX               = module.iam.ssm_prefix
    EVIDENCE_BUCKET          = module.storage.evidence_bucket_name
    BEDROCK_REGION           = local.bedrock_region
    BEDROCK_EMBEDDING_REGION = var.bedrock_embedding_region
    METRICS_NAMESPACE        = module.iam.metrics_namespace
  }

  function_config = {
    "agent-worker" = {
      timeout_seconds      = var.agent_worker_timeout_seconds
      memory_mb            = var.agent_worker_memory_mb
      reserved_concurrency = var.agent_worker_reserved_concurrency
      environment = {
        BEDROCK_MODEL_ID           = var.bedrock_model_id
        BEDROCK_EMBEDDING_MODEL_ID = var.bedrock_embedding_model_id
        MEMORY_TOP_K               = tostring(var.memory_top_k)
        CHAOS_KILL_RATE            = tostring(var.chaos_kill_rate)
      }
    }

    "scoring-api" = {
      timeout_seconds      = var.scoring_api_timeout_seconds
      memory_mb            = var.scoring_api_memory_mb
      reserved_concurrency = -1
      environment = {
        RISK_LOW_THRESHOLD    = tostring(var.risk_low_threshold)
        RISK_HIGH_THRESHOLD   = tostring(var.risk_high_threshold)
      }
    }

    "scoring-python" = {
      timeout_seconds      = var.scoring_python_timeout_seconds
      memory_mb            = var.scoring_python_memory_mb
      reserved_concurrency = -1
      environment          = {}
    }

    "dispatcher" = {
      timeout_seconds      = var.dispatcher_timeout_seconds
      memory_mb            = var.dispatcher_memory_mb
      reserved_concurrency = 1
      environment = {
        DISPATCHER_BATCH_SIZE         = tostring(var.dispatcher_batch_size)
        DISPATCHER_MAX_WORKER_INVOKES = tostring(var.dispatcher_max_worker_invokes)
        WORKER_FUNCTION_NAME          = local.function_names["agent-worker"]
      }
    }

    "reaper" = {
      timeout_seconds      = var.reaper_timeout_seconds
      memory_mb            = var.reaper_memory_mb
      reserved_concurrency = 1
      environment = {
        REAPER_STUCK_THRESHOLD_SECONDS = tostring(var.reaper_stuck_threshold_seconds)
      }
    }

    "salience-decay" = {
      timeout_seconds      = var.salience_decay_timeout_seconds
      memory_mb            = var.salience_decay_memory_mb
      reserved_concurrency = 1
      environment          = {}
    }

    "review-api" = {
      timeout_seconds      = var.review_api_timeout_seconds
      memory_mb            = var.review_api_memory_mb
      reserved_concurrency = -1
      environment          = {}
    }
  }

  schedules = {
    "dispatcher" = {
      schedule_expression = var.dispatcher_schedule_expression
      enabled             = var.schedules_enabled
    }
    "reaper" = {
      schedule_expression = var.reaper_schedule_expression
      enabled             = var.schedules_enabled
    }
    "salience-decay" = {
      schedule_expression = var.salience_decay_schedule_expression
      enabled             = var.schedules_enabled
    }
  }

  function_url_services  = ["scoring-api", "scoring-python", "review-api"]
  function_url_auth_type = var.scoring_api_url_auth_type

  canary_services = ["agent-worker", "scoring-api", "scoring-python", "review-api"]
}
module "monitoring" {
  source      = "./modules/monitoring"
  project     = var.project
  environment = var.environment

  alert_email           = var.alert_email
  billing_threshold_usd = var.billing_threshold_usd
  function_names        = local.function_names
  metrics_namespace     = module.iam.metrics_namespace
}
