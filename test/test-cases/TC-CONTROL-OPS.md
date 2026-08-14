# TC-CONTROL-OPS — Live operations: policy, memory, tasks, budget, rollback

Traces to: **Production Readiness, Agentic Memory Design**. These are the endpoints that **change the running system**, so every case below is written from an attacker's or a fat-fingered operator's point of view first, and the happy path second.

Automated guards: `internal/dashboardapi/ops_test.go`, `regions_test.go`, `node_control_test.go`, `cloudcost_test.go`, `internal/agent/policy_test.go`. Live cases marked *live*.

## A. Agent policy (`/control/policy`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-POL-01** | P0 | `GET /control/policy` on a fresh database | `200`; the `system_policy` table is **created on demand** with defaults (0.001 / 0.999 / 20 / escalate). No migration step required. |
| **TC-POL-02** | P0 | `POST` with `risk_low = 0.9`, `risk_high = 0.2` | `400` — `risk_low` must stay below `risk_high`. An inverted band would make every transaction both auto-approved and auto-blocked. |
| **TC-POL-03** | P0 | `POST` with `risk_low = -1` / `risk_high = 5` | `400` — the band is clamped to `[0,1]`. |
| **TC-POL-04** | P1 | `POST` with `dispatch_batch = 0` and `= 500` | `400` on both — 1..200 only. Zero would stall the fleet; 500 would fan out beyond the Bedrock quota. |
| **TC-POL-05** | P0 | `POST` with `fallback_action = "ignore"` | `400` — only `escalate` or `requeue`. |
| **TC-POL-06** | P1 | `POST` with `daily_budget_usd = 100000` | `400` — capped at 1000 so a typo cannot disable the guardrail. |
| **TC-POL-07** | P0 *live* | Set `risk_low = 0.2`, wait ~15s, feed cases | The agent auto-approves the 0.001–0.2 band **without a Bedrock call** — visible as `auto_approve` audit rows with no `bedrock_model`. Policy takes effect **without a deploy**. |
| **TC-POL-08** | P0 *live* | Set `fallback_action = requeue`, force a Bedrock failure (or run during throttling) | Tasks are re-queued (`task_requeued` audit rows) instead of landing in the human review queue. **This is the fix for escalation storms caused by infrastructure, not by ambiguity.** |
| **TC-POL-09** | P0 | Delete/rename the `system_policy` row, then process a task | The agent falls back to the compiled defaults — it must never read zero-valued thresholds (that would auto-approve everything). Asserted by `TestDefaultPolicyMatchesCompiledThresholds`. |
| **TC-POL-10** | P1 | `POST` with `CONTROL_TOKEN` set and no `X-Control-Token` | `403`. |

## B. Memory administration (`/control/memory`, `/control/memory/job`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-MEMOPS-01** | P0 | `POST {action:"drop_table"}` / missing `id` / empty action | `400` on all — the action is an allow-list, not a passthrough. |
| **TC-MEMOPS-02** | P1 | Pin a memory, then run the decay job | Salience is set to the 2.0 ceiling and **survives decay** — a pinned case can never be forgotten. |
| **TC-MEMOPS-03** | P1 | Archive a memory, then run a recall | The memory disappears from vector search results (partial index `WHERE archived = false`), and reappears after *unarchive*. |
| **TC-MEMOPS-04** | P1 | Unarchive a memory whose salience had decayed to ~0 | Salience is lifted to at least 0.5, otherwise the next decay run would immediately archive it again (silent no-op for the operator). |
| **TC-MEMOPS-05** | P0 | Delete a memory | Row is gone; `case_memory` count drops by one; no orphan is left in the vector index. |
| **TC-MEMOPS-06** | P0 | `POST {job:"archive_below", threshold: 0 / -1 / 2.5}` | `400` on all — the threshold must sit inside the salience domain `(0,2]`. |
| **TC-MEMOPS-07** | P1 *live* | `POST {job:"decay"}` | The real `salience-decay` Lambda is invoked (not an inline SQL copy), so the console exercises the same code path the scheduler does. |
| **TC-MEMOPS-08** | P2 | Act on a non-existent memory id | `404`, not a silent success. |

## C. Task-level control (`/control/task`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-TASK-01** | P0 | `POST {action:"override", verdict:"probably_fraud"}` | `400` — the verdict allow-list is enforced server-side even for a human. |
| **TC-TASK-02** | P0 | `POST {action:"override"}` without `reviewer_id` | `400` — an override with no named human would break the compliance trail. |
| **TC-TASK-03** | P0 | Override a case by **transaction id** (as the Transactions page does) | The task is resolved and the `audit_log` row is written with the **task's real id**, so the foreign key holds. |
| **TC-TASK-04** | P1 | Re-investigate (`requeue`) a completed case | Status returns to `pending`, verdict/scratchpad cleared, and a fresh agent re-runs it. |
| **TC-TASK-05** | P1 | `POST {action:"delete_everything"}` | `400` — unknown actions are rejected before any storage access. |
| **TC-TASK-06** | P1 *live* | Override, then open the Decision trace | The trace ends with a `human_reviewed` step showing the reviewer's name and note. |

## D. Bulk review (`/reviews/bulk`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-BULK-01** | P0 | Bulk approve without `reviewer_id` | `400` — no anonymous mass approvals. |
| **TC-BULK-02** | P0 | Bulk with 501 ids | `400` — capped at 500 per call. |
| **TC-BULK-03** | P1 | Bulk approve 50 cases where 5 were already re-queued | `200` with `{decided: 45, failed: 5}` — partial success is reported honestly, not swallowed. |
| **TC-BULK-04** | P1 *live* | Select all → "Back to agent" | Every selected case returns to `pending`; the queue empties immediately in the UI (optimistic update) and stays empty after the next poll. |

## E. Budget guardrail (`/control/budget`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-BUD-01** | P1 | `GET /control/budget` | Returns today's Bedrock spend, the cap, `used_pct`, `exceeded`. |
| **TC-BUD-02** | P0 *live* | Set a cap **below** today's spend with auto-pause **on**, then poll | `paused_by_rule: true` **and the EventBridge schedules are actually disabled** — verify on `/control/fleet`. The guardrail must act, not just warn. |
| **TC-BUD-03** | P1 *live* | Same with auto-pause **off** | `exceeded: true`, `paused_by_rule: false`, fleet keeps running. |
| **TC-BUD-04** | P2 | Set cap to 0 | Division-by-zero is avoided; `used_pct = 0`, `exceeded = false`. |

## F. Version rollback (`/control/rollback`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-RB-01** | P0 | `POST {service:"../etc/passwd"}` / unknown / empty | `400` — the service name is an allow-list; it becomes a Lambda ARN. |
| **TC-RB-02** | P1 *live* | Roll back `dashboard-api` | Alias `live` moves to the previous published version; response reports `from_version` → `to_version`; the Infrastructure page shows the new version within one poll. |
| **TC-RB-03** | P1 *live* | Roll back a function that has only version 1 | `400` with "no earlier published version" — never silently points the alias at `$LATEST`. |

## G. Per-node control (`/control/schedule`, `/control/invoke`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-NODE-01** | P0 | Invoke `dashboard-api` from the control plane | `400` — the control plane must not be able to invoke itself (trivial recursion / cost amplification). Asserted by `TestInvokableIsRestrictedToTheFleet`. |
| **TC-NODE-02** | P0 | Enable a schedule for a service that has none (`scoring-api`) | `400`. |
| **TC-NODE-03** | P1 *live* | Disable only `reaper`'s schedule | Only that EventBridge rule flips to `DISABLED`; the other three keep running — per-node control, not all-or-nothing. |
| **TC-NODE-04** | P1 *live* | "Run now" on `dispatcher` | Function is invoked asynchronously; pending tasks drop on the next poll. |

## H. Multi-region (`/control/regions`)

| ID | Priority | Steps | Expected |
|----|----------|-------|----------|
| **TC-REG-01** | P0 | `POST {action:"add", region:"x\"; DROP DATABASE hivemind; --"}` | `400` — the region name must match `^[a-z0-9][a-z0-9-]{1,62}$` before it is ever interpolated into `ALTER DATABASE`. |
| **TC-REG-02** | P0 | `POST {action:"nuke"}` | `400` **and no database round-trip** — the action allow-list runs first. |
| **TC-REG-03** | P0 | `POST {action:"drop"}` with `CONTROL_TOKEN` set, no header | `403` — dropping a region must never be unauthenticated. |
| **TC-REG-04** | P1 *live* | Add a region the cluster has not provisioned | `400` carrying CockroachDB's own message, so the operator knows to provision at the cluster tier first. |
| **TC-REG-05** | P1 *live* | Add a second region, then `survive_region` | `SHOW SURVIVAL GOAL` reports region failure; the Multi-region panel badge updates. |

## Why these are written as rejections first

Every endpoint in this file mutates production state: thresholds that decide whether money moves, memories the fleet reasons from, verdicts on customer transactions, and the version of code that is live. The automated tests drive the **real handlers with a nil database and no AWS client** — so if a validation guard is ever deleted, the test panics on the nil pool instead of quietly letting the mutation through. That inversion is the point: the suite fails loudly the moment an unsafe input can reach storage.
