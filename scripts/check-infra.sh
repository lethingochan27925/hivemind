#!/usr/bin/env bash
set -euo pipefail

# -- Load .env ----------------------------------------------------------------
ENV_FILE=".env"
[ -f "$ENV_FILE" ] || { echo "[ERROR] .env not found."; exit 1; }
set -a; source "$ENV_FILE"; set +a

# -- Config -------------------------------------------------------------------
PROJECT=${PROJECT:-"hivemind"}
ENVIRONMENT=${ENVIRONMENT:-"dev"}
AWS_REGION=${AWS_DEFAULT_REGION:-"ap-southeast-1"}

log()     { echo "[$(date '+%H:%M:%S')] [INFO]  $*"; }
section() { echo ""; echo "======================================"; echo "  $*"; echo "======================================"; }
none()    { echo "  (none)"; }

section "AWS ACCOUNT"
aws sts get-caller-identity --output table

# -- Tat ca resources co tag Project=hivemind ---------------------------------
section "ALL TAGGED RESOURCES (Project=${PROJECT})"
log "Querying all resources via Resource Groups Tagging API..."
aws resourcegroupstaggingapi get-resources \
  --region "$AWS_REGION" \
  --tag-filters "Key=Project,Values=${PROJECT}" \
  --query "ResourceTagMappingList[].[ResourceARN]" \
  --output table 2>/dev/null || none

# -- Global resources (khong co region) ---------------------------------------
section "GLOBAL RESOURCES"

log "S3 buckets..."
aws s3api list-buckets \
  --query "Buckets[?starts_with(Name, '${PROJECT}')].[Name,CreationDate]" \
  --output table 2>/dev/null || none

log "IAM roles..."
aws iam list-roles \
  --query "Roles[?starts_with(RoleName, '${PROJECT}')].[RoleName,CreateDate]" \
  --output table 2>/dev/null || none

log "IAM users..."
aws iam list-users \
  --query "Users[?starts_with(UserName, '${PROJECT}')].[UserName,CreateDate]" \
  --output table 2>/dev/null || none

log "CloudFront distributions..."
aws cloudfront list-distributions \
  --query "DistributionList.Items[?contains(Comment, '${PROJECT}')].[Id,DomainName,Status]" \
  --output table 2>/dev/null || none

# -- Cost estimate ------------------------------------------------------------
section "COST ESTIMATE (last 24h)"
aws cloudwatch get-metric-statistics \
  --namespace AWS/Billing \
  --metric-name EstimatedCharges \
  --dimensions Name=Currency,Value=USD \
  --start-time "$(date -u -d '24 hours ago' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -v-24H '+%Y-%m-%dT%H:%M:%SZ')" \
  --end-time "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --period 86400 \
  --statistics Maximum \
  --query "Datapoints[].[Timestamp,Maximum]" \
  --output table \
  --region us-east-1 2>/dev/null || none

echo ""
echo "===================================================="
echo "  Check complete"
echo "  Project : ${PROJECT}"
echo "  Region  : ${AWS_REGION}"
echo "===================================================="
