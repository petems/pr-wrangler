package tui

import (
	"context"

	"github.com/petems/pr-wrangler/internal/github"
)

type MockPRFetcher struct {
	PRs           []github.PR
	Errors        []github.SAMLErrorEntry
	Err           error
	ProgressSteps [][2]int
	Queries       []string
}

func (f *MockPRFetcher) FetchPRs(_ context.Context, query string, progress func(done, total int)) (github.FetchResult, error) {
	f.Queries = append(f.Queries, query)
	if progress != nil {
		for _, step := range f.ProgressSteps {
			progress(step[0], step[1])
		}
	}
	if f.Err != nil {
		return github.FetchResult{}, f.Err
	}
	return github.FetchResult{PRs: f.PRs, Errors: f.Errors}, nil
}
