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
TERRAFORM_DIR="deployments/terraform"
BOOTSTRAP_DIR="${TERRAFORM_DIR}/modules/bootstrap"

log()  { echo "[$(date '+%H:%M:%S')] [INFO]  $*"; }
ok()   { echo "[$(date '+%H:%M:%S')] [OK]    $*"; }
warn() { echo "[$(date '+%H:%M:%S')] [WARN]  $*"; }
err()  { echo "[$(date '+%H:%M:%S')] [ERROR] $*" >&2; exit 1; }

# -- Xac nhan truoc khi destroy -----------------------------------------------
log "WARNING: This will destroy ALL HiveMind infrastructure."
log "Project     : ${PROJECT}"
log "Environment : ${ENVIRONMENT}"
log "Region      : ${AWS_REGION}"
echo ""
read -r -p "Type 'yes' to confirm: " CONFIRM
[ "$CONFIRM" = "yes" ] || err "Aborted."

# -- Step 1: Empty S3 buckets truoc khi destroy -------------------------------
log "Step 1/4: Emptying S3 buckets..."

empty_bucket() {
  local BUCKET="$1"
  if aws s3api head-bucket --bucket "$BUCKET" 2>/dev/null; then
    log "Emptying bucket: ${BUCKET}..."
    aws s3 rm "s3://${BUCKET}" --recursive 2>/dev/null || true
    # Xoa tat ca versions
    aws s3api list-object-versions --bucket "$BUCKET" \
      --output json 2>/dev/null | \
      python3 -c "
import json, sys, subprocess
data = json.load(sys.stdin)
objs = []
for v in data.get('Versions') or []:
    objs.append({'Key': v['Key'], 'VersionId': v['VersionId']})
for m in data.get('DeleteMarkers') or []:
    objs.append({'Key': m['Key'], 'VersionId': m['VersionId']})
if objs:
    subprocess.run(['aws', 's3api', 'delete-objects',
      '--bucket', '${BUCKET}',
      '--delete', json.dumps({'Objects': objs, 'Quiet': True})])
" 2>/dev/null || true
    ok "Bucket ${BUCKET} emptied"
  else
    warn "Bucket ${BUCKET} not found, skipping"
  fi
}

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

empty_bucket "${PROJECT}-${ENVIRONMENT}-evidence"
empty_bucket "${PROJECT}-${ENVIRONMENT}-lambda-artifacts"
empty_bucket "${PROJECT}-${ENVIRONMENT}-dashboard"

# -- Step 2: Destroy root module ----------------------------------------------
log "Step 2/4: Destroying main infrastructure..."
cd "$TERRAFORM_DIR"

terraform destroy -input=false -auto-approve \
  -var="cockroachdb_connection_string=${DATABASE_URL}" \
  -var="cockroachdb_mcp_endpoint=${COCKROACHDB_MCP_ENDPOINT}"

ok "Main infrastructure destroyed"
cd - >/dev/null

# -- Step 3: Destroy bootstrap ------------------------------------------------
log "Step 3/4: Destroying bootstrap (S3 tfstate + DynamoDB)..."

TFSTATE_BUCKET="${PROJECT}-tfstate-${ACCOUNT_ID}"
empty_bucket "$TFSTATE_BUCKET"

cd "$BOOTSTRAP_DIR"

# Remove prevent_destroy resources khoi state de co the destroy
terraform state rm aws_s3_bucket.tfstate 2>/dev/null || true
terraform state rm aws_dynamodb_table.tfstate_lock 2>/dev/null || true

# Xoa bang AWS CLI truc tiep
aws s3api delete-bucket --bucket "$TFSTATE_BUCKET" --region "$AWS_REGION" 2>/dev/null || true
aws dynamodb delete-table --table-name "${PROJECT}-tfstate-lock" --region "$AWS_REGION" 2>/dev/null || true

ok "Bootstrap destroyed"
cd - >/dev/null

# -- Step 4: Kiem tra con sot lai gi khong ------------------------------------
log "Step 4/4: Checking for remaining billable resources..."
echo ""

FOUND=0

EKS=$(aws eks list-clusters --region "$AWS_REGION" \
  --query "clusters[?starts_with(@, '${PROJECT}')]" \
  --output text 2>/dev/null)
[ -n "$EKS" ] && warn "EKS clusters still running: ${EKS}" && FOUND=1

EC2=$(aws ec2 describe-instances --region "$AWS_REGION" \
  --filters "Name=tag:Project,Values=${PROJECT}" \
            "Name=instance-state-name,Values=running,stopped" \
  --query "Reservations[].Instances[].InstanceId" \
  --output text 2>/dev/null)
[ -n "$EC2" ] && warn "EC2 instances still exist: ${EC2}" && FOUND=1

NAT=$(aws ec2 describe-nat-gateways --region "$AWS_REGION" \
  --filter "Name=tag:Project,Values=${PROJECT}" \
           "Name=state,Values=available,pending" \
  --query "NatGateways[].NatGatewayId" \
  --output text 2>/dev/null)
[ -n "$NAT" ] && warn "NAT Gateways still running: ${NAT}" && FOUND=1

EIP=$(aws ec2 describe-addresses --region "$AWS_REGION" \
  --filters "Name=tag:Project,Values=${PROJECT}" \
  --query "Addresses[].AllocationId" \
  --output text 2>/dev/null)
[ -n "$EIP" ] && warn "Elastic IPs still allocated: ${EIP}" && FOUND=1

LB=$(aws elbv2 describe-load-balancers --region "$AWS_REGION" \
  --query "LoadBalancers[?contains(LoadBalancerName, '${PROJECT}')].LoadBalancerArn" \
  --output text 2>/dev/null)
[ -n "$LB" ] && warn "Load Balancers still running: ${LB}" && FOUND=1

S3=$(aws s3api list-buckets \
  --query "Buckets[?starts_with(Name, '${PROJECT}')].Name" \
  --output text 2>/dev/null)
[ -n "$S3" ] && warn "S3 buckets still exist: ${S3}" && FOUND=1

CF=$(aws cloudfront list-distributions \
  --query "DistributionList.Items[?contains(Comment, '${PROJECT}')].Id" \
  --output text 2>/dev/null)
[ -n "$CF" ] && warn "CloudFront distributions still active: ${CF}" && FOUND=1

ECR=$(aws ecr describe-repositories --region "$AWS_REGION" \
  --query "repositories[?starts_with(repositoryName, '${PROJECT}')].repositoryName" \
  --output text 2>/dev/null)
[ -n "$ECR" ] && warn "ECR repositories still exist (minimal cost): ${ECR}" && FOUND=1

echo ""
if [ "$FOUND" -eq 0 ]; then
  ok "No billable resources found. Infrastructure fully destroyed."
else
  warn "Some resources still exist. Check above and destroy manually if needed."
fi

echo ""
echo "===================================================="
echo "  HiveMind infrastructure destroyed"
echo "  Project : ${PROJECT}"
echo "  Region  : ${AWS_REGION}"
echo "===================================================="
