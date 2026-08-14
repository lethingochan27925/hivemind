package dashboardapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRegionNamePattern locks the allow-list for a value that is interpolated
// into an ALTER DATABASE statement. Anything that could carry SQL, quotes, or
// whitespace must be rejected outright - the quoting in the statement builder is
// the second line of defence, not the first.
func TestRegionNamePattern(t *testing.T) {
	valid := []string{
		"aws-ap-southeast-1",
		"aws-ap-south-1",
		"gcp-us-east1",
		"az-westeurope",
		"a1",
	}
	for _, r := range valid {
		if !regionNameRe.MatchString(r) {
			t.Errorf("region %q should be accepted", r)
		}
	}

	invalid := []string{
		"",                          // empty
		"a",                         // too short
		"AWS-AP-SOUTHEAST-1",        // upper case
		"aws ap southeast 1",        // spaces
		`aws"; DROP DATABASE x; --`, // SQL injection attempt
		"aws-ap-southeast-1'",       // stray quote
		"région-1",                  // non-ascii
		"-leading-dash",             // must start alphanumeric
		"a" + string(make([]byte, 200)),
	}
	for _, r := range invalid {
		if regionNameRe.MatchString(r) {
			t.Errorf("region %q must be rejected", r)
		}
	}
}

// TestAlterRegionValidatesBeforeTouchingDatabase drives the real handler with a
// nil pool: every one of these must be refused by validation. A panic here means
// a bad request reached the database.
func TestAlterRegionValidatesBeforeTouchingDatabase(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "")
	s := &Server{}

	cases := []struct {
		name string
		body map[string]string
	}{
		{"unknown action", map[string]string{"action": "nuke", "region": "aws-ap-southeast-1"}},
		{"empty action", map[string]string{"action": "", "region": "aws-ap-southeast-1"}},
		{"add with injection", map[string]string{"action": "add", "region": `x"; DROP DATABASE hivemind; --`}},
		{"drop with empty region", map[string]string{"action": "drop", "region": ""}},
		{"set_primary with spaces", map[string]string{"action": "set_primary", "region": "aws ap southeast 1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(c.body)
			req := httptest.NewRequest(http.MethodPost, "/control/regions", &buf)
			rec := httptest.NewRecorder()
			s.RegionsHandler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400", rec.Code)
			}
		})
	}
}

func TestRegionsHandlerRejectsUnknownMethod(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{}).RegionsHandler(rec, httptest.NewRequest(http.MethodDelete, "/control/regions", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: got %d, want 405", rec.Code)
	}
}

func TestRegionsRequireControlToken(t *testing.T) {
	t.Setenv("CONTROL_TOKEN", "s3cr3t")
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]string{"action": "drop", "region": "aws-ap-south-1"})
	req := httptest.NewRequest(http.MethodPost, "/control/regions", &buf)
	rec := httptest.NewRecorder()
	(&Server{}).RegionsHandler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403 - dropping a region must never be unauthenticated", rec.Code)
	}
}
