# ADR-0001 — CockroachDB as the unified agentic-memory substrate

**Status:** Accepted · **Context:** the fleet needs working, episodic (vector), and audit memory.

## Decision

Use one CockroachDB Cloud cluster for all three memory tiers — `tasks` (working), `case_memory` (episodic, `VECTOR(1024)`), `audit_log` (audit) — instead of a Redis + a dedicated vector DB + a Postgres.

## Why not Postgres?

Postgres genuinely covers the demo: `SKIP LOCKED` queues, `pgvector`, append-only audit — single-region, 20 agents, hundreds of tasks, RDS is fine. We say so plainly. The choice is about what breaks when the fleet is *real*:

1. **Region failure with RPO = 0 while still writable.** An agent holding a customer's transaction cannot lose its last write. Postgres failover risks the last records; CockroachDB keeps consensus on surviving regions with zero data loss.
2. **Horizontal write scale.** The workload is write-heavy by nature (heartbeats, audit, scratchpad, embeddings). A single-writer Postgres bottlenecks at fleet scale; CockroachDB adds nodes.
3. **One consistent system for three data shapes.** Redis + Pinecone + Postgres is three failure modes with no cross-store transaction. Here an `audit_log` row and the `tasks` update that produced it commit together, and a foreign key guarantees the audit row can't reference a missing task.

## Consequences

- Two CockroachDB capabilities would satisfy the contest; we use three (vector index, MCP, multi-region SQL) because each maps to a real fleet requirement.
- The vector index is *partial* (`WHERE archived = false`) so forgetting shrinks the search space — a CockroachDB feature that directly serves the memory design.
- Trade-off accepted: CockroachDB's per-statement latency is higher than a local Postgres; mitigated by keeping the agent's hot path to a bounded top-k retrieval, never a full-history scan.
