# HiveMind Test Suite

Written from a tester's seat: a documented **test plan**, structured **test cases** traced to the scoring criteria, and **runnable** tests that make every headline claim reproducible. Deterministic tests touch every layer — scoring, memory encoding, agent reasoning, the security guard, crash-resume — and run with no cloud and no database.

```
test/
├── README.md              ← you are here
├── TEST_PLAN.md           ← master QA plan: scope, strategy, entry/exit criteria
├── test-cases/            ← structured, human-readable test cases (the tester deliverable)
│   ├── TC-AGENT.md          agent verdict correctness + prompt-injection resistance
│   ├── TC-MEMORY.md         episodic recall, consolidation, salience decay
│   ├── TC-CONCURRENCY.md    SKIP LOCKED, heartbeat reaper, resume-after-crash
│   ├── TC-CONTROL-PLANE.md  control API + read-only DB console guard
│   ├── TC-DASHBOARD.md      the 8 dashboard pages
│   ├── TC-SCORING.md        risk routing thresholds + auto-decision
│   ├── TC-DATA-INTEGRITY.md schema constraints as a safety net
│   ├── TC-CICD.md           CI/CD pipeline & DevOps invariants
│   ├── TC-CONTROL-OPS.md    policy · memory ops · task control · bulk · budget · rollback · regions
│   └── TC-UX.md             search · i18n · theme · drag-resize · optimistic updates
└── integration/
    ├── reasoning_test.go    runnable Go — balance signal → verdict mapping (no cloud)
    ├── api_smoke.sh         runnable curl — smoke the deployed dashboard-api
    ├── pipeline_test.sh     runnable — assert the CI/CD pipeline invariants (no cloud)
    └── doc.go               makes the dir a normal buildable package
```

## Automated coverage matrix

Every row is a **hermetic** Go test — no network, no DB — so the whole matrix runs on every push in well under a second.

| Under test | Test file | Proves |
|------------|-----------|--------|
| `balanceSignal` | `internal/agent/reasoning_test.go` | DRAIN→fraud, FUNDS MOVED→legit, INCONCLUSIVE→escalate, incl. the "arrived full balance" edge — the 100%/100% discriminator |
| `BuildPrompt` | `test/integration/reasoning_test.go` | the balance finding reaches the model as primary evidence with the verdict contract |
| `SanitizeField` | `internal/agent/validation_test.go`, `test/integration/reasoning_test.go` | prompt-injection payloads are neutralised, length bounded |
| `classifyPattern` | `internal/agent/classify_test.go` | fraud-pattern taxonomy + precedence (balance_wipe > dest_no_update > high_amount > rapid_cashout) |
| `buildCaseSummary` | `internal/agent/classify_test.go` | episodic summary carries type/amount/error fields |
| `ParseScratchpad` / `Encode` | `internal/agent/resume_test.go` | crash-resume serialization round-trips; empty & malformed handled |
| `RiskTier` | `internal/scorer/router_test.go` | routing thresholds + exclusive boundaries (0.001 / 0.999 → medium) |
| `EncodeVector` | `pkg/cockroach/vector_test.go` | []float32 → CockroachDB VECTOR literal, incl. 1024-dim |
| `Transaction` / `flexFloat64` | `pkg/mcp/tools_test.go` | risk_score decodes from number, string, and scientific notation |
| `RunQuery` guard | `internal/dashboardapi/control_test.go` | read-only console rejects every mutating / multi-statement input **before** DB access; honours the control token; 405 on non-POST |
| `parseARN`, naming | `internal/dashboardapi/control_test.go` | inventory ARN parsing + project/env naming convention |
| `AmountRange`, `SignLabel` | `internal/memory/consolidation_test.go` | episodic fingerprint bucketing |
| Policy defaults | `internal/agent/policy_test.go` | missing/unreachable policy falls back to the calibrated thresholds, never to zeros |
| `TaskControl`, `BulkReview`, `MemoryAdmin`, `MemoryJob`, `RollbackService` | `internal/dashboardapi/ops_test.go` | every mutating endpoint rejects bad input **before** touching storage; control-token gate on all of them |
| `alterRegion`, `regionNameRe` | `internal/dashboardapi/regions_test.go` | SQL-injection region names rejected; unknown action refused before any DB round-trip |
| `invokable`, `ScheduleOne`, `InvokeService`, `Search` | `internal/dashboardapi/node_control_test.go` | control plane cannot invoke itself; useless search queries never reach the DB |
| `shortService`, `sortByCostDesc`, `estimateCost` | `internal/dashboardapi/cloudcost_test.go` | cloud-cost parsing/ordering and Haiku pricing maths |

DB-backed integration tests (`internal/memory/*_integration_test.go`) cover SKIP LOCKED claims and memory concurrency and need `DATABASE_URL`.

## Running

```bash
# 1. Everything hermetic — no AWS, no DB. This is the CI gate.
go test ./...
bash test/integration/pipeline_test.sh      # assert the CI/CD pipeline definitions

# 2. Just the correctness core (verdict discriminator + injection)
go test ./internal/agent/ ./test/integration/ ./internal/scorer/ -v

# 3. Integration against a live deployment
go test ./internal/memory/ -run Integration        # needs DATABASE_URL
bash test/integration/api_smoke.sh "$(terraform -chdir=terraform output -raw dashboard_api_url)"
```

## Test pyramid

```mermaid
flowchart TD
    U["Unit — deterministic, no I/O<br/>balanceSignal · RiskTier · EncodeVector · classifyPattern<br/>Scratchpad · RunQuery guard · flexFloat64 · SanitizeField"] --> I["Integration — needs DB / AWS<br/>SKIP LOCKED claims · memory concurrency · API smoke"]
    I --> M["Manual / demo — scripted scenarios<br/>crash-kill · region-kill · human review"]
```

Most assertions live at the bottom (fast, hermetic, run every push). The expensive resilience proofs (crash-kill, region-kill) are scripted manual scenarios in `TEST_PLAN.md` §7 and captured in the `evidence/` clips.
