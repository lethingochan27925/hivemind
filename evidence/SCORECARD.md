# HiveMind fleet scorecard

Generated 2026-08-16T17:16:25Z from the live system at `https://7ibnstkn5rtda4uw3hjebvtmh40rczqa.lambda-url.ap-southeast-1.on.aws`, using only the read-only SQL endpoint.
Reproduce with: `go run ./cmd/eval --api https://7ibnstkn5rtda4uw3hjebvtmh40rczqa.lambda-url.ap-southeast-1.on.aws`

## 1. Does it judge correctly?

| | labelled fraud | labelled legit |
|---|---|---|
| **verdict fraud** | 34 (caught) | 0 (false alarm) |
| **verdict legit** | 0 (missed) | 138 (correctly cleared) |

- **Recall 100.0%** — of every fraudulent transaction it decided on, this share was caught.
- **Precision 100.0%** — of everything it called fraud, this share really was.
- **F1 100.0%** · **Accuracy 100.0%** over 172 auto-decided cases.

## 2. How much work does it actually take off a human?

- **46.0% auto-resolved** without any human involvement.
- **202 cases escalated** — the agent flagged its own uncertainty instead of guessing.

## 3. Are those verdicts real reasoning?

- **4304 model decisions** vs **194 rule-based fallbacks** (4.3% fallback).
- Fallbacks are recorded with a NULL `bedrock_model`, so the audit trail never claims a rule was a model decision.

## 4. Does the shared memory earn its keep?

**Reuse.** 118 consolidated memories absorbed **4321 raw cases** — similar cases are merged into one
stronger memory instead of piling up duplicates, and each investigation recalls **2.91** of them on average.

**Decision quality** — the metric that matters, measured per investigated case:

| | investigations | auto-resolved | escalated | auto-resolve rate |
|---|---|---|---|---|
| **with recalled memory** | 42 | 19 | 23 | **45.2%** |
| **cold (no memory hit)** | 332 | 153 | 179 | 46.1% |

Cold cases resolved more often here (0.8 points), which usually means the recalled cases are the genuinely
harder ones. Read it with the reuse figures above, and confirm with `./scripts/memory-experiment.sh`.

**Cost of recall.** Average reasoning latency 4563 ms.
Recall adds three short summaries to the prompt, so parity here is the expected result - the model call dominates.
Memory is not sold as a speed-up; it is knowledge reuse that keeps the context window small and bounded.

## 5. Does the fleet survive itself?

- **531 distinct agents** wrote to the audit trail - real concurrent workers, not one process in a loop.
- **0 double claims** — must be zero (`SKIP LOCKED` + `UNIQUE(transaction_id)`).
- **8856 tasks re-queued** after a stale lease, **4988 resumed** from their checkpoint.

## 6. What did it cost?

- **2,781,570 tokens in / 283,294 tokens out** → **$1.0495** cumulative on Claude Haiku pricing.
- **$0.00023 per investigation** across 4,498 decisions → about **$0.12 per 500-case run**.

---

**Gate: PASS** — this command exits non-zero if recall drops below 95% or any transaction is claimed twice.
