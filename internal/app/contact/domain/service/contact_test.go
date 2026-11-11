package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/metrics"
)

type fakeProvider struct {
	calls atomic.Int64
	delay time.Duration
	err   error
}

func (f *fakeProvider) FetchContact(_ context.Context, id string) (model.Contact, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	if f.err != nil {
		return model.Contact{}, f.err
	}
	return model.Contact{ID: id, Name: "Contact " + id}, nil
}

type memCache struct {
	mu   sync.Mutex
	data map[string]model.Contact
	fail bool
}

func newMemCache() *memCache { return &memCache{data: map[string]model.Contact{}} }

func (m *memCache) Get(_ context.Context, id string) (model.Contact, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return model.Contact{}, false, errors.New("connection refused")
	}
	c, ok := m.data[id]
	return c, ok, nil
}

func (m *memCache) Set(_ context.Context, c model.Contact, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("connection refused")
	}
	m.data[c.ID] = c
	return nil
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestGetByID_MissThenHit(t *testing.T) {
	p := &fakeProvider{}
	m := &metrics.Counters{}
	s := NewContactService(p, newMemCache(), time.Minute, m, discard)

	for i := 0; i < 3; i++ {
		c, err := s.GetByID(context.Background(), "42")
		if err != nil {
			t.Fatal(err)
		}
		if c.ID != "42" {
			t.Fatalf("got id %q", c.ID)
		}
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
	if got := m.CacheHits.Load(); got != 2 {
		t.Fatalf("cache hits = %d, want 2", got)
	}
}

func TestGetByID_ConcurrentMissesAreCoalesced(t *testing.T) {
	p := &fakeProvider{delay: 50 * time.Millisecond}
	s := NewContactService(p, newMemCache(), time.Minute, &metrics.Counters{}, discard)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.GetByID(context.Background(), "hot-key"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := p.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cache stampede not prevented)", got)
	}
}

func TestGetByID_CacheDownFallsBackToProvider(t *testing.T) {
	c := newMemCache()
	c.fail = true
	m := &metrics.Counters{}
	s := NewContactService(&fakeProvider{}, c, time.Minute, m, discard)

	if _, err := s.GetByID(context.Background(), "1"); err != nil {
		t.Fatalf("expected graceful degradation, got %v", err)
	}
	if m.CacheErrors.Load() == 0 {
		t.Fatal("expected cache errors to be counted")
	}
}

func TestGetByID_ProviderErrorIsNotCached(t *testing.T) {
	c := newMemCache()
	s := NewContactService(&fakeProvider{err: errors.New("boom")}, c, time.Minute, &metrics.Counters{}, discard)

	if _, err := s.GetByID(context.Background(), "1"); err == nil {
		t.Fatal("expected error")
	}
	if len(c.data) != 0 {
		t.Fatal("errors must not be cached")
	}
}

func TestGetByID_NotFoundIsPropagated(t *testing.T) {
	s := NewContactService(&fakeProvider{err: model.ErrContactNotFound}, newMemCache(), time.Minute, &metrics.Counters{}, discard)

	if _, err := s.GetByID(context.Background(), "1"); !errors.Is(err, model.ErrContactNotFound) {
		t.Fatalf("err = %v, want ErrContactNotFound", err)
	}
}

func TestGetByID_NoCacheAlwaysCallsProvider(t *testing.T) {
	p := &fakeProvider{}
	s := NewContactService(p, nil, time.Minute, &metrics.Counters{}, discard)
	for i := 0; i < 5; i++ {
		if _, err := s.GetByID(context.Background(), "1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := p.calls.Load(); got != 5 {
		t.Fatalf("upstream calls = %d, want 5", got)
	}
}
