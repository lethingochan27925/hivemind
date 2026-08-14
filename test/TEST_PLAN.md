# Master Test Plan — HiveMind

**Author:** QA · **Version:** 2.0 · **System under test:** HiveMind agent fleet, agentic memory layer, and the control platform that operates them.

## 1. Purpose

Prove that HiveMind does what it claims — durable memory, no double-work, correct verdicts, a real human-in-the-loop, live-tunable policy, and survival through crashes and region loss — with evidence a judge can reproduce in minutes.

A second, equally important goal: prove that **nothing in the control plane can be misused**. Every endpoint that mutates the system is tested from the attacker's side first.

## 2. Scope

**In scope:** agent verdict logic; the three memory tiers and their administration; fleet concurrency and crash recovery; the full control plane (policy, memory ops, task control, bulk review, budget guardrail, rollback, per-node control, multi-region); global search; the dashboard (10 pages, bilingual, themed, resizable); CI/CD pipeline definitions; deployment reproducibility.

**Out of scope:** end-user authentication/RBAC, multi-tenancy, payment-gateway integration, sustained load beyond ~50 concurrent workers. These are documented roadmap items, not silent gaps.

## 3. Strategy

| Level | What | Automated? | Where |
|-------|------|-----------|-------|
| Unit — pure logic | `balanceSignal`, `SanitizeField`, `RiskTier`, `EncodeVector`, `classifyPattern`, `Scratchpad`, `flexFloat64`, cost maths, policy defaults | ✅ `go test` | package `*_test.go` |
| Unit — **handler guards** | Every mutating control endpoint driven with a **nil DB / no AWS**: rejection must happen before storage | ✅ `go test` | `internal/dashboardapi/*_test.go` |
| Integration | DB-backed: SKIP LOCKED claims, memory concurrency | ✅ (needs `DATABASE_URL`) | `internal/memory/*_integration_test.go` |
| Deployment smoke | Live control plane over HTTP, incl. the read-only SQL guard | ✅ shell | `test/integration/api_smoke.sh` |
| Pipeline | CI/CD definitions: OIDC-only, canary has rollback, smoke gate wired | ✅ shell | `test/integration/pipeline_test.sh` |
| System / resilience | crash-kill, region-kill, human review, policy takes effect live | Manual, scripted → `evidence/` | §7 |
| UX | search, i18n, theme, drag-resize, optimistic updates | Manual | `test-cases/TC-UX.md` |

## 4. Traceability to the contest scoring criteria

| Criterion | Covering test cases |
|-----------|--------------------|
| Agentic Memory Design | TC-MEMORY-01..08, TC-MEMOPS-01..08 |
| Technical Implementation | TC-AGENT-*, TC-CONTROL-PLANE-*, TC-CONTROL-OPS-*, TC-SCORE-* |
| Real-World Impact | TC-AGENT-11 (recall/precision), §6 metrics, TC-BUD-* |
| Production Readiness | TC-CONC-*, TC-CICD-*, TC-DI-*, TC-POL-*, TC-RB-*, TC-REG-* |
| Creativity & Originality | TC-AGENT-04 (categorical tool), TC-POL-08 (policy fixes escalation storms) |

## 5. Entry / exit criteria

**Entry:** infra deployed via `scripts/init.sh`; schema loaded; PaySim seeded; Bedrock model access granted.

**Exit (ship gate):**
- `go test ./...` green (hermetic set requires no cloud).
- `bash test/integration/pipeline_test.sh` green.
- `bash test/integration/api_smoke.sh $API` green — including every mutating-statement rejection.
- Verdict accuracy ≥ target on the labelled eval (achieved: 100% recall / 100% precision).
- Zero double-claims across a 20-worker run.
- A killed agent's task resumes at the same step in ≤30s, captured in `evidence/`.
- No secret in the pushed repo (gitleaks clean).

## 6. Metrics tracked (measured, not estimated)

| Metric | Target | Measured |
|--------|--------|----------|
| Recall / precision (labelled eval) | ≥ 65% | **100% / 100%** (34/34 fraud, 0/340 false positives) |
| Auto-resolved without a human | report | **~46%** of the investigate tier |
| Cost per investigation run | minimise | **~$0.09** (Haiku + Titan) |
| Double-claims under 20 workers | 0 | 0 |
| Task resume latency after kill | ≤ 30s | ≤ 30s |
| Escalations caused by infrastructure (not ambiguity) | 0 | 0 once `fallback_action = requeue` |

## 7. Manual system & resilience scenarios

| ID | Scenario | Steps | Pass condition |
|----|----------|-------|----------------|
| SYS-01 | Stream → verdict | Feed 200 cases, run dispatch | All reach a terminal status; verdict mix matches labels |
| SYS-02 | Human-in-the-loop | Force an `escalate`; approve it in the Review Queue | `review_decision` set; `human_reviewed` audit row with reviewer name |
| SYS-03 | Policy takes effect live | Widen the auto-decide band; feed cases | Auto-resolve share rises within seconds, **no deploy** |
| SYS-04 | Escalation storm control | Set `fallback_action=requeue`, run during Bedrock throttling | Tasks re-queue instead of flooding the review queue |
| RES-01 | Crash-kill | Kill an in-flight agent (or use the Chaos button) | Re-queued ≤30s; a new worker resumes from `scratchpad.step`; recovery tracker shows both stages |
| RES-02 | Region-kill | Take the primary CockroachDB region offline | Fleet keeps writing; audit trail unbroken; RPO = 0 |
| RES-03 | Reaper | Stall a worker so `heartbeat_at` goes stale | Re-queued within one 30s sweep |
| RES-04 | Budget guardrail | Set the cap below today's spend, auto-pause on | Schedules disable themselves; `/control/fleet` confirms |
| RES-05 | Bad deploy | Roll a service back from the Pipeline page | Alias returns to the previous version; service recovers |

## 8. Risks

| Risk | Mitigation |
|------|-----------|
| Bedrock throttling skews accuracy under load | Adaptive retry; `fallback_action=requeue`; fallback verdicts are excluded from the model-accuracy metric and marked `step="fallback"` with a NULL `bedrock_model` |
| Signature over-fits PaySim | `escalate` path prevents silent mislabels; memory accrues new patterns; documented in ADR-0002 |
| Control plane is unauthenticated in demo mode | `CONTROL_TOKEN` gate exists and is tested; production auth is a documented roadmap item |
| Cost Explorer queries are themselves billed | Server-side 6h cache; asserted in the code and documented on the panel |

## 9. Defect log convention

`SEV{1-4} — <component> — <symptom>`. SEV1 = data loss / wrong verdict shipped to `done`; SEV2 = fleet stall; SEV3 = telemetry or console wrong; SEV4 = cosmetic. Fixed defects are catalogued in [`docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md) §"Common failure modes".

**Defects found by this plan (and fixed):** review decisions on a re-queued case returned a raw SQL error (now `409` with a clear message); `alterRegion` reached the database before validating the action (now rejected first); the agent stamped `bedrock_model` on rule-based fallbacks (now NULL); `similarity_scores` were computed but never persisted (now written and surfaced in the Decision trace).
