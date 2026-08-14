package mcp

import (
	"encoding/json"
	"math"
	"testing"
)

// FuzzTransactionDecode throws arbitrary bytes at the struct the agent builds
// every single investigation from. The MCP server is an external system: its
// payload shape is not ours to guarantee, so the only acceptable outcomes are
// "decoded" or "returned an error" - never a panic that kills the worker.
func FuzzTransactionDecode(f *testing.F) {
	f.Add([]byte(`{"id":"t1","type":"TRANSFER","amount":450000,"risk_score":0.5}`))
	f.Add([]byte(`{"risk_score":"1.28e-06"}`))     // scientific notation as a string
	f.Add([]byte(`{"risk_score":"not-a-number"}`)) // must error, not panic
	f.Add([]byte(`{"risk_score":null}`))
	f.Add([]byte(`{"amount":1e309}`)) // overflows float64
	f.Add([]byte(`{`))                // truncated
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, raw []byte) {
		var txn Transaction
		if err := json.Unmarshal(raw, &txn); err != nil {
			return // rejecting a malformed payload is correct
		}
		// If it decoded, RiskScore must be callable and yield a usable number.
		got := txn.RiskScore()
		if math.IsNaN(got) {
			t.Fatalf("RiskScore() = NaN for payload %q - routing would compare false in every branch", raw)
		}
	})
}

// FuzzRiskScoreString focuses the fuzzer on the one field with a custom
// unmarshaller, because that is where a real production bug lived: the MCP
// server returns risk_score as a JSON number for ordinary values but as a
// string for very small ones.
func FuzzRiskScoreString(f *testing.F) {
	f.Add("0.5")
	f.Add("1.28e-06")
	f.Add("-0")
	f.Add("Infinity")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		payload, err := json.Marshal(map[string]string{"risk_score": s})
		if err != nil {
			t.Skip()
		}
		var txn Transaction
		if err := json.Unmarshal(payload, &txn); err != nil {
			return // unparseable strings must produce an error
		}
		if math.IsNaN(txn.RiskScore()) {
			t.Fatalf("risk_score %q decoded to NaN", s)
		}
	})
}
