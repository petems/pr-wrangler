package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// CommandRunner abstracts shell command execution for testability
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner is the production implementation of CommandRunner
type ExecRunner struct{}

func (e *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		// Include stderr in the error message if available
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
	}
	return out, err
}

// GHClient interacts with the gh CLI
type GHClient struct {
	Runner CommandRunner
}

func NewGHClient() *GHClient {
	return &GHClient{Runner: &ExecRunner{}}
}

// prViewFields are the fields requested from gh pr view (works with PR URLs, no repo context needed)
var prViewFields = "number,title,state,isDraft,url,headRefName,headRefOid,author,reviewDecision,statusCheckRollup,labels,mergeable"

// searchResult holds the minimal data from the GitHub Search API
type searchResult struct {
	URL               string
	Number            int
	RepoNameWithOwner string
	State             string
	IsDraft           bool
	Labels            []string
}

// apiSearchResponse mirrors the GitHub Search API response for issues/PRs
type apiSearchResponse struct {
	Items []struct {
		HTMLURL       string `json:"html_url"`
		Number        int    `json:"number"`
		State         string `json:"state"`
		Draft         bool   `json:"draft"`
		RepositoryURL string `json:"repository_url"`
		Labels        []struct {
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"items"`
}

// repoNameFromAPIURL extracts "Owner/Repo" from "https://api.github.com/repos/Owner/Repo"
func repoNameFromAPIURL(apiURL string) string {
	const prefix = "https://api.github.com/repos/"
	if strings.HasPrefix(apiURL, prefix) {
		return strings.TrimPrefix(apiURL, prefix)
	}
	return ""
}

// searchPRs uses the GitHub Search API to find PRs matching the query.
// This works without being in a git repository.
func (c *GHClient) searchPRs(ctx context.Context, query string) ([]searchResult, error) {
	fullQuery := query + " is:pr"
	out, err := c.Runner.Run(ctx, "gh", "api", "search/issues",
		"-X", "GET",
		"-f", "q="+fullQuery,
		"-f", "per_page=50",
		"-f", "sort=updated",
		"-f", "order=desc",
	)
	if err != nil {
		return nil, fmt.Errorf("searching PRs: %w", err)
	}

	var resp apiSearchResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}

	results := make([]searchResult, len(resp.Items))
	for i, item := range resp.Items {
		results[i] = searchResult{
			URL:               item.HTMLURL,
			Number:            item.Number,
			RepoNameWithOwner: repoNameFromAPIURL(item.RepositoryURL),
			State:             item.State,
			IsDraft:           item.Draft,
		}
		for _, l := range item.Labels {
			results[i].Labels = append(results[i].Labels, l.Name)
		}
	}
	return results, nil
}

// ParsePRViewOutput parses the JSON output of gh pr view --json
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

// FetchPRs uses the GitHub Search API to discover PRs, then fetches full details
// for each via gh pr view. Works from any directory (no git repo needed).
func (c *GHClient) FetchPRs(ctx context.Context, query string) ([]PR, error) {
	if query == "" {
		query = "author:@me is:open"
	}

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
	var wg sync.WaitGroup

	for i, sr := range results {
		wg.Add(1)
		go func(idx int, sr searchResult) {
			defer wg.Done()

			out, err := c.Runner.Run(ctx, "gh", "pr", "view", sr.URL,
				"--json", prViewFields,
			)
			if err != nil {
				ch <- indexedPR{idx: idx, err: fmt.Errorf("fetching PR %s: %w", sr.URL, err)}
				return
			}

			pr, err := ParsePRViewOutput(out)
			if err != nil {
				ch <- indexedPR{idx: idx, err: fmt.Errorf("parsing PR %s: %w", sr.URL, err)}
				return
			}
			pr.RepoNameWithOwner = sr.RepoNameWithOwner

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
