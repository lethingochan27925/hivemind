# TC-CONCURRENCY — Fleet coordination, crash recovery, resilience

Traces to: **Production Readiness**. Partly automated by `internal/memory/concurrency_integration_test.go` and `working_integration_test.go` (need `DATABASE_URL`); resilience cases are scripted manual (→ `evidence/`).

| ID | Priority | Precondition | Steps | Expected |
|----|----------|--------------|-------|----------|
| **TC-CONC-01** | P0 | 500 pending tasks; 20 workers | Start the fleet; let it drain the queue. | Every task reaches a terminal status; **zero double-claims** (no two `claimed_by` for one task); no task left `pending` at the end. |
| **TC-CONC-02** | P0 | Two workers poll simultaneously | Both run the claim query against the same pending row. | `FOR UPDATE SKIP LOCKED` gives the row to exactly one; the other skips to the next row. No block, no duplicate. |
| **TC-CONC-03** | P0 | A dispatcher race (two dispatchers, same transaction) | Both attempt to insert a task for one `transaction_id`. | `UNIQUE(transaction_id)` rejects the second; the transaction is investigated once. |
| **TC-CONC-04** | P0 | A worker mid-investigation with `scratchpad.step` set | `kill -9` the worker. | Task's `heartbeat_at` goes stale; `heartbeat-reaper` re-queues it within 30s; a new worker reads `scratchpad` and **resumes at the recorded step**, not from scratch. |
| **TC-CONC-05** | P1 | A stalled (not dead) worker | Let its heartbeat lapse while it's technically alive. | Reaper re-queues on staleness alone; if the old worker later completes, `UNIQUE`/status guards prevent a conflicting write. |
| **TC-CONC-06** | P1 | Fleet running, multi-region cluster | Take the **primary region** offline (kill-region). | Writes continue on surviving regions; audit trail has no gap; **RPO = 0**; when the region returns, it rejoins consensus. |
| **TC-CONC-07** | P2 | Idle fleet | Observe billing/compute at rest. | No Lambda compute billed while the queue is empty — cost scales to zero. |

## How to run the automated slice

```bash
export DATABASE_URL='postgresql://…cockroachlabs.cloud:26257/hivemind?sslmode=verify-full'
go test ./internal/memory/ -run Integration -v
```

## How to stage RES scenarios for the video

- **Crash-kill (TC-CONC-04):** `aws lambda` concurrency of 20, feed 500, then kill a batch of invocations; watch the Reaper re-queue and a fresh worker log `resuming from step=…`.
- **Region-kill (TC-CONC-06):** in the CockroachDB Cloud console, drop the primary region; keep the dashboard open — task throughput dips then continues, audit count keeps climbing.
