package github

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeFetcher struct {
	mu    sync.Mutex
	calls int
	res   FetchResult
}

func (f *fakeFetcher) FetchPRs(ctx context.Context, query string, progress func(done, total int)) (FetchResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if progress != nil {
		progress(0, 0)
	}
	return f.res, nil
}

func (f *fakeFetcher) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestCachedClientCacheBehavior(t *testing.T) {
	tests := []struct {
		name                string
		ttl                 time.Duration
		firstResult         FetchResult
		secondResult        FetchResult
		bypassSecondFetch   bool
		beforeSecondFetch   func()
		recordProgress      bool
		wantSecondFromCache bool
		wantCalls           int
		wantTitle           string
		wantProgress        [][2]int
	}{
		{
			name:                "returns cached result",
			ttl:                 5 * time.Minute,
			firstResult:         FetchResult{PRs: []PR{{Number: 1, Title: "a"}}},
			wantSecondFromCache: true,
			wantCalls:           1,
			wantTitle:           "a",
		},
		{
			name:                "cache hit reports initial and complete progress",
			ttl:                 5 * time.Minute,
			firstResult:         FetchResult{PRs: []PR{{Number: 1}}, Errors: []SAMLErrorEntry{{Index: 1}}},
			recordProgress:      true,
			wantSecondFromCache: true,
			wantCalls:           1,
			wantProgress:        [][2]int{{0, 2}, {2, 2}},
		},
		{
			name:                "bypasses cache on explicit refresh",
			ttl:                 5 * time.Minute,
			firstResult:         FetchResult{PRs: []PR{{Number: 1, Title: "v1"}}},
			secondResult:        FetchResult{PRs: []PR{{Number: 1, Title: "v2"}}},
			bypassSecondFetch:   true,
			wantSecondFromCache: false,
			wantCalls:           2,
			wantTitle:           "v2",
		},
		{
			name:                "skips cache when expired",
			ttl:                 5 * time.Millisecond,
			firstResult:         FetchResult{PRs: []PR{{Number: 1}}},
			beforeSecondFetch:   func() { time.Sleep(10 * time.Millisecond) },
			wantSecondFromCache: false,
			wantCalls:           2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &fakeFetcher{res: tt.firstResult}
			client := NewCachedClient(fetcher, tt.ttl)
			ctx := context.Background()

			first, err := client.FetchPRsCached(ctx, "", false, nil)
			if err != nil {
				t.Fatalf("first fetch: %v", err)
			}
			if first.FromCache {
				t.Fatalf("expected first fetch not from cache")
			}

			if tt.beforeSecondFetch != nil {
				tt.beforeSecondFetch()
			}
			if tt.secondResult.PRs != nil || tt.secondResult.Errors != nil {
				fetcher.mu.Lock()
				fetcher.res = tt.secondResult
				fetcher.mu.Unlock()
			}

			var steps [][2]int
			var progress func(done, total int)
			if tt.recordProgress {
				progress = func(done, total int) {
					steps = append(steps, [2]int{done, total})
				}
			}
			second, err := client.FetchPRsCached(ctx, "", tt.bypassSecondFetch, progress)
			if err != nil {
				t.Fatalf("second fetch: %v", err)
			}
			if second.FromCache != tt.wantSecondFromCache {
				t.Fatalf("second FromCache = %v, want %v", second.FromCache, tt.wantSecondFromCache)
			}
			if tt.wantTitle != "" && (len(second.PRs) != 1 || second.PRs[0].Title != tt.wantTitle) {
				t.Fatalf("second PRs = %+v, want title %q", second.PRs, tt.wantTitle)
			}
			if fetcher.CallCount() != tt.wantCalls {
				t.Fatalf("fetch calls = %d, want %d", fetcher.CallCount(), tt.wantCalls)
			}
			if len(steps) != len(tt.wantProgress) {
				t.Fatalf("progress steps = %v, want %v", steps, tt.wantProgress)
			}
			for i := range tt.wantProgress {
				if steps[i] != tt.wantProgress[i] {
					t.Fatalf("progress steps = %v, want %v", steps, tt.wantProgress)
				}
			}
		})
	}
}
