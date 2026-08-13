// ops.go: cac endpoint bien dashboard thanh control platform that su -
// chinh policy cua agent luc dang chay, quan tri episodic memory, thao tac
// tren tung task, duyet hang loat, guardrail chi phi, va rollback phien ban.
package dashboardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/lethingochan27925/hivemind/internal/review"
)

// --- Policy ------------------------------------------------------------------
// Mot bang mot dong: tham so van hanh cua fleet, sua duoc luc dang chay.
// Agent doc bang nay moi lan xu ly task nen thay doi co hieu luc ngay, khong
// can deploy lai.

type Policy struct {
	RiskLow           float64 `json:"risk_low"`
	RiskHigh          float64 `json:"risk_high"`
	DispatchBatch     int     `json:"dispatch_batch"`
	FallbackAction    string  `json:"fallback_action"` // escalate | requeue
	DailyBudgetUSD    float64 `json:"daily_budget_usd"`
	AutoPauseOnBudget bool    `json:"auto_pause_on_budget"`
	UpdatedBy         string  `json:"updated_by"`
	UpdatedAt         string  `json:"updated_at"`
}

const ensurePolicySQL = `
CREATE TABLE IF NOT EXISTS system_policy (
	id                   INT PRIMARY KEY DEFAULT 1,
	risk_low             FLOAT NOT NULL DEFAULT 0.001,
	risk_high            FLOAT NOT NULL DEFAULT 0.999,
	dispatch_batch       INT NOT NULL DEFAULT 20,
	fallback_action      STRING NOT NULL DEFAULT 'escalate',
	daily_budget_usd     FLOAT NOT NULL DEFAULT 5.0,
	auto_pause_on_budget BOOL NOT NULL DEFAULT false,
	updated_by           STRING,
	updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT single_row CHECK (id = 1)
)`

func (s *Server) ensurePolicy(ctx context.Context) error {
	if _, err := s.DB.Pool.Exec(ctx, ensurePolicySQL); err != nil {
		return err
	}
	_, err := s.DB.Pool.Exec(ctx, `INSERT INTO system_policy (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	return err
}

func (s *Server) readPolicy(ctx context.Context) (Policy, error) {
	var p Policy
	var updatedBy *string
	var updatedAt time.Time
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT risk_low, risk_high, dispatch_batch, fallback_action,
		       daily_budget_usd, auto_pause_on_budget, updated_by, updated_at
		FROM system_policy WHERE id = 1
	`).Scan(&p.RiskLow, &p.RiskHigh, &p.DispatchBatch, &p.FallbackAction,
		&p.DailyBudgetUSD, &p.AutoPauseOnBudget, &updatedBy, &updatedAt)
	if err != nil {
		return p, err
	}
	if updatedBy != nil {
		p.UpdatedBy = *updatedBy
	}
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

func (s *Server) PolicyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.ensurePolicy(ctx); err != nil {
		http.Error(w, fmt.Sprintf("policy store unavailable: %v", err), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, err := s.readPolicy(ctx)
		if err != nil {
			http.Error(w, "failed to read policy", http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	case http.MethodPost:
		if !controlAllowed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body Policy
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		// Chan gia tri vo nghia truoc khi ghi - policy nay dieu khien tien that.
		if body.RiskLow < 0 || body.RiskHigh > 1 || body.RiskLow >= body.RiskHigh {
			http.Error(w, "require 0 <= risk_low < risk_high <= 1", http.StatusBadRequest)
			return
		}
		if body.DispatchBatch < 1 || body.DispatchBatch > 200 {
			http.Error(w, "dispatch_batch must be 1-200", http.StatusBadRequest)
			return
		}
		if body.FallbackAction != "escalate" && body.FallbackAction != "requeue" {
			http.Error(w, "fallback_action must be escalate or requeue", http.StatusBadRequest)
			return
		}
		if body.DailyBudgetUSD < 0 || body.DailyBudgetUSD > 1000 {
			http.Error(w, "daily_budget_usd must be 0-1000", http.StatusBadRequest)
			return
		}
		if _, err := s.DB.Pool.Exec(ctx, `
			UPDATE system_policy
			SET risk_low = $1, risk_high = $2, dispatch_batch = $3, fallback_action = $4,
			    daily_budget_usd = $5, auto_pause_on_budget = $6,
			    updated_by = $7, updated_at = now()
			WHERE id = 1
		`, body.RiskLow, body.RiskHigh, body.DispatchBatch, body.FallbackAction,
			body.DailyBudgetUSD, body.AutoPauseOnBudget, nullIfEmpty(body.UpdatedBy)); err != nil {
			http.Error(w, fmt.Sprintf("failed to save policy: %v", err), http.StatusInternalServerError)
			return
		}
		p, _ := s.readPolicy(ctx)
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// --- Budget guardrail --------------------------------------------------------
// Doc chi tieu hom nay, so voi tran trong policy. Neu vuot va auto-pause dang
// bat thi TAT schedule ngay - guardrail that, khong phai canh bao suong.

type BudgetStatus struct {
	SpendToday   float64 `json:"spend_today"`
	Budget       float64 `json:"daily_budget_usd"`
	UsedPct      float64 `json:"used_pct"`
	Exceeded     bool    `json:"exceeded"`
	AutoPause    bool    `json:"auto_pause_on_budget"`
	PausedByRule bool    `json:"paused_by_rule"`
}

func (s *Server) GetBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	if err := s.ensurePolicy(ctx); err != nil {
		http.Error(w, "policy store unavailable", http.StatusInternalServerError)
		return
	}
	p, err := s.readPolicy(ctx)
	if err != nil {
		http.Error(w, "failed to read policy", http.StatusInternalServerError)
		return
	}

	var tokensIn, tokensOut int
	_ = s.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(tokens_in), 0), COALESCE(SUM(tokens_out), 0)
		FROM audit_log
		WHERE created_at >= current_date() AND tokens_in IS NOT NULL
	`).Scan(&tokensIn, &tokensOut)

	st := BudgetStatus{
		SpendToday: estimateCost(tokensIn, tokensOut),
		Budget:     p.DailyBudgetUSD,
		AutoPause:  p.AutoPauseOnBudget,
	}
	if st.Budget > 0 {
		st.UsedPct = st.SpendToday / st.Budget * 100
		st.Exceeded = st.SpendToday >= st.Budget
	}

	if st.Exceeded && st.AutoPause {
		if eb, _, err := awsClients(ctx); err == nil {
			for _, svc := range scheduledServices {
				_, _ = eb.DisableRule(ctx, &eventbridge.DisableRuleInput{Name: aws.String(ruleName(svc))})
			}
			st.PausedByRule = true
		}
	}

	writeJSON(w, st)
}

// --- Episodic memory administration -----------------------------------------

type MemoryRow struct {
	ID          string  `json:"id"`
	Summary     string  `json:"summary"`
	Verdict     string  `json:"verdict"`
	PatternType *string `json:"pattern_type,omitempty"`
	Salience    float64 `json:"salience"`
	RecallCount int     `json:"recall_count"`
	MergeCount  int     `json:"merge_count"`
	Archived    bool    `json:"archived"`
}

func (s *Server) MemoryAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listMemories(w, r)
	case http.MethodPost:
		s.actOnMemory(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	includeArchived := r.URL.Query().Get("archived") == "true"

	rows, err := s.DB.Pool.Query(r.Context(), `
		SELECT id, left(summary, 180), verdict, pattern_type,
		       salience, recall_count, merge_count, archived
		FROM case_memory
		WHERE ($1 OR archived = false)
		ORDER BY archived ASC, salience DESC, recall_count DESC
		LIMIT $2
	`, includeArchived, limit)
	if err != nil {
		http.Error(w, "failed to list memories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []MemoryRow{}
	for rows.Next() {
		var m MemoryRow
		if rows.Scan(&m.ID, &m.Summary, &m.Verdict, &m.PatternType,
			&m.Salience, &m.RecallCount, &m.MergeCount, &m.Archived) == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, out)
}

func (s *Server) actOnMemory(w http.ResponseWriter, r *http.Request) {
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Action string `json:"action"` // pin | unpin | archive | unarchive | delete
		ID     string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var stmt string
	switch body.Action {
	case "pin":
		stmt = `UPDATE case_memory SET salience = 2.0 WHERE id = $1`
	case "unpin":
		stmt = `UPDATE case_memory SET salience = 1.0 WHERE id = $1`
	case "archive":
		stmt = `UPDATE case_memory SET archived = true WHERE id = $1`
	case "unarchive":
		stmt = `UPDATE case_memory SET archived = false, salience = GREATEST(salience, 0.5) WHERE id = $1`
	case "delete":
		stmt = `DELETE FROM case_memory WHERE id = $1`
	default:
		http.Error(w, "action must be pin | unpin | archive | unarchive | delete", http.StatusBadRequest)
		return
	}

	tag, err := s.DB.Pool.Exec(r.Context(), stmt, body.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("memory %s failed: %v", body.Action, err), http.StatusBadRequest)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "memory not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "action": body.Action, "id": body.ID})
}

// MemoryJob chay cac job quan tri memory: decay (goi Lambda salience-decay
// that), hoac archive hang loat duoi mot nguong salience.
func (s *Server) MemoryJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Job       string  `json:"job"` // decay | archive_below
		Threshold float64 `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	switch body.Job {
	case "decay":
		_, lc, err := awsClients(ctx)
		if err != nil {
			http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
			return
		}
		if _, err := lc.Invoke(ctx, &lambda.InvokeInput{
			FunctionName: aws.String(functionName("salience-decay")),
			Qualifier:    aws.String("live"),
			Payload:      []byte("{}"),
		}); err != nil {
			http.Error(w, fmt.Sprintf("invoking salience-decay: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "job": "decay"})

	case "archive_below":
		if body.Threshold <= 0 || body.Threshold > 2 {
			http.Error(w, "threshold must be 0-2", http.StatusBadRequest)
			return
		}
		tag, err := s.DB.Pool.Exec(ctx,
			`UPDATE case_memory SET archived = true WHERE archived = false AND salience < $1`, body.Threshold)
		if err != nil {
			http.Error(w, fmt.Sprintf("archive failed: %v", err), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "job": "archive_below", "archived": tag.RowsAffected()})

	default:
		http.Error(w, "job must be decay or archive_below", http.StatusBadRequest)
	}
}

// --- Task-level control ------------------------------------------------------
// requeue: tra case ve cho agent dieu tra lai (thay vi bat nguoi xu ly).
// override: nguoi van hanh chot verdict, ghi audit human_reviewed.

func (s *Server) TaskControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Action     string `json:"action"` // requeue | override
		TaskID     string `json:"task_id"`
		Verdict    string `json:"verdict"`
		ReviewerID string `json:"reviewer_id"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.TaskID) == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	switch body.Action {
	case "requeue":
		tag, err := s.DB.Pool.Exec(ctx, `
			UPDATE tasks
			SET status = 'pending', claimed_by = NULL, claimed_at = NULL, heartbeat_at = NULL,
			    completed_at = NULL, verdict = NULL, confidence = NULL, step = NULL, scratchpad = NULL
			WHERE id = $1 OR transaction_id = $1
		`, body.TaskID)
		if err != nil {
			http.Error(w, fmt.Sprintf("requeue failed: %v", err), http.StatusBadRequest)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]string{"status": "ok", "action": "requeue", "task_id": body.TaskID})

	case "override":
		if body.Verdict != "fraud" && body.Verdict != "legit" && body.Verdict != "escalate" {
			http.Error(w, "verdict must be fraud | legit | escalate", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.ReviewerID) == "" {
			http.Error(w, "reviewer_id is required for an override", http.StatusBadRequest)
			return
		}
		// Nhan ca task_id lan transaction_id -> lay lai ID that de audit dung FK.
		var taskID, txnID string
		if err := s.DB.Pool.QueryRow(ctx, `
			UPDATE tasks
			SET verdict = $1, status = 'done', completed_at = now(),
			    reviewed_by = $2, reviewed_at = now(), review_decision = 'approved'
			WHERE id = $3 OR transaction_id = $3
			RETURNING id, transaction_id
		`, body.Verdict, body.ReviewerID, body.TaskID).Scan(&taskID, &txnID); err != nil {
			http.Error(w, fmt.Sprintf("override failed: %v", err), http.StatusBadRequest)
			return
		}
		notes := strings.TrimSpace(body.Notes)
		if notes == "" {
			notes = fmt.Sprintf("operator override -> %s", body.Verdict)
		}
		_, _ = s.DB.Pool.Exec(ctx, `
			INSERT INTO audit_log (task_id, transaction_id, agent_id, action, reasoning, reviewer_id, review_notes, created_at)
			VALUES ($1, $2, $3, 'human_reviewed', $4, $5, $6, now())
		`, taskID, txnID, "control-plane", notes, body.ReviewerID, notes)
		writeJSON(w, map[string]string{"status": "ok", "action": "override", "verdict": body.Verdict})

	default:
		http.Error(w, "action must be requeue or override", http.StatusBadRequest)
	}
}

// --- Bulk review -------------------------------------------------------------

func (s *Server) BulkReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		TaskIDs    []string `json:"task_ids"`
		Decision   string   `json:"decision"`
		ReviewerID string   `json:"reviewer_id"`
		Notes      string   `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Decision != "approved" && body.Decision != "rejected" {
		http.Error(w, "decision must be approved or rejected", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.ReviewerID) == "" {
		http.Error(w, "reviewer_id is required", http.StatusBadRequest)
		return
	}
	if len(body.TaskIDs) == 0 || len(body.TaskIDs) > 500 {
		http.Error(w, "task_ids must contain 1-500 ids", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	done, failed := 0, 0
	for _, id := range body.TaskIDs {
		if err := review.SubmitReview(ctx, s.DB, id, body.ReviewerID, body.Decision, body.Notes); err != nil {
			failed++
			continue
		}
		done++
	}
	writeJSON(w, map[string]int{"decided": done, "failed": failed})
}

// --- Version rollback --------------------------------------------------------
// Chuyen alias `live` ve ban published truoc do - phanh tay cho canary.

func (s *Server) RollbackService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !controlAllowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var body struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !knownService(body.Service) {
		http.Error(w, "unknown service", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	_, lc, err := awsClients(ctx)
	if err != nil {
		http.Error(w, "failed to init aws clients", http.StatusInternalServerError)
		return
	}
	fn := functionName(body.Service)

	alias, err := lc.GetAlias(ctx, &lambda.GetAliasInput{FunctionName: aws.String(fn), Name: aws.String("live")})
	if err != nil {
		http.Error(w, fmt.Sprintf("reading alias: %v", err), http.StatusInternalServerError)
		return
	}
	current, _ := strconv.Atoi(aws.ToString(alias.FunctionVersion))

	out, err := lc.ListVersionsByFunction(ctx, &lambda.ListVersionsByFunctionInput{FunctionName: aws.String(fn)})
	if err != nil {
		http.Error(w, fmt.Sprintf("listing versions: %v", err), http.StatusInternalServerError)
		return
	}
	versions := []int{}
	for _, v := range out.Versions {
		if n, err := strconv.Atoi(aws.ToString(v.Version)); err == nil {
			versions = append(versions, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))

	target := 0
	for _, v := range versions {
		if v < current {
			target = v
			break
		}
	}
	if target == 0 {
		http.Error(w, "no earlier published version to roll back to", http.StatusBadRequest)
		return
	}

	if _, err := lc.UpdateAlias(ctx, &lambda.UpdateAliasInput{
		FunctionName:    aws.String(fn),
		Name:            aws.String("live"),
		FunctionVersion: aws.String(strconv.Itoa(target)),
	}); err != nil {
		http.Error(w, fmt.Sprintf("rollback failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status": "ok", "service": body.Service,
		"from_version": current, "to_version": target,
	})
}

func knownService(svc string) bool {
	for _, s := range []string{"agent-worker", "dispatcher", "reaper", "salience-decay",
		"scoring-api", "scoring-python", "dashboard-api"} {
		if s == svc {
			return true
		}
	}
	return false
}
