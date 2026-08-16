#!/usr/bin/env bash
# deploy-dashboard.sh - build static export + sync S3 + invalidate CloudFront.
#
#   ./scripts/deploy-dashboard.sh
#
# API URL, bucket, distribution id deu duoc tra dong tu terraform output /
# AWS API (distribution tim theo Comment), nen tao lai ha tang khong can sua
# gi ca. Cuoi cung tu goi refresh-urls.sh de SUBMISSION.md luon dung URL.
set -euo pipefail
export AWS_PAGER="" # AWS CLI v2 pipes output through `less` by default when
                     # stdout is a terminal - never useful in a script.
cd "$(dirname "$0")/.."

# Tu nap AWS credentials tu .env neu shell chua co (terminal moi khong can source truoc)
if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

PROJECT="${PROJECT:-hivemind}"
ENVIRONMENT="${ENVIRONMENT:-dev}"

API=$(terraform -chdir=terraform output -raw dashboard_api_url)
BUCKET=$(terraform -chdir=terraform output -raw dashboard_bucket_name)
[ -n "$API" ] && [ -n "$BUCKET" ] || { echo "[ERROR] thieu terraform output (da apply chua?)"; exit 1; }

printf 'NEXT_PUBLIC_DASHBOARD_API_URL=%s\n' "${API%/}" > dashboard/.env.production.local
echo "[i] API : ${API%/}"
echo "[i] S3  : s3://${BUCKET}"

(cd dashboard && npm run build)
aws s3 sync dashboard/out "s3://${BUCKET}" --delete

DIST=$(aws cloudfront list-distributions \
  --query "DistributionList.Items[?Comment=='${PROJECT}-${ENVIRONMENT}-dashboard'].Id" \
  --output text)
if [ -n "$DIST" ] && [ "$DIST" != "None" ]; then
  aws cloudfront create-invalidation --distribution-id "$DIST" --paths '/*' >/dev/null
  echo "[OK] CloudFront ${DIST} invalidated"
else
  echo "[WARN] khong tim thay distribution comment=${PROJECT}-${ENVIRONMENT}-dashboard"
fi

bash scripts/refresh-urls.sh || true
echo "[DONE] dashboard: $(terraform -chdir=terraform output -raw dashboard_url)"
