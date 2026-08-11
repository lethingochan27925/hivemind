package scorer

import "testing"

// TestRiskTier locks the routing thresholds that decide which transactions the
// agent fleet ever sees. The boundaries matter: the thresholds are exclusive, so
// a score exactly on a threshold routes to "medium" (investigate), never auto-decided.
func TestRiskTier(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{-0.5, "low"},   // clamped-low nonsense still routes low
		{0.0, "low"},
		{0.0009, "low"},
		{LowThreshold, "medium"}, // 0.001 is NOT < 0.001 -> medium (boundary is investigate)
		{0.001, "medium"},
		{0.5, "medium"},
		{0.98, "medium"},
		{HighThreshold, "medium"}, // 0.999 is NOT > 0.999 -> medium (boundary is investigate)
		{0.9991, "high"},
		{1.0, "high"},
	}
	for _, c := range cases {
		if got := RiskTier(c.score); got != c.want {
			t.Errorf("RiskTier(%.4f) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestRiskTierThresholdConstants(t *testing.T) {
	if LowThreshold != 0.001 {
		t.Errorf("LowThreshold = %v, want 0.001 (PaySim bimodal calibration)", LowThreshold)
	}
	if HighThreshold != 0.999 {
		t.Errorf("HighThreshold = %v, want 0.999 (PaySim bimodal calibration)", HighThreshold)
	}
}
