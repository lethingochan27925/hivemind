#!/usr/bin/env bash
# refresh-urls.sh - dong bo moi URL phu thuoc ha tang tu terraform output vao
# cac file tai lieu (SUBMISSION.md). Chay sau moi lan tao lai ha tang, hoac de
# init.sh / deploy-dashboard.sh tu goi. Idempotent.
set -euo pipefail
cd "$(dirname "$0")/.."

# Tu nap AWS credentials tu .env neu shell chua co (terminal moi khong can source truoc)
if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

DASHBOARD_URL=$(terraform -chdir=terraform output -raw dashboard_url 2>/dev/null || true)
if [ -z "$DASHBOARD_URL" ]; then
  echo "[WARN] khong doc duoc terraform output dashboard_url - bo qua refresh"
  exit 0
fi

if [ -f SUBMISSION.md ]; then
  # Thay dong "**Live dashboard:** ..." bang URL hien tai, giu nguyen phan con lai.
  sed -i "s|^\*\*Live dashboard:\*\*.*|**Live dashboard:** ${DASHBOARD_URL}|" SUBMISSION.md
  echo "[OK] SUBMISSION.md: Live dashboard = ${DASHBOARD_URL}"
fi
