# Architecture

HiveMind is a **fleet of stateless Lambda workers** coordinating through **one strongly-consistent database**. No worker holds durable state in memory; everything that must survive a crash lives in CockroachDB. That single decision is what lets an agent be killed mid-investigation and have another pick up exactly where it left off.

## Components

| Component | Binary (`cmd/`) | Runtime | Responsibility |
|-----------|-----------------|---------|----------------|
| **Scoring API** | `scoring-api` | Lambda (Function URL) | Scores an incoming transaction; routes low→auto-approve, high→auto-block, medium→investigate. |
| **Dispatcher** | `dispatcher` | Lambda (EventBridge + on-demand) | Reads medium-risk transactions, writes one `tasks` row each (working memory), invokes the worker fleet. |
| **Agent Worker** | `worker` | Lambda ×N | Claims a task (`SKIP LOCKED`), validates input, recalls memory, calls Bedrock, writes a verdict + full audit trail. |
| **Heartbeat Reaper** | `heartbeat-reaper` | Lambda (EventBridge, 30s) | Re-queues tasks whose `heartbeat_at` has gone stale — a crashed agent's work returns to the pool. |
| **Salience Decay** | `salience-decay` | Lambda (EventBridge, 6h) | Ages unused memories down and archives the forgotten ones (GEM-style forgetting). |
| **Dashboard API** | `dashboard-api` | Lambda (Function URL) | Read endpoints for the UI + the **control plane** (fleet/dispatch/feed/db/query). |
| **Review CLI** | `review` | CLI | Operator tool for the human-in-the-loop review queue. |

All state lives in **CockroachDB Cloud**; reasoning and embeddings come from **Amazon Bedrock**; schedules are **EventBridge** rules; the operator UI is a **Next.js static site** on **S3 + CloudFront**.

## End-to-end flow

<p align="center">
  <img src="images/architecture-end-to-end-flow.png" alt="End-to-end flow: scoring routes transactions to auto-approve, auto-block or the dispatcher, the agent fleet claims tasks, recalls memory, reasons with Bedrock, and either closes the case or hands it to the human review queue" width="560">
</p>

## The investigation lifecycle (one task)

<p align="center">
  <img src="images/architecture-investigation-lifecycle.png" alt="Sequence diagram: dispatcher inserts a task, the worker claims it, calls MCP and vector-searches memory, reasons with Claude Haiku, then writes the verdict and either escalates or embeds and consolidates the case into memory" width="700">
</p>

Each arrow that touches `audit_log` writes exactly one append-only row (`mcp_query`, `memory_recall`, `bedrock_reasoning`, `verdict_*`, …) with tokens and latency, so a single `SELECT ... WHERE task_id = $1 ORDER BY created_at` replays the entire decision.

## Resilience model

Three independent failure modes, three mechanisms — all resolved in the database, none needing an external coordinator:

| Failure | Mechanism | Guarantee |
|---------|-----------|-----------|
| **Two agents grab the same task** | `SELECT … FOR UPDATE SKIP LOCKED` on the claim | Exactly-once claim; `tasks.transaction_id` is `UNIQUE`, so a transaction can never be held twice. |
| **An agent crashes mid-investigation** | `heartbeat_at` + Heartbeat Reaper (30s) | Stale task re-queues; a new worker reads `step` + `scratchpad` (JSONB) and **resumes at the same step**, not from scratch. |
| **A whole region goes down** | CockroachDB multi-region consensus | Writes continue on the surviving regions with **RPO = 0**; no audit gap, no lost verdict. |
| **An async invocation exhausts its retries** | SQS dead-letter queue (`async_dlq`, `terraform/modules/iam`), wired via `on_failure` destination on every asynchronously-invoked function (`agent-worker`, `dispatcher`, `reaper`, `salience-decay`) | A failure that survives Lambda's own retry budget sits visibly in a queue instead of vanishing with nothing in any log to say the invocation itself never ran. Deliberately **not** wired to `scoring-api`/`scoring-python`/`dashboard-api` — those are called synchronously over Function URL, and an async destination is a documented no-op for a synchronous invocation. |

CloudWatch also watches the two failure modes with no request-scoped signal to catch them: a `Bedrock` alarm on `InvocationThrottles` per model ID (Claude + Titan), and the canary's own alarm-history check (not just a final-state poll) during its observation window — see [CI/CD](CICD.md#the-canary).

## Why the boundaries are where they are

- **Scoring vs. investigation are separate services.** Scoring is a cheap classifier that sweeps every transaction; investigation is an expensive agent that only runs on the ~2% the classifier can't clear. Splitting them keeps cost bounded. See [ADR-0003](adr/0003-lambda-fleet-and-skip-locked.md).
- **Reads go through MCP, writes go through the SQL driver.** The agent's *exploration* of customer data is read-only by construction (the MCP server exposes only `SELECT`), while state mutations use the typed `pgx` driver. Least-privilege at the protocol layer. See [Security](SECURITY.md).
- **The dashboard is static.** No server to attack or scale; it talks only to the `dashboard-api` Lambda over a Function URL. The control plane is one small, IAM-scoped surface. See [API](API.md).
