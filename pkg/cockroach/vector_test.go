package cockroach

import "testing"

// TestEncodeVector verifies the []float32 -> CockroachDB VECTOR literal encoding.
// A malformed literal would make every case_memory insert fail, so this is the
// silent linchpin of the episodic-memory write path.
func TestEncodeVector(t *testing.T) {
	cases := []struct {
		name string
		in   []float32
		want string
	}{
		{"empty", []float32{}, "[]"},
		{"single", []float32{1.5}, "[1.5]"},
		{"integers", []float32{0, 1, 2}, "[0,1,2]"},
		{"negatives", []float32{-0.5, 0.5}, "[-0.5,0.5]"},
		{"mixed", []float32{-1, 0, 2.5}, "[-1,0,2.5]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EncodeVector(c.in); got != c.want {
				t.Errorf("EncodeVector(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestEncodeVectorDimension(t *testing.T) {
	// Titan v2 emits 1024-dim vectors; the literal must carry exactly that many
	// comma-separated values (n-1 commas).
	v := make([]float32, 1024)
	got := EncodeVector(v)
	commas := 0
	for _, r := range got {
		if r == ',' {
			commas++
		}
	}
	if commas != 1023 {
		t.Errorf("1024-dim vector encoded with %d commas, want 1023", commas)
	}
}
