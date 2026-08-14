// learn.go: vong lap hoc tu con nguoi - manh ghep cuoi cua agentic memory.
//
// Truoc file nay, fleet chi hoc tu case NO TU QUYET: reviewer quyet mot case
// escalate xong, quyet dinh do cap nhat task roi... bien mat. Moi tri thuc tu
// chuyen gia con nguoi roi rung ngoai bo nho.
//
// Gio moi phan quyet cua con nguoi (Review Queue approve/reject, hoac override
// tren trang Transactions) duoc nhung vao case_memory qua DUNG duong ong
// consolidation cua agent - va duoc GHIM salience 2.0: decay khong bao gio
// duoc phep quen dieu mot con nguoi da day.
//
// Best-effort co chu dich: loi o bat ky buoc nao chi ghi log, khong bao gio
// lam hong ban than thao tac review.
package dashboardapi

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/lethingochan27925/hivemind/internal/agent"
	"github.com/lethingochan27925/hivemind/internal/memory"
	"github.com/lethingochan27925/hivemind/pkg/bedrock"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

// Bedrock client khoi tao luoi va dung lai giua cac invoke (Lambda container
// song qua nhieu request). Chi can EmbedText - Claude khong bao gio bi goi
// tu control plane.
var (
	embedOnce   sync.Once
	embedClient *bedrock.Client
	embedErr    error
)

func embedder(ctx context.Context) (*bedrock.Client, error) {
	embedOnce.Do(func() {
		cfg, err := loadCfgOnce()
		if err != nil {
			embedErr = fmt.Errorf("loading config: %w", err)
			return
		}
		embedClient, embedErr = bedrock.NewClient(ctx,
			cfg.AWSRegionBedrock, cfg.AWSRegionEmbed,
			cfg.ClaudeModelID, cfg.TitanModelID, cfg.EmbedDim)
	})
	return embedClient, embedErr
}

// HumanVerdict dich quyet dinh cua reviewer sang verdict:
// approve = cho giao dich qua (legit), reject = chan (fraud).
func HumanVerdict(decision string) string {
	if decision == "rejected" {
		return "fraud"
	}
	return "legit"
}

// LearnFromReview la hook cho duong Review Queue (approve/reject): tu tim
// transaction cua task roi nho bai hoc.
func (s *Server) LearnFromReview(ctx context.Context, taskID, decision, reviewerID, notes string) error {
	var txnID string
	if err := s.DB.Pool.QueryRow(ctx,
		`SELECT transaction_id FROM tasks WHERE id = $1`, taskID).Scan(&txnID); err != nil {
		return fmt.Errorf("looking up transaction for task %s: %w", taskID, err)
	}
	return s.learnHumanLesson(ctx, taskID, txnID, HumanVerdict(decision), reviewerID, notes)
}

// learnHumanLesson nhung mot phan quyet cua con nguoi vao episodic memory.
func (s *Server) learnHumanLesson(ctx context.Context, taskID, txnID, verdict, reviewerID, notes string) error {
	if verdict != "fraud" && verdict != "legit" {
		return nil // escalate->escalate khong day duoc gi
	}

	var txn mcp.Transaction
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT type, amount, old_balance_orig, new_balance_orig,
		       old_balance_dest, new_balance_dest,
		       COALESCE(error_balance_orig, 0), COALESCE(error_balance_dest, 0)
		FROM transactions WHERE id = $1
	`, txnID).Scan(&txn.Type, &txn.Amount, &txn.OldBalanceOrig, &txn.NewBalanceOrig,
		&txn.OldBalanceDest, &txn.NewBalanceDest,
		&txn.ErrorBalanceOrig, &txn.ErrorBalanceDest); err != nil {
		return fmt.Errorf("loading transaction %s: %w", txnID, err)
	}

	rationale := fmt.Sprintf("Human review by %s -> %s", reviewerID, verdict)
	if notes != "" {
		rationale += ": " + notes
	}
	// Cung dinh dang summary voi agent de embedding cua bai hoc con nguoi nam
	// chung khong gian ngu nghia voi ky uc tu hoc - recall moi tim thay chung.
	summary := fmt.Sprintf(
		"type=%s amount=%.2f error_orig=%.2f error_dest=%.2f. Verdict: %s. %s",
		txn.Type, txn.Amount, txn.ErrorBalanceOrig, txn.ErrorBalanceDest, verdict, rationale,
	)

	bc, err := embedder(ctx)
	if err != nil {
		return err
	}
	emb, err := bc.EmbedText(ctx, summary)
	if err != nil {
		return fmt.Errorf("embedding lesson: %w", err)
	}

	memID, err := memory.WriteCaseMemory(ctx, s.DB, memory.NewCase{
		Summary:         summary,
		Verdict:         verdict,
		ConfidenceAvg:   1.0, // nhan tu con nguoi la ground truth
		PatternType:     agent.ClassifyPattern(&txn),
		KeySignals:      []string{"human_reviewed"},
		TransactionType: txn.Type,
		AmountRange:     memory.AmountRange(txn.Amount),
		ErrorOrigSign:   memory.SignLabel(txn.ErrorBalanceOrig),
		ErrorDestSign:   memory.SignLabel(txn.ErrorBalanceDest),
		EmbeddingStr:    cockroach.EncodeVector(emb),
		SourceTaskID:    taskID,
	})
	if err != nil {
		return fmt.Errorf("writing human lesson: %w", err)
	}

	// Ghim: salience 2.0 la tran ma decay khong vuot qua duoc - fleet khong
	// bao gio quen dieu mot con nguoi da day no.
	if _, err := s.DB.Pool.Exec(ctx,
		`UPDATE case_memory SET salience = 2.0 WHERE id = $1`, memID); err != nil {
		log.Printf("pinning human lesson %s: %v", memID, err)
	}

	_, _ = s.DB.Pool.Exec(ctx, `
		INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, reviewer_id, created_at)
		VALUES ($1, $2, 'control-plane', 'human_lesson_stored', $3, $4, now())
	`, taskID, txnID, rationale, reviewerID)
	return nil
}
