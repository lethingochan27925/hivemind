package dashboardapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every test here drives a REAL handler with a nil database and no AWS client.
// That is the point: each of these inputs must be rejected by validation BEFORE
// the handler reaches storage or the cloud. If a guard were ever removed, the
// handler would dereference the nil pool and the test would panic instead of
// quietly letting a bad mutation through.

func post(t *testing.T, h http.HandlerFunc, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encoding body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// --- Task control ------------------------------------------------------------

func TestTaskControlRejectsBadInput(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}

	cases := []struct {
		name string
		body map[string]string
		want int
	}{
		{"unknown action", map[string]string{"action": "delete_everything", "task_id": "t1"}, http.StatusBadRequest},
		{"missing task id", map[string]string{"action": "requeue"}, http.StatusBadRequest},
		{"override without verdict", map[string]string{"action": "override", "task_id": "t1", "reviewer_id": "qa"}, http.StatusBadRequest},
		{"override with invented verdict", map[string]string{"action": "override", "task_id": "t1", "verdict": "probably_fraud", "reviewer_id": "qa"}, http.StatusBadRequest},
		{"override without reviewer", map[string]string{"action": "override", "task_id": "t1", "verdict": "fraud"}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := post(t, s.TaskControl, "/control/task", c.body).Code; got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestTaskControlRejectsNonPost(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{}).TaskControl(rec, httptest.NewRequest(http.MethodGet, "/control/task", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d, want 405", rec.Code)
	}
}

func TestControlTokenGuardsEveryMutatingEndpoint(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "s3cr3t")
	s := &Server{}

	endpoints := map[string]http.HandlerFunc{
		"/control/task":       s.TaskControl,
		"/control/memory":     s.MemoryAdmin,
		"/control/memory/job": s.MemoryJob,
		"/control/rollback":   s.RollbackService,
		"/control/schedule":   s.ScheduleOne,
		"/control/invoke":     s.InvokeService,
	}
	for path, h := range endpoints {
		t.Run(path, func(t *testing.T) {
			// No X-Control-Token header at all.
			if got := post(t, h, path, map[string]string{"action": "requeue"}).Code; got != http.StatusForbidden {
				t.Errorf("%s: got %d, want 403 (token guard missing)", path, got)
			}
		})
	}
}

// --- Memory administration ---------------------------------------------------

func TestMemoryAdminRejectsBadAction(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}

	for _, body := range []map[string]string{
		{"action": "drop_table", "id": "m1"},
		{"action": "pin"},          // no id
		{"action": "", "id": "m1"}, // no action
	} {
		if got := post(t, s.MemoryAdmin, "/control/memory", body).Code; got != http.StatusBadRequest {
			t.Errorf("body %v: got %d, want 400", body, got)
		}
	}
}

func TestMemoryJobValidatesThreshold(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"unknown job", map[string]interface{}{"job": "wipe"}},
		{"threshold zero", map[string]interface{}{"job": "archive_below", "threshold": 0}},
		{"threshold negative", map[string]interface{}{"job": "archive_below", "threshold": -1}},
		{"threshold above salience ceiling", map[string]interface{}{"job": "archive_below", "threshold": 2.5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := post(t, s.MemoryJob, "/control/memory/job", c.body).Code; got != http.StatusBadRequest {
				t.Errorf("got %d, want 400", got)
			}
		})
	}
}

// --- Bulk review -------------------------------------------------------------

func TestBulkReviewValidation(t *testing.T) {
	s := &Server{}

	tooMany := make([]string, 501)
	for i := range tooMany {
		tooMany[i] = "t"
	}

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"invented decision", map[string]interface{}{"task_ids": []string{"t1"}, "decision": "maybe", "reviewer_id": "qa"}},
		{"no reviewer - breaks the audit trail", map[string]interface{}{"task_ids": []string{"t1"}, "decision": "approved"}},
		{"blank reviewer", map[string]interface{}{"task_ids": []string{"t1"}, "decision": "approved", "reviewer_id": "   "}},
		{"empty id list", map[string]interface{}{"task_ids": []string{}, "decision": "approved", "reviewer_id": "qa"}},
		{"over the 500 cap", map[string]interface{}{"task_ids": tooMany, "decision": "approved", "reviewer_id": "qa"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := post(t, s.BulkReview, "/reviews/bulk", c.body).Code; got != http.StatusBadRequest {
				t.Errorf("got %d, want 400", got)
			}
		})
	}
}

// --- Rollback ----------------------------------------------------------------

func TestRollbackRejectsUnknownService(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}
	for _, svc := range []string{"", "not-a-service", "../etc/passwd", "hivemind-dev-agent-worker"} {
		if got := post(t, s.RollbackService, "/control/rollback", map[string]string{"service": svc}).Code; got != http.StatusBadRequest {
			t.Errorf("service %q: got %d, want 400", svc, got)
		}
	}
}

func TestKnownService(t *testing.T) {
	for _, svc := range []string{"agent-worker", "dispatcher", "reaper", "salience-decay", "scoring-api", "scoring-python", "dashboard-api"} {
		if !knownService(svc) {
			t.Errorf("knownService(%q) = false, want true", svc)
		}
	}
	for _, svc := range []string{"", "worker", "AGENT-WORKER", "agent-worker "} {
		if knownService(svc) {
			t.Errorf("knownService(%q) = true, want false", svc)
		}
	}
}

// --- Helpers -----------------------------------------------------------------

func TestNullIfEmpty(t *testing.T) {
	if nullIfEmpty("") != nil {
		t.Error("empty string must map to SQL NULL")
	}
	if nullIfEmpty("   ") != nil {
		t.Error("whitespace-only must map to SQL NULL")
	}
	if v := nullIfEmpty("qa"); v != "qa" {
		t.Errorf("got %v, want qa", v)
	}
}

func TestPolicyJSONRoundTrip(t *testing.T) {
	// The dashboard sends this struct straight back; the field names are part of
	// the contract with the UI.
	raw := `{"risk_low":0.01,"risk_high":0.98,"dispatch_batch":25,"fallback_action":"requeue","daily_budget_usd":3.5,"auto_pause_on_budget":true}`
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.RiskLow != 0.01 || p.RiskHigh != 0.98 || p.DispatchBatch != 25 ||
		p.FallbackAction != "requeue" || p.DailyBudgetUSD != 3.5 || !p.AutoPauseOnBudget {
		t.Errorf("decoded policy wrong: %+v", p)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"risk_low", "risk_high", "dispatch_batch", "fallback_action", "daily_budget_usd", "auto_pause_on_budget"} {
		if !strings.Contains(string(out), key) {
			t.Errorf("serialized policy missing %q: %s", key, out)
		}
	}
}
