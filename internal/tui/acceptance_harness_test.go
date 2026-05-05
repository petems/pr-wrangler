package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
)

func TestAcceptanceHarness_RendersDashboardFromMockGitHub(t *testing.T) {
	t.Parallel()

	// A small deterministic fixture set: search -> PR -> reviews -> checks -> statuses.
	searchResp := map[string]any{
		"total_count":        1,
		"incomplete_results": false,
		"items": []map[string]any{
			{
				"number":         123,
				"repository_url": "https://api.github.com/repos/acme/widgets",
			},
		},
	}

	prResp := map[string]any{
		"number":   123,
		"title":    "Fix flaky test",
		"html_url": "https://github.com/acme/widgets/pull/123",
		"state":    "open",
		"draft":    false,
		"merged":   false,
		"user":     map[string]any{"login": "alice"},
		"head": map[string]any{
			"ref": "alice/flaky-fix",
			"sha": "deadbeef",
		},
		"labels":          []map[string]any{{"name": "bug"}},
		"mergeable":       true,
		"mergeable_state": "clean",
	}

	reviewsResp := []map[string]any{{
		"user":  map[string]any{"login": "bob"},
		"state": "APPROVED",
	}}

	checkRunsResp := map[string]any{
		"total_count": 1,
		"check_runs": []map[string]any{{
			"name":       "ci",
			"status":     "completed",
			"conclusion": "success",
		}},
	}

	combinedStatusResp := map[string]any{
		"state":    "success",
		"statuses": []map[string]any{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/search/issues":
			q := r.URL.Query().Get("q")
			if !strings.Contains(q, "is:pr") {
				t.Fatalf("expected search query to include is:pr, got %q", q)
			}
			_ = json.NewEncoder(w).Encode(searchResp)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/pulls/123":
			_ = json.NewEncoder(w).Encode(prResp)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/pulls/123/reviews":
			_ = json.NewEncoder(w).Encode(reviewsResp)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/commits/deadbeef/check-runs":
			_ = json.NewEncoder(w).Encode(checkRunsResp)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/commits/deadbeef/status":
			_ = json.NewEncoder(w).Encode(combinedStatusResp)
			return
		}

		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}

	client := gh.NewClient(srv.Client())
	client.BaseURL = baseURL
	client.UploadURL = baseURL

	ghClient := github.NewGHClientWithClient(client, "test-token")

	// Fetch directly via GH client, then render via the TUI model.
	ctx := context.Background()
	result, err := ghClient.FetchPRs(ctx, "author:@me is:open", nil)
	if err != nil {
		t.Fatalf("FetchPRs: %v", err)
	}

	cfg := config.DefaultConfig()
	// Keep the UI deterministic for width-dependent layouts.
	cfg.Views = []config.View{{Name: "default", Query: "author:@me is:open", Default: true}}

	m := NewModel(ghClient, nil, nil, cfg)
	m.width = 120
	m.height = 40
	m.loading = false
	m.allRows = buildRows(result.PRs, result.Errors)
	m.applyFilters()
	m.table = m.rebuildTable()

	view := m.View()
	if !strings.Contains(view, "PR Wrangler") {
		t.Fatalf("expected dashboard title to render")
	}
	if !strings.Contains(view, "Fix flaky test") {
		t.Fatalf("expected PR title to render in view, got:\n%s", view)
	}
	if !strings.Contains(view, "widgets") {
		t.Fatalf("expected repo name to render in view, got:\n%s", view)
	}
}
