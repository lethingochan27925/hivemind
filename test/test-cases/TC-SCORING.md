# TC-SCORING — Risk routing & auto-decision

Traces to: **Technical Implementation, Real-World Impact**. The routing thresholds decide which transactions a (costly) agent ever sees, so their boundaries are load-bearing. Automated by `internal/scorer/router_test.go`; the auto-decide behaviour is in `internal/agent/agent.go`.

| ID | Priority | Precondition | Steps | Expected |
|----|----------|--------------|-------|----------|
| **TC-SCORE-01** | P0 | — | `RiskTier(0.0)` | `low` — auto-approve territory. |
| **TC-SCORE-02** | P0 | — | `RiskTier(0.001)` (exactly `LowThreshold`) | `medium` — the threshold is **exclusive**; a score on the line is investigated, not silently approved. |
| **TC-SCORE-03** | P0 | — | `RiskTier(0.999)` (exactly `HighThreshold`) | `medium` — exclusive on the high side too; a borderline score is investigated, not auto-blocked. |
| **TC-SCORE-04** | P0 | — | `RiskTier(0.9991)` and `RiskTier(1.0)` | `high` — auto-block territory. |
| **TC-SCORE-05** | P1 | — | `RiskTier(0.5)`, `RiskTier(0.98)` | `medium` — the whole `(0.001, 0.999)` band routes to the agent (~1.9% of PaySim by design). |
| **TC-SCORE-06** | P1 | — | Confirm `LowThreshold == 0.001` and `HighThreshold == 0.999` | Calibrated for PaySim's bimodal score distribution; a standard 0.30/0.70 split would starve the agent of cases. |
| **TC-SCORE-07** | P1 | Agent processing a task | Feed a transaction with `risk_score < 0.001` | Agent takes the `auto_approve` path → verdict `legit`, `status=done`, audit action `auto_approve`, **no Bedrock call**. |
| **TC-SCORE-08** | P1 | Agent processing a task | Feed a transaction with `risk_score > 0.999` | Agent takes the `auto_block` path → verdict `fraud`, `status=done`, audit action `auto_block`, **no Bedrock call**. |
| **TC-SCORE-09** | P2 | Scoring service reachable | Score a transaction end-to-end via the SigV4-signed client | A signed request returns a `risk_score`; an unsigned request to the IAM-auth Function URL returns `403` (proves auth is enforced). |

## Why the exclusive boundary matters

A score sitting exactly on a threshold is the ambiguous case. Routing it to `medium` (investigate) rather than auto-deciding is the conservative choice — the system never auto-approves or auto-blocks a borderline transaction. TC-SCORE-02 and TC-SCORE-03 pin this so a future refactor can't quietly flip it to `<=` / `>=`.
