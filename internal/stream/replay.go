// replay.go: insert transactions da score vao CockroachDB, tao task cho medium-tier.
package stream

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lethingochan27925/hivemind/internal/scorer"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type ScoredTransaction struct {
	RawTransaction
	RiskScore float64
	RiskTier  string
}

func ScoreAndTag(txns []RawTransaction, riskScores []float64) []ScoredTransaction {
	scored := make([]ScoredTransaction, len(txns))
	for i, t := range txns {
		scored[i] = ScoredTransaction{
			RawTransaction: t,
			RiskScore:      riskScores[i],
			RiskTier:       scorer.RiskTier(riskScores[i]),
		}
	}
	return scored
}

// InsertTransactions writes a batch of scored transactions. Previously this
// ran one INSERT per row over its own network round-trip; for a 1000-row feed
// (the size cmd/gen-data and the dashboard's "Feed N" both default to) that
// is 1000 sequential round-trips to CockroachDB Cloud before the fleet can
// even start. pgx.Batch pipelines every statement over one connection in one
// round-trip (see the pgx docs' "Batch" type) — the standard way to remove
// per-row latency from bulk writes without hand-building a giant multi-row
// VALUES string. ON CONFLICT DO NOTHING per statement is preserved exactly,
// which a raw COPY (pgx.CopyFrom) cannot do.
func InsertTransactions(ctx context.Context, db *cockroach.Client, txns []ScoredTransaction) (int, error) {
	if len(txns) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, t := range txns {
		id := uuid.New().String()
		batch.Queue(`
			INSERT INTO transactions (
				id, step, type, amount,
				name_orig, old_balance_orig, new_balance_orig,
				name_dest, old_balance_dest, new_balance_dest,
				error_balance_orig, error_balance_dest,
				risk_score, risk_tier, is_fraud_label, arrived_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())
			ON CONFLICT DO NOTHING
		`, id, t.Step, t.Type, t.Amount,
			t.NameOrig, t.OldBalanceOrig, t.NewBalanceOrig,
			t.NameDest, t.OldBalanceDest, t.NewBalanceDest,
			t.ErrorBalanceOrig, t.ErrorBalanceDest,
			t.RiskScore, t.RiskTier, t.IsFraud)
	}

	br := db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	// Count rows CockroachDB actually inserted, not statements attempted - the
	// same distinction InsertMediumTasks below already has to get right,
	// since a blind per-statement counter over-reports the moment ON CONFLICT
	// skips a row.
	inserted := 0
	for range txns {
		tag, err := br.Exec()
		if err != nil {
			return inserted, fmt.Errorf("inserting transaction: %w", err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// InsertMediumTasks tao task cho cac transaction medium-tier chua co task.
// Dung mot cau lenh set-based thay vi N round-trip: dispatcher chay moi phut,
// quet toan bang roi insert tung dong se timeout khi du lieu lon.
// Tra ve so task THUC SU duoc tao - ban cu dem ca dong bi ON CONFLICT bo qua
// nen luon bao cao sai.
func InsertMediumTasks(ctx context.Context, db *cockroach.Client, batchSize int) (int, error) {
	tag, err := db.Pool.Exec(ctx, `
		INSERT INTO tasks (transaction_id, risk_score, status)
		SELECT id, risk_score, 'pending'
		FROM (
			SELECT tx.id, tx.risk_score
			FROM transactions tx
			LEFT JOIN tasks t ON t.transaction_id = tx.id
			WHERE tx.risk_tier = 'medium' AND t.id IS NULL
			ORDER BY tx.arrived_at ASC
			LIMIT $1
		) AS candidates
		ON CONFLICT (transaction_id) DO NOTHING
	`, batchSize)
	if err != nil {
		return 0, fmt.Errorf("inserting medium tasks: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
