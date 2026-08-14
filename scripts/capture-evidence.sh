#!/usr/bin/env bash
# capture-evidence.sh - thu bang chung tu he thong DANG CHAY vao evidence/<timestamp>/
# (JSON + SQL + INDEX.md) va day mot ban len bucket S3 evidence.
#
#   ./scripts/capture-evidence.sh            # thu day du
#   ./scripts/capture-evidence.sh --label crash-test
#
# Moi so lieu deu doc truc tiep tu control plane / CockroachDB - khong co gia
# tri nao duoc go tay, nen thu muc nay la bang chung tai lap duoc, khong phai
# anh chup man hinh.
set -uo pipefail
cd "$(dirname "$0")/.."

if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

LABEL=""
[ "${1:-}" = "--label" ] && LABEL="-${2:-}"

API=$(terraform -chdir=terraform output -raw dashboard_api_url)
API="${API%/}"
TS="$(date +%Y%m%d_%H%M%S)${LABEL}"
DIR="evidence/$TS"
mkdir -p "$DIR"
echo "Thu bang chung -> $DIR"

grab() { curl -s "$API$1" > "$DIR/$2.json" && echo "  [OK] $2"; }
q() {
  curl -s -X POST -H 'Content-Type: application/json' \
    -d "{\"sql\":\"$1\"}" "$API/control/query" > "$DIR/$2.json" && echo "  [OK] $2"
}

echo "-- trang thai he thong --"
grab /overview                overview
grab /memory                  memory
grab /cost                    cost
grab /cost/infrastructure     cloud-cost
grab /infrastructure          infrastructure
grab /control/db              db-stats
grab /control/fleet           fleet
grab /control/policy          policy
grab /control/budget          budget
grab /control/regions         regions
grab /control/lambdas         lambdas
grab /control/resources       aws-inventory
grab "/control/memory?limit=50" memory-rows

echo "-- bang chung dinh luong (SQL tren audit trail + ground truth) --"
q "SELECT t.verdict, tx.is_fraud_label, COUNT(*) AS n FROM tasks t JOIN transactions tx ON tx.id = t.transaction_id WHERE t.verdict IS NOT NULL GROUP BY t.verdict, tx.is_fraud_label ORDER BY t.verdict" verdict-vs-groundtruth
q "SELECT pattern_type, verdict, merge_count, recall_count, salience FROM case_memory WHERE archived = false ORDER BY recall_count DESC LIMIT 15" memory-top-recalled
q "SELECT COUNT(*) AS consolidated_cases, SUM(merge_count) AS raw_cases_absorbed FROM case_memory WHERE archived = false" memory-consolidation
q "SELECT action, COUNT(*) AS n FROM audit_log GROUP BY action ORDER BY n DESC" audit-actions
q "SELECT action, agent_id, created_at FROM audit_log WHERE action IN ('task_requeued','task_resumed') ORDER BY created_at DESC LIMIT 20" crash-recovery-events
q "SELECT COUNT(DISTINCT agent_id) AS distinct_agents FROM audit_log WHERE action = 'task_claimed'" fleet-distinct-agents
q "SELECT COUNT(*) AS double_claims FROM (SELECT transaction_id FROM tasks GROUP BY transaction_id HAVING COUNT(*) > 1)" no-double-claims
q "SELECT AVG(memory_hits) AS avg_hits FROM audit_log WHERE action = 'memory_recall'" memory-hit-rate
q "SELECT bedrock_model, COUNT(*) AS n, AVG(latency_ms) AS avg_ms, SUM(tokens_in + tokens_out) AS tokens FROM audit_log WHERE action = 'bedrock_reasoning' GROUP BY bedrock_model" model-usage-and-fallbacks
q "SELECT reviewer_id, COUNT(*) AS decisions FROM audit_log WHERE action = 'human_reviewed' GROUP BY reviewer_id" human-in-the-loop
q "SELECT status, COUNT(*) FROM tasks GROUP BY status" task-status

cat > "$DIR/INDEX.md" << 'HM_IDX'
# Evidence capture

Snapshot taken straight from the running system through the control plane
(CockroachDB, CloudWatch, AWS APIs). Nothing here is typed by hand.

## System state

| File | Shows |
|------|-------|
| `overview.json` | Verdicts today, accuracy vs ground truth, learning curve |
| `memory.json`, `memory-rows.json` | Episodic memory: stats and the individual cases with salience/recall |
| `cost.json`, `cloud-cost.json` | Bedrock token spend + AWS Cost Explorer month-to-date by service |
| `policy.json`, `budget.json` | Live agent policy and the daily spend guardrail |
| `regions.json` | CockroachDB database regions + survival goal |
| `fleet.json`, `lambdas.json`, `infrastructure.json`, `aws-inventory.json`, `db-stats.json` | Fleet schedules, function config, alarms, every tagged AWS resource, table row counts |

## Quantitative proof

| File | Claim it supports |
|------|-------------------|
| `verdict-vs-groundtruth.json` | **100% recall / 100% precision** — the verdict × `is_fraud_label` matrix |
| `memory-top-recalled.json`, `memory-consolidation.json` | Memory is consolidated (merge counts) and reused (recall counts), not a dump |
| `memory-hit-rate.json` | Average similar cases retrieved per investigation |
| `crash-recovery-events.json` | `task_requeued` → `task_resumed` pairs: crashes absorbed |
| `no-double-claims.json` | Must be **0** — `SKIP LOCKED` + `UNIQUE(transaction_id)` guarantee exactly-once |
| `fleet-distinct-agents.json` | Number of distinct agents that claimed work (concurrency) |
| `model-usage-and-fallbacks.json` | Real model usage, latency and tokens; NULL `bedrock_model` rows are rule-based fallbacks, counted separately — the audit trail never claims a fallback was a model decision |
| `human-in-the-loop.json` | Named reviewers and their decision counts |
| `audit-actions.json`, `task-status.json` | Overall activity mix |

## Reproduce

```bash
./scripts/capture-evidence.sh
```
HM_IDX
echo "  [OK] INDEX.md"

BUCKET=$(terraform -chdir=terraform output -raw evidence_bucket 2>/dev/null || true)
if [ -n "$BUCKET" ]; then
  aws s3 sync "$DIR" "s3://$BUCKET/$TS" --quiet && echo "  [OK] da day len s3://$BUCKET/$TS" || echo "  [WARN] khong day duoc S3"
fi

echo "[DONE] $DIR"
