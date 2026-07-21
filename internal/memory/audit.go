// audit.go: ghi audit_log - moi hanh dong cua agent deu duoc ghi lai de truy vet.
package memory

import (
	"context"
	"fmt"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type AuditEntry struct {
	TaskID           string
	TransactionID    string
	AgentID          string
	Action           string
	Reasoning        *string
	MemoryHits       *int
	SimilarityScores []float64
	TokensIn         *int
	TokensOut        *int
	BedrockModel     *string
	LatencyMs        *int
	ReviewerID       *string
	ReviewNotes      *string
}

func WriteAuditLog(ctx context.Context, db *cockroach.Client, e AuditEntry) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO audit_log (
			task_id, transaction_id, agent_id, action, reasoning,
			memory_hits, similarity_scores, tokens_in, tokens_out,
			bedrock_model, latency_ms, reviewer_id, review_notes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now())
	`,
		e.TaskID, e.TransactionID, e.AgentID, e.Action, e.Reasoning,
		e.MemoryHits, e.SimilarityScores, e.TokensIn, e.TokensOut,
		e.BedrockModel, e.LatencyMs, e.ReviewerID, e.ReviewNotes,
	)
	if err != nil {
		return fmt.Errorf("writing audit log: %w", err)
	}
	return nil
}
