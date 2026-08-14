// Package budget: mot cau hoi duy nhat - hom nay da tieu bao nhieu, va con
// duoc phep tieu khong.
//
// Truoc day guardrail ngan sach nam trong handler GET /cost/budget, nghia la
// no chi chay khi co nguoi mo trang Cost tren trinh duyet. Control plane la
// public demo, nen "phanh chi khi co nguoi nhin dong ho" khong phai la phanh.
// Package nay dat cau kiem tra vao noi duy nhat sinh chi phi Bedrock: dispatcher.
package budget

import (
	"context"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

// Gia Claude Haiku tren Bedrock (USD / 1K token). Giu dong bo voi
// internal/dashboardapi/cost.go - neu doi gia, doi ca hai.
const (
	claudeInputCostPer1K  = 0.00025
	claudeOutputCostPer1K = 0.00125

	// Khop voi default cua system_policy; dung khi bang policy chua ton tai.
	defaultDailyBudgetUSD = 5.0
)

// EstimateCost quy doi token da dung thanh USD.
func EstimateCost(tokensIn, tokensOut int) float64 {
	return float64(tokensIn)/1000.0*claudeInputCostPer1K +
		float64(tokensOut)/1000.0*claudeOutputCostPer1K
}

// Exceeded tra loi: chi tieu hom nay, tran ngan sach, va da vuot chua.
// budget <= 0 nghia la khong gioi han (khong bao gio vuot).
func Exceeded(ctx context.Context, db *cockroach.Client) (spendUSD, budgetUSD float64, exceeded bool, err error) {
	budgetUSD = defaultDailyBudgetUSD
	// Bang system_policy do dashboard-api tu tao lan dau dung den; neu chua co
	// (fleet chay truoc khi ai mo dashboard) thi dung default - khong tao bang
	// tu day de dispatcher khong can quyen DDL.
	_ = db.Pool.QueryRow(ctx,
		`SELECT daily_budget_usd FROM system_policy WHERE id = 1`).Scan(&budgetUSD)

	var tokensIn, tokensOut int
	if err = db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(tokens_in), 0), COALESCE(SUM(tokens_out), 0)
		FROM audit_log
		WHERE created_at >= current_date() AND tokens_in IS NOT NULL
	`).Scan(&tokensIn, &tokensOut); err != nil {
		return 0, budgetUSD, false, err
	}

	spendUSD = EstimateCost(tokensIn, tokensOut)
	return spendUSD, budgetUSD, budgetUSD > 0 && spendUSD >= budgetUSD, nil
}
