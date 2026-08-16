package dashboardapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPipelineRunsRejectsNonGet(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.GetPipelineRuns(rec, httptest.NewRequest(http.MethodPost, "/control/pipeline", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}
}

func TestGithubRepoDefault(t *testing.T) {
	t.Setenv("GITHUB_REPO", "")
	if got := githubRepo(); got != defaultGithubRepo {
		t.Errorf("githubRepo() with GITHUB_REPO unset = %q, want default %q", got, defaultGithubRepo)
	}
}

func TestGithubRepoOverride(t *testing.T) {
	t.Setenv("GITHUB_REPO", "someone/fork")
	if got := githubRepo(); got != "someone/fork" {
		t.Errorf("githubRepo() with GITHUB_REPO set = %q, want %q", got, "someone/fork")
	}
}
