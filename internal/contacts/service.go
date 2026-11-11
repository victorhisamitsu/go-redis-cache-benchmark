package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/cache"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/metrics"
)

// Fetcher loads a contact from the source of truth (the external API).
type Fetcher interface {
	FetchContact(ctx context.Context, id string) (Contact, error)
}

// Service serves contacts using the cache-aside pattern:
//
//  1. Try the cache.
//  2. On a miss, call the external API — but only once per key at a time
//     (singleflight), so a burst of requests for the same contact does not
//     turn into a burst of paid API calls.
//  3. Store the result in the cache with a TTL.
//
// If the cache is unavailable, the service degrades gracefully and keeps
// serving from the external API instead of failing the request.
type Service struct {
	fetcher Fetcher
	cache   cache.Cache // nil disables caching
	ttl     time.Duration
	group   singleflight.Group
	metrics *metrics.Counters
	log     *slog.Logger
}

// NewService wires the service. Pass a nil cache to disable caching.
func NewService(f Fetcher, c cache.Cache, ttl time.Duration, m *metrics.Counters, log *slog.Logger) *Service {
	return &Service{fetcher: f, cache: c, ttl: ttl, metrics: m, log: log}
}

func cacheKey(id string) string { return "contact:v1:" + id }

// Get returns the contact with the given id.
func (s *Service) Get(ctx context.Context, id string) (Contact, error) {
	s.metrics.Requests.Add(1)

	if s.cache == nil {
		s.metrics.UpstreamCalls.Add(1)
		return s.fetcher.FetchContact(ctx, id)
	}

	if c, ok := s.fromCache(ctx, id); ok {
		return c, nil
	}

	v, err, shared := s.group.Do(id, func() (any, error) {
		// Re-check: another request may have filled the cache meanwhile.
		if c, ok := s.fromCache(ctx, id); ok {
			return c, nil
		}
		s.metrics.UpstreamCalls.Add(1)
		c, err := s.fetcher.FetchContact(ctx, id)
		if err != nil {
			return Contact{}, err
		}
		s.store(ctx, id, c)
		return c, nil
	})
	if shared {
		s.metrics.Coalesced.Add(1)
	}
	if err != nil {
		return Contact{}, err
	}
	return v.(Contact), nil
}

func (s *Service) fromCache(ctx context.Context, id string) (Contact, bool) {
	b, err := s.cache.Get(ctx, cacheKey(id))
	switch {
	case err == nil:
	case errors.Is(err, cache.ErrMiss):
		s.metrics.CacheMisses.Add(1)
		return Contact{}, false
	default:
		s.metrics.CacheErrors.Add(1)
		s.log.Warn("cache get failed, falling back to upstream", "id", id, "err", err)
		return Contact{}, false
	}
	var c Contact
	if err := json.Unmarshal(b, &c); err != nil {
		s.metrics.CacheErrors.Add(1)
		return Contact{}, false
	}
	s.metrics.CacheHits.Add(1)
	return c, true
}

func (s *Service) store(ctx context.Context, id string, c Contact) {
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	if err := s.cache.Set(ctx, cacheKey(id), b, s.ttl); err != nil {
		s.metrics.CacheErrors.Add(1)
		s.log.Warn("cache set failed", "id", id, "err", err)
	}
}
