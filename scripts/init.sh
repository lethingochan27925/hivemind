#!/usr/bin/env bash
set -euo pipefail

TERRAFORM_DIR="terraform"
SKIP_BUILD_IMAGE=false

for arg in "$@"; do
  case "$arg" in
    --skip-build-image)
      SKIP_BUILD_IMAGE=true
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      echo "Usage: $0 [--skip-build-image]" >&2
      exit 1
      ;;
  esac
done

log()  { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
ok()   { printf '[%s] OK    %s\n' "$(date '+%H:%M:%S')" "$*"; }
warn() { printf '[%s] WARN  %s\n' "$(date '+%H:%M:%S')" "$*"; }
err()  { printf '[%s] ERROR %s\n' "$(date '+%H:%M:%S')" "$*" >&2; exit 1; }

ensure_jq() {
  if command -v jq >/dev/null 2>&1; then
    return
  fi
  log "jq not found, installing"
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -qq && sudo apt-get install -y -qq jq
  elif command -v brew >/dev/null 2>&1; then
    brew install jq
  else
    err "jq not found and no supported package manager (apt-get/brew) to auto-install it"
  fi
  command -v jq >/dev/null 2>&1 || err "jq installation failed"
  ok "jq installed"
}

ensure_gh() {
  if command -v gh >/dev/null 2>&1; then
    return
  fi
  log "gh CLI not found, installing"
  if command -v apt-get >/dev/null 2>&1; then
    (type -p wget >/dev/null || sudo apt-get install -y -qq wget) \
      && sudo mkdir -p -m 755 /etc/apt/keyrings \
      && wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg >/dev/null \
      && sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
      && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
        | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null \
      && sudo apt-get update -qq \
      && sudo apt-get install -y -qq gh
  elif command -v brew >/dev/null 2>&1; then
    brew install gh
  else
    warn "gh CLI not found and no supported package manager to auto-install it - secrets must be pushed manually"
    return
  fi
  command -v gh >/dev/null 2>&1 && ok "gh CLI installed" || warn "gh CLI installation failed - secrets must be pushed manually"
}

command -v terraform >/dev/null || err "terraform not found - install from https://developer.hashicorp.com/terraform/install"
command -v aws       >/dev/null || err "aws cli not found - install from https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
command -v docker    >/dev/null || err "docker not found - enable WSL integration in Docker Desktop settings"

ensure_jq
ensure_gh

[ -f .env ] || err ".env not found"
[ -d "$TERRAFORM_DIR" ] || err "${TERRAFORM_DIR}/ not found"

log "Loading AWS credentials from .env into this session"
AWS_ACCESS_KEY_ID=$(grep -oP '^AWS_ACCESS_KEY_ID\s*=\s*"?\K[^"]*' .env || true)
AWS_SECRET_ACCESS_KEY=$(grep -oP '^AWS_SECRET_ACCESS_KEY\s*=\s*"?\K[^"]*' .env || true)
[ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_SECRET_ACCESS_KEY" ] \
  || err "AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY not found in .env"
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

log "Verifying AWS credentials"
aws sts get-caller-identity >/dev/null || err "AWS credentials invalid or expired"
ok "AWS credentials valid"

log "Loading remaining secrets from .env"
source scripts/load-tf-vars.sh

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGION=$(aws configure get region 2>/dev/null || true)
[ -n "$REGION" ] || REGION="ap-southeast-1"
ECR_REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

GITHUB_REPO=$(git config --get remote.origin.url | sed -E 's#^(git@github\.com:|https://github\.com/)##; s#\.git$##')
[ -n "$GITHUB_REPO" ] || err "could not parse GitHub repo from git remote origin url"

ok "Account: ${ACCOUNT_ID} | Region: ${REGION} | Repo: ${GITHUB_REPO}"

log "Terraform init"
cd "$TERRAFORM_DIR"
terraform init
ok "Terraform initialized"

PROJECT=$(grep -oP '^project\s*=\s*"\K[^"]+' terraform.tfvars || true)
ENVIRONMENT=$(grep -oP '^environment\s*=\s*"\K[^"]+' terraform.tfvars || true)
[ -n "$PROJECT" ] && [ -n "$ENVIRONMENT" ] || err "could not read project/environment from terraform vars (check terraform.tfvars)"

OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/token.actions.githubusercontent.com"

log "Checking for existing OIDC provider"
if aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$OIDC_ARN" >/dev/null 2>&1; then
  if terraform state show module.github_oidc.aws_iam_openid_connect_provider.github >/dev/null 2>&1; then
    log "Already tracked in Terraform state, skipping import"
  else
    log "Provider exists on AWS but not in state, importing"
    terraform import module.github_oidc.aws_iam_openid_connect_provider.github "$OIDC_ARN"
  fi
else
  log "No existing provider, Terraform will create one"
fi
ok "OIDC provider ready"
cd - >/dev/null

log "Logging in to ECR"
if ! aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$ECR_REGISTRY"; then
  err "Docker login to ECR failed - check IAM permissions"
fi
ok "Docker authenticated"

log "Applying base infrastructure (storage, ECR, IAM, OIDC)"
cd "$TERRAFORM_DIR"
terraform apply -auto-approve \
  -target=module.storage \
  -target=module.ecr \
  -target=module.iam \
  -target=module.github_oidc
ok "Base infrastructure ready"
cd - >/dev/null

log "Verifying ECR repositories exist"
declare -A SERVICE_MAP=(
  [worker]=agent-worker
  [dispatcher]=dispatcher
  [heartbeat-reaper]=reaper
  [salience-decay]=salience-decay
  [scoring-api]=scoring-api
  [dashboard-api]=dashboard-api
)

for repo_name in "${SERVICE_MAP[@]}" scoring-python; do
  aws ecr describe-repositories \
    --repository-names "${PROJECT}/${ENVIRONMENT}/${repo_name}" \
    --region "$REGION" >/dev/null 2>&1 \
    || err "ECR repository ${PROJECT}/${ENVIRONMENT}/${repo_name} not found - base apply may have failed"
done
ok "All ECR repositories confirmed"

image_exists_in_ecr() {
  local repo_name="$1"
  aws ecr describe-images \
    --repository-name "${PROJECT}/${ENVIRONMENT}/${repo_name}" \
    --image-ids imageTag=latest \
    --region "$REGION" >/dev/null 2>&1
}

if [ "$SKIP_BUILD_IMAGE" = true ]; then
  log "Skip-build-image enabled: reusing existing ECR images where available"
else
  log "Building and pushing images (use --skip-build-image to reuse existing images)"
fi

for cmd_dir in "${!SERVICE_MAP[@]}"; do
  repo_name="${SERVICE_MAP[$cmd_dir]}"
  image="${ECR_REGISTRY}/${PROJECT}/${ENVIRONMENT}/${repo_name}:latest"

  if [ "$SKIP_BUILD_IMAGE" = true ] && image_exists_in_ecr "$repo_name"; then
    log "  ${repo_name}: found in ECR, skipping build"
    continue
  fi

  log "  ${repo_name}: building"
  docker build --provenance=false --sbom=false -f Dockerfile.lambda-go \
    --build-arg SERVICE_PATH="./cmd/${cmd_dir}" \
    -t "$image" . >/dev/null
  docker push "$image" >/dev/null
done

python_repo="scoring-python"
python_image="${ECR_REGISTRY}/${PROJECT}/${ENVIRONMENT}/${python_repo}:latest"

if [ "$SKIP_BUILD_IMAGE" = true ] && image_exists_in_ecr "$python_repo"; then
  log "  ${python_repo}: found in ECR, skipping build"
else
  log "  ${python_repo}: building"
  docker build --provenance=false --sbom=false -f services/scoring-python/Dockerfile -t "$python_image" . >/dev/null
  docker push "$python_image" >/dev/null
fi

ok "Images ready"

log "Applying full infrastructure (Lambda functions now have images)"
cd "$TERRAFORM_DIR"
terraform apply -auto-approve
ROLE_ARN=$(terraform output -raw github_actions_role_arn)
DASHBOARD_API_URL=$(terraform output -raw dashboard_api_url)
# Function URL cua Lambda luon ket thuc bang "/". Chuan hoa ngay tai bien he
# thong de frontend khong bao gio ghep ra path "//".
DASHBOARD_API_URL="${DASHBOARD_API_URL%/}"
SCORING_PYTHON_URL=$(terraform output -raw scoring_python_url)
DASHBOARD_BUCKET=$(terraform output -raw dashboard_bucket_name)
CLOUDFRONT_ID=$(aws cloudfront list-distributions \
  --query "DistributionList.Items[?Comment=='${PROJECT}-${ENVIRONMENT}-dashboard'].Id" \
  --output text)
cd - >/dev/null

# SSM param nay duoc Terraform tao voi gia tri "placeholder" va
# ignore_changes=[value] - dung y la de buoc apply ghi gia tri that vao,
# vi Function URL chi biet duoc SAU khi Lambda scoring-python ton tai.
log "Wiring scoring-python endpoint into SSM"
aws ssm put-parameter \
  --name "/${PROJECT}/${ENVIRONMENT}/scoring/python_endpoint" \
  --value "${SCORING_PYTHON_URL%/}/score" \
  --type String --overwrite --region "$REGION" >/dev/null
ok "Scoring endpoint: ${SCORING_PYTHON_URL%/}/score"

# Terraform khong phat hien thay doi khi tag anh van la ":latest", nen Lambda
# tiep tuc chay digest cu. Phai ep Lambda doc lai tag, roi tro alias "live" sang
# version moi - alias co ignore_changes=[function_version] nen Terraform khong
# tu lam viec nay. Thieu buoc nay, bootstrap chi deploy duoc code o lan dau tien.
log "Deploying pushed images to Lambda"
for repo_name in "${SERVICE_MAP[@]}" scoring-python; do
  fn="${PROJECT}-${ENVIRONMENT}-${repo_name}"
  image="${ECR_REGISTRY}/${PROJECT}/${ENVIRONMENT}/${repo_name}:latest"
  version=$(aws lambda update-function-code \
    --function-name "$fn" --image-uri "$image" --publish \
    --query Version --output text --region "$REGION") \
    || err "update-function-code failed for ${fn}"
  aws lambda wait function-updated --function-name "$fn" --region "$REGION"
  aws lambda update-alias --function-name "$fn" --name live \
    --function-version "$version" --region "$REGION" >/dev/null
  log "  ${fn} -> version ${version}"
done
ok "All Lambda functions running the newly pushed images"

ok "Backend infrastructure ready"

log "Building dashboard frontend (Next.js static export)"
cd dashboard
echo "NEXT_PUBLIC_DASHBOARD_API_URL=${DASHBOARD_API_URL}" > .env.production.local
npm install --no-audit --no-fund >/dev/null
npm run build >/dev/null
cd - >/dev/null
ok "Frontend built with API URL: ${DASHBOARD_API_URL}"

log "Uploading dashboard to S3"
aws s3 sync dashboard/out "s3://${DASHBOARD_BUCKET}" --delete >/dev/null
ok "Dashboard uploaded to s3://${DASHBOARD_BUCKET}"

if [ -n "$CLOUDFRONT_ID" ]; then
  log "Invalidating CloudFront cache"
  aws cloudfront create-invalidation --distribution-id "$CLOUDFRONT_ID" --paths "/*" >/dev/null
  ok "CloudFront cache invalidated"
else
  warn "Could not find CloudFront distribution to invalidate"
fi

ok "Init complete"
bash scripts/refresh-urls.sh || true
echo ""
echo "  Dashboard : $(terraform -chdir=terraform output -raw dashboard_url)"
echo "  API       : ${DASHBOARD_API_URL}"

echo ""

if gh auth status >/dev/null 2>&1; then
  log "Pushing secrets to GitHub automatically"
  bash scripts/push-secrets.sh "$ROLE_ARN"
  ok "GitHub secrets configured"
else
  warn "gh CLI not authenticated - run 'gh auth login', then push secrets manually:"
  echo "  bash scripts/push-secrets.sh \"${ROLE_ARN}\""
fi

echo ""
echo "Remaining manual step: confirm the SNS email subscription sent to your alert address."
echo ""
echo "After that, every push to main triggers the CI/CD pipeline automatically."
