package agent

import (
	"strings"
	"testing"

	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

// TestClassifyPattern pins the fraud-pattern taxonomy written into case_memory,
// including precedence: balance_wipe is checked before dest_no_update, which is
// checked before high_amount_transfer, then rapid_cashout, else unclassified.
func TestClassifyPattern(t *testing.T) {
	cases := []struct {
		name string
		txn  mcp.Transaction
		want string
	}{
		{
			name: "origin held the amount and was zeroed -> balance_wipe",
			txn:  mcp.Transaction{Type: "TRANSFER", Amount: 5000, OldBalanceOrig: 5000, NewBalanceOrig: 0},
			want: "balance_wipe",
		},
		{
			name: "large destination balance error -> dest_no_update",
			txn:  mcp.Transaction{Type: "TRANSFER", Amount: 3000, OldBalanceOrig: 9000, NewBalanceOrig: 6000, ErrorBalanceDest: 3000},
			want: "dest_no_update",
		},
		{
			name: "amount over 200k -> high_amount_transfer",
			txn:  mcp.Transaction{Type: "TRANSFER", Amount: 250000, OldBalanceOrig: 900000, NewBalanceOrig: 650000},
			want: "high_amount_transfer",
		},
		{
			name: "cash-out with no other signal -> rapid_cashout",
			txn:  mcp.Transaction{Type: "CASH_OUT", Amount: 3000, OldBalanceOrig: 9000, NewBalanceOrig: 6000},
			want: "rapid_cashout",
		},
		{
			name: "ordinary transfer -> unclassified",
			txn:  mcp.Transaction{Type: "TRANSFER", Amount: 3000, OldBalanceOrig: 9000, NewBalanceOrig: 6000},
			want: "unclassified",
		},
		{
			name: "balance_wipe wins even when amount is also high",
			txn:  mcp.Transaction{Type: "TRANSFER", Amount: 300000, OldBalanceOrig: 300000, NewBalanceOrig: 0},
			want: "balance_wipe",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			txn := c.txn
			if got := classifyPattern(&txn); got != c.want {
				t.Errorf("classifyPattern() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestBuildCaseSummary confirms the episodic-memory summary text carries the
// fields the embedding and the human reader rely on.
func TestBuildCaseSummary(t *testing.T) {
	txn := mcp.Transaction{Type: "TRANSFER", Amount: 1000, ErrorBalanceOrig: -500, ErrorBalanceDest: 500}
	got := buildCaseSummary(&txn)
	for _, want := range []string{"type=TRANSFER", "amount=1000.00", "error_orig=-500.00", "error_dest=500.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildCaseSummary() = %q, missing %q", got, want)
		}
	}
}
