// review.go: human-in-the-loop - duyet cac task co verdict escalate.
package review

import (
	"context"
	"fmt"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type PendingReview struct {
	TaskID        string
	TransactionID string
	Verdict       string
	Confidence    float64
}

func ListPendingReviews(ctx context.Context, db *cockroach.Client) ([]PendingReview, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, transaction_id, verdict, confidence
		FROM tasks
		WHERE status = 'pending_review'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing pending reviews: %w", err)
	}
	defer rows.Close()

	var reviews []PendingReview
	for rows.Next() {
		var r PendingReview
		if err := rows.Scan(&r.TaskID, &r.TransactionID, &r.Verdict, &r.Confidence); err != nil {
			return nil, fmt.Errorf("scanning review row: %w", err)
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}

// SubmitReview ghi quyet dinh nguoi duyet, chuyen task ve done va ghi audit trail.
func SubmitReview(ctx context.Context, db *cockroach.Client, taskID, reviewerID, decision, notes string) error {
	if decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid decision: %s", decision)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var transactionID string
	err = tx.QueryRow(ctx, `
		UPDATE tasks
		SET status = 'done',
			reviewed_by = $1,
			reviewed_at = now(),
			review_decision = $2
		WHERE id = $3 AND status = 'pending_review'
		RETURNING transaction_id
	`, reviewerID, decision, taskID).Scan(&transactionID)
	if err != nil {
		return fmt.Errorf("updating task review: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (
			task_id, transaction_id, agent_id, action, reviewer_id, review_notes, created_at
		) VALUES ($1, $2, $3, 'human_reviewed', $4, $5, now())
	`, taskID, transactionID, reviewerID, reviewerID, notes)
	if err != nil {
		return fmt.Errorf("writing review audit log: %w", err)
	}

	return tx.Commit(ctx)
}
