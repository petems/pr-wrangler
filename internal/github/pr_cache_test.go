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

func TestCachedClientReturnsCachedResult(t *testing.T) {
	fetcher := &fakeFetcher{res: FetchResult{PRs: []PR{{Number: 1, Title: "a"}}}}
	client := NewCachedClient(fetcher, 5*time.Minute)

	ctx := context.Background()
	first, err := client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if first.FromCache {
		t.Fatalf("expected first fetch not from cache")
	}

	second, err := client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !second.FromCache {
		t.Fatalf("expected second fetch from cache")
	}
	if fetcher.CallCount() != 1 {
		t.Fatalf("expected 1 fetch call, got %d", fetcher.CallCount())
	}
}

func TestCachedClientSkipsCacheWhenExpired(t *testing.T) {
	fetcher := &fakeFetcher{res: FetchResult{PRs: []PR{{Number: 1}}}}
	client := NewCachedClient(fetcher, 1*time.Nanosecond)

	ctx := context.Background()
	_, err := client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	time.Sleep(2 * time.Nanosecond)
	_, err = client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if fetcher.CallCount() != 2 {
		t.Fatalf("expected 2 fetch calls, got %d", fetcher.CallCount())
	}
}
