package agent

import (
	"testing"

	"github.com/lethingochan27925/hivemind/internal/scorer"
)

// TestDefaultPolicyMatchesCompiledThresholds is the safety net behind runtime
// policy: if the control plane is unreachable or the settings row is missing,
// the agent must fall back to exactly the calibrated thresholds it ships with -
// never to zero values, which would auto-approve or auto-block everything.
func TestDefaultPolicyMatchesCompiledThresholds(t *testing.T) {
	p := DefaultPolicy()

	if p.RiskLow != scorer.LowThreshold {
		t.Errorf("RiskLow = %v, want %v", p.RiskLow, scorer.LowThreshold)
	}
	if p.RiskHigh != scorer.HighThreshold {
		t.Errorf("RiskHigh = %v, want %v", p.RiskHigh, scorer.HighThreshold)
	}
	if p.FallbackAction != "escalate" {
		t.Errorf("FallbackAction = %q, want escalate (the safe default: a human sees it)", p.FallbackAction)
	}
	if !(p.RiskLow > 0 && p.RiskLow < p.RiskHigh && p.RiskHigh < 1) {
		t.Errorf("default band is nonsensical: %v .. %v", p.RiskLow, p.RiskHigh)
	}
}

// TestPolicyBandLeavesWorkForTheAgent guards against a default that would leave
// the investigation tier empty (auto-deciding everything without reasoning).
func TestPolicyBandLeavesWorkForTheAgent(t *testing.T) {
	p := DefaultPolicy()
	band := p.RiskHigh - p.RiskLow
	if band < 0.5 {
		t.Errorf("investigate band is only %.3f wide - the agent would rarely run", band)
	}
}
