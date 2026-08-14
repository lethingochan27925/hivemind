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
