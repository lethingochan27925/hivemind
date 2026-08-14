package pricing

import (
	"encoding/json"
	"testing"
)

// sku dung dang tra ve cua Pricing API: mot chuoi JSON moi san pham.
func sku(model, desc, unit, usd string) string {
	doc := map[string]interface{}{
		"product": map[string]interface{}{
			"attributes": map[string]string{"model": model, "inferenceType": "On-Demand"},
		},
		"terms": map[string]interface{}{
			"OnDemand": map[string]interface{}{
				"X.Y": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"X.Y.Z": map[string]interface{}{
							"description":  desc,
							"unit":         unit,
							"pricePerUnit": map[string]string{"USD": usd},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// Pricing API co the tra input va output o HAI SKU rieng (dang thuc te cua
// Bedrock) - parser phai gop qua nhieu san pham.
func TestExtractPricesAcrossTwoSKUs(t *testing.T) {
	inOnly := sku("Claude 3 Haiku", "USD per 1K input tokens", "1K tokens", "0.00025000")
	outOnly := sku("Claude 3 Haiku", "USD per 1K output tokens", "1K tokens", "0.00125000")

	in, out, ok := ExtractPrices([]string{inOnly}, "haiku")
	if !ok || in != 0.00025 || out != 0 {
		t.Fatalf("mot chieu phai bao ok de Get() bu gia niem yet: in=%v out=%v ok=%v", in, out, ok)
	}

	in, out, ok = ExtractPrices([]string{inOnly, outOnly}, "haiku")
	if !ok || in != 0.00025 || out != 0.00125 {
		t.Fatalf("got in=%v out=%v ok=%v, want 0.00025/0.00125/true", in, out, ok)
	}
}

func TestExtractPricesSingleSKUBothDimensions(t *testing.T) {
	doc := map[string]interface{}{
		"product": map[string]interface{}{
			"attributes": map[string]string{"model": "Claude 3 Haiku"},
		},
		"terms": map[string]interface{}{
			"OnDemand": map[string]interface{}{
				"T1": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"D1": map[string]interface{}{
							"description":  "USD per 1K input tokens",
							"unit":         "1K tokens",
							"pricePerUnit": map[string]string{"USD": "0.00025000"},
						},
						"D2": map[string]interface{}{
							"description":  "USD per 1K output tokens",
							"unit":         "1K tokens",
							"pricePerUnit": map[string]string{"USD": "0.00125000"},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	in, out, ok := ExtractPrices([]string{string(b)}, "haiku")
	if !ok || in != 0.00025 || out != 0.00125 {
		t.Fatalf("got in=%v out=%v ok=%v, want 0.00025/0.00125/true", in, out, ok)
	}
}

func TestExtractPricesPerTokenUnitConverts(t *testing.T) {
	doc := map[string]interface{}{
		"product": map[string]interface{}{"attributes": map[string]string{"model": "Claude 3 Haiku"}},
		"terms": map[string]interface{}{
			"OnDemand": map[string]interface{}{
				"T1": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"D1": map[string]interface{}{
							"description":  "USD per input token",
							"unit":         "tokens",
							"pricePerUnit": map[string]string{"USD": "0.00000025"},
						},
						"D2": map[string]interface{}{
							"description":  "USD per output token",
							"unit":         "tokens",
							"pricePerUnit": map[string]string{"USD": "0.00000125"},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(doc)
	in, out, ok := ExtractPrices([]string{string(b)}, "haiku")
	if !ok {
		t.Fatal("per-token unit phai duoc quy ve 1K")
	}
	if in < 0.00024 || in > 0.00026 || out < 0.00124 || out > 0.00126 {
		t.Fatalf("quy doi sai: in=%v out=%v", in, out)
	}
}

func TestExtractPricesWrongModelFallsThrough(t *testing.T) {
	b := sku("Claude 3 Opus", "USD per 1K input tokens", "1K tokens", "0.015")
	if _, _, ok := ExtractPrices([]string{b}, "haiku"); ok {
		t.Fatal("khong duoc khop model khac")
	}
}

func TestEstimate(t *testing.T) {
	p := Prices{InPer1K: 0.00025, OutPer1K: 0.00125}
	got := p.Estimate(1_000_000, 1_000_000)
	want := 0.25 + 1.25
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("Estimate(1M,1M) = %v, want %v", got, want)
	}
}

func TestModelKeyword(t *testing.T) {
	if modelKeyword("anthropic.claude-3-haiku-20240307-v1:0") != "haiku" {
		t.Fatal("phai rut ra 'haiku'")
	}
}
