package dashboardapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRunQueryRejectsMutations is the single most important safety test on the
// control plane: the read-only SQL console must reject every mutating or
// multi-statement input BEFORE it ever reaches the database. We drive the real
// handler with a nil DB — if any input slipped past validation it would panic on
// the nil pool, so a green run also proves nothing mutating touches the database.
func TestRunQueryRejectsMutations(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "") // guard disabled -> requests are authorised, isolating the SQL check

	rejected := []string{
		"DELETE FROM tasks",
		"DROP TABLE tasks",
		"UPDATE tasks SET status='done'",
		"INSERT INTO tasks (id) VALUES ('x')",
		"UPSERT INTO tasks (id) VALUES ('x')",
		"TRUNCATE tasks",
		"ALTER TABLE tasks ADD COLUMN x INT",
		"CREATE TABLE t (id INT)",
		"GRANT SELECT ON tasks TO analyst",
		"REVOKE SELECT ON tasks FROM analyst",
		"COMMENT ON TABLE tasks IS 'x'",
		"WITH x AS (DELETE FROM tasks RETURNING id) SELECT * FROM x", // WITH-prefixed but mutating
		"SELECT 1; DROP TABLE tasks",                                 // multi-statement
		"SELECT 1; SELECT 2",                                         // multi-statement
		"EXPLAIN ANALYZE SELECT 1",                                   // not SELECT/SHOW/WITH prefix
		"",                                                           // empty
		"   ",                                                        // whitespace only
	}

	for _, q := range rejected {
		body, _ := json.Marshal(map[string]string{"sql": q})
		req := httptest.NewRequest(http.MethodPost, "/control/query", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		(&Server{}).RunQuery(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("query %q: got %d, want %d (must be rejected before DB access)",
				q, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestRunQueryRejectsNonPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/control/query", nil)
	rec := httptest.NewRecorder()
	(&Server{}).RunQuery(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /control/query: got %d, want 405", rec.Code)
	}
}

func TestRunQueryHonoursControlToken(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "s3cr3t")
	body, _ := json.Marshal(map[string]string{"sql": "SELECT 1"})
	req := httptest.NewRequest(http.MethodPost, "/control/query", bytes.NewReader(body))
	// no X-Control-Token header
	rec := httptest.NewRecorder()
	(&Server{}).RunQuery(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("missing control token: got %d, want 403", rec.Code)
	}
}

// TestParseARN checks the ARN -> {service, name} extraction that builds the live
// infrastructure inventory and the architecture map.
func TestParseARN(t *testing.T) {
	cases := []struct {
		arn, svc, name string
	}{
		{"arn:aws:lambda:ap-southeast-1:123456789012:function:hivemind-dev-dispatcher", "lambda", "hivemind-dev-dispatcher"},
		{"arn:aws:s3:::hivemind-dev-dashboard", "s3", "hivemind-dev-dashboard"},
		{"arn:aws:events:ap-southeast-1:123456789012:rule/hivemind-dev-dispatcher-schedule", "events", "hivemind-dev-dispatcher-schedule"},
		{"arn:aws:iam::123456789012:role/hivemind-dev-agent-worker", "iam", "hivemind-dev-agent-worker"},
	}
	for _, c := range cases {
		got := parseARN(c.arn)
		if got.Service != c.svc || got.Name != c.name {
			t.Errorf("parseARN(%q) = {%q,%q}, want {%q,%q}", c.arn, got.Service, got.Name, c.svc, c.name)
		}
	}
}

// TestResourceNaming pins the project/environment naming convention the control
// plane uses to address EventBridge rules and Lambda functions.
func TestResourceNaming(t *testing.T) {
	t.Setenv("PROJECT", "")
	t.Setenv("ENVIRONMENT", "")
	if got := namePrefix(); got != "hivemind-dev" {
		t.Errorf("namePrefix() default = %q, want hivemind-dev", got)
	}
	if got := ruleName("dispatcher"); got != "hivemind-dev-dispatcher-schedule" {
		t.Errorf("ruleName(dispatcher) = %q", got)
	}
	if got := functionName("agent-worker"); got != "hivemind-dev-agent-worker" {
		t.Errorf("functionName(agent-worker) = %q", got)
	}

	t.Setenv("PROJECT", "hm")
	t.Setenv("ENVIRONMENT", "prod")
	if got := functionName("dispatcher"); got != "hm-prod-dispatcher" {
		t.Errorf("functionName override = %q, want hm-prod-dispatcher", got)
	}
}
