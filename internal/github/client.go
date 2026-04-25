package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	gh "github.com/google/go-github/v72/github"
)

const maxConcurrentPRDetails = 8

// GHClient interacts with the GitHub API using go-github.
type GHClient struct {
	client *gh.Client
	token  string
}

// NewGHClient creates a GHClient with a token resolved from the standard chain.
func NewGHClient() (*GHClient, error) {
	token, source, err := ResolveToken()
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("no GitHub token found. Run 'pr-wrangler auth login' to authenticate, or set GITHUB_TOKEN/GH_TOKEN env var")
	}
	_ = source // Could log this later

	return NewGHClientWithToken(token), nil
}

// NewGHClientWithToken creates a GHClient with an explicit token.
func NewGHClientWithToken(token string) *GHClient {
	return &GHClient{
		client: gh.NewClient(nil).WithAuthToken(token),
		token:  token,
	}
}

// Token returns the token used by this client (for passing to subprocesses).
func (c *GHClient) Token() string {
	return c.token
}

// Client returns the underlying go-github client.
func (c *GHClient) Client() *gh.Client {
	return c.client
}

// searchResult holds the minimal data from the GitHub Search API
type searchResult struct {
	URL               string
	Number            int
	RepoNameWithOwner string
	State             string
	IsDraft           bool
	Labels            []string
}

// searchPRs uses the GitHub Search API via go-github.
func (c *GHClient) searchPRs(ctx context.Context, query string) ([]searchResult, error) {
	fullQuery := query + " is:pr"

	opts := &gh.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: gh.ListOptions{
			PerPage: 50,
		},
	}

	var results []searchResult
	for {
		issueResult, resp, err := c.client.Search.Issues(ctx, fullQuery, opts)
		if err != nil {
			return nil, fmt.Errorf("searching PRs: %w", err)
		}

		for _, issue := range issueResult.Issues {
			sr := searchResult{
				URL:    issue.GetHTMLURL(),
				Number: issue.GetNumber(),
				State:  issue.GetState(),
			}

			// Extract owner/repo from the repository URL
			if issue.RepositoryURL != nil {
				sr.RepoNameWithOwner = repoNameFromAPIURL(issue.GetRepositoryURL())
			}

			// Pull request details for draft status
			if issue.PullRequestLinks != nil {
				// Search API doesn't directly expose draft; we'll get it in the detail fetch
				sr.IsDraft = false
			}

			for _, l := range issue.Labels {
				sr.Labels = append(sr.Labels, l.GetName())
			}

			results = append(results, sr)
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return results, nil
}

// fetchPRDetail fetches full PR details using go-github.
func (c *GHClient) fetchPRDetail(ctx context.Context, owner, repo string, number int) (PR, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return PR{}, fmt.Errorf("fetching PR #%d: %w", number, err)
	}

	result := PR{
		Number:            pr.GetNumber(),
		Title:             pr.GetTitle(),
		URL:               pr.GetHTMLURL(),
		Author:            pr.GetUser().GetLogin(),
		HeadRefName:       pr.GetHead().GetRef(),
		HeadCommitOID:     pr.GetHead().GetSHA(),
		State:             PRState(strings.ToUpper(pr.GetState())),
		MergedAt:          pr.MergedAt.GetTime(),
		IsDraft:           pr.GetDraft(),
		Mergeable:         strings.ToUpper(pr.GetMergeableState()),
		RepoNameWithOwner: owner + "/" + repo,
	}

	// Distinguish merged PRs from closed ones
	if pr.GetMerged() {
		result.State = PRStateMerged
	}

	for _, l := range pr.Labels {
		result.Labels = append(result.Labels, l.GetName())
	}

	// Fetch reviews for review decision
	reviewOpts := &gh.ListOptions{PerPage: 100}
	for {
		reviews, resp, err := c.client.PullRequests.ListReviews(ctx, owner, repo, number, reviewOpts)
		if err != nil {
			break
		}
		for _, rev := range reviews {
			result.Reviews = append(result.Reviews, Review{
				Author: rev.GetUser().GetLogin(),
				State:  rev.GetState(),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		reviewOpts.Page = resp.NextPage
	}
	result.ReviewDecision = deriveReviewDecision(result.Reviews)

	// Fetch check runs for CI status
	ref := pr.GetHead().GetSHA()
	if ref != "" {
		checkOpts := &gh.ListCheckRunsOptions{
			ListOptions: gh.ListOptions{PerPage: 100},
		}
		for {
			checkResult, resp, err := c.client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, checkOpts)
			if err != nil || checkResult == nil {
				break
			}
			for _, check := range checkResult.CheckRuns {
				result.StatusChecks = append(result.StatusChecks, StatusCheck{
					Name:       check.GetName(),
					Status:     check.GetStatus(),
					Conclusion: CIConclusion(strings.ToUpper(check.GetConclusion())),
				})
			}
			if resp == nil || resp.NextPage == 0 {
				break
			}
			checkOpts.Page = resp.NextPage
		}

		// Fetch commit statuses (classic status API)
		statusOpts := &gh.ListOptions{PerPage: 100}
		for {
			combinedStatus, resp, err := c.client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, statusOpts)
			if err != nil || combinedStatus == nil {
				break
			}
			for _, status := range combinedStatus.Statuses {
				result.StatusChecks = append(result.StatusChecks, statusCheckFromClassicStatus(status))
			}
			if resp == nil || resp.NextPage == 0 {
				break
			}
			statusOpts.Page = resp.NextPage
		}

		result.LatestCheckState = deriveCheckState(result.StatusChecks)
	}

	// Map mergeable state
	result.Mergeable = mapMergeableState(pr)

	return result, nil
}

func statusCheckFromClassicStatus(status *gh.RepoStatus) StatusCheck {
	state := strings.ToUpper(status.GetState())
	check := StatusCheck{Name: status.GetContext()}
	if state == "PENDING" {
		check.Status = "in_progress"
		return check
	}
	check.Status = "completed"
	check.Conclusion = CIConclusion(state)
	return check
}

// deriveReviewDecision determines the overall review decision from individual reviews.
func deriveReviewDecision(reviews []Review) ReviewDecision {
	if len(reviews) == 0 {
		return ReviewDecisionReviewRequired
	}

	// Track latest review per author
	latest := make(map[string]string)
	for _, r := range reviews {
		latest[r.Author] = r.State
	}

	hasApproval := false
	hasChangesRequested := false
	for _, state := range latest {
		switch state {
		case "APPROVED":
			hasApproval = true
		case "CHANGES_REQUESTED":
			hasChangesRequested = true
		}
	}

	if hasChangesRequested {
		return ReviewDecisionChangesRequested
	}
	if hasApproval {
		return ReviewDecisionApproved
	}
	return ReviewDecisionReviewRequired
}

// deriveCheckState computes the overall CI check state.
func deriveCheckState(checks []StatusCheck) string {
	if len(checks) == 0 {
		return ""
	}

	allSuccess := true
	anyFailure := false

	for _, check := range checks {
		if check.Status != "completed" {
			allSuccess = false
			continue
		}
		conclusion := strings.ToUpper(string(check.Conclusion))
		if conclusion != "SUCCESS" && conclusion != "NEUTRAL" && conclusion != "SKIPPED" {
			anyFailure = true
		}
	}

	if anyFailure {
		return "FAILURE"
	}
	if allSuccess {
		return "SUCCESS"
	}
	return "PENDING"
}

// mapMergeableState converts go-github's MergeableState to our format.
func mapMergeableState(pr *gh.PullRequest) string {
	if pr.MergeableState != nil {
		switch strings.ToUpper(*pr.MergeableState) {
		case "DIRTY":
			return string(MergeableConflicting)
		case "CLEAN", "HAS_HOOKS", "UNSTABLE":
			return string(MergeableMergeable)
		default:
			return string(MergeableUnknown)
		}
	}
	if pr.Mergeable != nil {
		if *pr.Mergeable {
			return string(MergeableMergeable)
		}
		return string(MergeableConflicting)
	}
	return string(MergeableUnknown)
}

// repoNameFromAPIURL extracts "Owner/Repo" from "https://api.github.com/repos/Owner/Repo"
func repoNameFromAPIURL(apiURL string) string {
	const prefix = "https://api.github.com/repos/"
	if strings.HasPrefix(apiURL, prefix) {
		return strings.TrimPrefix(apiURL, prefix)
	}
	return ""
}

// splitOwnerRepo splits "owner/repo" into its parts.
func splitOwnerRepo(nwo string) (string, string, error) {
	parts := strings.SplitN(nwo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q (expected owner/repo)", nwo)
	}
	return parts[0], parts[1], nil
}

func prSearchQuery(query string) string {
	if query == "" {
		return "author:@me is:open"
	}
	return query
}

// FetchPRs discovers PRs matching the query and fetches full details for each.
func (c *GHClient) FetchPRs(ctx context.Context, query string) ([]PR, error) {
	query = prSearchQuery(query)

	results, err := c.searchPRs(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	type indexedPR struct {
		idx int
		pr  PR
		err error
	}

	ch := make(chan indexedPR, len(results))
	sem := make(chan struct{}, maxConcurrentPRDetails)
	var wg sync.WaitGroup

	for i, sr := range results {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		wg.Add(1)
		go func(idx int, sr searchResult) {
			defer wg.Done()
			defer func() { <-sem }()

			owner, repo, err := splitOwnerRepo(sr.RepoNameWithOwner)
			if err != nil {
				ch <- indexedPR{idx: idx, err: err}
				return
			}

			pr, err := c.fetchPRDetail(ctx, owner, repo, sr.Number)
			if err != nil {
				ch <- indexedPR{idx: idx, err: err}
				return
			}

			ch <- indexedPR{idx: idx, pr: pr}
		}(i, sr)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	prs := make([]PR, len(results))
	for result := range ch {
		if result.err != nil {
			return nil, result.err
		}
		prs[result.idx] = result.pr
	}

	return prs, nil
}

// ParsePRViewOutput parses the JSON output of gh pr view --json.
// Retained for backward compatibility with tests and any remaining gh CLI usage.
func ParsePRViewOutput(data []byte) (PR, error) {
	var raw struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		HeadRefName string `json:"headRefName"`
		HeadRefOid  string `json:"headRefOid"`
		State       string `json:"state"`
		IsDraft     bool   `json:"isDraft"`
		Mergeable   string `json:"mergeable"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
		ReviewDecision    string `json:"reviewDecision"`
		StatusCheckRollup []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"statusCheckRollup"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return PR{}, fmt.Errorf("parsing PR view output: %w", err)
	}

	pr := PR{
		Number:         raw.Number,
		Title:          raw.Title,
		URL:            raw.URL,
		Author:         raw.Author.Login,
		HeadRefName:    raw.HeadRefName,
		HeadCommitOID:  raw.HeadRefOid,
		State:          PRState(raw.State),
		IsDraft:        raw.IsDraft,
		Mergeable:      raw.Mergeable,
		ReviewDecision: ReviewDecision(raw.ReviewDecision),
	}
	for _, l := range raw.Labels {
		pr.Labels = append(pr.Labels, l.Name)
	}

	// Derive latest check state
	if len(raw.StatusCheckRollup) > 0 {
		allSuccess := true
		anyFailure := false
		for _, check := range raw.StatusCheckRollup {
			if check.Status != "COMPLETED" {
				allSuccess = false
				continue
			}
			if check.Conclusion != "SUCCESS" && check.Conclusion != "NEUTRAL" && check.Conclusion != "SKIPPED" {
				anyFailure = true
			}
		}
		if anyFailure {
			pr.LatestCheckState = "FAILURE"
		} else if allSuccess {
			pr.LatestCheckState = "SUCCESS"
		} else {
			pr.LatestCheckState = "PENDING"
		}
	}

	return pr, nil
}
