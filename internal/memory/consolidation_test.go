package memory

import "testing"

func TestAmountRange(t *testing.T) {
	cases := []struct {
		amount   float64
		expected string
	}{
		{amount: 500, expected: "low"},
		{amount: 9999, expected: "low"},
		{amount: 10000, expected: "mid"},
		{amount: 50000, expected: "mid"},
		{amount: 99999, expected: "mid"},
		{amount: 100000, expected: "high"},
		{amount: 500000, expected: "high"},
		{amount: 0, expected: "low"},
	}

	for _, c := range cases {
		got := AmountRange(c.amount)
		if got != c.expected {
			t.Errorf("AmountRange(%.2f) = %q, want %q", c.amount, got, c.expected)
		}
	}
}

func TestSignLabel(t *testing.T) {
	cases := []struct {
		val      float64
		expected string
	}{
		{val: 0, expected: "near_zero"},
		{val: 0.5, expected: "near_zero"},
		{val: -0.5, expected: "near_zero"},
		{val: 0.99, expected: "near_zero"},
		{val: -0.99, expected: "near_zero"},
		{val: 1.0, expected: "positive"},
		{val: 100, expected: "positive"},
		{val: -1.0, expected: "negative"},
		{val: -100, expected: "negative"},
	}

	for _, c := range cases {
		got := SignLabel(c.val)
		if got != c.expected {
			t.Errorf("SignLabel(%.2f) = %q, want %q", c.val, got, c.expected)
		}
	}
}
