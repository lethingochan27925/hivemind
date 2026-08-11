# TC-DATA-INTEGRITY — Schema constraints as a safety net

Traces to: **Production Readiness, Agentic Memory Design**. The database enforces correctness the application might otherwise get wrong. These are DB-backed (need `DATABASE_URL`); each asserts the constraint **rejects** bad data. Schema: [`docs/DATA_MODEL.md`](../../docs/DATA_MODEL.md).

| ID | Priority | Table | Steps | Expected |
|----|----------|-------|-------|----------|
| **TC-DI-01** | P0 | `tasks` | `INSERT` a second task with the same `transaction_id` | Rejected by `UNIQUE(transaction_id)` — a transaction can never be investigated twice. |
| **TC-DI-02** | P0 | `tasks` | `UPDATE` a task to `verdict = 'maybe'` | Rejected by `verdict_check` — only `fraud|legit|escalate` are storable. The model cannot persist an invented verdict. |
| **TC-DI-03** | P0 | `tasks` | `UPDATE` a task to `status = 'sleeping'` | Rejected by `status_check` — only the 7 lifecycle states are valid. |
| **TC-DI-04** | P0 | `tasks` | `INSERT` a task whose `transaction_id` doesn't exist | Rejected by the FK to `transactions(id)` — no orphaned work. |
| **TC-DI-05** | P0 | `case_memory` | `INSERT`/`UPDATE` `salience = 2.5` (or `-0.1`) | Rejected by `salience_ck (0.0–2.0)` — reinforcement/decay cannot run out of range. |
| **TC-DI-06** | P1 | `case_memory` | `INSERT` with `verdict = 'unknown'` | Rejected by `verdict_ck`. |
| **TC-DI-07** | P1 | `case_memory` | `INSERT` with `transaction_type = 'PAYMENT'` | Rejected by `type_ck` — only `TRANSFER|CASH_OUT` (the PaySim types that ever contain fraud). |
| **TC-DI-08** | P0 | `audit_log` | `INSERT` with `action = 'hacked'` | Rejected by `action_ck` — only the 13 defined actions can be appended. |
| **TC-DI-09** | P0 | `audit_log` | `INSERT` referencing a non-existent `task_id` | Rejected by the FK — the audit trail can't reference a task that doesn't exist. |
| **TC-DI-10** | P1 | `transactions` | `INSERT` with `type = 'DEBIT'` or `risk_tier = 'critical'` | Rejected by `type_check` / `tier_check`. |
| **TC-DI-11** | P2 | `case_memory` | Read the vector index definition | Index is `WHERE archived = false` — archived memories are physically outside the search space (correctness *and* performance). |

## Why this belongs in the test plan

For an agent system, the database is the last line of defence against a model or a bug writing something nonsensical. These constraints turn "the agent should only ever produce a valid verdict" from a hope into an invariant the storage layer enforces. TC-DI-02, TC-DI-05, and TC-DI-08 are the ones a judge will find most convincing: the system cannot persist a bad verdict, an out-of-range salience, or a forged audit action even if every layer above failed.

## Running

```bash
export DATABASE_URL='postgresql://…cockroachlabs.cloud:26257/hivemind?sslmode=verify-full'
# each check is a one-liner; example:
psql "$DATABASE_URL" -c "UPDATE tasks SET verdict='maybe' WHERE false;" ; echo "expected: ERROR violates check constraint verdict_check"
```
