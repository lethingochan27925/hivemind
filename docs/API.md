# API Reference — `dashboard-api`

One Lambda behind a Function URL serves the whole operator surface: **observability reads**, the **control plane**, and **global search**. All responses are JSON. Mutating endpoints are POST-only and honour `X-Control-Token` when `CONTROL_TOKEN` is set on the function.

```bash
API=$(terraform -chdir=terraform output -raw dashboard_api_url); API="${API%/}"
```

## Observability

| Method | Path | Returns |
|--------|------|---------|
| GET | `/overview` | Verdict counts today, pending reviews, memory-hit trend, **learning curve**, live tasks, verdict accuracy |
| GET | `/reviews` | Escalated tasks awaiting a human decision |
| POST | `/reviews/decide` | Record one `approved`/`rejected` decision → `human_reviewed` audit row |
| POST | `/reviews/bulk` | Decide up to 500 cases at once (reviewer name required) |
| GET | `/memory` | Episodic stats: active/archived, avg salience, top patterns, active agents, recall impact |
| GET | `/transactions?risk_tier=` | Recent transactions, optionally filtered `low`/`medium`/`high` |
| GET | `/transactions/{id}/audit` | Full decision trail: action, agent, reasoning, **memory_hits + similarity_scores**, tokens, latency, `bedrock_model` |
| GET | `/infrastructure` | CloudWatch alarm state per service + crash/resume incidents |
| POST | `/infrastructure/simulate-crash` | Backdate an in-flight task's heartbeat (chaos test) |
| GET | `/cost` | Bedrock token spend today, per agent |
| GET | `/cost/infrastructure` | **AWS Cost Explorer** month-to-date by service (cached 6h) |

```bash
curl -s "$API/overview" | jq '{verdicts: .verdicts_today, accuracy: .verdict_accuracy_pct}'
curl -s "$API/transactions/$TXN/audit" | jq '.[] | {action, memory_hits, similarity_scores, latency_ms}'
```

## Search

| Method | Path | Returns |
|--------|------|---------|
| GET | `/control/search?q=` | Matching transactions, tasks, memories and agents (2–80 chars, parameterised) |

```bash
curl -s "$API/control/search?q=balance_wipe" | jq '{memories: (.memories|length), tasks: (.tasks|length)}'
curl -s -o /dev/null -w '%{http_code}\n' "$API/control/search?q=a"   # → 400, too short
```

## Fleet control

| Method | Path | Body | Effect |
|--------|------|------|--------|
| GET | `/control/fleet` | — | Schedule states + task counts by status |
| POST | `/control/fleet` | `{"action":"start"\|"pause"}` | Enable/disable all four schedules |
| POST | `/control/dispatch` | — | One dispatch cycle, synchronous |
| POST | `/control/feed` | `{"count":N}` | Re-queue N closed cases into the stream |
| POST | `/control/schedule` | `{"service":"reaper","action":"enable"\|"disable"}` | Toggle **one** node's schedule |
| POST | `/control/invoke` | `{"service":"dispatcher"}` | "Run now" — fleet services only |

```bash
curl -s "$API/control/fleet" | jq '{running, tasks}'
curl -s -XPOST "${hdr[@]}" -d '{"action":"pause"}' "$API/control/fleet"
```

`/control/invoke` refuses `dashboard-api`, `scoring-api`, `scoring-python` — the control plane must not be able to invoke itself.

## Agent policy & budget

| Method | Path | Body | Effect |
|--------|------|------|--------|
| GET | `/control/policy` | — | Current policy (table auto-created with defaults on first read) |
| POST | `/control/policy` | full policy object | Update thresholds, fan-out, fallback behaviour, budget |
| GET | `/control/budget` | — | Today's spend vs cap; **auto-pauses the fleet** if armed and exceeded |

```bash
curl -s "$API/control/policy" | jq
curl -s -XPOST "${hdr[@]}" -d '{"risk_low":0.01,"risk_high":0.98,"dispatch_batch":20,
  "fallback_action":"requeue","daily_budget_usd":5,"auto_pause_on_budget":true,
  "updated_by":"ops"}' "$API/control/policy"
```

Validation: `0 ≤ risk_low < risk_high ≤ 1`, `dispatch_batch` 1–200, `fallback_action ∈ {escalate, requeue}`, `daily_budget_usd` 0–1000. The agent re-reads this every task (15s cache) and falls back to compiled defaults if the row is missing.

## Memory administration

| Method | Path | Body | Effect |
|--------|------|------|--------|
| GET | `/control/memory?limit=&archived=` | — | Episodic rows with salience, recall/merge counts |
| POST | `/control/memory` | `{"action":"pin"\|"unpin"\|"archive"\|"unarchive"\|"delete","id":"…"}` | Administer one memory |
| POST | `/control/memory/job` | `{"job":"decay"}` or `{"job":"archive_below","threshold":0.3}` | Invoke the real decay Lambda, or bulk-archive |

```bash
curl -s "$API/control/memory?limit=10" | jq '.[] | {pattern_type, salience, recall_count}'
curl -s -XPOST "${hdr[@]}" -d '{"job":"decay"}' "$API/control/memory/job"
```

## Task control

| Method | Path | Body | Effect |
|--------|------|------|--------|
| POST | `/control/task` | `{"action":"requeue","task_id":"…"}` | Hand a case back to the fleet (accepts task **or** transaction id) |
| POST | `/control/task` | `{"action":"override","task_id":"…","verdict":"fraud","reviewer_id":"ops","notes":"…"}` | Operator override + `human_reviewed` audit row |

## Infrastructure & delivery

| Method | Path | Returns / effect |
|--------|------|------------------|
| GET | `/control/lambdas` | Live config of all seven functions |
| GET | `/control/resources` | Every AWS resource tagged `Project=hivemind` |
| GET | `/control/regions` | Database regions + survival goal |
| POST | `/control/regions` | `{"action":"add"\|"drop"\|"set_primary"\|"survive_region"\|"survive_zone","region":"aws-ap-south-1"}` |
| POST | `/control/rollback` | `{"service":"dashboard-api"}` → moves alias `live` to the previous version |

## Database console

| Method | Path | Returns |
|--------|------|---------|
| GET | `/control/db` | Row counts for the four tables + total |
| POST | `/control/query` | `{"sql":"SELECT …"}` → `{columns, rows, row_count, truncated}` |

**Read-only, enforced server-side:** must start `SELECT`/`SHOW`/`WITH`, single statement only, no mutating keyword (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, `CREATE`, `GRANT`, `REVOKE`, `TRUNCATE`, `UPSERT`, `COMMENT ON`), capped at 200 rows — with a read-scoped database role underneath as defence in depth.

```bash
curl -s -XPOST "${hdr[@]}" -d '{"sql":"SELECT verdict, COUNT(*) FROM tasks GROUP BY verdict"}' "$API/control/query" | jq
curl -s -o /dev/null -w '%{http_code}\n' -XPOST "${hdr[@]}" -d '{"sql":"DELETE FROM tasks"}' "$API/control/query"  # → 400
```

## Status codes

| Code | Meaning |
|------|---------|
| `200` | OK |
| `400` | Input rejected by an allow-list (unknown action/service/verdict/region, bad SQL, out-of-range policy) |
| `403` | `CONTROL_TOKEN` set and `X-Control-Token` missing or wrong |
| `404` | Target row not found (e.g. memory id) |
| `405` | Wrong method (mutations are POST-only) |
| `409` | Review decision on a case that was re-queued or already decided |
| `500` | Upstream failure (AWS/database) — the message carries the cause |

Every rejection path above is covered by `test/integration/api_smoke.sh` and the handler-guard unit tests.
