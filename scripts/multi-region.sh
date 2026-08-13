#!/usr/bin/env bash
# multi-region.sh - bat/tat/kiem tra multi-region cho HiveMind.
#
#   ./scripts/multi-region.sh status
#   ./scripts/multi-region.sh enable aws-ap-southeast-2 aws-ap-south-1
#   ./scripts/multi-region.sh disable aws-ap-southeast-2 aws-ap-south-1
#
# Hai tang tach biet:
#   - CLUSTER (provision node o region moi): lam tren CockroachDB Cloud console
#     vi dung den billing - script nay huong dan va cho ban xac nhan.
#   - DATABASE (SET PRIMARY / ADD / DROP REGION, survival goal): script tu chay
#     qua control API - cung chinh la panel Multi-region tren dashboard.
set -euo pipefail
cd "$(dirname "$0")/.."

# Tu nap AWS credentials tu .env neu shell chua co (terminal moi khong can source truoc)
if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

API=$(terraform -chdir=terraform output -raw dashboard_api_url)
API="${API%/}"
hdr=(-H "Content-Type: application/json")
[ -n "${CONTROL_TOKEN:-}" ] && hdr+=(-H "X-Control-Token: ${CONTROL_TOKEN}")

status() { curl -s "$API/control/regions" | python3 -m json.tool; }

post() { # action [region]
  curl -s -X POST "${hdr[@]}" -d "{\"action\":\"$1\",\"region\":\"${2:-}\"}" "$API/control/regions"
  echo
}

cmd="${1:-status}"
shift || true

case "$cmd" in
  status)
    status
    ;;

  enable)
    [ $# -ge 1 ] || { echo "usage: $0 enable <region> [region...]   (vd: aws-ap-southeast-2)"; exit 2; }
    echo "== Buoc 1/2: CLUSTER phai co san cac region: $* =="
    echo "   Mo CockroachDB Cloud console -> cluster -> Regions -> Add region(s)."
    echo "   Doi provision xong (console bao ready) roi quay lai day."
    read -r -p "   Da provision xong? [Enter de tiep tuc, Ctrl+C de dung] " _
    echo "== Buoc 2/2: DATABASE - primary + add regions + survival goal =="
    if ! curl -s "$API/control/regions" | grep -q '"multi_region":true'; then
      primary="${PRIMARY_REGION:-aws-ap-southeast-1}"
      echo "-- set primary: $primary"
      post set_primary "$primary"
    fi
    for r in "$@"; do
      echo "-- add region: $r"
      post add "$r"
    done
    echo "-- survival goal: SURVIVE REGION FAILURE"
    post survive_region
    status
    echo "[OK] Multi-region bat xong. Panel Multi-region tren dashboard se hien cac region."
    ;;

  disable)
    [ $# -ge 1 ] || { echo "usage: $0 disable <region> [region...]"; exit 2; }
    echo "-- ha survival goal ve ZONE truoc khi drop region"
    post survive_zone
    for r in "$@"; do
      echo "-- drop region: $r"
      post drop "$r"
    done
    status
    echo "!! Nho go region o tang CLUSTER tren Cloud console de ngung ton phi."
    ;;

  *)
    echo "usage: $0 status | enable <regions...> | disable <regions...>"
    exit 2
    ;;
esac
