// Package pricing: don gia token Bedrock lay TRUC TIEP tu AWS Pricing API,
// khong hardcode.
//
// Truoc day don gia nam cung trong code (0.00025/0.00125 moi 1K token) o hai
// file khac nhau - AWS doi gia la moi con so chi phi tren dashboard sai ma
// khong ai biet. Gio don gia duoc hoi tu Pricing API (cache 12h), va moi
// response /cost khai bao ro no dang dung gia song hay gia fallback - nguoi
// xem luon biet con so den tu dau.
//
// Fallback tinh la co chu dich: Pricing API sap, parser lac hau, hay thieu
// quyen IAM - dashboard van hien chi phi voi nhan "static" thay vi chet.
package pricing

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awspricing "github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/lethingochan27925/hivemind/internal/budget"
	"github.com/lethingochan27925/hivemind/internal/cache"
)

// Gia fallback (USD / 1K token) - Claude Haiku, khop voi bang gia cong bo.
// Chi duoc dung khi Pricing API khong tra loi duoc. Alias thang toi
// internal/budget - noi duy nhat khai bao con so gia - de ba noi dung gia
// (guardrail ngan sach, con so tinh o day, va gia song khi Pricing API sap)
// khong the lech nhau vi mot noi doi gia con hai noi kia quen.
const (
	fallbackInPer1K  = budget.ClaudeInputCostPer1K
	fallbackOutPer1K = budget.ClaudeOutputCostPer1K

	cacheTTL = 12 * time.Hour
	// Fallback chi cache ngan: mot lan truot mang khong duoc phep ghim
	// nhan "static" suot nua ngay.
	fallbackTTL = 5 * time.Minute
	// Pricing API chi co o vai region; us-east-1 phuc vu bang gia toan cau.
	pricingRegion = "us-east-1"
	maxPages      = 20
)

type Prices struct {
	InPer1K  float64 `json:"input_per_1k"`
	OutPer1K float64 `json:"output_per_1k"`
	Source   string  `json:"price_source"` // "aws-pricing-api" | "static"
	Model    string  `json:"priced_model"`
}

// Estimate quy doi token thanh USD theo bang gia nay.
func (p Prices) Estimate(tokensIn, tokensOut int) float64 {
	return float64(tokensIn)/1000.0*p.InPer1K + float64(tokensOut)/1000.0*p.OutPer1K
}

var priceCache = cache.NewTTL[Prices](cacheTTL)

// Get tra ve don gia cho model dang dung, tai region dang chay.
// An toan goi thuong xuyen: ket qua cache 12h, loi thi ve fallback ngay.
func Get(ctx context.Context, modelID, regionCode string) Prices {
	// fetchPrices khong bao gio tra error (moi nhanh deu tra ve mot Prices hop
	// le - toi thieu la fallback tinh), nen error o day chi con y nghia phong
	// thu - bo qua va dung fallback truc tiep neu cache.TTL vi ly do nao do
	// van truyen no ra.
	p, err := priceCache.Get(func() (Prices, time.Duration, error) {
		// Background, khong dung ctx cua request: cache.TTL giu khoa xuyen
		// suot fetch nay, nen moi request dong thoi mien cache deu dung
		// chung DUNG MOT lan goi Pricing API - no khong duoc phep bi huy chi
		// vi request dau tien kich hoat no da bi nguoi goi bo giua chung.
		return fetchPrices(context.Background(), modelID, regionCode)
	})
	if err != nil {
		return Prices{InPer1K: fallbackInPer1K, OutPer1K: fallbackOutPer1K, Source: "static", Model: modelID}
	}
	return p
}

func fetchPrices(ctx context.Context, modelID, regionCode string) (Prices, time.Duration, error) {
	in, out, _ := fetchFromAPI(ctx, modelID, regionCode)
	if in == 0 && out == 0 && regionCode != "us-east-1" {
		// Catalog khong liet ke model o region nay (ap-southeast-1: 90 SKU,
		// khong co Claude); gia token Bedrock dong nhat giua cac region nen
		// us-east-1 la nguon thay the tin cay.
		in, out, _ = fetchFromAPI(ctx, modelID, "us-east-1")
	}
	switch {
	case in > 0 && out > 0:
		return Prices{InPer1K: in, OutPer1K: out, Source: "aws-pricing-api", Model: modelID}, 0, nil
	case in > 0 || out > 0:
		// Thuc te do duoc (14/08/2026): catalog chi co SKU input cho Claude 3
		// Haiku, khong ton tai SKU output. Chieu nao co thi dung gia song,
		// chieu thieu bu bang gia niem yet - va NOI RO qua nhan "hybrid".
		if in == 0 {
			in = fallbackInPer1K
		}
		if out == 0 {
			out = fallbackOutPer1K
		}
		log.Printf("[pricing] catalog partial cho %s: dung hybrid (thieu mot chieu)", modelID)
		return Prices{InPer1K: in, OutPer1K: out, Source: "hybrid", Model: modelID}, 0, nil
	default:
		// fallbackTTL, khong phai cacheTTL mac dinh: mot lan truot mang khong
		// duoc phep ghim nhan "static" suot nua ngay - lan goi ke tiep thu
		// lai Pricing API som hon nhieu.
		log.Printf("[pricing] khong khop SKU token nao cho %s - dung gia niem yet", modelID)
		fallback := Prices{InPer1K: fallbackInPer1K, OutPer1K: fallbackOutPer1K, Source: "static", Model: modelID}
		return fallback, fallbackTTL, nil
	}
}

func fetchFromAPI(ctx context.Context, modelID, regionCode string) (float64, float64, bool) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(pricingRegion))
	if err != nil {
		log.Printf("[pricing] load config: %v", err)
		return 0, 0, false
	}
	client := awspricing.NewFromConfig(cfg)

	var priceList []string
	var next *string
	for page := 0; page < maxPages; page++ {
		out, err := client.GetProducts(ctx, &awspricing.GetProductsInput{
			ServiceCode: aws.String("AmazonBedrock"),
			Filters: []pricingtypes.Filter{{
				Type:  pricingtypes.FilterTypeTermMatch,
				Field: aws.String("regionCode"),
				Value: aws.String(regionCode),
			}},
			MaxResults: aws.Int32(100),
			NextToken:  next,
		})
		if err != nil {
			log.Printf("[pricing] GetProducts (region=%s, page %d): %v", regionCode, page, err)
			return 0, 0, false
		}
		priceList = append(priceList, out.PriceList...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		next = out.NextToken
	}

	return ExtractPrices(priceList, modelKeyword(modelID))
}

// modelKeyword rut tu khoa nhan dang model tu model ID day du:
// "anthropic.claude-3-haiku-20240307-v1:0" -> "haiku".
func modelKeyword(modelID string) string {
	lower := strings.ToLower(modelID)
	for _, kw := range []string{"haiku", "sonnet", "opus", "titan-embed", "titan"} {
		if strings.Contains(lower, kw) {
			return kw
		}
	}
	return lower
}

// ExtractPrices quet danh sach SKU cua Pricing API va tim don gia input/output
// per-1K-token cua model. Xuat ra ngoai de test duoc bang du lieu dong hop -
// parser nay PHAI hoat dong khi AWS doi cau truc mo ta, nen moi phep so khop
// deu phong thu: khong doan truong nao ton tai, chi doc nhung gi co mat.
func ExtractPrices(priceList []string, keyword string) (inPer1K, outPer1K float64, ok bool) {
	for _, raw := range priceList {
		var doc struct {
			Product struct {
				Attributes map[string]string `json:"attributes"`
			} `json:"product"`
			Terms struct {
				OnDemand map[string]struct {
					PriceDimensions map[string]struct {
						Description  string            `json:"description"`
						Unit         string            `json:"unit"`
						PricePerUnit map[string]string `json:"pricePerUnit"`
					} `json:"priceDimensions"`
				} `json:"OnDemand"`
			} `json:"terms"`
		}
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			continue
		}

		attrs, _ := json.Marshal(doc.Product.Attributes)
		if !strings.Contains(strings.ToLower(string(attrs)), keyword) {
			continue
		}

		for _, term := range doc.Terms.OnDemand {
			for _, dim := range term.PriceDimensions {
				desc := strings.ToLower(dim.Description + " " + dim.Unit)
				if !strings.Contains(desc, "token") {
					continue
				}
				usd, err := parseUSD(dim.PricePerUnit["USD"])
				if err != nil || usd <= 0 {
					continue
				}
				// Don vi thuong la "1K tokens"; neu tinh theo tung token thi quy ve 1K.
				if !strings.Contains(desc, "1k") && !strings.Contains(desc, "1000") {
					usd *= 1000
				}
				switch {
				case strings.Contains(desc, "input"):
					inPer1K = usd
				case strings.Contains(desc, "output"):
					outPer1K = usd
				}
			}
		}
		if inPer1K > 0 && outPer1K > 0 {
			return inPer1K, outPer1K, true
		}
	}
	// Tra ve ca ket qua MOT chieu: catalog Bedrock co model chi liet ke gia
	// input (Claude 3 Haiku la vi du thuc te) - nguoi goi quyet dinh bu the nao.
	return inPer1K, outPer1K, inPer1K > 0 || outPer1K > 0
}

func parseUSD(s string) (float64, error) {
	var v float64
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}
