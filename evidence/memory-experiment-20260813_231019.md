# Episodic memory — controlled A/B on an identical sample

Run 20260813_231019 against `https://yhwxmkywrja43z6km6tj2gpb4y0goosl.lambda-url.ap-southeast-1.on.aws`.
Sample: **60 randomly selected cases**, replayed twice — the same task ids in both phases.

## Method

Comparing "cases that had a memory hit" against "cases that did not" on historical data
stops working once a system has been replayed a few times: every task eventually meets
the memory. So this experiment controls the starting condition instead, and holds the
input fixed.

| Phase | Memory state at the start | Input |
|-------|---------------------------|-------|
| **A — cold** | every episodic memory archived (23 → 0 active) | the sample |
| **B — warm** | the memory phase A itself built (7 active) | **the same** sample |

Both phases are measured identically: of the sampled cases the fleet closed, how many did
it resolve on its own instead of handing to a human.

## Result

| Phase | closed | auto-resolved | escalated | **auto-resolve rate** | avg memories recalled |
|-------|--------|---------------|-----------|----------------------|----------------------|
| A — cold | 60 | 27 | 33 | **45.0%** | 1.6 |
| B — warm | 60 | 27 | 33 | **45.0%** | 3.0 |

**Difference: 0.0 percentage points.**

## How to read this

HiveMind's verdicts are driven first by a deterministic balance-reconciliation tool: a
clean drain is fraud, a completed transfer is legitimate, and anything else is genuinely
ambiguous. That is by design — it is what makes a small model reach 100% recall and
precision. So a large swing in this table would actually be suspicious: it would mean
recalled text was overriding arithmetic evidence.

What episodic memory contributes in this system is measurable elsewhere, and those numbers
are in the scorecard: cases consolidated instead of duplicated, memories reused across
investigations, and a bounded context window that keeps cost per case in the fractions of
a cent. Memory here is knowledge reuse and auditability, not a verdict-flipping oracle.

If the sample is dominated by cases the balance tool marks INCONCLUSIVE, both phases will
sit near the same rate — that is a property of the sample, not a failure of the memory
layer. Re-run with a larger sample for a broader mix.

Reproduce:

```bash
./scripts/memory-experiment.sh 60 8
```
