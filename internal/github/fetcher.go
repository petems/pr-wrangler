package github

import "context"

// PRFetcher fetches pull requests for the TUI.
type PRFetcher interface {
	FetchPRs(ctx context.Context, query string, progress func(done, total int)) (FetchResult, error)
}
