// transactions.go: GET /transactions, GET /transactions/{id}/audit - trang Execution Timeline.
package dashboardapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type TransactionSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	RiskScore float64   `json:"risk_score"`
	RiskTier  string    `json:"risk_tier"`
	Verdict   *string   `json:"verdict,omitempty"`
	ArrivedAt time.Time `json:"arrived_at"`
}

type AuditStep struct {
	Action           string    `json:"action"`
	AgentID          string    `json:"agent_id"`
	Reasoning        *string   `json:"reasoning,omitempty"`
	MemoryHits       *int      `json:"memory_hits,omitempty"`
	SimilarityScores []float64 `json:"similarity_scores,omitempty"`
	TokensIn         *int      `json:"tokens_in,omitempty"`
	TokensOut        *int      `json:"tokens_out,omitempty"`
	LatencyMs        *int      `json:"latency_ms,omitempty"`
	BedrockModel     *string   `json:"bedrock_model,omitempty"`
	CreatedAt        string    `json:"created_at"`
}

func (s *Server) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	riskTier := r.URL.Query().Get("risk_tier")
	limitClause := "LIMIT 100"

	query := `
		SELECT
			tx.id, tx.type, tx.amount, tx.risk_score, tx.risk_tier,
			t.verdict, tx.arrived_at
		FROM transactions tx
		LEFT JOIN tasks t ON t.transaction_id = tx.id
	`
	var args []interface{}
	if riskTier != "" {
		query += " WHERE tx.risk_tier = $1"
		args = append(args, riskTier)
	}
	query += " ORDER BY tx.arrived_at DESC " + limitClause

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		http.Error(w, "failed to load transactions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []TransactionSummary
	for rows.Next() {
		var t TransactionSummary
		if err := rows.Scan(&t.ID, &t.Type, &t.Amount, &t.RiskScore, &t.RiskTier, &t.Verdict, &t.ArrivedAt); err != nil {
			http.Error(w, "failed to scan transaction", http.StatusInternalServerError)
			return
		}
		results = append(results, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) GetTransactionAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	txnID := strings.TrimPrefix(r.URL.Path, "/transactions/")
	txnID = strings.TrimSuffix(txnID, "/audit")
	if txnID == "" {
		http.Error(w, "transaction id required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT action, agent_id, reasoning, memory_hits, similarity_scores,
		       tokens_in, tokens_out, latency_ms, bedrock_model, created_at
		FROM audit_log
		WHERE transaction_id = $1
		ORDER BY created_at ASC
	`, txnID)
	if err != nil {
		http.Error(w, "failed to load audit trail", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var steps []AuditStep
	for rows.Next() {
		var a AuditStep
		var createdAt time.Time
		if err := rows.Scan(&a.Action, &a.AgentID, &a.Reasoning, &a.MemoryHits, &a.SimilarityScores,
			&a.TokensIn, &a.TokensOut, &a.LatencyMs, &a.BedrockModel, &createdAt); err != nil {
			http.Error(w, "failed to scan audit step", http.StatusInternalServerError)
			return
		}
		a.CreatedAt = createdAt.Format(time.RFC3339)
		steps = append(steps, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(steps)
}
