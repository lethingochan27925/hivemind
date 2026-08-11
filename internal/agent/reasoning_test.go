package agent

import (
	"strings"
	"testing"

	"github.com/lethingochan27925/hivemind/pkg/mcp"
)

// TestBalanceSignal locks in the discriminator that separates fraud from legit.
// It is the white-box unit test behind the "100% recall/precision" claim: pure,
// deterministic, no Bedrock, no database. See docs/AGENT_REASONING.md.
func TestBalanceSignal(t *testing.T) {
	cases := []struct {
		name       string
		txn        mcp.Transaction
		wantPrefix string
	}{
		{
			name: "origin held the amount and was zeroed, destination not credited -> DRAIN (fraud)",
			txn: mcp.Transaction{
				Type: "TRANSFER", Amount: 450000,
				OldBalanceOrig: 450000, NewBalanceOrig: 0,
				OldBalanceDest: 0, NewBalanceDest: 0,
			},
			wantPrefix: "DRAIN",
		},
		{
			name: "destination balance rose by the amount -> FUNDS MOVED (legit)",
			txn: mcp.Transaction{
				Type: "TRANSFER", Amount: 1000,
				OldBalanceOrig: 5000, NewBalanceOrig: 4000,
				OldBalanceDest: 200, NewBalanceDest: 1200,
			},
			wantPrefix: "FUNDS MOVED",
		},
		{
			name: "full-balance transfer that DID arrive is not a false DRAIN -> FUNDS MOVED",
			txn: mcp.Transaction{
				Type: "TRANSFER", Amount: 450000,
				OldBalanceOrig: 450000, NewBalanceOrig: 0,
				OldBalanceDest: 0, NewBalanceDest: 450000,
			},
			wantPrefix: "FUNDS MOVED",
		},
		{
			name: "matches neither clean drain nor completed transfer -> INCONCLUSIVE (escalate)",
			txn: mcp.Transaction{
				Type: "CASH_OUT", Amount: 1000,
				OldBalanceOrig: 5000, NewBalanceOrig: 4000,
				OldBalanceDest: 0, NewBalanceDest: 0,
			},
			wantPrefix: "INCONCLUSIVE",
		},
		{
			name: "origin equals amount but was not emptied -> INCONCLUSIVE",
			txn: mcp.Transaction{
				Type: "TRANSFER", Amount: 1000,
				OldBalanceOrig: 1000, NewBalanceOrig: 500,
				OldBalanceDest: 0, NewBalanceDest: 0,
			},
			wantPrefix: "INCONCLUSIVE",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			txn := c.txn
			got := balanceSignal(&txn)
			if !strings.HasPrefix(got, c.wantPrefix) {
				t.Errorf("balanceSignal() = %q, want prefix %q", got, c.wantPrefix)
			}
		})
	}
}
