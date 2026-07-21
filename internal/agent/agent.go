// agent.go: vong lap chinh cua worker - claim, recall, reason, verdict, ghi memory.
package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/memory"
	"github.com/lethingochan27925/hivemind/internal/scorer"
	"github.com/lethingochan27925/hivemind/pkg/bedrock"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

type Worker struct {
	ID      string
	db      *cockroach.Client
	bedrock *bedrock.Client
	mcp     *mcp.Tools
	cfg     *config.Config
}

func NewWorker(cfg *config.Config, db *cockroach.Client, bedrockClient *bedrock.Client, mcpClient *mcp.Client) *Worker {
	hostname, _ := os.Hostname()
	return &Worker{
		ID:      hostname,
		db:      db,
		bedrock: bedrockClient,
		mcp:     mcp.NewTools(mcpClient),
		cfg:     cfg,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	fmt.Printf("[%s] Worker started (Bedrock Claude Haiku + Titan Embeddings v2 + MCP)\n", w.ID)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		task, err := memory.ClaimNextTask(ctx, w.db, w.ID)
		if err != nil {
			fmt.Printf("Error claiming task: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if task == nil {
			fmt.Println("No pending tasks, sleeping...")
			time.Sleep(w.cfg.WorkerPollInterval)
			continue
		}

		fmt.Printf("Claimed task %s\n", task.ID)

		txn, err := w.mcp.GetTransaction(task.TransactionID)
		if err != nil {
			fmt.Printf("  [error] GetTransaction failed: %v\n", err)
			memory.FailTask(ctx, w.db, task.ID)
			continue
		}
		if txn == nil {
			fmt.Printf("  [error] Transaction %s not found\n", task.TransactionID)
			memory.FailTask(ctx, w.db, task.ID)
			continue
		}

		if txn.RiskScore() < scorer.LowThreshold {
			w.autoDecide(ctx, task.ID, txn, "legit", "auto_approve")
			continue
		}
		if txn.RiskScore() > scorer.HighThreshold {
			w.autoDecide(ctx, task.ID, txn, "fraud", "auto_block")
			continue
		}

		embedding, err := w.bedrock.EmbedText(ctx, buildCaseSummary(txn))
		var memoryHits []memory.CaseMemoryHit
		if err != nil {
			fmt.Printf("  [warn] EmbedText failed: %v\n", err)
		} else {
			embeddingStr := cockroach.EncodeVector(embedding)
			memoryHits, err = memory.RetrieveCaseMemory(ctx, w.db, embeddingStr, txn.Type, 3)
			if err != nil {
				fmt.Printf("  [warn] RetrieveCaseMemory failed: %v\n", err)
			}
		}
		fmt.Printf("  Memory hits: %d\n", len(memoryHits))

		customerHistory, err := w.mcp.GetCustomerContext(txn.NameOrig, 5)
		if err != nil {
			fmt.Printf("  [warn] GetCustomerContext failed: %v\n", err)
			customerHistory = nil
		}
		fmt.Printf("  Customer history (MCP): %d past transactions\n", len(customerHistory))

		memoryTexts := formatMemoryHits(memoryHits)
		customerTexts := formatCustomerHistory(customerHistory)

		result := CallClaude(ctx, w.bedrock, txn, memoryTexts, customerTexts)

		newStatus := "done"
		if result.Verdict == "escalate" {
			newStatus = "pending_review"
		}

		if err := memory.CompleteTask(ctx, w.db, task.ID, newStatus, result.Verdict, result.Step, result.Confidence); err != nil {
			fmt.Printf("  [error] CompleteTask failed: %v\n", err)
			continue
		}

		fmt.Printf("  Done | risk=%.6f | verdict=%s | confidence=%.2f | status=%s\n",
			txn.RiskScore(), result.Verdict, result.Confidence, newStatus)
	}
}

func (w *Worker) autoDecide(ctx context.Context, taskID string, txn *mcp.Transaction, verdict, action string) {
	if err := memory.CompleteTask(ctx, w.db, taskID, "done", verdict, action, 1.0); err != nil {
		fmt.Printf("  [error] CompleteTask (auto) failed: %v\n", err)
		return
	}
	fmt.Printf("  Auto-decided | risk=%.6f | verdict=%s | action=%s | status=done\n",
		txn.RiskScore(), verdict, action)
}

func buildCaseSummary(txn *mcp.Transaction) string {
	return fmt.Sprintf("type=%s amount=%.2f risk=%.6f error_orig=%.2f error_dest=%.2f",
		txn.Type, txn.Amount, txn.RiskScore(), txn.ErrorBalanceOrig, txn.ErrorBalanceDest)
}

func formatMemoryHits(hits []memory.CaseMemoryHit) []string {
	texts := make([]string, len(hits))
	for i, h := range hits {
		pattern := ""
		if h.PatternType != nil {
			pattern = *h.PatternType
		}
		texts[i] = fmt.Sprintf("  - verdict=%s pattern=%s: %s", h.Verdict, pattern, h.Summary)
	}
	return texts
}

func formatCustomerHistory(rows []mcp.CustomerHistoryRow) []string {
	texts := make([]string, len(rows))
	for i, r := range rows {
		verdict := "pending"
		if r.Verdict != nil {
			verdict = *r.Verdict
		}
		texts[i] = fmt.Sprintf("  - %s amount=%.2f verdict=%s risk=%.3f", r.Type, r.Amount, verdict, r.RiskScore)
	}
	return texts
}
