// policy.go: doc tham so van hanh do control plane dat (bang system_policy).
// Agent doc moi lan xu ly task (co cache ngan) nen operator chinh nguong tren
// dashboard la co hieu luc ngay, khong can deploy lai. Thieu bang/loi doc thi
// dung mac dinh bien dich san - fleet khong bao gio dung vi policy.
package agent

import (
	"context"
	"sync"
	"time"

	"github.com/lethingochan27925/hivemind/internal/scorer"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

type RuntimePolicy struct {
	RiskLow        float64
	RiskHigh       float64
	FallbackAction string // escalate | requeue
}

func DefaultPolicy() RuntimePolicy {
	return RuntimePolicy{
		RiskLow:        scorer.LowThreshold,
		RiskHigh:       scorer.HighThreshold,
		FallbackAction: "escalate",
	}
}

const policyTTL = 15 * time.Second

var (
	policyMu     sync.Mutex
	policyCache  RuntimePolicy
	policyLoaded time.Time
)

func LoadPolicy(ctx context.Context, db *cockroach.Client) RuntimePolicy {
	policyMu.Lock()
	defer policyMu.Unlock()

	if !policyLoaded.IsZero() && time.Since(policyLoaded) < policyTTL {
		return policyCache
	}

	p := DefaultPolicy()
	var low, high float64
	var fallback string
	err := db.Pool.QueryRow(ctx,
		`SELECT risk_low, risk_high, fallback_action FROM system_policy WHERE id = 1`,
	).Scan(&low, &high, &fallback)
	if err == nil && low >= 0 && high <= 1 && low < high {
		p.RiskLow, p.RiskHigh = low, high
		if fallback == "requeue" || fallback == "escalate" {
			p.FallbackAction = fallback
		}
	}

	policyCache, policyLoaded = p, time.Now()
	return p
}
