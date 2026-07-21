// episodic.go: doc case_memory bang vector search - recall case tuong tu.
package memory

import (
	"context"
	"fmt"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type CaseMemoryHit struct {
	ID            string
	Summary       string
	Verdict       string
	ConfidenceAvg *float64
	PatternType   *string
	KeySignals    []string
	Salience      float64
	RecallCount   int
	Distance      float64
}

// RetrieveCaseMemory tim top-k case gan nhat theo vector distance, chi trong
// cung transaction_type. Sau khi tim thay, tang salience va recall_count
// cua cac case do (retrieval-induced adaptation, theo GEM framework).
func RetrieveCaseMemory(ctx context.Context, db *cockroach.Client, embeddingStr, txnType string, topK int) ([]CaseMemoryHit, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, summary, verdict, confidence_avg, pattern_type,
			   key_signals, salience, recall_count,
			   embedding <=> $1::vector AS distance
		FROM case_memory
		WHERE archived = false AND transaction_type = $2
		ORDER BY embedding <=> $1::vector
		LIMIT $3
	`, embeddingStr, txnType, topK)
	if err != nil {
		return nil, fmt.Errorf("querying case memory: %w", err)
	}
	defer rows.Close()

	var hits []CaseMemoryHit
	var ids []string
	for rows.Next() {
		var h CaseMemoryHit
		if err := rows.Scan(&h.ID, &h.Summary, &h.Verdict, &h.ConfidenceAvg,
			&h.PatternType, &h.KeySignals, &h.Salience, &h.RecallCount, &h.Distance); err != nil {
			return nil, fmt.Errorf("scanning case memory row: %w", err)
		}
		hits = append(hits, h)
		ids = append(ids, h.ID)
	}

	if len(ids) > 0 {
		_, err := db.Pool.Exec(ctx, `
			UPDATE case_memory
			SET salience = LEAST(salience + 0.1, 2.0),
				recall_count = recall_count + 1,
				last_recalled_at = now()
			WHERE id = ANY($1::UUID[])
		`, ids)
		if err != nil {
			return nil, fmt.Errorf("updating salience after recall: %w", err)
		}
	}

	return hits, nil
}
