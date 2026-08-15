# ADR-0002 — A deterministic categorical tool instead of asking the LLM to do arithmetic

**Status:** Accepted · **Context:** Claude Haiku (small, cheap) must reach correct fraud verdicts.

## Decision

Compute the balance reconciliation in Go (`balanceSignal`) and hand the model a **categorical** finding (`DRAIN` / `FUNDS MOVED` / `INCONCLUSIVE`) as primary evidence, rather than handing it raw balances and asking it to calculate.

## Why

Four prompt-only iterations plateaued: small models reason poorly over multi-step arithmetic (does `old_balance_orig − amount ≈ 0`? was the destination credited?). But they reason *well* over labelled categories. So we moved the calculation — which is trivial and deterministic — into code, and left the model the judgement it's actually good at.

On the labelled eval the `DRAIN` signature separates 34/34 fraud from 0/340 legit → 100% recall and precision. The model still weighs memory hits and customer history and may override on strong contrary evidence; it is simply no longer the calculator.

## Consequences

- **Correctness comes from design, not model size** — so the cheapest Bedrock model suffices ($0.00023/investigation), which is itself a production-readiness win.
- The arithmetic is unit-testable in isolation (`test/integration/reasoning_test.go`, `internal/agent/reasoning_test.go`) — deterministic Go, no model call, no flakiness.
- Generalisation risk: the signature is tuned to PaySim's fraud fingerprint. New fraud shapes need new signals — mitigated by the `escalate` path (uncertain cases go to humans, never silently mislabelled) and by the memory layer accumulating new patterns.
- The `< 1.0` tolerances are a tuning knob; documented as such so a dataset change triggers a re-check.
