# Episodic memory — controlled A/B

Run at 20260813_220356 against `https://yhwxmkywrja43z6km6tj2gpb4y0goosl.lambda-url.ap-southeast-1.on.aws`, 100 cases per phase, 6 minutes of processing each.

## Method

Comparing "cases with a memory hit" against "cases without" on historical data is
misleading once a system has been running: every task eventually has a hit. So the
starting condition is controlled instead.

| Phase | Starting condition |
|-------|--------------------|
| **A — cold** | Every episodic memory archived first, so the fleet investigates with an empty memory |
| **B — warm** | The same volume of cases again, now with the memory that phase A itself built |

Both phases are measured the same way: of the cases the fleet closed in that window,
how many did it resolve on its own instead of handing to a human.

## Result

| Phase | investigated | auto-resolved | escalated | **auto-resolve rate** | avg memories recalled |
|-------|--------------|---------------|-----------|----------------------|----------------------|
| A — cold | 61 | 0 | 61 | **0.0%** | 0 |
| B — warm | 57 | 0 | 57 | **0.0%** | 0 |

**Difference: 0.0 percentage points.**

Episodic memories created during phase A: 25 (from an archived baseline of 23).

## How to read this

A positive difference means recalled experience let the fleet close more cases without
a human — the practical payoff of the memory layer. A difference near zero means the
balance-reconciliation tool already decides most cases on its own, and memory is
earning its keep through consolidation and auditability rather than through verdicts.
Either way the number here is measured, not asserted.

Reproduce:

```bash
./scripts/memory-experiment.sh 100 6
```
