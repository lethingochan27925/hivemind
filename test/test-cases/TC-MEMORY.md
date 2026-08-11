# TC-MEMORY — Episodic memory: recall, consolidation, forgetting

Traces to: **Agentic Memory Design** (primary scoring criterion). Automated by `internal/memory/consolidation_test.go`; DB-backed cases need `DATABASE_URL`.

| ID | Priority | Precondition | Steps | Expected |
|----|----------|--------------|-------|----------|
| **TC-MEMORY-01** | P0 | Empty `case_memory` | Close a fraud case → construction runs (summary → Titan embed → insert). | One row inserted; `embedding` is 1024-dim non-null; `salience=1.0`, `merge_count=1`, `archived=false`. |
| **TC-MEMORY-02** | P0 | TC-MEMORY-01 done | Close a **near-identical** fraud case (similarity > 0.92 to the existing one). | **No new row.** The existing case is merged: `merge_count` increments, `last_merged_at` updates, summary refreshed. Table does not bloat with duplicates. |
| **TC-MEMORY-03** | P0 | TC-MEMORY-01 done | Close a **distinct** case (similarity ≤ 0.92). | A new row is inserted (not merged). Consolidation threshold respected. |
| **TC-MEMORY-04** | P0 | ≥ 3 relevant memories exist | Investigate a new alert; observe the recall step. | SQL pre-filter (`transaction_type`, `amount_range`, `archived=false`) narrows candidates; vector search returns top-k; top-3 summaries appear in the prompt; `recall_count` increments and `last_recalled_at` updates on the hits. |
| **TC-MEMORY-05** | P1 | A memory with `last_recalled_at` > 7 days ago | Run `salience-decay`. | `salience` multiplied by 0.95; a memory whose salience drops below 0.10 flips `archived=true`. |
| **TC-MEMORY-06** | P0 | An archived memory exists | Run a vector recall. | The archived row is **absent** from results — it is excluded by the partial vector index (`WHERE archived=false`), not just filtered in app code. |
| **TC-MEMORY-07** | P1 | — | Attempt to write `salience = 2.5`. | Rejected by CHECK `salience BETWEEN 0.0 AND 2.0`. Reinforcement cannot run away. |
| **TC-MEMORY-08** | P2 | Recall path exercised | Confirm the context window size. | Prompt carries system prompt + **top-3** summaries + current alert only — never a full-history dump. |

## The "wow" moment to capture on video

TC-MEMORY-04 is the demo's centrepiece: **the same fraud pattern appears twice.** First occurrence, the agent works it cold. Second occurrence, the agent recalls the consolidated memory (show the SQL: `summary`, `similarity_score`, `recall_count++`) and resolves faster and more confidently. That single before/after is the clearest proof that memory turns the fleet into an organisation that learns.
