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
	"log"

	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

// Gia Claude Haiku tren Bedrock (USD / 1K token) - nguon gia tinh DUY NHAT.
// internal/dashboardapi/cost.go tham chieu thang toi hai hang so nay (khong
// khai bao rieng ban sao) de ngan sach va con so hien tren dashboard khong
// bao gio lech nhau chi vi mot noi doi gia con noi kia quen.
const (
	ClaudeInputCostPer1K  = 0.00025
	ClaudeOutputCostPer1K = 0.00125

	// Khop voi default cua system_policy; dung khi bang policy chua ton tai.
	defaultDailyBudgetUSD = 5.0
)

// EstimateCost quy doi token da dung thanh USD.
func EstimateCost(tokensIn, tokensOut int) float64 {
	return float64(tokensIn)/1000.0*ClaudeInputCostPer1K +
		float64(tokensOut)/1000.0*ClaudeOutputCostPer1K
}

// Exceeded tra loi: chi tieu hom nay, tran ngan sach, va da vuot chua.
// budget <= 0 nghia la khong gioi han (khong bao gio vuot).
func Exceeded(ctx context.Context, db *cockroach.Client) (spendUSD, budgetUSD float64, exceeded bool, err error) {
	budgetUSD = defaultDailyBudgetUSD
	// Bang system_policy do dashboard-api tu tao lan dau dung den; neu chua co
	// (fleet chay truoc khi ai mo dashboard) thi dung default - khong tao bang
	// tu day de dispatcher khong can quyen DDL. Chi loi "bang chua ton tai" va
	// "chua co dong nao" duoc coi la binh thuong va bo qua trong im lang; loi
	// khac (mang, quyen, timeout) duoc log ra - truoc day MOI loi deu bi nuot,
	// nen mot loi mang keo dai co the khien guardrail chay sai ngan sach hang
	// gio ma khong ai trong log thay dau hieu gi.
	if err := db.Pool.QueryRow(ctx,
		`SELECT daily_budget_usd FROM system_policy WHERE id = 1`).Scan(&budgetUSD); err != nil {
		if cockroach.ShouldWarn(err) {
			log.Printf("[warn] budget.Exceeded: reading system_policy failed, using default $%.2f: %v", defaultDailyBudgetUSD, err)
		}
		budgetUSD = defaultDailyBudgetUSD
	}

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
