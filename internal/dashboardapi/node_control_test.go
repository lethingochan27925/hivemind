package dashboardapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestInvokableIsRestrictedToTheFleet pins the allow-list behind "Run now" and
// the per-node schedule switch. Request-driven services (scoring, the control
// plane itself) must never be invokable from the browser - invoking dashboard-api
// from dashboard-api would be a trivially reachable recursion.
func TestInvokableIsRestrictedToTheFleet(t *testing.T) {
	for _, svc := range []string{"agent-worker", "dispatcher", "reaper", "salience-decay"} {
		if !invokable(svc) {
			t.Errorf("%s should be invokable", svc)
		}
	}
	for _, svc := range []string{"dashboard-api", "scoring-api", "scoring-python", "", "agent worker", "AGENT-WORKER"} {
		if invokable(svc) {
			t.Errorf("%s must NOT be invokable from the control plane", svc)
		}
	}
}

func TestScheduleOneValidation(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}

	cases := []struct {
		name string
		body map[string]string
	}{
		{"service with no schedule", map[string]string{"service": "dashboard-api", "action": "enable"}},
		{"unknown service", map[string]string{"service": "ghost", "action": "enable"}},
		{"unknown action", map[string]string{"service": "reaper", "action": "delete"}},
		{"empty action", map[string]string{"service": "reaper", "action": ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := post(t, s.ScheduleOne, "/control/schedule", c.body).Code; got != http.StatusBadRequest {
				t.Errorf("got %d, want 400", got)
			}
		})
	}
}

func TestInvokeServiceRejectsNonFleet(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}
	for _, svc := range []string{"dashboard-api", "scoring-python", "", "../../etc"} {
		if got := post(t, s.InvokeService, "/control/invoke", map[string]string{"service": svc}).Code; got != http.StatusBadRequest {
			t.Errorf("service %q: got %d, want 400", svc, got)
		}
	}
}

func TestNodeControlRejectsNonPost(t *testing.T) {
	s := &Server{}
	for name, h := range map[string]http.HandlerFunc{
		"ScheduleOne":   s.ScheduleOne,
		"InvokeService": s.InvokeService,
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/control/x", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s GET: got %d, want 405", name, rec.Code)
		}
	}
}

// TestSearchRejectsUselessQueries - the search endpoint fans out to four ILIKE
// queries, so a 1-character or oversized query must be refused before any of
// them run.
func TestSearchRejectsUselessQueries(t *testing.T) {
	s := &Server{}
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	for _, q := range []string{"", "a", " ", long} {
		// Query phai duoc encode: mot dau cach tho se lam hong dong request.
		req := httptest.NewRequest(http.MethodGet, "/control/search?q="+url.QueryEscape(q), nil)
		rec := httptest.NewRecorder()
		s.Search(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("q=%q: got %d, want 400", q, rec.Code)
		}
	}
}

func TestSearchRejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{}).Search(rec, httptest.NewRequest(http.MethodPost, "/control/search?q=fraud", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}
}
