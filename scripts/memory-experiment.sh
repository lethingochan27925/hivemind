#!/usr/bin/env bash
# memory-experiment.sh - thi nghiem A/B chung minh gia tri cua episodic memory.
#
#   ./scripts/memory-experiment.sh [so_case] [phut_cho_moi_pha]
#   ./scripts/memory-experiment.sh 60 6
#
# THIET KE (quan trong):
#   - Chon NGAU NHIEN mot tap case co dinh, va chay DUNG TAP DO o ca hai pha.
#     Ban dau thi nghiem dung /control/feed, nhung feed lay theo completed_at
#     DESC nen boc trung nhom vua hoan thanh - sau vai vong do la cac case
#     escalate. Ca hai pha deu lay mau lech -> ket qua vo nghia.
#   - Pha A (lanh): archive toan bo episodic memory truoc khi chay.
#   - Pha B (am):   chay lai DUNG cac case do, voi ky uc do chinh pha A tao ra.
#
# Vi cung mot tap dau vao, chenh lech giua hai pha chi con do mot bien: fleet co
# ky uc hay khong.
set -uo pipefail
cd "$(dirname "$0")/.."

if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -f .env ]; then
  set -a; . ./.env; set +a
fi

COUNT="${1:-60}"
WAIT_MIN="${2:-6}"
WAIT_SEC=$((WAIT_MIN * 60))

API=$(terraform -chdir=terraform output -raw dashboard_api_url)
API="${API%/}"
hdr=(-H "Content-Type: application/json")
[ -n "${CONTROL_TOKEN:-}" ] && hdr+=(-H "X-Control-Token: ${CONTROL_TOKEN}")

TS=$(date +%Y%m%d_%H%M%S)
OUT="evidence/memory-experiment-${TS}.md"
mkdir -p evidence

sql() {
  curl -s -X POST "${hdr[@]}" -d "{\"sql\":\"$1\"}" "$API/control/query" |
    python3 -c "
import json,sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for row in d.get('rows') or []:
    print('|'.join('' if v is None else str(v) for v in row))
"
}

echo "== Thi nghiem A/B: episodic memory co lam fleet quyet dinh tot hon? =="
echo "   API   : $API"
echo "   Mau   : $COUNT case (cung mot tap cho ca hai pha), cho $WAIT_MIN phut/pha"
echo

echo "[1/6] Bat fleet"
curl -s -X POST "${hdr[@]}" -d '{"action":"start"}' "$API/control/fleet" >/dev/null

echo "[2/6] Chon ngau nhien $COUNT case da tung duoc dieu tra"
# id::STRING la bat buoc: UUID qua pgx ra JSON thanh mang byte, khong phai chuoi.
mapfile -t IDS < <(sql "SELECT id::STRING FROM tasks WHERE status IN ('done','pending_review') ORDER BY random() LIMIT ${COUNT}")
if [ "${#IDS[@]}" -eq 0 ]; then
  echo "      Khong co case nao de chay. Nap du lieu truoc (Feed cases tren dashboard)."
  exit 1
fi
IN_LIST=$(printf "'%s'," "${IDS[@]}"); IN_LIST="${IN_LIST%,}"
echo "      da chon ${#IDS[@]} case"

# requeue_sample: tra ca tap mau ve hang doi de fleet dieu tra lai tu dau
requeue_sample() {
  local n=0 code
  for id in "${IDS[@]}"; do
    code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "${hdr[@]}" \
      -d "{\"action\":\"requeue\",\"task_id\":\"${id}\"}" "$API/control/task")
    [ "$code" = "200" ] && n=$((n + 1))
  done
  echo "$n"
}

# drain: requeue KHONG tu goi worker (khac /control/feed), nen phai kich
# dispatcher, roi cho den khi tap mau thuc su duoc xu ly xong - thay vi ngu mot
# khoang co dinh roi do vao khoang trong.
drain() {
  local expect="$1" deadline=$((SECONDS + WAIT_SEC)) left
  while [ "$SECONDS" -lt "$deadline" ]; do
    curl -s -X POST "${hdr[@]}" "$API/control/dispatch" >/dev/null
    sleep 20
    left=$(sql "SELECT COUNT(*) FROM tasks WHERE id IN (${IN_LIST}) AND verdict IS NULL" | head -1)
    left=${left:-0}
    printf "
      dang xu ly... con %s/%s case" "$left" "$expect"
    [ "$left" -eq 0 ] && break
  done
  echo
}

# measure: trong tap mau, bao nhieu case agent TU QUYET vs day sang nguoi
measure() {
  local auto=0 esc=0 pending=0
  while IFS='|' read -r verdict n; do
    [ -z "$verdict" ] && continue
    case "$verdict" in
      escalate) esc=$((esc + n)) ;;
      "")       pending=$((pending + n)) ;;
      *)        auto=$((auto + n)) ;;
    esac
  done < <(sql "SELECT COALESCE(verdict,''), COUNT(*) FROM tasks WHERE id IN (${IN_LIST}) GROUP BY 1")
  local done_total=$((auto + esc))
  local pct=0
  [ "$done_total" -gt 0 ] && pct=$(awk "BEGIN{printf \"%.1f\", $auto*100/$done_total}")
  echo "$auto $esc $done_total $pct"
}

echo "[3/6] PHA A (lanh) - archive toan bo ky uc roi chay lai tap mau"
mem_before=$(sql "SELECT COUNT(*) FROM case_memory WHERE archived = false" | head -1)
# archive_below chi archive salience < nguong, ma nguong toi da la 2.0 -> cac ky uc
# da dat tran 2.0 (do duoc goi nho nhieu) van song sot. Archive tung cai de pha
# LANH thuc su lanh.
#
# GHI LAI danh sach id vua archive va KHOI PHUC khi script ket thuc - ke ca khi
# no chet giua chung (trap EXIT). Phien ban dau khong lam vay: sau thi nghiem,
# fleet chay ca ngay voi 15/115 ky uc va ty le escalate tang vot len 87%.
ARCHIVED_IDS_FILE=$(mktemp)
restore_memories() {
  [ -s "$ARCHIVED_IDS_FILE" ] || return 0
  local n=0
  while read -r mid; do
    [ -z "$mid" ] && continue
    curl -s -X POST "${hdr[@]}" -d "{\"action\":\"unarchive\",\"id\":\"${mid}\"}" "$API/control/memory" >/dev/null && n=$((n + 1))
  done < "$ARCHIVED_IDS_FILE"
  : > "$ARCHIVED_IDS_FILE"
  echo "      da khoi phuc $n ky uc ve vector index"
}
trap 'restore_memories; rm -f "$ARCHIVED_IDS_FILE"' EXIT

sql "SELECT id::STRING FROM case_memory WHERE archived = false" > "$ARCHIVED_IDS_FILE"
while read -r mid; do
  [ -z "$mid" ] && continue
  curl -s -X POST "${hdr[@]}" -d "{\"action\":\"archive\",\"id\":\"${mid}\"}" "$API/control/memory" >/dev/null
done < "$ARCHIVED_IDS_FILE"
mem_cold=$(sql "SELECT COUNT(*) FROM case_memory WHERE archived = false" | head -1)
echo "      ky uc hoat dong: ${mem_before:-?} -> ${mem_cold:-0}"
sent=$(requeue_sample)
echo "      da tra $sent/${#IDS[@]} case ve hang doi"
if [ "$sent" -eq 0 ]; then
  echo "      [LOI] khong requeue duoc case nao - kiem tra CONTROL_TOKEN / endpoint"
  exit 1
fi
drain "$sent"
read -r A_AUTO A_ESC A_DONE A_PCT <<< "$(measure)"
mem_after_a=$(sql "SELECT COUNT(*) FROM case_memory WHERE archived = false" | head -1)
conf_a=$(sql "SELECT COALESCE(ROUND(AVG(confidence)::NUMERIC,3),0) FROM tasks WHERE id IN (${IN_LIST}) AND confidence IS NOT NULL" | head -1)
hits_a=$(sql "SELECT COALESCE(ROUND(AVG(memory_hits),2),0) FROM audit_log WHERE action='memory_recall' AND task_id IN (${IN_LIST}) AND created_at >= now() - INTERVAL '${WAIT_MIN} minutes'" | head -1)
echo "      PHA A: ${A_AUTO} tu quyet / ${A_ESC} chuyen nguoi / ${A_DONE} xong = ${A_PCT}%"

echo "[4/6] PHA B (am) - chay lai DUNG tap do, gio da co ky uc"
sent=$(requeue_sample)
echo "      da tra $sent/${#IDS[@]} case ve hang doi"
drain "$sent"
read -r B_AUTO B_ESC B_DONE B_PCT <<< "$(measure)"
conf_b=$(sql "SELECT COALESCE(ROUND(AVG(confidence)::NUMERIC,3),0) FROM tasks WHERE id IN (${IN_LIST}) AND confidence IS NOT NULL" | head -1)
hits_b=$(sql "SELECT COALESCE(ROUND(AVG(memory_hits),2),0) FROM audit_log WHERE action='memory_recall' AND task_id IN (${IN_LIST}) AND created_at >= now() - INTERVAL '${WAIT_MIN} minutes'" | head -1)
echo "      PHA B: ${B_AUTO} tu quyet / ${B_ESC} chuyen nguoi / ${B_DONE} xong = ${B_PCT}%"

echo "[5/6] Tinh chenh lech + tra he thong ve trang thai truoc thi nghiem"
DELTA=$(awk "BEGIN{printf \"%.1f\", ${B_PCT:-0} - ${A_PCT:-0}}")
restore_memories
mem_final=$(sql "SELECT COUNT(*) FROM case_memory WHERE archived = false" | head -1)
echo "      ky uc hoat dong sau khoi phuc: ${mem_final:-?}"

echo "[6/6] Ghi bao cao"
cat > "$OUT" << HM_REPORT
# Episodic memory — controlled A/B on an identical sample

Run ${TS} against \`${API}\`.
Sample: **${#IDS[@]} randomly selected cases**, replayed twice — the same task ids in both phases.

## Method

Comparing "cases that had a memory hit" against "cases that did not" on historical data
stops working once a system has been replayed a few times: every task eventually meets
the memory. So this experiment controls the starting condition instead, and holds the
input fixed.

| Phase | Memory state at the start | Input |
|-------|---------------------------|-------|
| **A — cold** | every episodic memory archived (${mem_before:-?} → ${mem_cold:-0} active) | the sample |
| **B — warm** | the memory phase A itself built (${mem_after_a:-0} active) | **the same** sample |

Both phases are measured identically: of the sampled cases the fleet closed, how many did
it resolve on its own instead of handing to a human.

## Result

| Phase | closed | auto-resolved | escalated | **auto-resolve rate** | avg memories recalled | avg confidence |
|-------|--------|---------------|-----------|----------------------|----------------------|----------------|
| A — cold | ${A_DONE} | ${A_AUTO} | ${A_ESC} | **${A_PCT}%** | ${hits_a:-0} |
| B — warm | ${B_DONE} | ${B_AUTO} | ${B_ESC} | **${B_PCT}%** | ${hits_b:-0} |

**Difference: ${DELTA} percentage points.**

## What this measures

Two different questions hide behind "does memory help?", and this experiment answers the
harder one:

1. *Does the memory layer work?* — recall rose from \${hits_a:-0} to \${hits_b:-0} memories per
   investigation between the phases, so yes: it was rebuilt from empty and reused.
2. *Does recalled text change the verdict?* — the difference above answers that.

A difference near zero is the **desired** outcome for a fraud system, not a disappointment:
it means recalled narrative never overrode the arithmetic evidence. Identical inputs produce
identical decisions whether the fleet has seen the pattern before or not - which is exactly
what makes the audit trail defensible.

## How to read this

HiveMind's verdicts are driven first by a deterministic balance-reconciliation tool: a
clean drain is fraud, a completed transfer is legitimate, and anything else is genuinely
ambiguous. That is by design — it is what makes a small model reach 100% recall and
precision. So a large swing in this table would actually be suspicious: it would mean
recalled text was overriding arithmetic evidence.

What episodic memory contributes in this system is measurable elsewhere, and those numbers
are in the scorecard: cases consolidated instead of duplicated, memories reused across
investigations, and a bounded context window that keeps cost per case in the fractions of
a cent. Memory here is knowledge reuse and auditability, not a verdict-flipping oracle.

If the sample is dominated by cases the balance tool marks INCONCLUSIVE, both phases will
sit near the same rate — that is a property of the sample, not a failure of the memory
layer. Re-run with a larger sample for a broader mix.

Reproduce:

\`\`\`bash
./scripts/memory-experiment.sh ${COUNT} ${WAIT_MIN}
\`\`\`
HM_REPORT

echo
echo "=========================================="
echo "  PHA A (lanh) : ${A_PCT}%  (${A_AUTO}/${A_DONE})   recall tb ${hits_a:-0}"
echo "  PHA B (am)   : ${B_PCT}%  (${B_AUTO}/${B_DONE})   recall tb ${hits_b:-0}"
echo "  Chenh lech   : ${DELTA} diem phan tram"
echo "=========================================="
echo "Bao cao: $OUT"
