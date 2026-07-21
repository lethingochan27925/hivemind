// consolidation.go: ghi case_memory moi, merge vao case tuong tu (>0.92) thay vi insert trung.
package memory

import (
	"context"
	"fmt"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

const consolidationThreshold = 0.92

type NewCase struct {
	Summary         string
	Verdict         string
	ConfidenceAvg   float64
	PatternType     string
	KeySignals      []string
	TransactionType string
	AmountRange     string
	ErrorOrigSign   string
	ErrorDestSign   string
	EmbeddingStr    string
	SourceTaskID    string
}

func WriteCaseMemory(ctx context.Context, db *cockroach.Client, c NewCase) error {
	var existingID string
	var similarity float64

	err := db.Pool.QueryRow(ctx, `
		SELECT id, 1 - (embedding <=> $1::vector) AS similarity
		FROM case_memory
		WHERE archived = false AND transaction_type = $2
		ORDER BY embedding <=> $1::vector
		LIMIT 1
	`, c.EmbeddingStr, c.TransactionType).Scan(&existingID, &similarity)

	foundExisting := err == nil

	if foundExisting && similarity > consolidationThreshold {
		_, err := db.Pool.Exec(ctx, `
			UPDATE case_memory
			SET summary = $1, merge_count = merge_count + 1, last_merged_at = now()
			WHERE id = $2
		`, c.Summary, existingID)
		if err != nil {
			return fmt.Errorf("merging case memory: %w", err)
		}
		return nil
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO case_memory (
			summary, verdict, confidence_avg, pattern_type, key_signals,
			transaction_type, amount_range, error_orig_sign, error_dest_sign,
			embedding, source_task_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, now())
	`, c.Summary, c.Verdict, c.ConfidenceAvg, c.PatternType, c.KeySignals,
		c.TransactionType, c.AmountRange, c.ErrorOrigSign, c.ErrorDestSign,
		c.EmbeddingStr, c.SourceTaskID)
	if err != nil {
		return fmt.Errorf("inserting case memory: %w", err)
	}
	return nil
}

func AmountRange(amount float64) string {
	if amount < 10_000 {
		return "low"
	} else if amount < 100_000 {
		return "mid"
	}
	return "high"
}

func SignLabel(val float64) string {
	if val < 1.0 && val > -1.0 {
		return "near_zero"
	}
	if val > 0 {
		return "positive"
	}
	return "negative"
}
