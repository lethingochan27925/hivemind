// agent.go: vong lap chinh cua worker - claim, recall, reason, verdict, ghi memory.
// Ghi scratchpad sau moi buoc de resume dung cho neu bi crash giua chung.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/memory"
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
		w.processTask(ctx, task)
	}
}

// RunOnce claim va xu ly dung 1 task roi return. Dung cho Lambda, noi
// EventBridge tu goi lai lien tuc thay vi giu function song mai bang vong lap.
// Tra ve true neu co task da xu ly, false neu khong co task nao dang cho.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	task, err := memory.ClaimNextTask(ctx, w.db, w.ID)
	if err != nil {
		return false, fmt.Errorf("claiming task: %w", err)
	}
	if task == nil {
		fmt.Println("No pending tasks")
		return false, nil
	}

	fmt.Printf("Claimed task %s\n", task.ID)
	w.processTask(ctx, task)
	return true, nil
}

// writeAudit ghi audit va bao ro khi that bai. Truoc day gia tri tra ve cua
// WriteAuditLog bi bo qua o ca 5 cho goi, nen audit co the mat ban ghi ma
// khong ai biet - go vet khong bat duoc loai loi nay.
func (w *Worker) writeAudit(ctx context.Context, entry memory.AuditEntry) {
	if err := memory.WriteAuditLog(ctx, w.db, entry); err != nil {
		fmt.Printf("  [error] audit write failed (action=%s task=%s): %v\n",
			entry.Action, entry.TaskID, err)
	}
}

// failTask danh dau task hong kem ly do, de con dieu tra duoc vi sao.
func (w *Worker) failTask(ctx context.Context, task *memory.Task, reason string) {
	fmt.Printf("  [error] %s\n", reason)
	if err := memory.FailTask(ctx, w.db, task, w.ID, reason); err != nil {
		fmt.Printf("  [error] recording task failure: %v\n", err)
	}
}

func (w *Worker) processTask(ctx context.Context, task *memory.Task) {
	sp, err := ParseScratchpad(task.Scratchpad)
	if err != nil {
		fmt.Printf("  [warn] parsing scratchpad failed, starting fresh: %v\n", err)
		sp = &Scratchpad{}
	}
	resuming := task.Step != nil && *task.Step != ""
	if resuming {
		fmt.Printf("  Resuming from step=%s (retry_count=%d)\n", *task.Step, sp.RetryCount)
		sp.RetryCount++
		// Ghi audit task_resumed: enum co san nhung truoc day khong noi nao ghi,
		// khien cau chuyen crash -> requeue -> resume dut o buoc cuoi.
		resumeNote := fmt.Sprintf("resumed at step=%s from scratchpad (retry #%d)", *task.Step, sp.RetryCount)
		w.writeAudit(ctx, memory.AuditEntry{
			TaskID:        task.ID,
			TransactionID: task.TransactionID,
			AgentID:       w.ID,
			Action:        "task_resumed",
			Reasoning:     &resumeNote,
		})
	}

	txn, err := w.mcp.GetTransaction(task.TransactionID)
	if err != nil {
		w.failTask(ctx, task, fmt.Sprintf("MCP GetTransaction failed: %v", err))
		return
	}
	if txn == nil {
		w.failTask(ctx, task, fmt.Sprintf("MCP returned no transaction %s", task.TransactionID))
		return
	}

	// Nguong do control plane dat (system_policy), fallback ve hang so mac dinh.
	pol := LoadPolicy(ctx, w.db)

	if txn.RiskScore() < pol.RiskLow {
		w.autoDecide(ctx, task.ID, task.TransactionID, txn, "legit", "auto_approve")
		return
	}
	if txn.RiskScore() > pol.RiskHigh {
		w.autoDecide(ctx, task.ID, task.TransactionID, txn, "fraud", "auto_block")
		return
	}

	memoryHits := w.stepMemoryRecall(ctx, task.ID, txn, resuming, sp)
	customerHistory := w.stepCustomerContext(ctx, task.ID, task.TransactionID, txn, resuming, sp)

	memoryTexts := formatMemoryHits(memoryHits)
	customerTexts := formatCustomerHistory(customerHistory)
	result := CallClaude(ctx, w.bedrock, txn, memoryTexts, customerTexts)

	// Khi model khong tra loi duoc, verdict den tu ruleBasedFallback chu khong
	// phai suy luan that. Policy 'requeue' tra task ve hang doi de thu lai sau,
	// thay vi day mot phan doan yeu sang cho nguoi duyet.
	if result.Step == "fallback" && pol.FallbackAction == "requeue" {
		if _, err := w.db.Pool.Exec(ctx, `
			UPDATE tasks
			SET status = 'pending', claimed_by = NULL, claimed_at = NULL,
			    heartbeat_at = NULL, step = NULL
			WHERE id = $1
		`, task.ID); err != nil {
			fmt.Printf("  [error] requeue after fallback failed: %v\n", err)
		} else {
			reason := "model unavailable - requeued by policy instead of escalating"
			w.writeAudit(ctx, memory.AuditEntry{
				TaskID:        task.ID,
				TransactionID: task.TransactionID,
				AgentID:       w.ID,
				Action:        "task_requeued",
				Reasoning:     &reason,
			})
			fmt.Println("  Requeued by fallback policy (model unavailable)")
			return
		}
	}

	sp.PartialReasoning = result.Rationale
	w.saveCheckpoint(ctx, task.ID, "reasoned", sp)

	// Chi ghi bedrock_model khi Claude that su tra loi. Neu roi vao
	// ruleBasedFallback thi de NULL, de mot cau SQL phan biet duoc verdict do
	// LLM suy luan voi verdict do quy tac nguong risk_score sinh ra. Truoc day
	// ca hai deu duoc ghi la "bedrock_reasoning" kem ten model Claude, khien
	// audit trail khang dinh sai ve nguon goc quyet dinh.
	var bedrockModel *string
	if result.Step == "bedrock_reasoning" {
		bedrockModel = &w.bedrock.ClaudeModelID
	}

	w.writeAudit(ctx, memory.AuditEntry{
		TaskID:        task.ID,
		TransactionID: task.TransactionID,
		AgentID:       w.ID,
		Action:        "bedrock_reasoning",
		Reasoning:     &result.Rationale,
		TokensIn:      result.TokensIn,
		TokensOut:     result.TokensOut,
		BedrockModel:  bedrockModel,
		LatencyMs:     &result.LatencyMs,
	})

	newStatus := "done"
	if result.Verdict == "escalate" {
		newStatus = "pending_review"
	}

	if err := memory.CompleteTask(ctx, w.db, task.ID, newStatus, result.Verdict, "done", result.Confidence); err != nil {
		fmt.Printf("  [error] CompleteTask failed: %v\n", err)
		return
	}

	w.writeCaseMemoryAndAudit(ctx, txn, task.ID, task.TransactionID, result)

	verdictAction := fmt.Sprintf("verdict_%s", result.Verdict)
	w.writeAudit(ctx, memory.AuditEntry{
		TaskID:        task.ID,
		TransactionID: task.TransactionID,
		AgentID:       w.ID,
		Action:        verdictAction,
		Reasoning:     &result.Rationale,
	})

	fmt.Printf("  Done | risk=%.6f | verdict=%s | confidence=%.2f | status=%s\n",
		txn.RiskScore(), result.Verdict, result.Confidence, newStatus)
}

// stepMemoryRecall la checkpoint 1: vector recall tu case_memory.
// Neu dang resume va scratchpad da co ket qua buoc nay, dung lai, khong goi lai Bedrock/DB.
func (w *Worker) stepMemoryRecall(ctx context.Context, taskID string, txn *mcp.Transaction, resuming bool, sp *Scratchpad) []memory.CaseMemoryHit {
	var memoryHits []memory.CaseMemoryHit

	if resuming && sp.TopKCases != nil {
		json.Unmarshal(sp.TopKCases, &memoryHits)
		fmt.Printf("  Memory hits (resumed): %d\n", len(memoryHits))
		return memoryHits
	}

	embedding, err := w.bedrock.EmbedText(ctx, buildCaseSummary(txn))
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

	hitsJSON, _ := json.Marshal(memoryHits)
	sp.TopKCases = hitsJSON
	w.saveCheckpoint(ctx, taskID, "memory_recalled", sp)

	// So luong hit PHAI duoc ghi vao audit: day la du lieu goc de chung minh
	// memory co lam agent tot hon hay khong, va la nguon cua bieu do
	// "Memory recall hits" tren dashboard. Thieu no thi ca hai deu rong.
	hitCount := len(memoryHits)
	// Similarity tung hit (1 - distance) cung duoc ghi: day la du lieu goc cho
	// Decision Trace tren dashboard - verdict dua tren memory nao, gan den dau.
	similarities := make([]float64, 0, len(memoryHits))
	for _, h := range memoryHits {
		similarities = append(similarities, 1.0-h.Distance)
	}
	w.writeAudit(ctx, memory.AuditEntry{
		TaskID:           taskID,
		TransactionID:    txn.ID,
		AgentID:          w.ID,
		Action:           "memory_recall",
		MemoryHits:       &hitCount,
		SimilarityScores: similarities,
	})

	return memoryHits
}

// stepCustomerContext la checkpoint 2: query lich su khach hang qua MCP.
func (w *Worker) stepCustomerContext(ctx context.Context, taskID, transactionID string, txn *mcp.Transaction, resuming bool, sp *Scratchpad) []mcp.CustomerHistoryRow {
	var customerHistory []mcp.CustomerHistoryRow

	if resuming && sp.MCPResult != nil {
		json.Unmarshal(sp.MCPResult, &customerHistory)
		fmt.Printf("  Customer history (resumed): %d past transactions\n", len(customerHistory))
		return customerHistory
	}

	var err error
	customerHistory, err = w.mcp.GetCustomerContext(txn.NameOrig, 5)
	if err != nil {
		fmt.Printf("  [warn] GetCustomerContext failed: %v\n", err)
		customerHistory = nil
	}
	fmt.Printf("  Customer history (MCP): %d past transactions\n", len(customerHistory))

	mcpJSON, _ := json.Marshal(customerHistory)
	sp.MCPResult = mcpJSON
	w.saveCheckpoint(ctx, taskID, "mcp_queried", sp)

	w.writeAudit(ctx, memory.AuditEntry{
		TaskID:        taskID,
		TransactionID: transactionID,
		AgentID:       w.ID,
		Action:        "mcp_query",
	})

	return customerHistory
}

func (w *Worker) saveCheckpoint(ctx context.Context, taskID, step string, sp *Scratchpad) {
	encoded, err := sp.Encode()
	if err != nil {
		fmt.Printf("  [warn] encoding scratchpad failed: %v\n", err)
		return
	}
	if err := memory.SaveScratchpad(ctx, w.db, taskID, step, encoded); err != nil {
		fmt.Printf("  [warn] saving scratchpad failed: %v\n", err)
	}
}

func (w *Worker) writeCaseMemoryAndAudit(ctx context.Context, txn *mcp.Transaction, taskID, transactionID string, result ReasoningResult) {
	pattern := ClassifyPattern(txn)
	summary := fmt.Sprintf(
		"type=%s amount=%.2f error_orig=%.2f error_dest=%.2f. Verdict: %s. %s",
		txn.Type, txn.Amount, txn.ErrorBalanceOrig, txn.ErrorBalanceDest,
		result.Verdict, result.Rationale,
	)

	embedding, err := w.bedrock.EmbedText(ctx, summary)
	if err != nil {
		fmt.Printf("  [warn] EmbedText for case_memory failed: %v\n", err)
		return
	}
	embeddingStr := cockroach.EncodeVector(embedding)

	keySignals := []string{}
	if pattern != "unclassified" {
		keySignals = append(keySignals, pattern)
	}

	_, err = memory.WriteCaseMemory(ctx, w.db, memory.NewCase{
		Summary:         summary,
		Verdict:         result.Verdict,
		ConfidenceAvg:   result.Confidence,
		PatternType:     pattern,
		KeySignals:      keySignals,
		TransactionType: txn.Type,
		AmountRange:     memory.AmountRange(txn.Amount),
		ErrorOrigSign:   memory.SignLabel(txn.ErrorBalanceOrig),
		ErrorDestSign:   memory.SignLabel(txn.ErrorBalanceDest),
		EmbeddingStr:    embeddingStr,
		SourceTaskID:    taskID,
	})
	if err != nil {
		fmt.Printf("  [warn] WriteCaseMemory failed: %v\n", err)
	}
}

// ClassifyPattern gan nhan chu ky gian lan cho mot giao dich. Xuat ra ngoai
// vi vong lap hoc-tu-con-nguoi (dashboardapi/learn.go) phai gan nhan y het
// agent - hai nguon ky uc chung mot he phan loai.
func ClassifyPattern(txn *mcp.Transaction) string {
	if txn.OldBalanceOrig == txn.Amount && txn.NewBalanceOrig == 0 {
		return "balance_wipe"
	}
	if txn.ErrorBalanceDest > 1000 || txn.ErrorBalanceDest < -1000 {
		return "dest_no_update"
	}
	if txn.Amount > 200000 {
		return "high_amount_transfer"
	}
	if txn.Type == "CASH_OUT" {
		return "rapid_cashout"
	}
	return "unclassified"
}

func (w *Worker) autoDecide(ctx context.Context, taskID, transactionID string, txn *mcp.Transaction, verdict, action string) {
	if err := memory.CompleteTask(ctx, w.db, taskID, "done", verdict, action, 1.0); err != nil {
		fmt.Printf("  [error] CompleteTask (auto) failed: %v\n", err)
		return
	}
	w.writeAudit(ctx, memory.AuditEntry{
		TaskID:        taskID,
		TransactionID: transactionID,
		AgentID:       w.ID,
		Action:        action,
	})
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
