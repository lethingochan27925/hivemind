// control.go: control-plane endpoints. Unlike the read-only pages, these MUTATE
// the live system — start/pause the fleet (EventBridge schedules), trigger a
// dispatch cycle (invoke the dispatcher Lambda), and feed cases into the queue.
// Mutating endpoints are gated by an optional shared token (CONTROL_TOKEN); the
// dashboard is a public NONE-auth Function URL, so in production this must be a
// real auth layer — documented as such.
package dashboardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
)

// scheduledServices are the EventBridge-driven Lambdas that form the fleet.
var scheduledServices = []string{"agent-worker", "dispatcher", "reaper", "salience-decay"}

func namePrefix() string {
	p := os.Getenv("PROJECT")
	e := os.Getenv("ENVIRONMENT")
	if p == "" {
		p = "hivemind"
	}
	if e == "" {
		e = "dev"
	}
	return p + "-" + e
}

func ruleName(svc string) string     { return namePrefix() + "-" + svc + "-schedule" }
func functionName(svc string) string { return namePrefix() + "-" + svc }

// --- AWS clients (lazy) ------------------------------------------------------

var (
	awsClientsMu sync.Mutex
	ebClient     *eventbridge.Client
	lambdaClient *lambda.Client
)

// awsClients is called from every request handler that touches EventBridge or
// Lambda, so on a Lambda container serving concurrent requests the old
// check-then-set on ebClient/lambdaClient (no lock) was a real data race:
// two goroutines could both see nil and both call LoadDefaultConfig, one
// write clobbering the other's client pointer mid-use. A mutex-guarded
// double-checked init fixes the race while still retrying on the next call if
// LoadDefaultConfig ever fails transiently (a plain sync.Once would instead
// wedge every future request behind that one failure for the life of the
// container).
func awsClients(ctx context.Context) (*eventbridge.Client, *lambda.Client, error) {
	awsClientsMu.Lock()
	defer awsClientsMu.Unlock()

	if ebClient != nil && lambdaClient != nil {
		return ebClient, lambdaClient, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, nil, err
	}
	ebClient = eventbridge.NewFromConfig(cfg)
	lambdaClient = lambda.NewFromConfig(cfg)
	return ebClient, lambdaClient, nil
}

// controlAllowed enforces the optional shared token on mutating requests.
func controlAllowed(r *http.Request) bool {
	want := os.Getenv("CONTROL_TOKEN")
	if want == "" {
		return true // guard disabled (demo mode)
	}
	return r.Header.Get("X-Control-Token") == want
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- GET /control/fleet ------------------------------------------------------

type ScheduleState struct {
	Service string `json:"service"`
	State   string `json:"state"` // ENABLED | DISABLED | UNKNOWN
}

type FleetStatus struct {
	Running   bool            `json:"running"` // true when all schedules enabled
	Schedules []ScheduleState `json:"schedules"`
	Tasks     map[string]int  `json:"tasks"` // status -> count
}

// FleetHandler dispatches /control/fleet by method: GET reads status, POST sets it.
func (s *Server) FleetHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.GetFleetStatus(w, r)
	case http.MethodPost:
		s.SetFleetState(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) GetFleetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eb, _, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	status := FleetStatus{Tasks: map[string]int{}}
	allEnabled := true
	for _, svc := range scheduledServices {
		st := "UNKNOWN"
		out, err := eb.DescribeRule(ctx, &eventbridge.DescribeRuleInput{Name: aws.String(ruleName(svc))})
		if err == nil {
			st = string(out.State)
		}
		if st != "ENABLED" {
			allEnabled = false
		}
		status.Schedules = append(status.Schedules, ScheduleState{Service: svc, State: st})
	}
	status.Running = allEnabled

	rows, err := s.DB.Pool.Query(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var st string
			var n int
			if rows.Scan(&st, &n) == nil {
				status.Tasks[st] = n
			}
		}
	}

	writeJSON(w, status)
}

// --- POST /control/fleet {action: start|pause} -------------------------------

func (s *Server) SetFleetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Action != "start" && body.Action != "pause" {
		http.Error(w, "action must be start or pause", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	eb, _, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	for _, svc := range scheduledServices {
		name := aws.String(ruleName(svc))
		if body.Action == "start" {
			_, err = eb.EnableRule(ctx, &eventbridge.EnableRuleInput{Name: name})
		} else {
			_, err = eb.DisableRule(ctx, &eventbridge.DisableRuleInput{Name: name})
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to %s %s: %v", body.Action, svc, err), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, map[string]interface{}{"status": "ok", "action": body.Action, "running": body.Action == "start"})
}

// --- POST /control/dispatch --------------------------------------------------
// Invoke one dispatcher cycle synchronously and return its result.

func (s *Server) RunDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	_, lc, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	out, err := lc.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(functionName("dispatcher")),
		Qualifier:      aws.String("live"),
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        []byte("{}"),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("invoking dispatcher: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","dispatcher_result":%s}`, string(out.Payload))
}

// --- POST /control/feed {count} ----------------------------------------------
// Requeue N already-processed tasks back to pending so the fleet re-investigates
// them — the whole demo can be driven from the dashboard, no SQL console needed.

func (s *Server) FeedStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Count <= 0 || body.Count > 1000 {
		body.Count = 50
	}

	ctx := r.Context()
	tag, err := s.DB.Pool.Exec(ctx, `
		UPDATE tasks
		SET status = 'pending', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
		    completed_at = NULL, verdict = NULL, confidence = NULL, step = NULL,
		    scratchpad = NULL, reviewed_by = NULL, reviewed_at = NULL, review_decision = NULL
		WHERE id IN (
			SELECT id FROM tasks
			WHERE status IN ('done', 'pending_review')
			ORDER BY completed_at DESC NULLS LAST
			LIMIT $1
		)
	`, body.Count)
	if err != nil {
		http.Error(w, fmt.Sprintf("feeding stream: %v", err), http.StatusInternalServerError)
		return
	}

	// Kick a dispatch cycle so the fleet picks them up immediately (best effort).
	if _, lc, err := awsClients(ctx); err == nil {
		_, _ = lc.Invoke(ctx, &lambda.InvokeInput{
			FunctionName:   aws.String(functionName("dispatcher")),
			Qualifier:      aws.String("live"),
			InvocationType: lambdatypes.InvocationTypeEvent,
			Payload:        []byte("{}"),
		})
	}

	writeJSON(w, map[string]interface{}{"status": "ok", "requeued": tag.RowsAffected()})
}

// --- GET /control/lambdas ----------------------------------------------------
// Live configuration of every function in the fleet — the "nodes" view.

type LambdaInfo struct {
	Service      string `json:"service"`
	State        string `json:"state"`
	Version      string `json:"version"`
	MemoryMB     int32  `json:"memory_mb"`
	TimeoutSec   int32  `json:"timeout_sec"`
	LastModified string `json:"last_modified"`
	Runtime      string `json:"runtime"`
}

func (s *Server) ListLambdas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, lc, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}

	all := []string{"agent-worker", "dispatcher", "reaper", "salience-decay", "scoring-api", "scoring-python", "dashboard-api"}
	infos := make([]LambdaInfo, 0, len(all))
	for _, svc := range all {
		info := LambdaInfo{Service: svc, State: "UNKNOWN"}
		out, err := lc.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
			FunctionName: aws.String(functionName(svc)),
			Qualifier:    aws.String("live"),
		})
		if err == nil {
			info.State = string(out.State)
			if out.Version != nil {
				info.Version = *out.Version
			}
			if out.MemorySize != nil {
				info.MemoryMB = *out.MemorySize
			}
			if out.Timeout != nil {
				info.TimeoutSec = *out.Timeout
			}
			if out.LastModified != nil {
				info.LastModified = *out.LastModified
			}
			info.Runtime = string(out.PackageType)
		}
		infos = append(infos, info)
	}
	writeJSON(w, infos)
}

// --- GET /control/resources --------------------------------------------------
// Every AWS resource Terraform tagged Project=<project>, via the Resource Groups
// Tagging API — the full infrastructure inventory (S3, CloudFront, EventBridge,
// IAM, SSM, ECR, CloudWatch, Lambda...), grouped by service.

type ResourceInfo struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	ARN     string `json:"arn"`
}

// parseARN pulls the AWS service and a friendly resource name out of an ARN.
func parseARN(arn string) ResourceInfo {
	svc, name := "aws", arn
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) >= 6 {
		svc = parts[2]
		res := parts[5]
		if i := strings.LastIndexAny(res, "/:"); i >= 0 {
			name = res[i+1:]
		} else {
			name = res
		}
	}
	return ResourceInfo{Service: svc, Name: name, ARN: arn}
}

// billingResourceRegion is the one place this project puts anything outside
// its home region: the billing alarm and its SNS topic
// (terraform/modules/monitoring/main.tf) live in us-east-1 because that's the
// only region AWS/Billing metrics ever publish to. resourcegroupstaggingapi
// is itself regional - a client built from the Lambda's own execution region
// (ap-southeast-1) only ever sees resources tagged there, so without this,
// "every resource Terraform tagged Project=hivemind" quietly excluded the
// two resources living furthest from home.
const billingResourceRegion = "us-east-1"

func (s *Server) ListResources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	project := os.Getenv("PROJECT")
	if project == "" {
		project = "hivemind"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		http.Error(w, "failed to init aws config", http.StatusInternalServerError)
		return
	}
	resources, err := listTaggedResources(ctx, cfg, project)
	if err != nil {
		http.Error(w, fmt.Sprintf("listing resources: %v", err), http.StatusInternalServerError)
		return
	}

	if cfg.Region != billingResourceRegion {
		billingCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(billingResourceRegion))
		if err == nil {
			// Best-effort: a transient failure to reach us-east-1 shouldn't
			// blank the other 50+ resources the home-region scan already found.
			if extra, err := listTaggedResources(ctx, billingCfg, project); err == nil {
				resources = append(resources, extra...)
			}
		}
	}

	writeJSON(w, resources)
}

func listTaggedResources(ctx context.Context, cfg aws.Config, project string) ([]ResourceInfo, error) {
	client := resourcegroupstaggingapi.NewFromConfig(cfg)

	resources := []ResourceInfo{}
	var token *string
	for {
		out, err := client.GetResources(ctx, &resourcegroupstaggingapi.GetResourcesInput{
			TagFilters:      []rgttypes.TagFilter{{Key: aws.String("Project"), Values: []string{project}}},
			PaginationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, m := range out.ResourceTagMappingList {
			if m.ResourceARN != nil {
				resources = append(resources, parseARN(*m.ResourceARN))
			}
		}
		if out.PaginationToken == nil || *out.PaginationToken == "" {
			break
		}
		token = out.PaginationToken
	}
	return resources, nil
}

// --- GET /control/db ---------------------------------------------------------
// Row counts of the memory tables in CockroachDB — the state store at a glance.

type TableStat struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

type DbStats struct {
	Database  string      `json:"database"`
	Tables    []TableStat `json:"tables"`
	TotalRows int64       `json:"total_rows"`
}

func (s *Server) GetDbStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats := DbStats{}
	_ = s.DB.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&stats.Database)

	// Table names are a fixed allow-list, never user input.
	for _, t := range []string{"transactions", "tasks", "case_memory", "audit_log"} {
		var n int64
		if err := s.DB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			continue
		}
		stats.Tables = append(stats.Tables, TableStat{Table: t, Rows: n})
		stats.TotalRows += n
	}
	writeJSON(w, stats)
}

// --- POST /control/query -----------------------------------------------------
// A READ-ONLY SQL console. Strictly validated: single statement, must start with
// SELECT/SHOW/WITH, and no mutating keyword anywhere. On a public endpoint this
// still needs real auth in production — documented as such.

// Khop theo ranh gioi tu, khong phai chuoi con: "CREATE TABLE" bi chan, con
// "created_at" - mot cot chi-doc binh thuong - van dung duoc. Truoc day dung
// strings.Contains nen moi truy van loc theo thoi gian deu bi tu choi.
var mutatingRe = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|grant|revoke|upsert|comment\s+on)\b`)

const queryRowLimit = 200

func (s *Server) RunQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	q := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body.SQL), ";"))
	lower := strings.ToLower(q)
	if q == "" {
		http.Error(w, "empty query", http.StatusBadRequest)
		return
	}
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "show") || strings.HasPrefix(lower, "with")) {
		http.Error(w, "only SELECT / SHOW / WITH queries are allowed", http.StatusBadRequest)
		return
	}
	if strings.Contains(q, ";") {
		http.Error(w, "only a single statement is allowed", http.StatusBadRequest)
		return
	}
	if mutatingRe.MatchString(q) {
		http.Error(w, "read-only console: mutating statements are rejected", http.StatusBadRequest)
		return
	}

	// Console nay public: mot cau join tich Descartes co the chay het 30s
	// timeout cua Lambda va dot RU cua CockroachDB serverless. 5 giay du cho
	// moi truy van hop le tren du lieu demo.
	const queryTimeout = 5 * time.Second
	qctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	rows, err := s.DB.Pool.Query(qctx, q)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusBadRequest)
		return
	}
	defer rows.Close()

	cols := []string{}
	for _, fd := range rows.FieldDescriptions() {
		cols = append(cols, string(fd.Name))
	}

	out := [][]interface{}{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		out = append(out, vals)
		if len(out) >= queryRowLimit {
			break
		}
	}

	writeJSON(w, map[string]interface{}{
		"columns":   cols,
		"rows":      out,
		"row_count": len(out),
		"truncated": len(out) >= queryRowLimit,
	})
}
