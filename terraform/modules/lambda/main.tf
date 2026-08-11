locals {
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    Module      = "lambda"
  }
}

# =============================================================================
# Log groups -- tao truoc, co retention.
# Neu de Lambda tu tao thi retention = never expire -> ton tien am tham.
# =============================================================================
resource "aws_cloudwatch_log_group" "functions" {
  for_each = var.function_names

  name              = "/aws/lambda/${each.value}"
  retention_in_days = var.log_retention_days
  tags              = merge(local.common_tags, { Name = each.value })
}

# =============================================================================
# Functions
# =============================================================================
resource "aws_lambda_function" "functions" {
  for_each = var.function_names

  function_name = each.value
  role          = var.role_arns[each.key]
  package_type  = "Image"
  image_uri     = var.image_uris[each.key]

  timeout                        = var.function_config[each.key].timeout_seconds
  memory_size                    = var.function_config[each.key].memory_mb
  reserved_concurrent_executions = var.function_config[each.key].reserved_concurrency
  publish                        = true

  environment {
    variables = merge(var.common_env, var.function_config[each.key].environment)
  }

  tags = merge(local.common_tags, { Name = each.value })

  depends_on = [aws_cloudwatch_log_group.functions]
}

resource "aws_lambda_alias" "live" {
  for_each = var.function_names

  name             = "live"
  function_name    = aws_lambda_function.functions[each.key].function_name
  function_version = aws_lambda_function.functions[each.key].version

  dynamic "routing_config" {
    for_each = contains(var.canary_services, each.key) ? [1] : []
    content {
      additional_version_weights = {}
    }
  }

  lifecycle {
    ignore_changes = [function_version, routing_config]
  }
}

# =============================================================================
# EventBridge schedules -- thay cho K8s CronJob
# =============================================================================
resource "aws_cloudwatch_event_rule" "schedules" {
  for_each = var.schedules

  name                = "${var.function_names[each.key]}-schedule"
  description         = "Trigger ${var.function_names[each.key]}"
  schedule_expression = each.value.schedule_expression
  state               = each.value.enabled ? "ENABLED" : "DISABLED"
  tags                = local.common_tags
}

resource "aws_cloudwatch_event_target" "schedules" {
  for_each = var.schedules

  rule      = aws_cloudwatch_event_rule.schedules[each.key].name
  target_id = each.key
  arn       = aws_lambda_alias.live[each.key].arn
}

resource "aws_lambda_permission" "eventbridge" {
  for_each = var.schedules

  statement_id  = "AllowInvokeFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_alias.live[each.key].function_name
  qualifier     = aws_lambda_alias.live[each.key].name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedules[each.key].arn
}

# =============================================================================
# Function URLs
# =============================================================================
resource "aws_lambda_function_url" "endpoints" {
  for_each = var.function_url_services

  function_name      = aws_lambda_function.functions[each.key].function_name
  qualifier          = aws_lambda_alias.live[each.key].name
  authorization_type = each.value

  # Provider tu goi AddPermission("FunctionURLAllowPublicAccess") khi tao
  # Function URL voi auth NONE, va bo qua xung dot neu statement da ton tai.
  # Ep quyen khai bao tuong minh tao TRUOC de tan dung co che bo qua do:
  # neu nguoc thu tu, resource cua ta dinh 409 ResourceConflictException.
  depends_on = [
    aws_lambda_permission.function_url_public,
    aws_lambda_permission.function_url_invoke,
  ]
}

# Function URL voi authorization_type = "NONE" van can resource-based policy
# cho phep invoke public -- khong co thi AWS tra 403 truoc khi ham chay.
# Khai bao tuong minh thay vi dua vao hanh vi ngam cua provider, va phai gan
# dung qualifier cua alias ma URL tro toi (URL nam tren alias, khong phai $LATEST).
# Chi tao cho service that su de NONE -> derive tu cung mot source of truth.
resource "aws_lambda_permission" "function_url_public" {
  for_each = { for svc, auth in var.function_url_services : svc => auth if auth == "NONE" }

  statement_id           = "FunctionURLAllowPublicAccess"
  action                 = "lambda:InvokeFunctionUrl"
  function_name          = aws_lambda_function.functions[each.key].function_name
  qualifier              = aws_lambda_alias.live[each.key].name
  principal              = "*"
  function_url_auth_type = "NONE"
}

# Tu 10/2025 AWS yeu cau Function URL public phai co CA HAI quyen:
# lambda:InvokeFunctionUrl (resource tren) va lambda:InvokeFunction (resource nay).
# Thieu mot trong hai -> 403 AccessDeniedException truoc khi ham kip chay.
# invoked_via_function_url la bat buoc ve bao mat: khong co no, Principal "*"
# cho phep goi thang ham qua Lambda API chu khong chi qua URL.
resource "aws_lambda_permission" "function_url_invoke" {
  for_each = { for svc, auth in var.function_url_services : svc => auth if auth == "NONE" }

  statement_id             = "FunctionURLInvokeAllowPublicAccess"
  action                   = "lambda:InvokeFunction"
  function_name            = aws_lambda_function.functions[each.key].function_name
  qualifier                = aws_lambda_alias.live[each.key].name
  principal                = "*"
  invoked_via_function_url = true
}
