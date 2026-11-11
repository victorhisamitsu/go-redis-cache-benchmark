package contacts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/cache"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/metrics"
)

type fakeFetcher struct {
	calls atomic.Int64
	delay time.Duration
	err   error
}

func (f *fakeFetcher) FetchContact(_ context.Context, id string) (Contact, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	if f.err != nil {
		return Contact{}, f.err
	}
	return Contact{ID: id, Name: "Contact " + id}, nil
}

type memCache struct {
	mu   sync.Mutex
	data map[string][]byte
	fail bool
}

func newMemCache() *memCache { return &memCache{data: map[string][]byte{}} }

func (m *memCache) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return nil, errors.New("connection refused")
	}
	b, ok := m.data[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return b, nil
}

func (m *memCache) Set(_ context.Context, key string, v []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("connection refused")
	}
	m.data[key] = v
	return nil
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestGet_MissThenHit(t *testing.T) {
	f := &fakeFetcher{}
	m := &metrics.Counters{}
	s := NewService(f, newMemCache(), time.Minute, m, discard)

	for i := 0; i < 3; i++ {
		c, err := s.Get(context.Background(), "42")
		if err != nil {
			t.Fatal(err)
		}
		if c.ID != "42" {
			t.Fatalf("got id %q", c.ID)
		}
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
	if got := m.CacheHits.Load(); got != 2 {
		t.Fatalf("cache hits = %d, want 2", got)
	}
}

func TestGet_ConcurrentMissesAreCoalesced(t *testing.T) {
	f := &fakeFetcher{delay: 50 * time.Millisecond}
	s := NewService(f, newMemCache(), time.Minute, &metrics.Counters{}, discard)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Get(context.Background(), "hot-key"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := f.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cache stampede not prevented)", got)
	}
}

func TestGet_CacheDownFallsBackToUpstream(t *testing.T) {
	f := &fakeFetcher{}
	c := newMemCache()
	c.fail = true
	m := &metrics.Counters{}
	s := NewService(f, c, time.Minute, m, discard)

	if _, err := s.Get(context.Background(), "1"); err != nil {
		t.Fatalf("expected graceful degradation, got %v", err)
	}
	if m.CacheErrors.Load() == 0 {
		t.Fatal("expected cache errors to be counted")
	}
}

func TestGet_UpstreamErrorIsNotCached(t *testing.T) {
	f := &fakeFetcher{err: errors.New("boom")}
	c := newMemCache()
	s := NewService(f, c, time.Minute, &metrics.Counters{}, discard)

	if _, err := s.Get(context.Background(), "1"); err == nil {
		t.Fatal("expected error")
	}
	if len(c.data) != 0 {
		t.Fatal("errors must not be cached")
	}
}

func TestGet_NoCacheAlwaysCallsUpstream(t *testing.T) {
	f := &fakeFetcher{}
	s := NewService(f, nil, time.Minute, &metrics.Counters{}, discard)
	for i := 0; i < 5; i++ {
		if _, err := s.Get(context.Background(), "1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.calls.Load(); got != 5 {
		t.Fatalf("upstream calls = %d, want 5", got)
	}
}
