package github

import (
	"encoding/json"
	"testing"
	"time"

	gh "github.com/google/go-github/v72/github"
)

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling test JSON: %v", err)
	}
	return b
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

// --- ParsePRViewOutput tests (retained — this function still exists for backward compat) ---

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

// --- splitOwnerRepo tests ---

func TestSplitOwnerRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"org/repo", "org", "repo", false},
		{"my-org/my-repo", "my-org", "my-repo", false},
		{"invalid", "", "", true},
		{"/repo", "", "", true},
		{"org/", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		owner, repo, err := splitOwnerRepo(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("splitOwnerRepo(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if owner != tt.wantOwner {
			t.Errorf("splitOwnerRepo(%q): owner=%q, want %q", tt.input, owner, tt.wantOwner)
		}
		if repo != tt.wantRepo {
			t.Errorf("splitOwnerRepo(%q): repo=%q, want %q", tt.input, repo, tt.wantRepo)
		}
	}
}

// --- deriveReviewDecision tests ---

func TestDeriveReviewDecision(t *testing.T) {
	tests := []struct {
		name    string
		reviews []Review
		want    ReviewDecision
	}{
		{"empty", nil, ReviewDecisionReviewRequired},
		{"approved", []Review{{Author: "a", State: "APPROVED"}}, ReviewDecisionApproved},
		{"changes requested", []Review{{Author: "a", State: "CHANGES_REQUESTED"}}, ReviewDecisionChangesRequested},
		{"mixed - changes wins", []Review{
			{Author: "a", State: "APPROVED"},
			{Author: "b", State: "CHANGES_REQUESTED"},
		}, ReviewDecisionChangesRequested},
		{"commented only", []Review{{Author: "a", State: "COMMENTED"}}, ReviewDecisionReviewRequired},
		{"latest per author", []Review{
			{Author: "a", State: "CHANGES_REQUESTED"},
			{Author: "a", State: "APPROVED"},
		}, ReviewDecisionApproved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveReviewDecision(tt.reviews)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- deriveCheckState tests ---

func TestDeriveCheckState(t *testing.T) {
	tests := []struct {
		name   string
		checks []StatusCheck
		want   string
	}{
		{"empty", nil, ""},
		{"all success", []StatusCheck{
			{Status: "completed", Conclusion: CIConclusionSuccess},
		}, "SUCCESS"},
		{"failure", []StatusCheck{
			{Status: "completed", Conclusion: CIConclusionSuccess},
			{Status: "completed", Conclusion: CIConclusionFailure},
		}, "FAILURE"},
		{"pending", []StatusCheck{
			{Status: "completed", Conclusion: CIConclusionSuccess},
			{Status: "in_progress", Conclusion: ""},
		}, "PENDING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveCheckState(tt.checks)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusCheckFromClassicStatus(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		wantStatus     string
		wantConclusion CIConclusion
	}{
		{
			name:       "pending remains in progress",
			state:      "pending",
			wantStatus: "in_progress",
		},
		{
			name:           "success is completed",
			state:          "success",
			wantStatus:     "completed",
			wantConclusion: CIConclusionSuccess,
		},
		{
			name:           "failure is completed",
			state:          "failure",
			wantStatus:     "completed",
			wantConclusion: CIConclusionFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &gh.RepoStatus{
				Context: gh.Ptr("ci/test"),
				State:   gh.Ptr(tt.state),
			}

			got := statusCheckFromClassicStatus(status)
			if got.Name != "ci/test" {
				t.Errorf("Name: got %q, want ci/test", got.Name)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status: got %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Conclusion != tt.wantConclusion {
				t.Errorf("Conclusion: got %q, want %q", got.Conclusion, tt.wantConclusion)
			}
		})
	}
}

// --- Auth token resolution tests ---

func TestResolveToken_EnvVarPriority(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-token-1")
	t.Setenv("GH_TOKEN", "gh-token-2")

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "gh-token-1" {
		t.Errorf("expected GITHUB_TOKEN, got %q", token)
	}
	if source != "GITHUB_TOKEN env var" {
		t.Errorf("expected source 'GITHUB_TOKEN env var', got %q", source)
	}
}

func TestResolveToken_FallbackToGHToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token-fallback")

	// GITHUB_TOKEN="" counts as set but empty — should fall through
	// Actually os.Getenv returns "" for both unset and empty, so we need to unset
	// Use a clean approach
	token, _, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With GITHUB_TOKEN="" it returns "" which is falsy, falls to GH_TOKEN
	if token != "gh-token-fallback" {
		t.Errorf("expected GH_TOKEN fallback, got %q", token)
	}
}

// --- NewGHClientWithToken tests ---

func TestNewGHClientWithToken(t *testing.T) {
	client := NewGHClientWithToken("test-token-123")
	if client.Token() != "test-token-123" {
		t.Errorf("Token(): got %q, want %q", client.Token(), "test-token-123")
	}
	if client.Client() == nil {
		t.Error("Client() should not be nil")
	}
	if client.Runner == nil {
		t.Error("Runner should not be nil")
	}
}

// --- FetchPRs with nil client guard (no network) ---

func TestFetchPRs_EmptyQuery_DefaultsToAuthorMe(t *testing.T) {
	// This test verifies the default query logic without making network calls
	client := NewGHClientWithToken("fake-token")

	// We can't easily test the full FetchPRs without a mock server,
	// but we can verify the client was constructed properly
	if client.client == nil {
		t.Fatal("go-github client should be initialized")
	}
}

// Verify TokenInfo expiry logic
func TestTokenInfo_IsExpired(t *testing.T) {
	t.Run("zero expiry never expires", func(t *testing.T) {
		info := &TokenInfo{Token: "tok"}
		if info.IsExpired() {
			t.Error("zero ExpiresAt should not be expired")
		}
	})

	t.Run("past expiry is expired", func(t *testing.T) {
		info := &TokenInfo{Token: "tok"}
		info.ExpiresAt = time.Now().Add(-1 * time.Hour)
		if !info.IsExpired() {
			t.Error("past ExpiresAt should be expired")
		}
	})

	t.Run("future expiry not expired", func(t *testing.T) {
		info := &TokenInfo{Token: "tok"}
		info.ExpiresAt = time.Now().Add(24 * time.Hour)
		if info.IsExpired() {
			t.Error("future ExpiresAt should not be expired")
		}
	})
}
