// pipeline.go: GET /control/pipeline - proxies GitHub Actions run state for
// the Pipeline page.
//
// The dashboard used to call api.github.com directly from the browser,
// unauthenticated, on every viewer's own IP - 60 requests/hour per GitHub's
// documented unauthenticated rate limit (docs.github.com/en/rest/using-the-
// rest-api/rate-limits-for-the-rest-api). Several people looking at the demo
// from the same office/VPN egress IP could exhaust that in minutes and start
// seeing 403s. Proxying through here means every viewer shares ONE upstream
// budget from the Lambda's IP instead of spending their own, a short
// in-Lambda-container cache means N viewers within the cache window cost
// GitHub one call instead of N, and an optional GITHUB_TOKEN (unauthenticated
// if unset - this repo is public, so an unauthenticated call still works)
// raises that shared budget to 5,000/hour.
package dashboardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lethingochan27925/hivemind/internal/cache"
)

const (
	pipelineCacheTTL  = 30 * time.Second
	defaultGithubRepo = "lethingochan27925/hivemind"
)

var (
	// cache.TTL holds its lock across the fetch itself, so every request
	// that piles up in the instant after the 30s window expires shares ONE
	// upstream GitHub call instead of firing its own - the whole point of
	// this proxy (see file header) - with no separate coalescing mechanism
	// needed.
	pipelineCache = cache.NewTTL[pipelineFetchResult](pipelineCacheTTL)

	// One shared client so keep-alive connections to api.github.com are
	// reused across requests instead of paying a fresh TCP+TLS handshake on
	// every cache miss.
	pipelineHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

type pipelineFetchResult struct {
	body   []byte
	status int
}

func githubRepo() string {
	if v := os.Getenv("GITHUB_REPO"); v != "" {
		return v
	}
	return defaultGithubRepo
}

func (s *Server) GetPipelineRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := pipelineCache.Get(func() (pipelineFetchResult, time.Duration, error) {
		// Background, not r.Context(): this fetch is shared with every
		// request that piled up on the cache's lock, so it must not be
		// canceled just because the ONE caller who happened to trigger it
		// navigated away first. pipelineHTTPClient's own 10s timeout still
		// bounds it.
		return fetchPipelineRuns(context.Background())
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.status)
	w.Write(result.body)
}

// fetchPipelineRuns does the actual upstream call. Non-200 responses are
// passed through (GitHub's error body already explains rate limiting, a bad
// repo name, etc.) with ttl < 0 so cache.TTL never pins one of those - the
// next request tries GitHub again immediately instead of repeating a stale
// rate-limit response for 30s.
func fetchPipelineRuns(ctx context.Context) (pipelineFetchResult, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/"+githubRepo()+"/actions/runs?per_page=40", nil)
	if err != nil {
		return pipelineFetchResult{}, 0, fmt.Errorf("building GitHub request failed: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := pipelineHTTPClient.Do(req)
	if err != nil {
		return pipelineFetchResult{}, 0, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pipelineFetchResult{}, 0, fmt.Errorf("reading GitHub API response failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return pipelineFetchResult{body: body, status: resp.StatusCode}, -1, nil
	}

	// Validate before caching - never cache a malformed response.
	var check json.RawMessage
	if err := json.Unmarshal(body, &check); err != nil {
		return pipelineFetchResult{}, 0, fmt.Errorf("GitHub API returned invalid JSON")
	}

	return pipelineFetchResult{body: body, status: http.StatusOK}, 0, nil
}
