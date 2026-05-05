package tui

import (
	"context"

	"github.com/petems/pr-wrangler/internal/github"
)

type MockPRFetcher struct {
	PRs    []github.PR
	Errors []github.SAMLErrorEntry
	Err    error

	Queries       []string
	ProgressCalls [][2]int
}

func (f *MockPRFetcher) FetchPRs(_ context.Context, query string, progress func(done, total int)) (github.FetchResult, error) {
	f.Queries = append(f.Queries, query)

	if f.Err != nil {
		return github.FetchResult{PRs: f.PRs, Errors: f.Errors}, f.Err
	}

	total := len(f.PRs) + len(f.Errors)
	if progress != nil {
		progress(0, total)
		f.ProgressCalls = append(f.ProgressCalls, [2]int{0, total})
		for done := 1; done <= total; done++ {
			progress(done, total)
			f.ProgressCalls = append(f.ProgressCalls, [2]int{done, total})
		}
	}

	return github.FetchResult{PRs: f.PRs, Errors: f.Errors}, nil
}
