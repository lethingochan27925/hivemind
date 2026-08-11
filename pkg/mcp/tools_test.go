package mcp

import (
	"encoding/json"
	"math"
	"testing"
)

// TestTransactionRiskScoreParsing exercises flexFloat64: the MCP server returns
// risk_score as a JSON number for ordinary values but as a JSON string for very
// small scientific-notation values. Both must decode, or medium-risk routing breaks.
func TestTransactionRiskScoreParsing(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want float64
	}{
		{"numeric", `{"id":"t1","risk_score":0.5}`, 0.5},
		{"numeric integer", `{"id":"t1","risk_score":1}`, 1.0},
		{"string decimal", `{"id":"t1","risk_score":"0.25"}`, 0.25},
		{"string scientific", `{"id":"t1","risk_score":"1.28e-06"}`, 1.28e-06},
		{"string zero", `{"id":"t1","risk_score":"0"}`, 0.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var txn Transaction
			if err := json.Unmarshal([]byte(c.raw), &txn); err != nil {
				t.Fatalf("unmarshal(%s): %v", c.raw, err)
			}
			if math.Abs(txn.RiskScore()-c.want) > 1e-12 {
				t.Errorf("RiskScore() = %v, want %v", txn.RiskScore(), c.want)
			}
		})
	}
}

func TestTransactionRiskScoreInvalid(t *testing.T) {
	var txn Transaction
	if err := json.Unmarshal([]byte(`{"risk_score":"not-a-number"}`), &txn); err == nil {
		t.Error("expected error for non-numeric risk_score string, got nil")
	}
}

func TestTransactionFullDecode(t *testing.T) {
	raw := `{"id":"t1","step":5,"type":"TRANSFER","amount":450000,
	         "name_orig":"C1","old_balance_orig":450000,"new_balance_orig":0,
	         "name_dest":"C2","old_balance_dest":0,"new_balance_dest":0,
	         "risk_score":0.87,"risk_tier":"medium","is_fraud_label":true}`
	var txn Transaction
	if err := json.Unmarshal([]byte(raw), &txn); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if txn.Type != "TRANSFER" || txn.Amount != 450000 || txn.NameOrig != "C1" || !txn.IsFraudLabel {
		t.Errorf("decoded transaction fields wrong: %+v", txn)
	}
	if math.Abs(txn.RiskScore()-0.87) > 1e-12 {
		t.Errorf("RiskScore() = %v, want 0.87", txn.RiskScore())
	}
}
