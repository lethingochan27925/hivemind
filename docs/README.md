# HiveMind Documentation

> Distributed memory & control plane for production agent fleets, proven on a fraud-investigation workload.
> **Stack:** CockroachDB Cloud (multi-region, vector) · Amazon Bedrock (Claude Haiku + Titan) · AWS Lambda · Next.js control plane.

This folder is the **reference documentation** for HiveMind. The root [`README.md`](../README.md) is the product/scope narrative; the docs below are the technical deep-dives, organised so a judge (or a new engineer) can find any answer in one hop.

## Map

| # | Document | Answers |
|---|----------|---------|
| 1 | [Architecture](ARCHITECTURE.md) | What are the components, how does a transaction flow end-to-end, how does the fleet survive crashes and region loss? |
| 2 | [Agentic Memory](AGENTIC_MEMORY.md) | How is CockroachDB used as a *production* memory layer — episodic (vector), working, audit — not a toy query? |
| 3 | [Data Model](DATA_MODEL.md) | The real schema: 4 tables, every index and why it exists, the ER diagram. |
| 4 | [Agent Reasoning](AGENT_REASONING.md) | How does the agent decide fraud/legit/escalate, and why does a *small* LLM get it right? |
| 5 | [Control Plane](CONTROL_PLANE.md) | What can be changed from the console, and what stops a mistake becoming an incident. |
| 6 | [CI/CD & DevOps](CICD.md) | The 8-workflow pipeline: verify → build → stage → smoke → canary → promote/rollback. |
| 7 | [API Reference](API.md) | Every dashboard-api endpoint — observability reads and the control plane — with request/response shapes and `curl` examples. |
| 8 | [Deployment Runbook](DEPLOYMENT.md) | Reproduce the whole system from zero: prerequisites, `scripts/init.sh`, Bedrock access, dashboard deploy. |
| 9 | [Security & Hardening](SECURITY.md) | IAM least-privilege, secrets in SSM, prompt-injection defence, read-only DB console, control-token gating. |
| — | [Architecture Decision Records](adr/) | The *why* behind the load-bearing choices, one decision per file. |
| [WORKFLOWS.md](WORKFLOWS.md) | Every routine as a single `make` target, grouped by situation |

## Quick facts

| | |
|---|---|
| **Go module** | `github.com/lethingochan27925/hivemind` |
| **Primary region** | `ap-southeast-1` (Singapore); CockroachDB multi-region SGP / Jakarta / Mumbai |
| **Reasoning model** | Amazon Bedrock — Claude Haiku (`ap-southeast-1`) |
| **Embedding model** | Amazon Bedrock — Titan Embeddings v2, 1024-dim (`us-east-1`) |
| **Memory store** | CockroachDB Cloud — `transactions`, `tasks`, `case_memory` (VECTOR 1024), `audit_log` |
| **Compute** | AWS Lambda (container images, Go) + EventBridge schedules |
| **Control plane** | Next.js static export on S3 + CloudFront, backed by `dashboard-api` Lambda |
| **Measured** | 100% recall & precision on the labelled eval · ~46% auto-resolved without a human · ~$0.09 per investigation run |
| **Console** | 10 pages, bilingual EN/VI, light/dark, drag-resizable panels, ⌘K command palette |

## For hackathon judges — the 60-second path

1. Read [Agentic Memory](AGENTIC_MEMORY.md) §"Why this is production-grade" — the core scoring criterion.
2. Skim [Data Model](DATA_MODEL.md) — the schema is real, dumped from the live cluster.
3. Open the live **Architecture** and **Database** pages in the dashboard — the topology and row counts are generated from the running system, not drawn.
4. Run the test suite: [`test/`](../test/) — `go test ./...` plus the API smoke script prove the claims above are reproducible.
