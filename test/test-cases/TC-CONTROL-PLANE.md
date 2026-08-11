# TC-CONTROL-PLANE — Control API & read-only DB console

Traces to: **Technical Implementation, Production Readiness**. Smoke-automated by `test/integration/api_smoke.sh`. `$API` = `terraform output -raw dashboard_api_url`.

| ID | Priority | Precondition | Steps | Expected |
|----|----------|--------------|-------|----------|
| **TC-CP-01** | P0 | Deployed | `GET $API/control/fleet` | `200`; `{running, schedules[], tasks{}}`; schedule states are `ENABLED`/`DISABLED`; task counts are integers. |
| **TC-CP-02** | P0 | Deployed | `POST $API/control/fleet {"action":"pause"}` then `GET` again | Schedules become `DISABLED`; `running=false`. `{"action":"start"}` reverses it. |
| **TC-CP-03** | P1 | Pending tasks exist | `POST $API/control/dispatch` | `200`; `dispatcher_result.workers_invoked > 0`; pending count drops on the next `GET /control/fleet`. |
| **TC-CP-04** | P1 | Deployed | `POST $API/control/feed {"count":50}` | `200`; `requeued` ≈ 50; those transactions re-enter the stream. |
| **TC-CP-05** | P0 | Deployed | `POST $API/control/query {"sql":"DELETE FROM tasks"}` — and `DROP`, `UPDATE`, `INSERT`, and a `;`-chained statement | Every one returns **`400`** with a rejection reason. **No mutation occurs** (verify row counts unchanged). |
| **TC-CP-06** | P0 | Deployed | `POST $API/control/query {"sql":"SELECT verdict, COUNT(*) FROM tasks GROUP BY verdict"}` | `200`; `{columns, rows, row_count, truncated}`; shape matches. |
| **TC-CP-07** | P1 | ≥ 200 matching rows | `POST $API/control/query` a SELECT that would return > 200 rows | `row_count ≤ 200`, `truncated=true`. Result set is capped. |
| **TC-CP-08** | P1 | Deployed | `GET $API/control/db` | `200`; row counts for `transactions`, `tasks`, `case_memory`, `audit_log`; `total_rows` = their sum. |
| **TC-CP-09** | P1 | Deployed | `GET $API/control/lambdas` | `200`; each of the 7 functions with `state`, `version`, `memory_mb`, `timeout_sec`. |
| **TC-CP-10** | P1 | Deployed | `GET $API/control/resources` | `200`; the tagged AWS inventory (≈ 50+ resources) with `service`/`name`/`arn`; drives the live architecture map. |
| **TC-CP-11** | P2 | `CONTROL_TOKEN` set on the Lambda | `POST` a mutating endpoint **without** `X-Control-Token` | Rejected. With the correct token → allowed. |

## Evidence of TC-CP-05 already captured

The read-only guard was verified live during development: `DELETE`, `DROP`, and multi-statement inputs all returned `400`, while `SELECT`/`SHOW`/`WITH` returned rows — with a `LIMIT 200` cap applied. This is the single most important safety test on the control plane and must stay green.
