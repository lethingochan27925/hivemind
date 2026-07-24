// cost.go: GET /cost - token usage va chi phi uoc tinh, mot phan cua trang Fleet & Memory.
package dashboardapi

import (
	"context"
	"encoding/json"
	"net/http"
)

const (
	claudeInputCostPer1K  = 0.00025
	claudeOutputCostPer1K = 0.00125
)

type AgentCost struct {
	AgentID          string  `json:"agent_id"`
	TotalTokensIn    int     `json:"total_tokens_in"`
	TotalTokensOut   int     `json:"total_tokens_out"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

type CostResponse struct {
	TotalTokensToday int         `json:"total_tokens_today"`
	EstimatedCostUSD float64     `json:"estimated_cost_usd_today"`
	ByAgent          []AgentCost `json:"by_agent"`
}

func (s *Server) GetCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	rows, err := s.DB.Pool.Query(ctx, `
		SELECT
			agent_id,
			COALESCE(SUM(tokens_in), 0) AS tokens_in,
			COALESCE(SUM(tokens_out), 0) AS tokens_out
		FROM audit_log
		WHERE created_at >= current_date()
		  AND tokens_in IS NOT NULL
		GROUP BY agent_id
	`)
	if err != nil {
		http.Error(w, "failed to load cost data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var byAgent []AgentCost
	var totalTokens int
	var totalCost float64

	for rows.Next() {
		var a AgentCost
		if err := rows.Scan(&a.AgentID, &a.TotalTokensIn, &a.TotalTokensOut); err != nil {
			http.Error(w, "failed to scan cost row", http.StatusInternalServerError)
			return
		}
		a.EstimatedCostUSD = estimateCost(a.TotalTokensIn, a.TotalTokensOut)
		totalTokens += a.TotalTokensIn + a.TotalTokensOut
		totalCost += a.EstimatedCostUSD
		byAgent = append(byAgent, a)
	}

	resp := CostResponse{
		TotalTokensToday: totalTokens,
		EstimatedCostUSD: totalCost,
		ByAgent:          byAgent,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func estimateCost(tokensIn, tokensOut int) float64 {
	inCost := float64(tokensIn) / 1000.0 * claudeInputCostPer1K
	outCost := float64(tokensOut) / 1000.0 * claudeOutputCostPer1K
	return inCost + outCost
}

var _ context.Context
