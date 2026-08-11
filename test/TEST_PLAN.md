# Master Test Plan — HiveMind

**Author:** QA · **Version:** 1.0 · **System under test:** HiveMind agent fleet + memory + control plane.

## 1. Purpose

Prove that HiveMind does what its pitch claims — durable memory, no double-work, correct verdicts, a real human-in-the-loop, and survival through crashes and region loss — with evidence a judge can reproduce.

## 2. Scope

**In scope:** agent verdict logic, the three memory tiers, fleet concurrency & recovery, the control-plane API + read-only DB console, the dashboard, security (injection, least-privilege, read-only guards), deployment reproducibility.

**Out of scope:** end-user authn, multi-tenancy, payment-gateway integration, load beyond ~50 concurrent workers (documented roadmap, not this milestone).

## 3. Strategy

| Level | What | Automated? | Where |
|-------|------|-----------|-------|
| Unit | Pure logic: `balanceSignal`, `SanitizeField`, consolidation math | ✅ `go test` | package `_test.go` + `test/integration/reasoning_test.go` |
| Integration | DB-backed: SKIP LOCKED claims, memory concurrency; deployed API smoke | ✅ / semi | `internal/memory/*_integration_test.go`, `test/integration/api_smoke.sh` |
| System | End-to-end stream → verdict → review on the real deployment | Manual, scripted | §7 scenarios |
| Resilience | crash-kill, region-kill, reaper recovery | Manual, scripted → `evidence/` | §7 scenarios |
| Security | injection, read-only guard, least-privilege | ✅ / manual | TC-AGENT, TC-CONTROL-PLANE |

## 4. Traceability to the contest scoring criteria

| Criterion | Covering test cases |
|-----------|--------------------|
| Agentic Memory Design | TC-MEMORY-01..06 |
| Technical Implementation | TC-AGENT-*, TC-CONTROL-PLANE-*, integration tests |
| Real-World Impact | TC-AGENT-01..03 (verdict accuracy), §6 metrics |
| Production Readiness | TC-CONCURRENCY-*, TC-CONTROL-PLANE-05 (read-only guard), §7 resilience |
| Creativity & Originality | TC-AGENT-04 (categorical-tool insight) |

## 5. Entry / exit criteria

**Entry:** infra deployed via `scripts/init.sh`; schema loaded; PaySim seeded; Bedrock model access granted.

**Exit (ship gate):**
- `go test ./... -short` green.
- Verdict accuracy ≥ target on the labelled eval (achieved: 100% recall/precision).
- Zero double-claims observed across a 20-worker run of 500 tasks.
- A killed agent's task resumes at the same step in ≤30s (evidence captured).
- `/control/query` rejects every mutating statement (TC-CONTROL-PLANE-05 pass).
- No secret in the pushed repo (pre-push scan clean).

## 6. Metrics tracked (measured, not estimated)

| Metric | Target | Measured |
|--------|--------|----------|
| Recall / precision (labelled eval) | ≥ 65% | **100% / 100%** (34/34 fraud, 0/340 false positives) |
| Auto-resolved without a human | report | **~46%** |
| Cost per investigation run | minimise | **~$0.09** |
| Double-claims under 20 workers | 0 | 0 |
| Task resume latency after kill | ≤ 30s | ≤ 30s |

## 7. Manual system & resilience scenarios

| ID | Scenario | Steps | Pass condition |
|----|----------|-------|----------------|
| SYS-01 | Stream → verdict | Feed 500 PaySim via `/control/feed`; run `/control/dispatch` | All tasks reach a terminal status; verdict mix matches labels |
| SYS-02 | Human-in-the-loop | Force an `escalate`; open Review Queue; approve | `tasks.review_decision` set; `audit_log` gets `human_reviewed` with `reviewer_id` |
| RES-01 | Crash-kill | `kill -9` 10 workers mid-investigation | Tasks re-queue ≤30s; a new worker resumes from `scratchpad.step`; no task lost |
| RES-02 | Region-kill | Take the primary CockroachDB region offline | Fleet keeps writing; audit trail unbroken; RPO = 0 |
| RES-03 | Reaper | Stall a worker so `heartbeat_at` goes stale | `heartbeat-reaper` re-queues within one 30s cycle |

## 8. Risks

| Risk | Mitigation |
|------|-----------|
| Bedrock throttling skews accuracy under load | Adaptive retry; fallback verdict recorded and excluded from model-accuracy metric |
| Signature over-fits PaySim | `escalate` path prevents silent mislabels; memory accrues new patterns; documented in ADR-0002 |
| Vector-index API differences | Verified against the live cluster; schema dumped into `DATA_MODEL.md` |

## 9. Defect log convention

File in the repo issue tracker as `SEV{1-4} — <component> — <symptom>`. SEV1 = data loss / wrong verdict shipped to `done`; SEV2 = fleet stall; SEV3 = dashboard/telemetry wrong; SEV4 = cosmetic. Historical fixes are catalogued in [`docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md) §"Common failure modes".
