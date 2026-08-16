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
	"time"

	"github.com/lethingochan27925/hivemind/internal/budget"
	"github.com/lethingochan27925/hivemind/internal/cache"
	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/pricing"
)

// Gia fallback tinh - chi con dung cho estimateCost(), giu lai de cac bai test
// hermetic (cloudcost_test) khong phu thuoc mang. Duong nong dung livePrices.
// Alias thang toi internal/budget - noi duy nhat khai bao con so gia - de
// khong the co hai gia Bedrock khac nhau lech nhau giua ngan sach va dashboard.
const (
	claudeInputCostPer1K  = budget.ClaudeInputCostPer1K
	claudeOutputCostPer1K = budget.ClaudeOutputCostPer1K
)

// Config nap mot lan cho ca doi container - moi lan Load() la nhieu round-trip
// SSM, khong co ly do tra gia do tren tung request.
//
// cache.Forever: chi cache KHI THANH CONG (cache.TTL khong bao gio ghi mot
// loi vao cache - xem doc cua no), nen mot that bai tam thoi (vi du SSM
// eventually-consistent luc cold start) khong bi dong bang mai mai nhu
// sync.Once tung lam - request sau tu hoi phuc ngay khi dieu kien het.
var cfgCache = cache.NewTTL[*config.Config](cache.Forever)

func loadCfgOnce() (*config.Config, error) {
	return cfgCache.Get(func() (*config.Config, time.Duration, error) {
		cfg, err := config.Load()
		return cfg, 0, err
	})
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
// khi livePrices khong dung duoc. Nguon gia song la internal/pricing; nguon
// gia tinh la internal/budget (tinh toan thuc su cung nam o do).
func estimateCost(tokensIn, tokensOut int) float64 {
	return budget.EstimateCost(tokensIn, tokensOut)
}
