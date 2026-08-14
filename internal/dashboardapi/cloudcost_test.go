package dashboardapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShortService(t *testing.T) {
	cases := map[string]string{
		"Amazon Elastic Compute Cloud - Compute": "Elastic Compute Cloud",
		"AWS Lambda":                             "Lambda",
		"Amazon Simple Storage Service":          "Simple Storage Service",
		"AWS Key Management Service":             "Key Management Service",
		"Amazon CloudFront":                      "CloudFront",
		"Tax":                                    "Tax",
		"":                                       "",
	}
	for in, want := range cases {
		if got := shortService(in); got != want {
			t.Errorf("shortService(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortByCostDesc(t *testing.T) {
	items := []ServiceCost{
		{Service: "s3", USD: 0.02},
		{Service: "lambda", USD: 1.5},
		{Service: "cloudwatch", USD: 0.4},
		{Service: "ecr", USD: 3.0},
	}
	sortByCostDesc(items)
	want := []string{"ecr", "lambda", "cloudwatch", "s3"}
	for i, w := range want {
		if items[i].Service != w {
			t.Fatalf("position %d = %s, want %s (order: %+v)", i, items[i].Service, w, items)
		}
	}

	// Stable on the degenerate inputs.
	sortByCostDesc(nil)
	sortByCostDesc([]ServiceCost{})
	one := []ServiceCost{{Service: "only", USD: 1}}
	sortByCostDesc(one)
	if one[0].Service != "only" {
		t.Error("single-element slice mangled")
	}
}

func TestShortErrIsBounded(t *testing.T) {
	long := errors.New(strings.Repeat("x", 500))
	if got := shortErr(long); len(got) > 160 {
		t.Errorf("shortErr must bound the message, got %d chars", len(got))
	}
	// A typical AWS error keeps its useful head.
	awsErr := errors.New("operation error Cost Explorer: GetCostAndUsage, https response error StatusCode: 403")
	if got := shortErr(awsErr); !strings.Contains(got, "Cost Explorer") {
		t.Errorf("shortErr dropped the useful part: %q", got)
	}
}

func TestCloudCostRejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{}).GetCloudCost(rec, httptest.NewRequest(http.MethodPost, "/cost/infrastructure", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}
}

func TestEstimateCostMatchesHaikuPricing(t *testing.T) {
	// 1M in + 1M out at the documented Haiku rates.
	got := estimateCost(1_000_000, 1_000_000)
	want := 1000*claudeInputCostPer1K + 1000*claudeOutputCostPer1K
	if got != want {
		t.Errorf("estimateCost(1M,1M) = %v, want %v", got, want)
	}
	if estimateCost(0, 0) != 0 {
		t.Error("zero tokens must cost zero")
	}
	// Output tokens are the expensive half - a regression that swapped the two
	// constants would silently under-report spend.
	if estimateCost(0, 1000) <= estimateCost(1000, 0) {
		t.Error("output tokens must be priced above input tokens")
	}
}
