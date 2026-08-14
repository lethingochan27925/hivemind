package dashboardapi

import "testing"

// Approve = cho giao dich qua, reject = chan. Neu mapping nay lat nguoc, fleet
// se hoc DUNG BAI HOC NGUOC LAI tu moi lan review - loi te nhat co the co
// trong mot vong lap hoc, va khong loi runtime nao lo no ra.
func TestHumanVerdict(t *testing.T) {
	cases := map[string]string{
		"approved": "legit",
		"rejected": "fraud",
		"":         "legit", // default an toan: khong chan tien cua khach
	}
	for decision, want := range cases {
		if got := HumanVerdict(decision); got != want {
			t.Errorf("HumanVerdict(%q) = %q, want %q", decision, got, want)
		}
	}
}
