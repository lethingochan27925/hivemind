// cloudcost.go: GET /cost/infrastructure - chi phi AWS that (Cost Explorer),
// khong chi token Bedrock. Cho phep trang Cost tra loi "he thong nay ton bao
// nhieu", tinh ca Lambda, S3, CloudFront, CloudWatch, ECR...
//
// Luu y van hanh: Cost Explorer chi co endpoint o us-east-1, phai duoc bat
// trong tai khoan, va moi request ton ~$0.01 - nen client cache 6 tieng.
package dashboardapi

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type ServiceCost struct {
	Service string  `json:"service"`
	USD     float64 `json:"usd"`
}

type CloudCost struct {
	Available    bool          `json:"available"`
	Note         string        `json:"note,omitempty"`
	PeriodStart  string        `json:"period_start"`
	PeriodEnd    string        `json:"period_end"`
	TotalUSD     float64       `json:"total_usd"`
	ByService    []ServiceCost `json:"by_service"`
	RefreshedAt  string        `json:"refreshed_at"`
	CacheMinutes int           `json:"cache_minutes"`
}

const cloudCostTTL = 6 * time.Hour

var (
	ccMu     sync.Mutex
	ccCache  *CloudCost
	ccLoaded time.Time
)

func (s *Server) GetCloudCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ccMu.Lock()
	defer ccMu.Unlock()

	if ccCache != nil && time.Since(ccLoaded) < cloudCostTTL {
		writeJSON(w, ccCache)
		return
	}

	ctx := r.Context()
	out := &CloudCost{CacheMinutes: int(cloudCostTTL.Minutes())}

	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	out.PeriodStart = start.Format("2006-01-02")
	out.PeriodEnd = now.AddDate(0, 0, 1).Format("2006-01-02")

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		out.Note = "AWS config unavailable"
		writeJSON(w, out)
		return
	}

	ce := costexplorer.NewFromConfig(cfg)
	res, err := ce.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &cetypes.DateInterval{Start: &out.PeriodStart, End: &out.PeriodEnd},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: strPtr("SERVICE")},
		},
	})
	if err != nil {
		// Cost Explorer chua bat / thieu quyen: bao ro thay vi de trang trong.
		out.Note = "Cost Explorer unavailable: " + shortErr(err)
		writeJSON(w, out)
		return
	}

	for _, period := range res.ResultsByTime {
		for _, g := range period.Groups {
			if len(g.Keys) == 0 || g.Metrics == nil {
				continue
			}
			m, ok := g.Metrics["UnblendedCost"]
			if !ok || m.Amount == nil {
				continue
			}
			amt, err := strconv.ParseFloat(*m.Amount, 64)
			if err != nil || amt <= 0 {
				continue
			}
			out.ByService = append(out.ByService, ServiceCost{Service: shortService(g.Keys[0]), USD: amt})
			out.TotalUSD += amt
		}
	}

	sortByCostDesc(out.ByService)
	out.Available = true
	out.RefreshedAt = time.Now().UTC().Format(time.RFC3339)

	ccCache, ccLoaded = out, time.Now()
	writeJSON(w, out)
}

// shortService rut gon ten dai cua AWS ("Amazon Elastic Compute Cloud - Compute").
func shortService(s string) string {
	s = strings.TrimPrefix(s, "Amazon ")
	s = strings.TrimPrefix(s, "AWS ")
	if i := strings.Index(s, " - "); i > 0 {
		s = s[:i]
	}
	return s
}

func shortErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ", "); i > 0 && i < 120 {
		msg = msg[:i]
	}
	if len(msg) > 160 {
		msg = msg[:160]
	}
	return msg
}

func sortByCostDesc(items []ServiceCost) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].USD > items[j-1].USD; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func strPtr(s string) *string { return &s }
