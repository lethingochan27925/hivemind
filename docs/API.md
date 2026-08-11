# API Reference — `dashboard-api`

One Lambda behind a Function URL serves the whole operator surface: **observability reads** (what the fleet is doing) and the **control plane** (act on the fleet, the database, and the infrastructure). All responses are JSON; CORS is open for GET, and mutating control endpoints accept an optional `X-Control-Token` header.

Base URL: `terraform -chdir=terraform output -raw dashboard_api_url` (a `*.lambda-url.ap-southeast-1.on.aws` origin).

## Observability (read-only)

| Method | Path | Returns |
|--------|------|---------|
| GET | `/overview` | Today's verdict counts, pending-review count, memory-hit trend, live claimed tasks, verdict accuracy. |
| GET | `/reviews` | The human review queue — escalated tasks awaiting a decision. |
| POST | `/reviews/decide` | Record an `approved`/`rejected` decision (writes `audit_log` `human_reviewed`). |
| GET | `/memory` | Episodic-memory stats: active/archived cases, avg salience, top patterns, active agents, recall impact. |
| GET | `/transactions?risk_tier=` | Recent transactions, optionally filtered by `low`/`medium`/`high`. |
| GET | `/transactions/{id}/audit` | The full append-only decision trail for one transaction. |
| GET | `/infrastructure` | Per-service CloudWatch alarm state + recent incidents. |
| GET | `/cost` | Token spend and estimated USD, today and per agent. |

```bash
curl -s "$API/overview" | jq
curl -s "$API/transactions?risk_tier=high" | jq '.[0]'
curl -s "$API/transactions/$TXN_ID/audit" | jq '.[].action'
```

## Control plane

### Fleet

```bash
# status: are the EventBridge schedules enabled + live task counts by status
curl -s "$API/control/fleet" | jq
# → { "running": true, "schedules": [{"service":"dispatcher","state":"ENABLED"}, …],
#     "tasks": {"pending": 12, "investigating": 3, "done": 981, …} }

# start / pause the whole fleet (enables/disables the EventBridge rules)
curl -s -XPOST "$API/control/fleet" -H 'Content-Type: application/json' \
     -H "X-Control-Token: $TOKEN" -d '{"action":"pause"}'
```

### Dispatch & feed

```bash
# run one dispatch cycle synchronously — invokes the worker fleet now
curl -s -XPOST "$API/control/dispatch" -H "X-Control-Token: $TOKEN" | jq
# → { "status":"ok", "dispatcher_result": {"tasks_created":20,"pending_tasks":20,"workers_invoked":20} }

# re-queue N transactions back into the stream (demo replay)
curl -s -XPOST "$API/control/feed" -H 'Content-Type: application/json' \
     -H "X-Control-Token: $TOKEN" -d '{"count":50}' | jq
```

### Infrastructure inventory

```bash
# every deployed Lambda's live config (state, version, memory, timeout)
curl -s "$API/control/lambdas" | jq '.[] | {service, state, version}'

# every resource Terraform tagged Project=hivemind (drives the live architecture map)
curl -s "$API/control/resources" | jq 'group_by(.service) | map({service: .[0].service, n: length})'
```

### Database console

```bash
# table row counts (transactions / tasks / case_memory / audit_log) + total
curl -s "$API/control/db" | jq
# → { "database":"hivemind", "total_rows":13959,
#     "tables":[{"table":"transactions","rows":1002}, …] }

# read-only SQL console — SELECT / SHOW / WITH only, single statement, LIMIT 200
curl -s -XPOST "$API/control/query" -H 'Content-Type: application/json' \
     -H "X-Control-Token: $TOKEN" \
     -d '{"sql":"SELECT verdict, COUNT(*) FROM tasks WHERE verdict IS NOT NULL GROUP BY verdict"}' | jq
# → { "columns":["verdict","count"], "rows":[["legit",...],["fraud",...]], "row_count":3, "truncated":false }
```

## Read-only query guard

`/control/query` is **defence-in-depth read-only**, enforced server-side ([`internal/dashboardapi/control.go`](../internal/dashboardapi/control.go)):

1. The statement must start with `SELECT`, `SHOW`, or `WITH`.
2. Exactly one statement (no `;`-chaining).
3. Any mutating keyword (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, `CREATE`, `GRANT`, `TRUNCATE`, …) is rejected.
4. A `LIMIT 200` is enforced; larger result sets report `truncated: true`.

Even if every check were bypassed, the database role the endpoint uses is itself read-scoped — the guard is the outer layer, not the only one. A `DELETE` returns `400`, verified in the test suite.

## Auth model

Control mutations honour an optional `CONTROL_TOKEN` env on the Lambda. When set, mutating endpoints require a matching `X-Control-Token`; when unset (local/dev), they're open. The dashboard forwards `NEXT_PUBLIC_CONTROL_TOKEN` automatically when configured. Read endpoints are always open (the data is non-sensitive synthetic PaySim). See [Security](SECURITY.md).
