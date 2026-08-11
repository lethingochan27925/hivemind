// replay.go: insert transactions da score vao CockroachDB, tao task cho medium-tier.
package stream

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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

func InsertTransactions(ctx context.Context, db *cockroach.Client, txns []ScoredTransaction) (int, error) {
	inserted := 0
	for _, t := range txns {
		id := uuid.New().String()
		_, err := db.Pool.Exec(ctx, `
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
		if err != nil {
			return inserted, fmt.Errorf("inserting transaction: %w", err)
		}
		inserted++
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
