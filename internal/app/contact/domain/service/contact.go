// Package service holds the contact business rules.
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/metrics"
)

// ContactProvider loads contacts from the source of truth (the external CRM).
type ContactProvider interface {
	FetchContact(ctx context.Context, id string) (model.Contact, error)
}

// ContactCache stores contacts for fast reads.
type ContactCache interface {
	Get(ctx context.Context, id string) (c model.Contact, found bool, err error)
	Set(ctx context.Context, c model.Contact, ttl time.Duration) error
}

// ContactService is the domain entry point used by use cases.
type ContactService interface {
	GetByID(ctx context.Context, id string) (model.Contact, error)
}

// contactService serves contacts using the cache-aside pattern:
//
//  1. Try the cache.
//  2. On a miss, call the external CRM — only once per key at a time
//     (singleflight), so a burst of requests for the same contact does not
//     turn into a burst of paid API calls.
//  3. Store the result with a TTL.
//
// If the cache is unavailable, it degrades gracefully and keeps serving from
// the external CRM instead of failing the request.
type contactService struct {
	provider ContactProvider
	cache    ContactCache // nil disables caching
	ttl      time.Duration
	group    singleflight.Group
	metrics  *metrics.Counters
	log      *slog.Logger
}

// NewContactService wires the service. Pass a nil cache to disable caching.
func NewContactService(p ContactProvider, c ContactCache, ttl time.Duration, m *metrics.Counters, log *slog.Logger) ContactService {
	return &contactService{
		provider: p,
		cache:    c,
		ttl:      ttl,
		metrics:  m,
		log:      log.With("component", "contact_service"),
	}
}

// GetByID returns the contact with the given id.
func (s *contactService) GetByID(ctx context.Context, id string) (model.Contact, error) {
	s.metrics.Requests.Add(1)

	if s.cache == nil {
		return s.fetch(ctx, id)
	}

	if c, ok := s.fromCache(ctx, id); ok {
		return c, nil
	}

	v, err, shared := s.group.Do(id, func() (any, error) {
		// Re-check: another request may have filled the cache meanwhile.
		if c, ok := s.fromCache(ctx, id); ok {
			return c, nil
		}
		c, err := s.fetch(ctx, id)
		if err != nil {
			return model.Contact{}, err
		}
		s.store(ctx, c)
		return c, nil
	})
	if shared {
		s.metrics.Coalesced.Add(1)
		s.log.DebugContext(ctx, "request coalesced by singleflight", "contact_id", id)
	}
	if err != nil {
		return model.Contact{}, err
	}
	return v.(model.Contact), nil
}

func (s *contactService) fetch(ctx context.Context, id string) (model.Contact, error) {
	s.metrics.UpstreamCalls.Add(1)
	start := time.Now()
	c, err := s.provider.FetchContact(ctx, id)
	elapsed := time.Since(start)

	switch {
	case errors.Is(err, model.ErrContactNotFound):
		s.log.InfoContext(ctx, "contact not found in crm", "contact_id", id, "duration_ms", elapsed.Milliseconds())
	case err != nil:
		s.log.ErrorContext(ctx, "crm fetch failed", "contact_id", id, "duration_ms", elapsed.Milliseconds(), "err", err)
	default:
		s.log.InfoContext(ctx, "contact fetched from crm", "contact_id", id, "duration_ms", elapsed.Milliseconds())
	}
	return c, err
}

func (s *contactService) fromCache(ctx context.Context, id string) (model.Contact, bool) {
	c, found, err := s.cache.Get(ctx, id)
	switch {
	case err != nil:
		s.metrics.CacheErrors.Add(1)
		s.log.WarnContext(ctx, "cache get failed, falling back to crm", "contact_id", id, "err", err)
		return model.Contact{}, false
	case !found:
		s.metrics.CacheMisses.Add(1)
		s.log.DebugContext(ctx, "cache miss", "contact_id", id)
		return model.Contact{}, false
	default:
		s.metrics.CacheHits.Add(1)
		s.log.DebugContext(ctx, "cache hit", "contact_id", id)
		return c, true
	}
}

func (s *contactService) store(ctx context.Context, c model.Contact) {
	if err := s.cache.Set(ctx, c, s.ttl); err != nil {
		s.metrics.CacheErrors.Add(1)
		s.log.WarnContext(ctx, "cache set failed", "contact_id", c.ID, "err", err)
	}
}
