package agent

import (
	"bytes"
	"testing"
)

// TestScratchpadRoundTrip proves the crash-resume contract at the unit level:
// whatever a worker checkpoints must decode back identically, so a re-queued task
// resumes at the same step instead of restarting. This is the durable-working-
// memory guarantee, isolated from the database.
func TestScratchpadRoundTrip(t *testing.T) {
	orig := &Scratchpad{
		PartialReasoning: "origin drained to zero, destination not credited",
		RetryCount:       2,
		TopKCases:        []byte(`[{"id":"c1","verdict":"fraud"}]`),
		MCPResult:        []byte(`[{"type":"TRANSFER"}]`),
	}
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := ParseScratchpad(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PartialReasoning != orig.PartialReasoning {
		t.Errorf("PartialReasoning = %q, want %q", got.PartialReasoning, orig.PartialReasoning)
	}
	if got.RetryCount != orig.RetryCount {
		t.Errorf("RetryCount = %d, want %d", got.RetryCount, orig.RetryCount)
	}
	if !bytes.Contains(got.TopKCases, []byte(`"c1"`)) {
		t.Errorf("TopKCases lost in round-trip: %s", got.TopKCases)
	}
	if !bytes.Contains(got.MCPResult, []byte(`"TRANSFER"`)) {
		t.Errorf("MCPResult lost in round-trip: %s", got.MCPResult)
	}
}

func TestParseScratchpadEmpty(t *testing.T) {
	for _, raw := range [][]byte{nil, {}} {
		sp, err := ParseScratchpad(raw)
		if err != nil {
			t.Fatalf("ParseScratchpad(%v): unexpected error %v", raw, err)
		}
		if sp == nil {
			t.Fatalf("ParseScratchpad(%v) = nil, want zero-valued scratchpad", raw)
		}
		if sp.RetryCount != 0 || sp.PartialReasoning != "" {
			t.Errorf("empty scratchpad not zero-valued: %+v", sp)
		}
	}
}

func TestParseScratchpadInvalid(t *testing.T) {
	if _, err := ParseScratchpad([]byte("{not valid json")); err == nil {
		t.Error("expected error on malformed scratchpad JSON, got nil")
	}
}
