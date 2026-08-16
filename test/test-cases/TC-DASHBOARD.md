# TC-DASHBOARD — Mission Control UI (10 pages)

Traces to: **Real-World Impact, Production Readiness** (observability). Manual/exploratory unless a component test exists. Precondition for all: dashboard deployed, `dashboard-api` reachable, PaySim seeded.

| ID | Page | Steps | Expected |
|----|------|-------|----------|
| **TC-DASH-01** | Mission Control (`/`) | Load the page. | KPI row (verdicts today, pending reviews, accuracy), verdict donut, memory-hit trend, live task list, and the **Fleet Control** bar all render with real numbers; live clock ticks; polling pauses when the tab is hidden. |
| **TC-DASH-02** | Fleet Control bar | Click Pause, then Start; click Run dispatch; enter N and Feed. | Actions hit `/control/*`; task counts update on the next poll; buttons disable while in flight. |
| **TC-DASH-03** | Review Queue (`/reviews`) | With an escalated case present, approve one and reject another. | Case leaves the queue; `POST /reviews/decide` succeeds; the decision shows in that transaction's audit trail. |
| **TC-DASH-04** | Fleet & Memory (`/memory`) | Load with agents active. | Active-agent list renders (no crash on `TIMESTAMPTZ`); memory stats (active/archived, avg salience, top patterns) populate. |
| **TC-DASH-05** | Transactions (`/transactions`) | Filter by `high`; open a row's audit. | Filtered feed loads; the audit endpoint returns the decision trail; repeated filter clicks don't error. |
| **TC-DASH-06** | Database (`/database`) | Load; click each preset; type a `SELECT`; type a `DELETE`. | Table stat cards show live counts; presets return tables; `DELETE` surfaces the server's `400` inline, no client crash. |
| **TC-DASH-07** | Architecture (`/architecture`) | Load. | Tiered map self-assembles from `/control/lambdas` + `/control/db` + `/control/resources`; Lambda nodes coloured by live state with a pulse on Active; CockroachDB node shows real row counts; platform-service counts match the inventory below. |
| **TC-DASH-08** | Cost (`/cost`) | Load. | Token totals and estimated USD render, today and per agent. |
| **TC-DASH-09** | Infrastructure (`/infrastructure`) | Load. | Per-service alarm health + the Nodes (Lambda) table render. |
| **TC-DASH-10** | Routing/reload | Deep-link to `/architecture` and hard-reload. | Page loads (CloudFront rewrites extensionless URI → `.html`); no 403. |
| **TC-DASH-11** | Resilience of UI | Point the app at an unreachable API. | Pages show empty/disconnected states, not blank crashes (null-guards hold). |
| **TC-DASH-12** | Theme / responsiveness | Resize to mobile; toggle theme. | Layout reflows; no horizontal body scroll; ops-console dark palette holds. |
| **TC-DASH-13** | Training Lab (`/training`) | Ingest a batch, run a training run of 2–3 batches, then approve/reject a case from the Review Queue while it's live. | Each batch reports memory formation, decision mix and cost; the run is saved to CockroachDB and appears under Saved runs; the human review is embedded into `case_memory` (see TC-MEMOPS) — the page never fine-tunes model weights, and says so. |
| **TC-DASH-14** | Pipeline (`/pipeline`) | Load the page twice within 30s from two tabs. | Both loads show the same GitHub Actions run state; the second load does not trigger a second upstream GitHub call (server-side 30s shared cache) — confirm via response timing or server logs, not just the UI. |

## Design-intent checks (from the brief)

- No AI-generated look: **no** purple gradients, glassmorphism, or emoji. Ops-console density (Grafana/Datadog/Linear reference).
- Bold titles for focus/separation; base font legible (15px); numbers tabular.
- The dashboard is a **control plane**, not just a viewer — every page that shows state also lets you act on it where it makes sense.
