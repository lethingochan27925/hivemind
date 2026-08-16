# Evidence

Reproducible proof for every claim HiveMind makes. Each subdirectory is one capture taken **from the running system** — control-plane JSON plus SQL over the append-only audit trail. No number in here is typed by hand.

```bash
./scripts/capture-evidence.sh                    # full snapshot
./scripts/capture-evidence.sh --label crash-test # snapshot tied to a scenario
```

Each capture lands in `evidence/<timestamp>[-label]/` with its own `INDEX.md` explaining every file, and is mirrored to the versioned S3 evidence bucket.

**Reading two captures side by side:** the fleet keeps running between captures, so counters like `raw_cases_absorbed`, `recall_count`, and `merge_count` are *cumulative since the table was last emptied* — not per-run. Two captures taken hours apart with the fleet active in between will show different totals for the same claim; that is the counters doing their job, not a discrepancy. Compare captures taken back-to-back (e.g. `baseline` → `after-run` in the sequence below) when the point is to isolate the effect of one specific run.

## Claim → file map

| Claim in README / SUBMISSION | Evidence file | How to read it |
|------------------------------|---------------|----------------|
| **100% fraud recall, 100% precision** | `verdict-vs-groundtruth.json` | Cross-tab of the agent's verdict against PaySim's `is_fraud_label`. Fraud rows must all carry verdict `fraud`; no `legit` row may carry label `true`. |
| **~46% auto-resolved without a human** | `overview.json` | `verdicts_today`: `fraud + legit` over the total handled. |
| **$0.00023 per investigation** | `cost.json`, `cloud-cost.json` | Token spend per agent from `audit_log`, plus real AWS spend by service from Cost Explorer. |
| **Memory is consolidated, not dumped** | `memory-consolidation.json` | `raw_cases_absorbed` ≫ `consolidated_cases`: similar cases merged above the 0.92 threshold instead of inserting duplicates. |
| **Memory is actually reused** | `memory-top-recalled.json`, `memory-hit-rate.json` | Non-zero `recall_count` per case, and the average hits per investigation. |
| **Exactly-once investigation under 20 workers** | `no-double-claims.json` | Must be `0`. `SKIP LOCKED` + `UNIQUE(transaction_id)`. |
| **Real fleet concurrency** | `fleet-distinct-agents.json` | Distinct `agent_id` values that claimed work. |
| **Crashes are a non-event** | `crash-recovery-events.json` | `task_requeued` followed by `task_resumed` for the same task — the reaper and the checkpoint resume, timestamped. |
| **Audit honesty** | `model-usage-and-fallbacks.json` | Rows with a NULL `bedrock_model` are rule-based fallbacks; the trail never claims a fallback was a model decision. |
| **Human-in-the-loop is real** | `human-in-the-loop.json` | Named reviewers and their decision counts from `human_reviewed` rows. |
| **Multi-region posture** | `regions.json` | Database regions and the survival goal, read live. |
| **Live-tunable policy** | `policy.json`, `budget.json` | The thresholds the fleet is running under right now, and the spend guardrail. |
| **Infrastructure is what the docs say** | `aws-inventory.json`, `lambdas.json`, `infrastructure.json` | Every resource tagged `Project=hivemind`, live function config, CloudWatch alarm states. |

## Suggested capture sequence for a demo or submission

1. `./scripts/capture-evidence.sh --label baseline` — before feeding anything.
2. Feed cases, let the fleet drain, then `--label after-run` — accuracy and cost become meaningful.
3. Feed the **same** pattern a second time, then `--label memory-recall` — recall counts and the learning curve show the payoff of episodic memory.
4. Press **Simulate agent crash** on the Infrastructure page, wait for the recovery tracker, then `--label crash-test` — captures the `task_requeued` → `task_resumed` pair.
5. (Optional) Enable a second database region, then `--label multi-region`.

## Screenshots and clips

Put manual artefacts next to the JSON in the same directory so they stay tied to the numbers they illustrate:

| File | Should show |
|------|-------------|
| `mission-control.png` | KPI row, verdict split, fleet learning curve |
| `decision-trace.png` | One case with recalled memories and their real vector similarity |
| `chaos-recovery.png` | The three-stage tracker: killed → re-queued (+Xs) → resumed (+Ys) |
| `architecture.png` | The topology generated from the live AWS inventory |
| `pipeline.png` | The release chain green end to end |
| `region-kill.mp4` | Primary region taken offline while the fleet keeps writing |
