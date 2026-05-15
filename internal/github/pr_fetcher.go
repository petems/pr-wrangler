package github

import "context"

// PRFetcher fetches pull requests for a GitHub search query.
type PRFetcher interface {
	FetchPRs(ctx context.Context, query string, progress func(done, total int)) (FetchResult, error)
}
