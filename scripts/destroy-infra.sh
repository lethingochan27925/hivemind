#!/usr/bin/env bash
set -euo pipefail

TERRAFORM_DIR="terraform"

log()  { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
ok()   { printf '[%s] OK    %s\n' "$(date '+%H:%M:%S')" "$*"; }
warn() { printf '[%s] WARN  %s\n' "$(date '+%H:%M:%S')" "$*"; }
err()  { printf '[%s] ERROR %s\n' "$(date '+%H:%M:%S')" "$*" >&2; exit 1; }

command -v terraform >/dev/null || err "terraform not found"
command -v aws       >/dev/null || err "aws cli not found"
[ -f .env ] || err ".env not found"
[ -d "$TERRAFORM_DIR" ] || err "${TERRAFORM_DIR}/ not found"

log "Loading AWS credentials from .env"
AWS_ACCESS_KEY_ID=$(grep -oP '^AWS_ACCESS_KEY_ID\s*=\s*"?\K[^"]*' .env || true)
AWS_SECRET_ACCESS_KEY=$(grep -oP '^AWS_SECRET_ACCESS_KEY\s*=\s*"?\K[^"]*' .env || true)
[ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_SECRET_ACCESS_KEY" ] \
  || err "AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY not found in .env"
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

aws sts get-caller-identity >/dev/null || err "AWS credentials invalid or expired"

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGION=$(aws configure get region 2>/dev/null || true)
[ -n "$REGION" ] || REGION="ap-southeast-1"

cd "$TERRAFORM_DIR"
PROJECT=$(grep -oP '^project\s*=\s*"\K[^"]+' terraform.tfvars || true)
ENVIRONMENT=$(grep -oP '^environment\s*=\s*"\K[^"]+' terraform.tfvars || true)
cd - >/dev/null

[ -n "$PROJECT" ] && [ -n "$ENVIRONMENT" ] || err "could not read project/environment from terraform.tfvars"

echo ""
echo "This will PERMANENTLY DELETE all AWS infrastructure for:"
echo "  Project:     ${PROJECT}"
echo "  Environment: ${ENVIRONMENT}"
echo "  Account:     ${ACCOUNT_ID}"
echo "  Region:      ${REGION}"
echo ""
read -p "Type the environment name (${ENVIRONMENT}) to confirm: " confirm
[ "$confirm" = "$ENVIRONMENT" ] || { log "Confirmation did not match, aborted"; exit 0; }

log "Loading Terraform secrets from .env"
source scripts/load-tf-vars.sh

log "Emptying S3 buckets (Terraform cannot destroy non-empty buckets)"
for bucket_suffix in dashboard evidence lambda-artifacts; do
  bucket_name="${PROJECT}-${ENVIRONMENT}-${bucket_suffix}"
  if aws s3api head-bucket --bucket "$bucket_name" 2>/dev/null; then
    aws s3 rm "s3://${bucket_name}" --recursive >/dev/null
    ok "  Emptied ${bucket_name}"
  else
    log "  ${bucket_name}: does not exist, skipping"
  fi
done

log "Deleting images from ECR repositories (Terraform cannot destroy non-empty repos)"
declare -a REPOS=(agent-worker dispatcher reaper salience-decay scoring-api scoring-python review-api)

for repo in "${REPOS[@]}"; do
  repo_name="${PROJECT}/${ENVIRONMENT}/${repo}"
  image_ids=$(aws ecr list-images --repository-name "$repo_name" --region "$REGION" --query 'imageIds' --output json 2>/dev/null || echo "[]")

  if [ "$image_ids" != "[]" ] && [ -n "$image_ids" ]; then
    aws ecr batch-delete-image \
      --repository-name "$repo_name" \
      --image-ids "$image_ids" \
      --region "$REGION" >/dev/null
    ok "  Deleted images from ${repo_name}"
  else
    log "  ${repo_name}: no images or repository does not exist, skipping"
  fi
done

log "Running terraform destroy"
cd "$TERRAFORM_DIR"
terraform init >/dev/null
terraform destroy -auto-approve
cd - >/dev/null

ok "Infrastructure destroyed"

log "Checking for remaining OIDC provider"
OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/token.actions.githubusercontent.com"
if aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$OIDC_ARN" >/dev/null 2>&1; then
  warn "OIDC provider still exists on AWS: ${OIDC_ARN}"
  warn "This may be shared with other projects - not deleted automatically."
  warn "Delete manually if you're sure it's unused:"
  echo "  aws iam delete-open-id-connect-provider --open-id-connect-provider-arn \"${OIDC_ARN}\""
else
  ok "OIDC provider removed"
fi

echo ""
echo "Destroy complete. To rebuild from scratch, run:"
echo "  bash scripts/bootstrap.sh"
