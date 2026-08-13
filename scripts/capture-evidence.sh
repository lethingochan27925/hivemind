#!/usr/bin/env bash
# capture-evidence.sh - tu dong thu bang chung he thong vao evidence/<timestamp>/
# (JSON + INDEX.md) va day mot ban len bucket S3 evidence. Chay truoc/sau moi
# canh demo (crash test, memory recall, multi-region) de co so lieu dinh kem.
set -euo pipefail
cd "$(dirname "$0")/.."

# Tu nap AWS credentials tu .env neu shell chua co
if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

API=$(terraform -chdir=terraform output -raw dashboard_api_url)
API="${API%/}"
TS=$(date +%Y%m%d_%H%M%S)
DIR="evidence/$TS"
mkdir -p "$DIR"
echo "Thu bang chung -> $DIR"

grab() { curl -s "$API$1" > "$DIR/$2.json" && echo "  [OK] $2"; }
q() {
  curl -s -X POST -H 'Content-Type: application/json' \
    -d "{\"sql\":\"$1\"}" "$API/control/query" > "$DIR/$2.json" && echo "  [OK] $2"
}

# Trang thai he thong
grab /overview overview
grab /memory memory
grab /cost cost
grab /control/db db-stats
grab /control/fleet fleet
grab /control/regions regions
grab /control/lambdas lambdas
grab /infrastructure infrastructure

# Bang chung SQL (append-only audit + ground truth)
q "SELECT t.verdict, tx.is_fraud_label, COUNT(*) AS n FROM tasks t JOIN transactions tx ON tx.id = t.transaction_id WHERE t.verdict IS NOT NULL GROUP BY t.verdict, tx.is_fraud_label ORDER BY t.verdict" verdict-vs-groundtruth
q "SELECT pattern_type, verdict, merge_count, recall_count, salience FROM case_memory WHERE archived = false ORDER BY recall_count DESC LIMIT 10" memory-top-recalled
q "SELECT action, COUNT(*) AS n FROM audit_log GROUP BY action ORDER BY n DESC" audit-actions
q "SELECT action, agent_id, created_at FROM audit_log WHERE action IN ('task_requeued','task_resumed') ORDER BY created_at DESC LIMIT 20" crash-recovery-events
q "SELECT COUNT(DISTINCT agent_id) AS distinct_agents FROM audit_log WHERE action = 'task_claimed'" fleet-distinct-agents
q "SELECT status, COUNT(*) FROM tasks GROUP BY status" task-status

cat > "$DIR/INDEX.md" << 'HM_IDX'
# Evidence capture

Snapshot tu he thong dang chay (dashboard-api -> CockroachDB / CloudWatch / GitHub la nguon that, khong dan dung tay).

| File | Chung minh |
|------|-----------|
| overview.json | Verdict hom nay, accuracy vs ground-truth, learning curve |
| memory.json | Episodic memory: active/archived, salience, patterns, impact |
| verdict-vs-groundtruth.json | Ma tran verdict x is_fraud_label (recall/precision) |
| memory-top-recalled.json | Case duoc recall/merge nhieu nhat (fleet dang hoc) |
| crash-recovery-events.json | task_requeued -> task_resumed (chaos test) |
| fleet-distinct-agents.json | So agent phan biet da claim task (concurrency) |
| regions.json | Cau hinh multi-region + survival goal |
| audit-actions.json, task-status.json, db-stats.json, fleet.json, cost.json, lambdas.json, infrastructure.json | Trang thai van hanh tong the |
HM_IDX
echo "  [OK] INDEX.md"

# Day len bucket evidence (versioned) - khong chan neu thieu quyen
BUCKET=$(terraform -chdir=terraform output -raw evidence_bucket 2>/dev/null || true)
if [ -n "$BUCKET" ]; then
  aws s3 sync "evidence/$TS" "s3://$BUCKET/$TS" --quiet && echo "  [OK] da day len s3://$BUCKET/$TS" || echo "  [WARN] khong day duoc S3 (bo qua)"
fi

echo "[DONE] $DIR - commit thu muc evidence/ vao repo lam bang chung."
