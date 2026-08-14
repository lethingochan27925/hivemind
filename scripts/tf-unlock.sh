#!/usr/bin/env bash
# tf-unlock.sh - go lock terraform CHET, va chi lock chet.
#
#   ./scripts/tf-unlock.sh          # chi go khi lock gia hon TF_LOCK_MAX_AGE_MIN (mac dinh 20')
#   ./scripts/tf-unlock.sh --yes    # go bat ke tuoi (ban tu chiu trach nhiem)
#
# Vi sao khong go vo dieu kien: lock co the dang duoc giu boi mot `apply` DANG
# CHAY (CI hoac may khac). Pha lock luc do la hai tien trinh cung ghi state.
# Mot apply khoe manh xong trong 1-3 phut va deploy da duoc tuan tu hoa bang
# concurrency group, nen lock gia hon nguong tuoi chi con mot cach giai thich:
# tien trinh giu no da chet (runner bi kill, terraform crash, mat mang).
#
# Lock cua backend S3 (use_lockfile) la mot object JSON canh state file - doc
# truc tiep tu S3 nen khong can parse stderr cua terraform.
set -uo pipefail
cd "$(dirname "$0")/.."

MODE="${1:-}"
MAX_AGE_MIN="${TF_LOCK_MAX_AGE_MIN:-20}"

if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

BUCKET=$(grep -oE 'bucket[[:space:]]*=[[:space:]]*"[^"]+"' terraform/versions.tf | head -1 | cut -d'"' -f2)
KEY=$(grep -oE 'key[[:space:]]*=[[:space:]]*"[^"]+"' terraform/versions.tf | head -1 | cut -d'"' -f2)
if [ -z "$BUCKET" ] || [ -z "$KEY" ]; then
  echo "[FAIL] khong doc duoc backend bucket/key tu terraform/versions.tf"; exit 2
fi
LOCK_URI="s3://${BUCKET}/${KEY}.tflock"

INFO=$(aws s3 cp "$LOCK_URI" - 2>/dev/null) || { echo "[i] khong co lock nao ($LOCK_URI)"; exit 1; }

read -r LOCK_ID AGE_MIN WHO OP < <(python3 - "$INFO" << 'PY'
import json, sys
from datetime import datetime, timezone
d = json.loads(sys.argv[1])
created = d.get("Created", "")
# "2026-08-14T07:37:58.673846699Z" - fromisoformat khong nhan 9 chu so le
base = created.split(".")[0].replace("Z", "")
try:
    t = datetime.fromisoformat(base).replace(tzinfo=timezone.utc)
    age = (datetime.now(timezone.utc) - t).total_seconds() / 60
except ValueError:
    age = -1
print(d.get("ID", "?"), round(age, 1), d.get("Who", "?"), d.get("Operation", "?"))
PY
)

echo "[i] lock: id=${LOCK_ID}"
echo "         giu boi ${WHO} (${OP}), tuoi ${AGE_MIN} phut (nguong ${MAX_AGE_MIN})"

if [ "$MODE" != "--yes" ]; then
  # so sanh so thuc bang awk (bash chi so sanh nguyen)
  if ! awk "BEGIN{exit !(${AGE_MIN} >= ${MAX_AGE_MIN})}"; then
    echo "[!] lock moi ${AGE_MIN} phut - co the mot apply DANG chay that."
    echo "    Cho no xong, hoac chac chan roi thi: ./scripts/tf-unlock.sh --yes"
    exit 1
  fi
fi

terraform -chdir=terraform force-unlock -force "$LOCK_ID" \
  && echo "[OK] da go lock ${LOCK_ID}" \
  || { echo "[FAIL] force-unlock that bai"; exit 2; }
