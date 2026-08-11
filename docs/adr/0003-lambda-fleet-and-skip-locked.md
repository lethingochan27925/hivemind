# ADR-0003 — Stateless Lambda fleet coordinating via `SKIP LOCKED`

**Status:** Accepted · **Context:** N agents must process a task stream concurrently, survive crashes, and cost little at idle.

## Decision

Run agents as **stateless AWS Lambda workers** (container images, Go, one task per invocation) that coordinate purely through the `tasks` table using `SELECT … FOR UPDATE SKIP LOCKED`. No long-running processes, no external queue, no leader.

## Why

- **Coordination lives in the database we already trust.** `SKIP LOCKED` gives exactly-once claiming across any number of concurrent workers with no broker (SQS/Kafka) and no coordinator. `UNIQUE(transaction_id)` makes double-investigation impossible even under dispatcher races.
- **Crash recovery is free.** Because progress (`step`, `scratchpad`) is committed to the DB, a killed worker loses nothing — the Heartbeat Reaper re-queues the stale task in ≤30s and another worker resumes. A long-running stateful process would need its own checkpointing.
- **Cost scales to zero.** Idle fleet = no compute billed. A burst of medium-risk transactions fans out to many concurrent Lambda invocations, then drains back to zero.

## Alternatives considered

| Option | Rejected because |
|--------|------------------|
| EKS / Kubernetes long-running agents | Always-on cost; bespoke checkpointing; heavier ops for a demo — replaced EKS in scope v3 |
| SQS/Kafka work queue | A second system with its own failure mode; `SKIP LOCKED` already gives ordered, exactly-once claiming inside the source of truth |
| Step Functions | Orchestration we don't need — the state machine *is* the `tasks.status` column |

## Consequences

- Every Lambda binary is a thin `lambda.Start` handler wrapping a `RunOnce`; the same code runs as a local loop for dev.
- The `live` alias + `ignore_changes=[function_version]` pattern requires an explicit publish-then-update-alias step after each image push (see [Deployment](../DEPLOYMENT.md)).
- Bedrock throttling under a 20-worker burst is handled with adaptive retry rather than by capping concurrency, preserving throughput.
