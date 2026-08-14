// cost.go: GET /cost - token usage THAT tu audit_log, nhan voi don gia lay
// SONG tu AWS Pricing API (internal/pricing, cache 12h). Response khai bao ro
// nguon gia (aws-pricing-api | static) de nguoi xem biet con so den tu dau.
//
// Hoa don thuc te cua toan bo ha tang (Cost Explorer, tre ~24h) nam o
// cloudcost.go - hai con so bo sung nhau: mot realtime uoc tinh tu token that,
// mot la tien AWS thuc su tinh.
package dashboardapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/pricing"
)

// Gia fallback tinh - chi con dung cho estimateCost(), giu lai de cac bai test
// hermetic (cloudcost_test) khong phu thuoc mang. Duong nong dung livePrices.
const (
	claudeInputCostPer1K  = 0.00025
	claudeOutputCostPer1K = 0.00125
)

// Config nap mot lan cho ca doi container - moi lan Load() la nhieu round-trip
// SSM, khong co ly do tra gia do tren tung request.
var (
	cfgOnce   sync.Once
	cfgCached *config.Config
	cfgErr    error
)

func loadCfgOnce() (*config.Config, error) {
	cfgOnce.Do(func() { cfgCached, cfgErr = config.Load() })
	return cfgCached, cfgErr
}

// livePrices: don gia hien hanh cho model dang chay. Moi that bai (config,
// Pricing API, parser) deu do ve fallback tinh co dan nhan - khong bao gio loi.
func livePrices(ctx context.Context) pricing.Prices {
	cfg, err := loadCfgOnce()
	if err != nil {
		return pricing.Prices{
			InPer1K: claudeInputCostPer1K, OutPer1K: claudeOutputCostPer1K,
			Source: "static", Model: "unknown",
		}
	}
	return pricing.Get(ctx, cfg.ClaudeModelID, cfg.AWSRegionBedrock)
}

type AgentCost struct {
	AgentID          string  `json:"agent_id"`
	TotalTokensIn    int     `json:"total_tokens_in"`
	TotalTokensOut   int     `json:"total_tokens_out"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

type CostResponse struct {
	TotalTokensToday int         `json:"total_tokens_today"`
	EstimatedCostUSD float64     `json:"estimated_cost_usd_today"`
	InputPer1K       float64     `json:"input_per_1k"`
	OutputPer1K      float64     `json:"output_per_1k"`
	PriceSource      string      `json:"price_source"`
	PricedModel      string      `json:"priced_model"`
	ByAgent          []AgentCost `json:"by_agent"`
}

func (s *Server) GetCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	prices := livePrices(ctx)

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
		a.EstimatedCostUSD = prices.Estimate(a.TotalTokensIn, a.TotalTokensOut)
		totalTokens += a.TotalTokensIn + a.TotalTokensOut
		totalCost += a.EstimatedCostUSD
		byAgent = append(byAgent, a)
	}

	resp := CostResponse{
		TotalTokensToday: totalTokens,
		EstimatedCostUSD: totalCost,
		InputPer1K:       prices.InPer1K,
		OutputPer1K:      prices.OutPer1K,
		PriceSource:      prices.Source,
		PricedModel:      prices.Model,
		ByAgent:          byAgent,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// estimateCost: duong fallback tinh, giu cho test hermetic va lam gia chot
// khi livePrices khong dung duoc. Nguon gia song la internal/pricing.
func estimateCost(tokensIn, tokensOut int) float64 {
	inCost := float64(tokensIn) / 1000.0 * claudeInputCostPer1K
	outCost := float64(tokensOut) / 1000.0 * claudeOutputCostPer1K
	return inCost + outCost
}
