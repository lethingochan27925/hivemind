package cockroach

import "testing"

func TestMaxConnsFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want int32
	}{
		{"unset", "", defaultMaxConns},
		{"valid override", "25", 25},
		{"zero is invalid, falls back", "0", defaultMaxConns},
		{"negative is invalid, falls back", "-5", defaultMaxConns},
		{"garbage is invalid, falls back", "not-a-number", defaultMaxConns},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("COCKROACH_MAX_CONNS", c.env)
			if got := maxConnsFromEnv(); got != c.want {
				t.Errorf("maxConnsFromEnv() with COCKROACH_MAX_CONNS=%q = %d, want %d", c.env, got, c.want)
			}
		})
	}
}
