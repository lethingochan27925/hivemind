// policy.go: doc tham so van hanh do control plane dat (bang system_policy).
// Agent doc moi lan xu ly task (co cache ngan) nen operator chinh nguong tren
// dashboard la co hieu luc ngay, khong can deploy lai. Thieu bang/loi doc thi
// dung mac dinh bien dich san - fleet khong bao gio dung vi policy.
package agent

import (
	"context"
	"fmt"
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
	switch {
	case cockroach.ShouldWarn(err):
		// A transient DB error here is silent by design at the call site (the
		// fleet must never stall on a policy read), but silent must not mean
		// invisible: a long-lived error would otherwise run the whole fleet on
		// compiled-in defaults for hours with nothing in the logs to explain why.
		// system_policy not existing yet (ShouldWarn excludes it, via
		// IsColdStartMiss) is not this: the table is created lazily by
		// dashboard-api, so every agent-worker seeing it missing before anyone
		// has opened the dashboard is normal, not a fault — and without that
		// carve-out this logged on every single task, every policyTTL,
		// fleet-wide, for the entirely expected cold-start window (the same
		// classification budget.Exceeded applies to the same table, for the
		// same reason - see cockroach.ShouldWarn's doc).
		fmt.Printf("[warn] LoadPolicy: reading system_policy failed, using defaults: %v\n", err)
	case err != nil:
		// Cold-start miss: fall through silently to defaults below.
	case !(low >= 0 && high <= 1 && low < high):
		fmt.Printf("[warn] LoadPolicy: system_policy has out-of-range thresholds (low=%v high=%v), using defaults\n", low, high)
	default:
		p.RiskLow, p.RiskHigh = low, high
		if fallback == "requeue" || fallback == "escalate" {
			p.FallbackAction = fallback
		}
	}

	policyCache, policyLoaded = p, time.Now()
	return p
}
