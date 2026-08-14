#!/usr/bin/env bash
# api_smoke.sh — smoke-test the deployed control plane end to end.
#
#   bash test/integration/api_smoke.sh "$(terraform -chdir=terraform output -raw dashboard_api_url)" [CONTROL_TOKEN]
#
# Two halves:
#   1. every read endpoint answers 200,
#   2. every mutating endpoint REJECTS bad input (the important half) - the
#      read-only SQL guard, unknown services, invented verdicts, injection in a
#      region name, unbounded bulk decisions.
# Exits non-zero if any check fails, so it doubles as a CD gate.
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

check() { # desc method path expected_code [body]
  local desc="$1" method="$2" path="$3" want="$4" body="${5:-}"
  local code
  if [ -n "$body" ]; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "${hdr[@]}" -d "$body" "$API$path")
  else
    code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "$API$path")
  fi
  if [ "$code" = "$want" ]; then
    printf '  [PASS]  %-44s %s\n' "$desc" "$code"
    pass=$((pass + 1))
  else
    printf '  [FAIL]  %-44s got %s want %s\n' "$desc" "$code" "$want"
    fail=$((fail + 1))
  fi
}

echo "Smoke: $API"

echo "-- observability reads --"
check "GET /overview"                 GET  /overview             200
check "GET /memory"                   GET  /memory               200
check "GET /cost"                     GET  /cost                 200
check "GET /infrastructure"           GET  /infrastructure       200
check "GET /reviews"                  GET  /reviews              200

echo "-- control plane reads --"
check "GET /control/fleet"            GET  /control/fleet        200
check "GET /control/db"               GET  /control/db           200
check "GET /control/lambdas"          GET  /control/lambdas      200
check "GET /control/resources"        GET  /control/resources    200
check "GET /control/regions"          GET  /control/regions      200
check "GET /control/policy"           GET  /control/policy       200
check "GET /control/budget"           GET  /control/budget       200
check "GET /cost/infrastructure"      GET  /cost/infrastructure  200

echo "-- global search --"
check "search: valid query"           GET  "/control/search?q=fraud"  200
check "search: 1 char rejected"       GET  "/control/search?q=a"      400

echo "-- read-only SQL console (the critical safety test) --"
check "SELECT allowed"                POST /control/query 200 '{"sql":"SELECT COUNT(*) FROM tasks"}'
check "DELETE rejected"               POST /control/query 400 '{"sql":"DELETE FROM tasks"}'
check "DROP rejected"                 POST /control/query 400 '{"sql":"DROP TABLE tasks"}'
check "UPDATE rejected"               POST /control/query 400 '{"sql":"UPDATE tasks SET status='"'"'done'"'"'"}'
check "multi-statement rejected"      POST /control/query 400 '{"sql":"SELECT 1; SELECT 2"}'

echo "-- policy validation --"
check "inverted risk band rejected"   POST /control/policy 400 '{"risk_low":0.9,"risk_high":0.2,"dispatch_batch":20,"fallback_action":"escalate","daily_budget_usd":5}'
check "out-of-range band rejected"    POST /control/policy 400 '{"risk_low":-1,"risk_high":5,"dispatch_batch":20,"fallback_action":"escalate","daily_budget_usd":5}'
check "bad fallback action rejected"  POST /control/policy 400 '{"risk_low":0.001,"risk_high":0.999,"dispatch_batch":20,"fallback_action":"ignore","daily_budget_usd":5}'
check "huge dispatch batch rejected"  POST /control/policy 400 '{"risk_low":0.001,"risk_high":0.999,"dispatch_batch":9999,"fallback_action":"escalate","daily_budget_usd":5}'

echo "-- task / review guards --"
check "invented verdict rejected"     POST /control/task    400 '{"action":"override","task_id":"00000000-0000-0000-0000-000000000000","verdict":"probably_fraud","reviewer_id":"qa"}'
check "override w/o reviewer rejected" POST /control/task   400 '{"action":"override","task_id":"00000000-0000-0000-0000-000000000000","verdict":"fraud"}'
check "unknown task action rejected"  POST /control/task    400 '{"action":"delete_everything","task_id":"x"}'
check "bulk w/o reviewer rejected"    POST /reviews/bulk    400 '{"task_ids":["x"],"decision":"approved"}'
check "bulk bad decision rejected"    POST /reviews/bulk    400 '{"task_ids":["x"],"decision":"maybe","reviewer_id":"qa"}'

echo "-- memory / node / rollback / region guards --"
check "memory bad action rejected"    POST /control/memory     400 '{"action":"drop_table","id":"x"}'
check "memory bad threshold rejected" POST /control/memory/job 400 '{"job":"archive_below","threshold":9}'
check "invoke dashboard-api rejected" POST /control/invoke     400 '{"service":"dashboard-api"}'
check "schedule unknown svc rejected" POST /control/schedule   400 '{"service":"ghost","action":"enable"}'
check "rollback unknown svc rejected" POST /control/rollback   400 '{"service":"../etc/passwd"}'
check "region injection rejected"     POST /control/regions    400 '{"action":"add","region":"x\"; DROP DATABASE hivemind; --"}'
check "region bad action rejected"    POST /control/regions    400 '{"action":"nuke","region":"aws-ap-southeast-1"}'

echo
echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
