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

func TestCachedClientCacheHitReportsInitialAndCompleteProgress(t *testing.T) {
	fetcher := &fakeFetcher{res: FetchResult{PRs: []PR{{Number: 1}}, Errors: []SAMLErrorEntry{{Index: 1}}}}
	client := NewCachedClient(fetcher, 5*time.Minute)

	ctx := context.Background()
	if _, err := client.FetchPRsCached(ctx, "", false, nil); err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	var steps [][2]int
	_, err := client.FetchPRsCached(ctx, "", false, func(done, total int) {
		steps = append(steps, [2]int{done, total})
	})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	want := [][2]int{{0, 2}, {2, 2}}
	if len(steps) != len(want) {
		t.Fatalf("progress steps = %v, want %v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("progress steps = %v, want %v", steps, want)
		}
	}
}

func TestCachedClientBypassesCacheOnExplicitRefresh(t *testing.T) {
	fetcher := &fakeFetcher{res: FetchResult{PRs: []PR{{Number: 1, Title: "v1"}}}}
	client := NewCachedClient(fetcher, 5*time.Minute)

	ctx := context.Background()
	if _, err := client.FetchPRsCached(ctx, "", false, nil); err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	fetcher.mu.Lock()
	fetcher.res = FetchResult{PRs: []PR{{Number: 1, Title: "v2"}}}
	fetcher.mu.Unlock()

	res, err := client.FetchPRsCached(ctx, "", true /* bypassCache */, nil)
	if err != nil {
		t.Fatalf("bypass fetch: %v", err)
	}
	if res.FromCache {
		t.Fatalf("expected explicit refresh to bypass cache, got FromCache=true")
	}
	if len(res.PRs) != 1 || res.PRs[0].Title != "v2" {
		t.Fatalf("expected fresh result, got %+v", res.PRs)
	}
	if fetcher.CallCount() != 2 {
		t.Fatalf("expected 2 fetch calls (initial + bypass), got %d", fetcher.CallCount())
	}
}

func TestCachedClientSkipsCacheWhenExpired(t *testing.T) {
	fetcher := &fakeFetcher{res: FetchResult{PRs: []PR{{Number: 1}}}}
	client := NewCachedClient(fetcher, 5*time.Millisecond)

	ctx := context.Background()
	_, err := client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err = client.FetchPRsCached(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if fetcher.CallCount() != 2 {
		t.Fatalf("expected 2 fetch calls, got %d", fetcher.CallCount())
	}
}
