// eval: cham diem fleet agent tren du lieu that, tu he thong dang chay.
//
//	go run ./cmd/eval --api https://<dashboard-api>.lambda-url.ap-southeast-1.on.aws
//	go run ./cmd/eval --api "$API" --out evidence/scorecard.md
//
// KHONG can credential AWS hay mat khau database: no chi dung endpoint SQL
// chi-doc cua control plane. Nghia la bat ky ai co URL demo deu tu kiem chung
// duoc moi con so trong README - do la khac biet giua "chung toi tuyen bo 100%"
// va "ban tu chay lai va thay 100%".
//
// Scorecard tra loi dung mot cau hoi: fleet nay co that su lam duoc viec khong?
//  1. No phan doan dung khong?          -> confusion matrix vs nhan goc
//  2. No tu lam duoc bao nhieu?         -> ty le auto-resolve vs day cho nguoi
//  3. Phan doan den tu suy luan that?   -> ty le fallback (model khong tra loi)
//  4. Bo nho co tao ra gia tri?         -> so sanh case co recall vs case lanh
//  5. No chiu duoc su co khong?         -> requeue/resume, trung claim
//  6. No ton bao nhieu?                 -> token va USD
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type queryResult struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int             `json:"row_count"`
}

type client struct {
	api   string
	token string
	http  *http.Client
}

func (c *client) query(sql string) (*queryResult, error) {
	body, _ := json.Marshal(map[string]string{"sql": sql})
	req, err := http.NewRequest(http.MethodPost, c.api+"/control/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Control-Token", c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var qr queryResult
	if err := json.Unmarshal(raw, &qr); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &qr, nil
}

// num doc mot o ket qua ve float64. pgx tra ve so duoi nhieu dang (int64, float,
// hoac chuoi cho NUMERIC), nen cho phep ca ba thay vi gia dinh mot kieu.
func num(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case bool:
		if t {
			return 1
		}
		return 0
	case nil:
		return 0
	}
	return 0
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

type scorecard struct {
	GeneratedAt string `json:"generated_at"`
	API         string `json:"api"`

	TruePositive  int `json:"true_positive"`
	FalsePositive int `json:"false_positive"`
	TrueNegative  int `json:"true_negative"`
	FalseNegative int `json:"false_negative"`
	Escalated     int `json:"escalated"`

	Recall     float64 `json:"recall_pct"`
	Precision  float64 `json:"precision_pct"`
	F1         float64 `json:"f1_pct"`
	Accuracy   float64 `json:"accuracy_pct"`
	AutoResolv float64 `json:"auto_resolved_pct"`

	ModelDecisions  int     `json:"model_decisions"`
	FallbackVerdict int     `json:"fallback_verdicts"`
	FallbackPct     float64 `json:"fallback_pct"`

	ColdCases       int     `json:"cold_cases"`
	ColdLatencyMs   float64 `json:"cold_avg_latency_ms"`
	RecallCases     int     `json:"cases_with_memory"`
	RecallLatencyMs float64 `json:"memory_avg_latency_ms"`
	MemorySpeedup   float64 `json:"memory_speedup_pct"`
	AvgMemoryHits   float64 `json:"avg_memory_hits"`

	WarmAuto    int     `json:"warm_auto_resolved"`
	WarmEsc     int     `json:"warm_escalated"`
	ColdAuto    int     `json:"cold_auto_resolved"`
	ColdEsc     int     `json:"cold_escalated"`
	WarmAutoPct float64 `json:"warm_auto_resolved_pct"`
	ColdAutoPct float64 `json:"cold_auto_resolved_pct"`

	ConsolidatedCases int `json:"consolidated_cases"`
	RawCasesAbsorbed  int `json:"raw_cases_absorbed"`

	DistinctAgents int `json:"distinct_agents"`
	DoubleClaims   int `json:"double_claims"`
	Requeued       int `json:"tasks_requeued"`
	Resumed        int `json:"tasks_resumed"`

	TokensIn    int     `json:"tokens_in"`
	TokensOut   int     `json:"tokens_out"`
	CostUSD     float64 `json:"estimated_cost_usd_total"`
	CostPerCase float64 `json:"estimated_cost_usd_per_investigation"`
	CostPer500  float64 `json:"estimated_cost_usd_per_500_case_run"`
}

const (
	inputCostPer1K  = 0.00025
	outputCostPer1K = 0.00125
)

func main() {
	api := flag.String("api", os.Getenv("HIVEMIND_API"), "dashboard-api base URL")
	token := flag.String("token", os.Getenv("CONTROL_TOKEN"), "control token, if the API requires one")
	out := flag.String("out", "", "also write the scorecard to this markdown file")
	jsonOut := flag.String("json", "", "also write the raw numbers to this JSON file")
	flag.Parse()

	if *api == "" {
		fmt.Fprintln(os.Stderr, "usage: eval --api https://<dashboard-api-url>")
		os.Exit(2)
	}
	c := &client{api: strings.TrimRight(*api, "/"), token: *token, http: &http.Client{Timeout: 30 * time.Second}}

	sc := scorecard{GeneratedAt: time.Now().UTC().Format(time.RFC3339), API: c.api}

	// 1. Confusion matrix against the ground-truth label.
	qr, err := c.query(`SELECT t.verdict, tx.is_fraud_label, COUNT(*) AS n FROM tasks t JOIN transactions tx ON tx.id = t.transaction_id WHERE t.verdict IS NOT NULL GROUP BY t.verdict, tx.is_fraud_label`)
	fatal(err, "confusion matrix")
	for _, r := range qr.Rows {
		verdict, label, n := str(r[0]), num(r[1]) == 1, int(num(r[2]))
		switch {
		case verdict == "fraud" && label:
			sc.TruePositive += n
		case verdict == "fraud" && !label:
			sc.FalsePositive += n
		case verdict == "legit" && !label:
			sc.TrueNegative += n
		case verdict == "legit" && label:
			sc.FalseNegative += n
		case verdict == "escalate":
			sc.Escalated += n
		}
	}
	auto := sc.TruePositive + sc.FalsePositive + sc.TrueNegative + sc.FalseNegative
	total := auto + sc.Escalated
	sc.Recall = pct(sc.TruePositive, sc.TruePositive+sc.FalseNegative)
	sc.Precision = pct(sc.TruePositive, sc.TruePositive+sc.FalsePositive)
	if sc.Recall+sc.Precision > 0 {
		sc.F1 = 2 * sc.Recall * sc.Precision / (sc.Recall + sc.Precision)
	}
	sc.Accuracy = pct(sc.TruePositive+sc.TrueNegative, auto)
	sc.AutoResolv = pct(auto, total)

	// 2. Were those verdicts actually reasoned, or rule-based fallbacks?
	qr, err = c.query(`SELECT bedrock_model IS NULL AS is_fallback, COUNT(*) AS n FROM audit_log WHERE action = 'bedrock_reasoning' GROUP BY 1`)
	fatal(err, "fallback share")
	for _, r := range qr.Rows {
		if num(r[0]) == 1 {
			sc.FallbackVerdict = int(num(r[1]))
		} else {
			sc.ModelDecisions = int(num(r[1]))
		}
	}
	sc.FallbackPct = pct(sc.FallbackVerdict, sc.ModelDecisions+sc.FallbackVerdict)

	// 3. Does episodic memory pay for itself? Compare investigations that had a
	//    recall hit against cold ones, on the same metric: reasoning latency.
	// Gop theo task TRUOC khi join: mot task bi re-queue nhieu lan se co nhieu
	// hang memory_recall va bedrock_reasoning, join thang se nhan ban so lieu.
	qr, err = c.query(`SELECT CASE WHEN m.hits > 0 THEN 'warm' ELSE 'cold' END AS bucket, COUNT(*) AS n, AVG(b.ms) AS avg_ms FROM (SELECT task_id, MAX(memory_hits) AS hits FROM audit_log WHERE action = 'memory_recall' GROUP BY task_id) m JOIN (SELECT task_id, AVG(latency_ms) AS ms FROM audit_log WHERE action = 'bedrock_reasoning' GROUP BY task_id) b ON b.task_id = m.task_id GROUP BY 1`)
	fatal(err, "memory effect")
	for _, r := range qr.Rows {
		switch str(r[0]) {
		case "warm":
			sc.RecallCases, sc.RecallLatencyMs = int(num(r[1])), num(r[2])
		case "cold":
			sc.ColdCases, sc.ColdLatencyMs = int(num(r[1])), num(r[2])
		}
	}
	if sc.ColdLatencyMs > 0 && sc.RecallLatencyMs > 0 {
		sc.MemorySpeedup = (sc.ColdLatencyMs - sc.RecallLatencyMs) / sc.ColdLatencyMs * 100
	}

	qr, err = c.query(`SELECT COALESCE(AVG(memory_hits), 0) FROM audit_log WHERE action = 'memory_recall'`)
	fatal(err, "avg memory hits")
	if len(qr.Rows) > 0 {
		sc.AvgMemoryHits = num(qr.Rows[0][0])
	}

	// Gia tri that cua memory: case co ky uc lien quan co duoc agent tu quyet
	// nhieu hon khong (thay vi day sang nguoi)?
	// Dung lan recall DAU TIEN cua moi task. Neu lay MAX qua moi lan chay thi
	// sau vai vong feed, task nao cung "tung co hit" -> nhom cold rong va phep
	// so sanh mat nghia. Ket qua o day van la quan sat tren du lieu lich su;
	// bang chung manh hon la thi nghiem co kiem soat: scripts/memory-experiment.sh
	qr, err = c.query(`SELECT CASE WHEN f.first_hits > 0 THEN 'warm' ELSE 'cold' END AS bucket, CASE WHEN t.verdict = 'escalate' THEN 'escalated' ELSE 'auto' END AS outcome, COUNT(*) AS n FROM tasks t JOIN (SELECT task_id, first_value(memory_hits) OVER (PARTITION BY task_id ORDER BY created_at) AS first_hits, row_number() OVER (PARTITION BY task_id ORDER BY created_at) AS rn FROM audit_log WHERE action = 'memory_recall') f ON f.task_id = t.id AND f.rn = 1 WHERE t.verdict IS NOT NULL GROUP BY 1, 2`)
	fatal(err, "memory decision quality")
	for _, r := range qr.Rows {
		n := int(num(r[2]))
		switch str(r[0]) + "/" + str(r[1]) {
		case "warm/auto":
			sc.WarmAuto = n
		case "warm/escalated":
			sc.WarmEsc = n
		case "cold/auto":
			sc.ColdAuto = n
		case "cold/escalated":
			sc.ColdEsc = n
		}
	}
	sc.WarmAutoPct = pct(sc.WarmAuto, sc.WarmAuto+sc.WarmEsc)
	sc.ColdAutoPct = pct(sc.ColdAuto, sc.ColdAuto+sc.ColdEsc)

	qr, err = c.query(`SELECT COUNT(*) AS cases, COALESCE(SUM(merge_count), 0) AS absorbed FROM case_memory WHERE archived = false`)
	fatal(err, "consolidation")
	if len(qr.Rows) > 0 {
		sc.ConsolidatedCases, sc.RawCasesAbsorbed = int(num(qr.Rows[0][0])), int(num(qr.Rows[0][1]))
	}

	// 4. Fleet behaviour: concurrency and crash recovery.
	// Dem tren toan bo audit trail: agent ghi audit o moi buoc, nen day la bang
	// chung truc tiep ve so worker da thuc su lam viec.
	qr, err = c.query(`SELECT COUNT(DISTINCT agent_id) FROM audit_log WHERE agent_id <> 'control-plane'`)
	fatal(err, "distinct agents")
	if len(qr.Rows) > 0 {
		sc.DistinctAgents = int(num(qr.Rows[0][0]))
	}

	qr, err = c.query(`SELECT COUNT(*) FROM (SELECT transaction_id FROM tasks GROUP BY transaction_id HAVING COUNT(*) > 1)`)
	fatal(err, "double claims")
	if len(qr.Rows) > 0 {
		sc.DoubleClaims = int(num(qr.Rows[0][0]))
	}

	qr, err = c.query(`SELECT action, COUNT(*) FROM audit_log WHERE action IN ('task_requeued', 'task_resumed') GROUP BY action`)
	fatal(err, "recovery events")
	for _, r := range qr.Rows {
		switch str(r[0]) {
		case "task_requeued":
			sc.Requeued = int(num(r[1]))
		case "task_resumed":
			sc.Resumed = int(num(r[1]))
		}
	}

	// 5. Cost.
	qr, err = c.query(`SELECT COALESCE(SUM(tokens_in), 0), COALESCE(SUM(tokens_out), 0) FROM audit_log WHERE tokens_in IS NOT NULL`)
	fatal(err, "tokens")
	if len(qr.Rows) > 0 {
		sc.TokensIn, sc.TokensOut = int(num(qr.Rows[0][0])), int(num(qr.Rows[0][1]))
	}
	sc.CostUSD = float64(sc.TokensIn)/1000*inputCostPer1K + float64(sc.TokensOut)/1000*outputCostPer1K
	if decisions := sc.ModelDecisions + sc.FallbackVerdict; decisions > 0 {
		sc.CostPerCase = sc.CostUSD / float64(decisions)
		sc.CostPer500 = sc.CostPerCase * 500
	}

	md := render(sc)
	fmt.Print(md)

	if *out != "" {
		if err := os.WriteFile(*out, []byte(md), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", *out, err)
		} else {
			fmt.Fprintf(os.Stderr, "\nScorecard written to %s\n", *out)
		}
	}
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(sc, "", "  ")
		if err := os.WriteFile(*jsonOut, b, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", *jsonOut, err)
		}
	}

	// Exit non-zero when the fleet is demonstrably not doing its job, so this
	// can be used as a gate in CI, not only as a report.
	if sc.DoubleClaims > 0 || (sc.TruePositive+sc.FalseNegative > 0 && sc.Recall < 95) {
		os.Exit(1)
	}
}

func pct(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func fatal(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "querying %s: %v\n", what, err)
		os.Exit(1)
	}
}

func render(s scorecard) string {
	var b strings.Builder
	p := func(format string, args ...interface{}) { fmt.Fprintf(&b, format+"\n", args...) }

	p("# HiveMind fleet scorecard")
	p("")
	p("Generated %s from the live system at `%s`, using only the read-only SQL endpoint.", s.GeneratedAt, s.API)
	p("Reproduce with: `go run ./cmd/eval --api %s`", s.API)
	p("")

	p("## 1. Does it judge correctly?")
	p("")
	p("| | labelled fraud | labelled legit |")
	p("|---|---|---|")
	p("| **verdict fraud** | %d (caught) | %d (false alarm) |", s.TruePositive, s.FalsePositive)
	p("| **verdict legit** | %d (missed) | %d (correctly cleared) |", s.FalseNegative, s.TrueNegative)
	p("")
	p("- **Recall %.1f%%** — of every fraudulent transaction it decided on, this share was caught.", s.Recall)
	p("- **Precision %.1f%%** — of everything it called fraud, this share really was.", s.Precision)
	p("- **F1 %.1f%%** · **Accuracy %.1f%%** over %d auto-decided cases.", s.F1, s.Accuracy, s.TruePositive+s.FalsePositive+s.TrueNegative+s.FalseNegative)
	p("")

	p("## 2. How much work does it actually take off a human?")
	p("")
	p("- **%.1f%% auto-resolved** without any human involvement.", s.AutoResolv)
	p("- **%d cases escalated** — the agent flagged its own uncertainty instead of guessing.", s.Escalated)
	p("")

	p("## 3. Are those verdicts real reasoning?")
	p("")
	p("- **%d model decisions** vs **%d rule-based fallbacks** (%.1f%% fallback).", s.ModelDecisions, s.FallbackVerdict, s.FallbackPct)
	p("- Fallbacks are recorded with a NULL `bedrock_model`, so the audit trail never claims a rule was a model decision.")
	p("")

	p("## 4. Does the shared memory earn its keep?")
	p("")
	p("**Reuse.** %d consolidated memories absorbed **%d raw cases** — similar cases are merged into one", s.ConsolidatedCases, s.RawCasesAbsorbed)
	p("stronger memory instead of piling up duplicates, and each investigation recalls **%.2f** of them on average.", s.AvgMemoryHits)
	p("")
	p("**Decision quality** — the metric that matters, measured per investigated case:")
	p("")
	p("| | investigations | auto-resolved | escalated | auto-resolve rate |")
	p("|---|---|---|---|---|")
	p("| **with recalled memory** | %d | %d | %d | **%.1f%%** |", s.WarmAuto+s.WarmEsc, s.WarmAuto, s.WarmEsc, s.WarmAutoPct)
	p("| **cold (no memory hit)** | %d | %d | %d | %.1f%% |", s.ColdAuto+s.ColdEsc, s.ColdAuto, s.ColdEsc, s.ColdAutoPct)
	p("")
	switch {
	case s.WarmAuto+s.WarmEsc == 0 || s.ColdAuto+s.ColdEsc == 0:
		p("_One bucket is empty: after several replay cycles every task has already met the memory at least once,")
		p("so this historical split cannot separate the two conditions. Run the controlled experiment instead:_")
		p("`./scripts/memory-experiment.sh 100 6` — it archives the memory, measures a cold pass, then a warm one.")
	case s.WarmAutoPct >= s.ColdAutoPct:
		p("A case that could draw on past experience was resolved without a human **%.1f points more often**.", s.WarmAutoPct-s.ColdAutoPct)
	default:
		p("Cold cases resolved more often here (%.1f points), which usually means the recalled cases are the genuinely", s.ColdAutoPct-s.WarmAutoPct)
		p("harder ones. Read it with the reuse figures above, and confirm with `./scripts/memory-experiment.sh`.")
	}
	p("")
	if s.ColdLatencyMs > 0 && s.RecallLatencyMs > 0 {
		p("**Cost of recall.** Reasoning latency: %.0f ms with memory vs %.0f ms cold (%.1f%%).", s.RecallLatencyMs, s.ColdLatencyMs, s.MemorySpeedup)
	} else {
		p("**Cost of recall.** Average reasoning latency %.0f ms.", s.RecallLatencyMs)
	}
	p("Recall adds three short summaries to the prompt, so parity here is the expected result - the model call dominates.")
	p("Memory is not sold as a speed-up; it is knowledge reuse that keeps the context window small and bounded.")
	p("")

	p("## 5. Does the fleet survive itself?")
	p("")
	p("- **%d distinct agents** wrote to the audit trail - real concurrent workers, not one process in a loop.", s.DistinctAgents)
	p("- **%d double claims** — must be zero (`SKIP LOCKED` + `UNIQUE(transaction_id)`).", s.DoubleClaims)
	p("- **%d tasks re-queued** after a stale lease, **%d resumed** from their checkpoint.", s.Requeued, s.Resumed)
	p("")

	p("## 6. What did it cost?")
	p("")
	p("- **%s tokens in / %s tokens out** → **$%.4f** cumulative on Claude Haiku pricing.", thousands(s.TokensIn), thousands(s.TokensOut), s.CostUSD)
	p("- **$%.5f per investigation** across %s decisions → about **$%.2f per 500-case run**.", s.CostPerCase, thousands(s.ModelDecisions+s.FallbackVerdict), s.CostPer500)
	p("")

	verdict := "PASS"
	if s.DoubleClaims > 0 || (s.TruePositive+s.FalseNegative > 0 && s.Recall < 95) {
		verdict = "FAIL"
	}
	p("---")
	p("")
	p("**Gate: %s** — this command exits non-zero if recall drops below 95%% or any transaction is claimed twice.", verdict)
	return b.String()
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
