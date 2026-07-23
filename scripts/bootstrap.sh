#!/usr/bin/env bash
set -euo pipefail

TERRAFORM_DIR="terraform"

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

log "Verifying AWS credentials"
aws sts get-caller-identity >/dev/null || err "AWS credentials invalid or expired"

log "Loading secrets from .env"
source scripts/load-tf-vars.sh

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGION=$(aws configure get region)
[ -n "$REGION" ] || REGION="ap-southeast-1"
ECR_REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

GITHUB_REPO=$(git config --get remote.origin.url | sed -E 's#^(git@github\.com:|https://github\.com/)##; s#\.git$##')
[ -n "$GITHUB_REPO" ] || err "could not parse GitHub repo from git remote origin url"

ok "Account: ${ACCOUNT_ID} | Region: ${REGION} | Repo: ${GITHUB_REPO}"

log "Terraform init"
cd "$TERRAFORM_DIR"
terraform init
ok "Terraform initialized"

PROJECT=$(terraform console <<< 'var.project' 2>/dev/null | tr -d '"' || true)
ENVIRONMENT=$(terraform console <<< 'var.environment' 2>/dev/null | tr -d '"' || true)
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

log "Verifying ECR repositories exist before building images"
declare -A SERVICE_MAP=(
  [worker]=agent-worker
  [dispatcher]=dispatcher
  [heartbeat-reaper]=reaper
  [salience-decay]=salience-decay
  [scoring-api]=scoring-api
  [review-api]=review-api
)

for repo_name in "${SERVICE_MAP[@]}" scoring-python; do
  aws ecr describe-repositories \
    --repository-names "${PROJECT}/${ENVIRONMENT}/${repo_name}" \
    --region "$REGION" >/dev/null 2>&1 \
    || err "ECR repository ${PROJECT}/${ENVIRONMENT}/${repo_name} not found - base apply may have failed"
done
ok "All ECR repositories confirmed"

log "Building and pushing images"
for cmd_dir in "${!SERVICE_MAP[@]}"; do
  repo_name="${SERVICE_MAP[$cmd_dir]}"
  image="${ECR_REGISTRY}/${PROJECT}/${ENVIRONMENT}/${repo_name}:latest"
  log "  ${repo_name}"
  docker build -f Dockerfile.lambda-go \
    --build-arg SERVICE_PATH="./cmd/${cmd_dir}" \
    -t "$image" . >/dev/null
  docker push "$image" >/dev/null
done

python_image="${ECR_REGISTRY}/${PROJECT}/${ENVIRONMENT}/scoring-python:latest"
log "  scoring-python"
docker build -f services/scoring-python/Dockerfile -t "$python_image" . >/dev/null
docker push "$python_image" >/dev/null

ok "All images pushed"

log "Applying full infrastructure (Lambda functions now have images)"
cd "$TERRAFORM_DIR"
terraform apply -auto-approve
ROLE_ARN=$(terraform output -raw github_actions_role_arn)
cd - >/dev/null

ok "Bootstrap complete"

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
