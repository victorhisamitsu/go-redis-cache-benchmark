package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

// Integration test: runs against a real Redis (REDIS_ADDR, default
// localhost:6379) and is skipped when none is available.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available at %s: %v", addr, err)
	}
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})
	return rdb
}

func TestContactRedis_SetGetAndTTL(t *testing.T) {
	rdb := newTestRedis(t)
	repo := NewContactRedis(rdb)
	ctx := context.Background()

	if _, found, err := repo.Get(ctx, "1"); err != nil || found {
		t.Fatalf("expected miss, got found=%v err=%v", found, err)
	}

	want := model.Contact{ID: "1", Name: "Ana", Segment: "retail", OptIn: true}
	if err := repo.Set(ctx, want, time.Minute); err != nil {
		t.Fatal(err)
	}

	got, found, err := repo.Get(ctx, "1")
	if err != nil || !found {
		t.Fatalf("expected hit, got found=%v err=%v", found, err)
	}
	if got.Name != want.Name || got.Segment != want.Segment || got.OptIn != want.OptIn {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if ttl := rdb.TTL(ctx, keyPrefix+"1").Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected ttl %v", ttl)
	}
}
