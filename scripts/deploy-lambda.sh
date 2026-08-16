#!/usr/bin/env bash
# deploy-lambda.sh - build + push + publish + update alias cho Lambda, dung cach.
#
#   ./scripts/deploy-lambda.sh                      # tat ca 7 function
#   ./scripts/deploy-lambda.sh agent-worker dashboard-api
#
# Moi gia tri (account, region, ten repo/function) deu duoc suy ra dong -
# KHONG hardcode, nen tao lai ha tang khong lam hong script nay.
# Hai bay duoc xu ly san:
#   - docker build phai co --provenance=false --sbom=false (Lambda khong nhan
#     OCI manifest list co attestation).
#   - alias `live` co ignore_changes trong Terraform, nen sau khi push image
#     phai update-function-code --publish roi update-alias thu cong.
set -euo pipefail
export AWS_PAGER="" # AWS CLI v2 pipes output through `less` by default when
                     # stdout is a terminal - never useful in a script.
cd "$(dirname "$0")/.."

# Tu nap AWS credentials tu .env neu shell chua co (terminal moi khong can source truoc)
if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

# Docker chet -> `aws ecr get-login-password | docker login` vo pipe, va aws CLI
# (viet bang Python) chi kip keu "BrokenPipeError" kho hieu roi thoat. Kiem tra
# truoc de loi that (daemon khong chay) hien ra thay vi trieu chung.
if ! docker info >/dev/null 2>&1; then
  echo "[ERROR] Docker daemon khong chay. Mo Docker Desktop roi chay lai."
  exit 1
fi

PROJECT="${PROJECT:-hivemind}"
ENVIRONMENT="${ENVIRONMENT:-dev}"
REGION="${AWS_DEFAULT_REGION:-ap-southeast-1}"
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REG="${ACCOUNT}.dkr.ecr.${REGION}.amazonaws.com"

# This script updates an alias straight to 100% - no canary, no CloudWatch
# check, unlike .github/workflows/deploy-canary.yml. Fine for "dev" (this
# script's whole reason to exist is a fast local loop there); running it
# against anything else deploys untested code directly to real traffic, so it
# stops and asks unless the environment is explicitly acknowledged.
if [ "$ENVIRONMENT" != "dev" ]; then
  if [ -t 0 ] && [ "${CONFIRM_DEPLOY:-}" != "1" ]; then
    read -r -p "ENVIRONMENT=$ENVIRONMENT — this deploys straight to 100%, no canary. Continue? [y/N] " reply
    case "$reply" in
      [yY]|[yY][eE][sS]) ;;
      *) echo "[ABORT] Not confirmed."; exit 1 ;;
    esac
  elif [ "${CONFIRM_DEPLOY:-}" != "1" ]; then
    echo "[ERROR] ENVIRONMENT=$ENVIRONMENT and no TTY to confirm interactively."
    echo "        Set CONFIRM_DEPLOY=1 to proceed non-interactively (e.g. from a script)."
    exit 1
  fi
fi

# service -> duong dan cmd/ (scoring-python co Dockerfile rieng)
declare -A CMD_PATH=(
  [agent-worker]=./cmd/worker
  [dispatcher]=./cmd/dispatcher
  [reaper]=./cmd/heartbeat-reaper
  [salience-decay]=./cmd/salience-decay
  [scoring-api]=./cmd/scoring-api
  [dashboard-api]=./cmd/dashboard-api
)

SERVICES=("$@")
# Mac dinh: 6 image Go (build ~10s/cai). scoring-python nang (pandas/sklearn)
# va gan nhu khong doi -> chi build khi duoc goi ten ro rang:
#   ./scripts/deploy-lambda.sh scoring-python
[ ${#SERVICES[@]} -eq 0 ] && SERVICES=(agent-worker dispatcher reaper salience-decay scoring-api dashboard-api)

aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REG" >/dev/null
echo "[i] registry: $REG"

for svc in "${SERVICES[@]}"; do
  repo="${REG}/${PROJECT}/${ENVIRONMENT}/${svc}"
  fn="${PROJECT}-${ENVIRONMENT}-${svc}"
  echo "== ${svc} =="

  if [ "$svc" = "scoring-python" ]; then
    docker build --provenance=false --sbom=false \
      -f services/scoring-python/Dockerfile -t "${repo}:latest" .
  else
    [ -n "${CMD_PATH[$svc]:-}" ] || { echo "[ERROR] service la: ${svc}"; exit 1; }
    docker build --provenance=false --sbom=false -f Dockerfile.lambda-go \
      --build-arg SERVICE_PATH="${CMD_PATH[$svc]}" -t "${repo}:latest" .
  fi

  docker push "${repo}:latest"

  V=$(aws lambda update-function-code --function-name "$fn" \
        --image-uri "${repo}:latest" --publish \
        --query Version --output text)
  aws lambda wait function-updated --function-name "$fn"
  aws lambda update-alias --function-name "$fn" --name live --function-version "$V" >/dev/null
  echo "[OK] ${fn} -> v${V} (alias live)"
done

echo "[DONE] deploy xong: ${SERVICES[*]}"
