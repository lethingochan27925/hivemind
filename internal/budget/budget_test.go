package budget

import "testing"

func TestEstimateCost(t *testing.T) {
	cases := []struct {
		in, out int
		want    float64
	}{
		{0, 0, 0},
		{1000, 0, 0.00025},
		{0, 1000, 0.00125},
		{1000, 1000, 0.0015},
		// Mot ngay demo dien hinh: ~4M token vao, 300K ra ~ $1.375
		{4_000_000, 300_000, 1.375},
	}
	for _, c := range cases {
		got := EstimateCost(c.in, c.out)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("EstimateCost(%d, %d) = %v, want %v", c.in, c.out, got, c.want)
		}
	}
}
