// router.go: phan loai risk_tier tu risk_score, nguong da hieu chinh cho PaySim bimodal.
package scorer

const (
	LowThreshold  = 0.001
	HighThreshold = 0.999
)

func RiskTier(riskScore float64) string {
	if riskScore < LowThreshold {
		return "low"
	}
	if riskScore > HighThreshold {
		return "high"
	}
	return "medium"
}
