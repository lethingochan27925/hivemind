#!/usr/bin/env bash
# api_smoke.sh — smoke-test the deployed dashboard-api control plane.
#
#   bash test/integration/api_smoke.sh "$(terraform -chdir=terraform output -raw dashboard_api_url)" [CONTROL_TOKEN]
#
# Exits non-zero if any check fails, so it doubles as a CI gate.
set -uo pipefail

API="${1:-}"
TOKEN="${2:-}"
if [ -z "$API" ]; then
  echo "usage: $0 <API_BASE_URL> [CONTROL_TOKEN]" >&2
  exit 2
fi
API="${API%/}"

hdr=(-H "Content-Type: application/json")
[ -n "$TOKEN" ] && hdr+=(-H "X-Control-Token: $TOKEN")

pass=0
fail=0

# check <desc> <method> <path> <expected_code> [body]
check() {
  local desc="$1" method="$2" path="$3" want="$4" body="${5:-}"
  local code
  if [ -n "$body" ]; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "${hdr[@]}" -d "$body" "$API$path")
  else
    code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "$API$path")
  fi
  if [ "$code" = "$want" ]; then
    printf '  [PASS]  %-34s %s\n' "$desc" "$code"
    pass=$((pass + 1))
  else
    printf '  [FAIL]  %-34s got %s want %s\n' "$desc" "$code" "$want"
    fail=$((fail + 1))
  fi
}

echo "Smoke: $API"
echo "-- observability reads --"
check "GET /overview"             GET  /overview          200
check "GET /cost"                 GET  /cost              200
echo "-- control plane reads --"
check "GET /control/fleet"        GET  /control/fleet     200
check "GET /control/db"           GET  /control/db        200
check "GET /control/lambdas"      GET  /control/lambdas   200
check "GET /control/resources"    GET  /control/resources 200
echo "-- read-only query guard (the critical safety test) --"
check "SELECT allowed"            POST /control/query     200 '{"sql":"SELECT COUNT(*) FROM tasks"}'
check "DELETE rejected"           POST /control/query     400 '{"sql":"DELETE FROM tasks"}'
check "DROP rejected"             POST /control/query     400 '{"sql":"DROP TABLE tasks"}'
check "UPDATE rejected"           POST /control/query     400 '{"sql":"UPDATE tasks SET status='"'"'done'"'"'"}'
check "multi-statement rejected"  POST /control/query     400 '{"sql":"SELECT 1; SELECT 2"}'

echo
echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
