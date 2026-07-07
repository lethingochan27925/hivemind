#!/usr/bin/env bash
set -euo pipefail

PROJECT=${PROJECT:-"hivemind"}
ENVIRONMENT=${ENVIRONMENT:-"dev"}
AWS_REGION=${AWS_REGION:-$(aws configure get region 2>/dev/null || echo "ap-southeast-1")}
TERRAFORM_DIR="deployments/terraform"
BOOTSTRAP_DIR="${TERRAFORM_DIR}/modules/bootstrap"

log()  { echo "[$(date '+%H:%M:%S')] [INFO]  $*"; }
ok()   { echo "[$(date '+%H:%M:%S')] [OK]    $*"; }
err()  { echo "[$(date '+%H:%M:%S')] [ERROR] $*" >&2; exit 1; }

# -- Check dependencies --------------------------------------------------------
log "Checking dependencies..."
command -v terraform >/dev/null || err "terraform not found"
command -v aws       >/dev/null || err "aws cli not found"
command -v kubectl   >/dev/null || err "kubectl not found"
aws sts get-caller-identity --region "$AWS_REGION" >/dev/null || err "AWS credentials not configured"
log "Using region: ${AWS_REGION}"
ok "Dependencies OK"

# -- Step 1: Bootstrap ---------------------------------------------------------
log "Step 1/4: Provisioning bootstrap..."
cd "$BOOTSTRAP_DIR"
terraform init -input=false
terraform apply -input=false -auto-approve \
  -var="project=${PROJECT}" \
  -var="environment=${ENVIRONMENT}"

BUCKET_NAME=$(terraform output -raw tfstate_bucket_name)
LOCK_TABLE=$(terraform output -raw tfstate_lock_table_name)
BUCKET_REGION=$(terraform output -raw tfstate_region)
ok "Bootstrap done — bucket: ${BUCKET_NAME}, region: ${BUCKET_REGION}"
cd - >/dev/null

# -- Step 2: Write backend config ----------------------------------------------
log "Step 2/4: Configuring remote backend..."
python3 - <<PYEOF
import re
path = "${TERRAFORM_DIR}/versions.tf"
with open(path) as f:
    content = f.read()
new_backend = '''  backend "s3" {
    bucket       = "${BUCKET_NAME}"
    key          = "${PROJECT}/${ENVIRONMENT}/terraform.tfstate"
    region       = "${BUCKET_REGION}"
    use_lockfile = true
    encrypt      = true
  }'''
content = re.sub(
    r'backend "s3" \{[^}]*\}',
    new_backend.strip(),
    content,
    flags=re.DOTALL
)
with open(path, "w") as f:
    f.write(content)
print("Backend config written")
PYEOF
ok "Backend config updated"

# -- Step 3: Terraform init with remote backend --------------------------------
log "Step 3/4: Initializing Terraform with remote backend..."
cd "$TERRAFORM_DIR"
terraform init \
  -input=false \
  -backend-config="bucket=${BUCKET_NAME}" \
  -backend-config="key=${PROJECT}/${ENVIRONMENT}/terraform.tfstate" \
  -backend-config="region=${BUCKET_REGION}" \
  -backend-config="use_lockfile=true" \
  -backend-config="encrypt=true"
ok "Terraform initialized"

# -- Step 4: Terraform apply ---------------------------------------------------
log "Step 4/4: Provisioning infrastructure (EKS ~15-20 min)..."
[ -f "terraform.tfvars" ] || err "terraform.tfvars not found. Copy terraform.tfvars.example and fill in values."
terraform apply -input=false -auto-approve
ok "Infrastructure provisioned"

EKS_CLUSTER=$(terraform output -raw eks_cluster_name)
log "Updating kubeconfig for cluster: ${EKS_CLUSTER}..."
aws eks update-kubeconfig --name "$EKS_CLUSTER" --region "$BUCKET_REGION"
ok "kubeconfig updated"
cd - >/dev/null

echo ""
echo "===================================================="
echo "  HiveMind infrastructure ready"
echo "  Cluster : ${EKS_CLUSTER}"
echo "  Region  : ${BUCKET_REGION}"
echo "  Next    : bash scripts/deploy.sh"
echo "===================================================="
