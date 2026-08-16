// consolidation.go: ghi case_memory moi, merge vao case tuong tu (>0.92) thay vi insert trung.
package memory

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/jackc/pgx/v5"
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

// WriteCaseMemory reads the nearest existing memory and either merges into it
// or inserts a new one. Read-then-write like this is a classic check-then-act
// race: two agent-worker Lambdas consolidating similar cases at the same
// instant can both miss each other's not-yet-committed row and each insert a
// near-duplicate instead of merging into one. CockroachDB's transactions are
// always SERIALIZABLE (unlike Postgres' default READ COMMITTED), so wrapping
// the read and the write in one transaction is enough for the database to
// detect that race itself; it aborts the loser with a retryable ("40001")
// error instead of allowing the anomaly. crdbpgx.ExecuteTx is CockroachDB's
// own documented client helper for that retry loop (see "Build a Go App with
// CockroachDB" / crdbpgx in the CockroachDB docs) — it re-runs the closure
// until it commits, so a caller never has to handle 40001 by hand.
func WriteCaseMemory(ctx context.Context, db *cockroach.Client, c NewCase) (string, error) {
	var resultID string
	err := crdbpgx.ExecuteTx(ctx, db.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var existingID string
		var similarity float64

		// FOR UPDATE: a second transaction that would merge into this same
		// row serializes behind this one instead of racing it.
		err := tx.QueryRow(ctx, `
			SELECT id, 1 - (embedding <=> $1::vector) AS similarity
			FROM case_memory
			WHERE archived = false AND transaction_type = $2
			ORDER BY embedding <=> $1::vector
			LIMIT 1
			FOR UPDATE
		`, c.EmbeddingStr, c.TransactionType).Scan(&existingID, &similarity)

		// pgx.ErrNoRows is the only expected "failure" here (no candidate row
		// exists yet - fall through to INSERT below). Any other error - a
		// dropped connection, or exactly the retryable 40001 serialization
		// conflict this whole rewrite exists to let CockroachDB signal - used
		// to be treated identically to "not found", so the closure proceeded
		// to INSERT inside what could already be a poisoned transaction. That
		// INSERT would then fail with a second, unrelated-looking error
		// ("current transaction is aborted"), masking the real cause and
		// defeating crdbpgx.ExecuteTx's retry: it can only retry an error this
		// closure actually returns to it.
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("finding similar case memory: %w", err)
		}
		foundExisting := err == nil

		if foundExisting && similarity > consolidationThreshold {
			if _, err := tx.Exec(ctx, `
				UPDATE case_memory
				SET summary = $1, merge_count = merge_count + 1, last_merged_at = now()
				WHERE id = $2
			`, c.Summary, existingID); err != nil {
				return fmt.Errorf("merging case memory: %w", err)
			}
			resultID = existingID
			return nil
		}

		var newID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO case_memory (
				summary, verdict, confidence_avg, pattern_type, key_signals,
				transaction_type, amount_range, error_orig_sign, error_dest_sign,
				embedding, source_task_id, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, now())
			RETURNING id
		`, c.Summary, c.Verdict, c.ConfidenceAvg, c.PatternType, c.KeySignals,
			c.TransactionType, c.AmountRange, c.ErrorOrigSign, c.ErrorDestSign,
			c.EmbeddingStr, c.SourceTaskID).Scan(&newID); err != nil {
			return fmt.Errorf("inserting case memory: %w", err)
		}
		resultID = newID
		return nil
	})
	if err != nil {
		return "", err
	}
	return resultID, nil
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
