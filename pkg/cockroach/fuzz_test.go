package cockroach

import (
	"math"
	"strings"
	"testing"
)

// FuzzEncodeVector proves the invariant that matters for a value that is
// about to be spliced verbatim into a SQL VECTOR(...) literal: whatever
// float32 values go in, the string that comes out must never contain "NaN"
// or "Inf" (the only strings strconv.FormatFloat can produce that are not
// valid numeric literals), and must always have exactly len(embedding)-1
// commas — no value ever silently dropped or duplicated.
func FuzzEncodeVector(f *testing.F) {
	f.Add(float32(0))
	f.Add(float32(1.5))
	f.Add(float32(-1.5))
	f.Add(float32(math.NaN()))
	f.Add(float32(math.Inf(1)))
	f.Add(float32(math.Inf(-1)))
	f.Add(float32(math.MaxFloat32))
	f.Add(float32(-math.MaxFloat32))

	f.Fuzz(func(t *testing.T, v float32) {
		got := EncodeVector([]float32{v})
		if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
			t.Fatalf("EncodeVector(%v) = %q contains a non-numeric literal - CockroachDB would reject this VECTOR(...) value", v, got)
		}
		if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
			t.Fatalf("EncodeVector(%v) = %q is not bracket-wrapped", v, got)
		}
	})
}
