package github

import (
	"context"
	"sync"
	"time"
)

type cacheKey string

type prCacheEntry struct {
	result    FetchResult
	fetchedAt time.Time
}

// prCache is a lightweight in-memory cache for PR list fetch results.
// It is scoped to a single pr-wrangler process and intended to speed up
// initial/refresh loads when users repeat the same query.
type prCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]prCacheEntry
}

func newPRCache() *prCache {
	return &prCache{entries: make(map[cacheKey]prCacheEntry)}
}

func (c *prCache) get(key cacheKey, ttl time.Duration) (FetchResult, bool) {
	if ttl <= 0 {
		return FetchResult{}, false
	}

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return FetchResult{}, false
	}
	if time.Since(entry.fetchedAt) > ttl {
		return FetchResult{}, false
	}
	return entry.result, true
}

func (c *prCache) set(key cacheKey, value FetchResult) {
	c.mu.Lock()
	c.entries[key] = prCacheEntry{result: value, fetchedAt: time.Now()}
	c.mu.Unlock()
}

// CachedClient wraps a PR fetcher and caches results by query for a short TTL.
// When a cached result exists, FetchPRsCached returns it immediately and (if
// refresh is true) starts a background refresh which updates the cache when
// complete.
type CachedClient struct {
	fetcher PRFetcher
	cache   *prCache
	ttl     time.Duration
}

func NewCachedClient(fetcher PRFetcher, ttl time.Duration) *CachedClient {
	return &CachedClient{fetcher: fetcher, cache: newPRCache(), ttl: ttl}
}

type CachedFetchResult struct {
	FetchResult
	FromCache bool
}

func (c *CachedClient) FetchPRsCached(ctx context.Context, query string, refresh bool, progress func(done, total int)) (CachedFetchResult, error) {
	key := cacheKey(EffectiveQuery(query))
	if cached, ok := c.cache.get(key, c.ttl); ok {
		if refresh {
			go func() {
				res, err := c.fetcher.FetchPRs(context.Background(), query, nil)
				if err != nil {
					return
				}
				c.cache.set(key, res)
			}()
		}
		if progress != nil {
			progress(len(cached.PRs)+len(cached.Errors), len(cached.PRs)+len(cached.Errors))
		}
		return CachedFetchResult{FetchResult: cached, FromCache: true}, nil
	}

	res, err := c.fetcher.FetchPRs(ctx, query, progress)
	if err != nil {
		return CachedFetchResult{}, err
	}
	c.cache.set(key, res)
	return CachedFetchResult{FetchResult: res}, nil
}
