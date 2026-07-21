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

func InsertMediumTasks(ctx context.Context, db *cockroach.Client) (int, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, risk_score FROM transactions
		WHERE risk_tier = 'medium'
	`)
	if err != nil {
		return 0, fmt.Errorf("querying medium transactions: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var txnID string
		var riskScore float64
		if err := rows.Scan(&txnID, &riskScore); err != nil {
			return count, fmt.Errorf("scanning transaction: %w", err)
		}

		taskID := uuid.New().String()
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO tasks (id, transaction_id, risk_score, status, created_at)
			VALUES ($1, $2, $3, 'pending', now())
			ON CONFLICT (transaction_id) DO NOTHING
		`, taskID, txnID, riskScore)
		if err != nil {
			return count, fmt.Errorf("inserting task: %w", err)
		}
		count++
	}
	return count, nil
}
