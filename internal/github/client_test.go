package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// mockRunner implements CommandRunner for testing.
// It returns different output based on the command being run.
type mockRunner struct {
	mu sync.Mutex
	// calls records each invocation for assertion
	calls [][]string
	// handlers maps a command prefix to its response
	handlers map[string]mockResponse
	// fallback is used when no handler matches
	fallback mockResponse
}

type mockResponse struct {
	output []byte
	err    error
}

func newMockRunner() *mockRunner {
	return &mockRunner{handlers: make(map[string]mockResponse)}
}

func (m *mockRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)

	m.mu.Lock()
	m.calls = append(m.calls, call)
	defer m.mu.Unlock()

	key := strings.Join(call, " ")
	for prefix, resp := range m.handlers {
		if strings.HasPrefix(key, prefix) {
			return resp.output, resp.err
		}
	}
	return m.fallback.output, m.fallback.err
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling test JSON: %v", err)
	}
	return b
}

// searchAPIResponse builds a GitHub Search API response
func searchAPIResponse(items ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"items": items}
}

// searchItem builds a single item in the search API response
func searchItem(number int, repo, htmlURL string) map[string]interface{} {
	return map[string]interface{}{
		"html_url":       htmlURL,
		"number":         number,
		"state":          "open",
		"draft":          false,
		"repository_url": "https://api.github.com/repos/" + repo,
		"labels":         []interface{}{},
	}
}

// prViewJSON builds a realistic gh pr view --json response
func prViewJSON(overrides map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"number":            42,
		"title":             "Fix widget alignment",
		"url":               "https://github.com/org/repo/pull/42",
		"author":            map[string]interface{}{"login": "alice"},
		"headRefName":       "fix-widgets",
		"headRefOid":        "abc123",
		"state":             "OPEN",
		"isDraft":           false,
		"mergeable":         "MERGEABLE",
		"labels":            []interface{}{},
		"reviewDecision":    "APPROVED",
		"statusCheckRollup": []interface{}{},
	}
	for k, v := range overrides {
		base[k] = v
	}
	return base
}

// --- ParsePRViewOutput tests ---

func TestParsePRViewOutput_BasicFields(t *testing.T) {
	data := mustJSON(t, prViewJSON(map[string]interface{}{
		"number":         99,
		"title":          "Add tests",
		"url":            "https://github.com/org/repo/pull/99",
		"headRefName":    "add-tests",
		"headRefOid":     "def456",
		"state":          "OPEN",
		"isDraft":        true,
		"mergeable":      "CONFLICTING",
		"reviewDecision": "CHANGES_REQUESTED",
	}))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 99 {
		t.Errorf("Number: got %d, want 99", pr.Number)
	}
	if pr.Title != "Add tests" {
		t.Errorf("Title: got %q", pr.Title)
	}
	if pr.HeadRefName != "add-tests" {
		t.Errorf("HeadRefName: got %q", pr.HeadRefName)
	}
	if pr.HeadCommitOID != "def456" {
		t.Errorf("HeadCommitOID: got %q", pr.HeadCommitOID)
	}
	if pr.State != PRStateOpen {
		t.Errorf("State: got %q", pr.State)
	}
	if !pr.IsDraft {
		t.Error("IsDraft: expected true")
	}
	if pr.Mergeable != "CONFLICTING" {
		t.Errorf("Mergeable: got %q", pr.Mergeable)
	}
	if pr.ReviewDecision != ReviewDecisionChangesRequested {
		t.Errorf("ReviewDecision: got %q", pr.ReviewDecision)
	}
}

func TestParsePRViewOutput_Author(t *testing.T) {
	data := mustJSON(t, prViewJSON(nil))
	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Author != "alice" {
		t.Errorf("expected Author='alice', got %q", pr.Author)
	}
}

func TestParsePRViewOutput_Labels(t *testing.T) {
	data := mustJSON(t, prViewJSON(map[string]interface{}{
		"labels": []interface{}{
			map[string]interface{}{"name": "bug"},
			map[string]interface{}{"name": "priority:high"},
		},
	}))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(pr.Labels))
	}
	if pr.Labels[0] != "bug" || pr.Labels[1] != "priority:high" {
		t.Errorf("labels: got %v", pr.Labels)
	}
}

func TestParsePRViewOutput_StatusCheckRollup_AllSuccess(t *testing.T) {
	data := mustJSON(t, prViewJSON(map[string]interface{}{
		"statusCheckRollup": []interface{}{
			map[string]interface{}{"status": "COMPLETED", "conclusion": "SUCCESS"},
			map[string]interface{}{"status": "COMPLETED", "conclusion": "NEUTRAL"},
			map[string]interface{}{"status": "COMPLETED", "conclusion": "SKIPPED"},
		},
	}))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.LatestCheckState != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %q", pr.LatestCheckState)
	}
}

func TestParsePRViewOutput_StatusCheckRollup_Failure(t *testing.T) {
	data := mustJSON(t, prViewJSON(map[string]interface{}{
		"statusCheckRollup": []interface{}{
			map[string]interface{}{"status": "COMPLETED", "conclusion": "SUCCESS"},
			map[string]interface{}{"status": "COMPLETED", "conclusion": "FAILURE"},
		},
	}))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.LatestCheckState != "FAILURE" {
		t.Errorf("expected FAILURE, got %q", pr.LatestCheckState)
	}
}

func TestParsePRViewOutput_StatusCheckRollup_Pending(t *testing.T) {
	data := mustJSON(t, prViewJSON(map[string]interface{}{
		"statusCheckRollup": []interface{}{
			map[string]interface{}{"status": "COMPLETED", "conclusion": "SUCCESS"},
			map[string]interface{}{"status": "IN_PROGRESS", "conclusion": ""},
		},
	}))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.LatestCheckState != "PENDING" {
		t.Errorf("expected PENDING, got %q", pr.LatestCheckState)
	}
}

func TestParsePRViewOutput_StatusCheckRollup_Empty(t *testing.T) {
	data := mustJSON(t, prViewJSON(nil))

	pr, err := ParsePRViewOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.LatestCheckState != "" {
		t.Errorf("expected empty LatestCheckState, got %q", pr.LatestCheckState)
	}
}

func TestParsePRViewOutput_InvalidJSON(t *testing.T) {
	_, err := ParsePRViewOutput([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- repoNameFromAPIURL tests ---

func TestRepoNameFromAPIURL(t *testing.T) {
	got := repoNameFromAPIURL("https://api.github.com/repos/org/repo")
	if got != "org/repo" {
		t.Errorf("expected 'org/repo', got %q", got)
	}
}

func TestRepoNameFromAPIURL_InvalidPrefix(t *testing.T) {
	got := repoNameFromAPIURL("https://example.com/something")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- FetchPRs integration tests (mocked) ---

func TestFetchPRs_EndToEnd(t *testing.T) {
	runner := newMockRunner()

	// Mock search API response
	searchResp := searchAPIResponse(
		searchItem(42, "org/repo", "https://github.com/org/repo/pull/42"),
	)
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchResp)}

	// Mock pr view response
	viewResp := prViewJSON(map[string]interface{}{
		"number": 42,
		"title":  "Fix widget alignment",
	})
	runner.handlers["gh pr view"] = mockResponse{output: mustJSON(t, viewResp)}

	client := &GHClient{Runner: runner}
	prs, err := client.FetchPRs(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 42 {
		t.Errorf("Number: got %d, want 42", prs[0].Number)
	}
	if prs[0].Title != "Fix widget alignment" {
		t.Errorf("Title: got %q", prs[0].Title)
	}
	if prs[0].RepoNameWithOwner != "org/repo" {
		t.Errorf("RepoNameWithOwner: got %q, want 'org/repo'", prs[0].RepoNameWithOwner)
	}
	if prs[0].Author != "alice" {
		t.Errorf("Author: got %q, want 'alice'", prs[0].Author)
	}
}

func TestFetchPRs_MultiplePRs(t *testing.T) {
	runner := newMockRunner()

	searchResp := searchAPIResponse(
		searchItem(1, "org/repo-a", "https://github.com/org/repo-a/pull/1"),
		searchItem(2, "org/repo-b", "https://github.com/org/repo-b/pull/2"),
	)
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchResp)}

	// Both pr view calls will get this response; the number will be overridden
	// but for this test we just check we get 2 PRs back
	runner.handlers["gh pr view"] = mockResponse{output: mustJSON(t, prViewJSON(nil))}

	client := &GHClient{Runner: runner}
	prs, err := client.FetchPRs(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
}

func TestFetchPRs_EmptySearchResults(t *testing.T) {
	runner := newMockRunner()
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchAPIResponse())}

	client := &GHClient{Runner: runner}
	prs, err := client.FetchPRs(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Fatalf("expected 0 PRs, got %d", len(prs))
	}
}

func TestFetchPRs_DefaultQueryIsAuthorMe(t *testing.T) {
	runner := newMockRunner()
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchAPIResponse())}

	client := &GHClient{Runner: runner}
	_, _ = client.FetchPRs(context.Background(), "")

	// Verify the search call includes "author:@me is:open"
	if len(runner.calls) == 0 {
		t.Fatal("expected at least one call")
	}
	searchCall := strings.Join(runner.calls[0], " ")
	if !strings.Contains(searchCall, "author:@me is:open is:pr") {
		t.Errorf("expected default query with 'author:@me is:open is:pr', got: %s", searchCall)
	}
}

func TestFetchPRs_CustomQuery(t *testing.T) {
	runner := newMockRunner()
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchAPIResponse())}

	client := &GHClient{Runner: runner}
	_, _ = client.FetchPRs(context.Background(), "author:bob is:open")

	searchCall := strings.Join(runner.calls[0], " ")
	if !strings.Contains(searchCall, "author:bob is:open is:pr") {
		t.Errorf("expected custom query in search call, got: %s", searchCall)
	}
}

func TestFetchPRs_SearchAPIError(t *testing.T) {
	runner := newMockRunner()
	runner.handlers["gh api search/issues"] = mockResponse{err: fmt.Errorf("API error")}

	client := &GHClient{Runner: runner}
	_, err := client.FetchPRs(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "searching PRs") {
		t.Errorf("expected 'searching PRs' in error, got: %v", err)
	}
}

func TestFetchPRs_PRViewError(t *testing.T) {
	runner := newMockRunner()

	searchResp := searchAPIResponse(
		searchItem(42, "org/repo", "https://github.com/org/repo/pull/42"),
	)
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchResp)}
	runner.handlers["gh pr view"] = mockResponse{err: fmt.Errorf("view failed")}

	client := &GHClient{Runner: runner}
	_, err := client.FetchPRs(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching PR") {
		t.Errorf("expected 'fetching PR' in error, got: %v", err)
	}
}

func TestFetchPRs_SearchInvalidJSON(t *testing.T) {
	runner := newMockRunner()
	runner.handlers["gh api search/issues"] = mockResponse{output: []byte("not json")}

	client := &GHClient{Runner: runner}
	_, err := client.FetchPRs(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchPRs_SetsRepoNameFromSearch(t *testing.T) {
	runner := newMockRunner()

	searchResp := searchAPIResponse(
		searchItem(42, "myorg/myrepo", "https://github.com/myorg/myrepo/pull/42"),
	)
	runner.handlers["gh api search/issues"] = mockResponse{output: mustJSON(t, searchResp)}
	runner.handlers["gh pr view"] = mockResponse{output: mustJSON(t, prViewJSON(nil))}

	client := &GHClient{Runner: runner}
	prs, err := client.FetchPRs(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prs[0].RepoNameWithOwner != "myorg/myrepo" {
		t.Errorf("RepoNameWithOwner: got %q, want 'myorg/myrepo'", prs[0].RepoNameWithOwner)
	}
}
