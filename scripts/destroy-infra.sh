#!/usr/bin/env bash
set -euo pipefail

# AWS CLI v2 pipes any output it decides is "long enough" through `less` by
# default when stdout is a terminal - `s3api head-bucket` in particular
# started returning a small JSON body (BucketRegion, AccessPointAlias) on
# success in a recent CLI version where it used to print nothing, and that
# was enough to trigger the pager: every head-bucket call left `less`'s own
# chrome (~ padding, (END)) bleeding into this script's output, looking like
# a hang or a broken loop even though the script was working correctly.
export AWS_PAGER=""

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
# `aws s3 rm --recursive` alone is not enough on a versioned bucket (evidence,
# lambda-artifacts): it only adds delete markers over the current objects,
# leaving every prior version behind, so S3 still refuses DeleteBucket with
# BucketNotEmpty. Purge Versions + DeleteMarkers via s3api too, every time -
# harmless no-op on the non-versioned dashboard bucket. This used to live in
# a separate loop after `cd "$TERRAFORM_DIR"` that read bucket names via
# `terraform -chdir=terraform output` - chdir'ing to "terraform" while
# already inside terraform/ resolves to a directory that doesn't exist, so
# that loop silently iterated zero times and never actually ran.
for bucket_suffix in dashboard evidence lambda-artifacts; do
  bucket_name="${PROJECT}-${ENVIRONMENT}-${bucket_suffix}"
  if aws s3api head-bucket --bucket "$bucket_name" >/dev/null 2>&1; then
    aws s3 rm "s3://${bucket_name}" --recursive >/dev/null

    aws s3api list-object-versions --bucket "$bucket_name" --output json \
      --query '{Objects: Versions[].{Key:Key,VersionId:VersionId}}' > /tmp/_v.json 2>/dev/null \
      && aws s3api delete-objects --bucket "$bucket_name" --delete file:///tmp/_v.json >/dev/null 2>&1 || true
    aws s3api list-object-versions --bucket "$bucket_name" --output json \
      --query '{Objects: DeleteMarkers[].{Key:Key,VersionId:VersionId}}' > /tmp/_d.json 2>/dev/null \
      && aws s3api delete-objects --bucket "$bucket_name" --delete file:///tmp/_d.json >/dev/null 2>&1 || true

    ok "  Emptied ${bucket_name}"
  else
    log "  ${bucket_name}: does not exist, skipping"
  fi
done

log "Deleting images from ECR repositories (Terraform cannot destroy non-empty repos)"
declare -a REPOS=(agent-worker dispatcher reaper salience-decay scoring-api scoring-python dashboard-api)

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
echo "  bash scripts/init.sh"
