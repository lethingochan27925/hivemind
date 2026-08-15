# HiveMind — Hackathon Submission

**Hackathon:** CockroachDB × AWS — Build with Agentic Memory
**Repository:** https://github.com/lethingochan27925/hivemind (public, Apache 2.0)
**Live dashboard:** https://d3hfe5ggacbomx.cloudfront.net
**Demo video (< 3 min):** _<YouTube/Vimeo link>_

## One-line pitch

HiveMind is a **distributed memory and control plane for production agent fleets**, built on CockroachDB. To prove it withstands the harshest workload, it runs a fleet of fraud-investigation agents over PaySim data — where losing state means losing money and every decision must be auditable.

## What it does

A stream of payment transactions is scored; the ~2% the classifier can't clear are handed to a fleet of stateless Lambda agents. Each agent claims a case, recalls similar past cases from shared **episodic memory**, reasons with Claude Haiku, and returns `fraud` / `legit` / `escalate` — writing an append-only audit trail for every step. Crashed agents resume mid-case; a whole region can fail with zero data loss. Humans review only the cases the agents themselves flag as uncertain. Operators watch and drive the entire system from a live control-plane dashboard.

## CockroachDB — tools used and what each does

> Minimum required: 2. HiveMind uses three, because each maps to a real requirement of running an agent fleet.

| CockroachDB capability | Where it's used | What it does |
|------------------------|-----------------|--------------|
| **Managed MCP Server** | `pkg/mcp` — the agent's `get_transaction`, `get_customer_context`, `search_similar_cases` tools | The agent explores customer data through a **read-only** protocol surface (SELECT only). It cannot mutate state while investigating — least-privilege at the protocol layer. |
| **Distributed Vector Indexing** | `case_memory.embedding VECTOR(1024)`, `VECTOR INDEX … vector_l2_ops WHERE archived = false` | Powers episodic recall: an agent embeds the current alert (Titan) and retrieves the top-k most similar past cases. The index is **partial on live memories**, so salience-based forgetting shrinks the search space. |
| **Multi-region, strongly-consistent SQL** | `tasks` (working memory), `audit_log` (audit) | `SELECT … FOR UPDATE SKIP LOCKED` coordinates 20+ agents with no broker; `UNIQUE(transaction_id)` guarantees exactly-once investigation; foreign-keyed append-only audit survives a region loss with **RPO = 0**. |

Provisioning uses the **`ccloud` CLI** + Terraform (`scripts/init.sh`). All three memory tiers live in **one** cluster, so an audit row and the task update that produced it commit together — a guarantee a Redis + Pinecone + Postgres split cannot make.

## AWS — services used and what each does

> Minimum required: 1.

| AWS service | What it does |
|-------------|--------------|
| **Amazon Bedrock** | **Claude Haiku** for case reasoning (verdict + rationale) and **Titan Embeddings v2** (1024-dim) for episodic-memory vectors. |
| **AWS Lambda** | All compute — 7 Go container-image functions (scoring, dispatcher, agent-worker ×N, heartbeat-reaper, salience-decay, dashboard-api). Scales to zero at idle, fans out on bursts. |
| **Amazon EventBridge** | Schedules the dispatcher, the heartbeat reaper (30s), and salience decay (6h). The control plane enables/disables these rules to start/pause the fleet. |
| **S3 + CloudFront** | Hosts the Next.js static dashboard; a CloudFront Function rewrites extensionless routes. |
| **CloudWatch** | Logs and alarms per function — the observability the dashboard surfaces. |
| **SSM Parameter Store** | Secrets (DB URL, MCP key) read at cold start — never committed, never baked into images. |
| **ECR · IAM** | Image registry; least-privilege roles per function. |

## How the scoring criteria are met

| Criterion | Evidence |
|-----------|----------|
| **Agentic Memory Design** | Three memory tiers, each with its own access pattern and lifecycle; vector recall + consolidation (merge > 0.92) + salience decay. See [`docs/AGENTIC_MEMORY.md`](docs/AGENTIC_MEMORY.md). |
| **Technical Implementation** | Real MCP integration, partial vector index, `SKIP LOCKED` coordination, SigV4-signed inter-Lambda calls, adaptive Bedrock retry. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). |
| **Real-World Impact** | Measured on the labelled eval: **100% recall & precision** (34/34 fraud, 0/340 false positives), **~46%** auto-resolved without a human, **$0.00023 per investigation** (≈ $0.11 per 500-case run). Reproducible via [`test/`](test/). |
| **Production Readiness** | Least-privilege IAM, secrets in SSM, prompt-injection defence, read-only DB console, crash/region resilience, CI/CD. See [`docs/SECURITY.md`](docs/SECURITY.md). |
| **Creativity & Originality** | A deterministic categorical tool (`balanceSignal`) makes a *small* model correct — design, not model size. See [`docs/adr/0002`](docs/adr/0002-categorical-tool-for-small-llm.md). |

## Reproduce it

```bash
git clone https://github.com/lethingochan27925/hivemind && cd hivemind
# prerequisites: AWS creds + Bedrock model access, a CockroachDB Cloud cluster, Terraform, Docker, Go 1.25, Node 22
./scripts/init.sh                      # stands up all infrastructure from zero
go test ./...                          # hermetic unit + logic tests
bash test/integration/api_smoke.sh "$(terraform -chdir=terraform output -raw dashboard_api_url)"
```

Full runbook: [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).

## Feedback on CockroachDB's AI tooling

- **Vector index + partial predicate is the standout.** Being able to declare `VECTOR INDEX … WHERE archived = false` means "forgetting" is a first-class query-planner concern, not app-side filtering — recall stays fast as the corpus grows. This mapped cleanly onto salience-driven memory management.
- **MCP Server as a read-only exploration surface** fit the agent-safety model better than expected: exposing only `SELECT` gave us protocol-level least-privilege for free, so the agent physically cannot mutate while investigating.
- **One system for three data shapes** (vector, transactional, append-only audit) removed an entire class of cross-store consistency bugs. The single friction point was vector-literal encoding from Go — solved with a small `EncodeVector` helper (unit-tested).

## Repository map

| Path | What |
|------|------|
| `cmd/` | The 7 Lambda entrypoints |
| `internal/agent` | Claim → recall → reason → verdict → audit; `balanceSignal` |
| `internal/memory` | Working / episodic / audit memory operations |
| `pkg/{cockroach,bedrock,mcp}` | CockroachDB, Bedrock, and MCP clients |
| `internal/dashboardapi` | Read endpoints + control plane |
| `dashboard/` | Next.js control-plane UI (10 pages) |
| `terraform/` | All infrastructure as code |
| `docs/` | Reference documentation (architecture, memory, data model, API, security, ADRs) |
| `test/` | Test plan, test cases, runnable tests |
