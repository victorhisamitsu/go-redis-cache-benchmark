// Package cache defines the cache abstraction used by the service
// and a Redis-backed implementation.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrMiss is returned when the key is not present in the cache.
var ErrMiss = errors.New("cache: miss")

// Cache is a minimal byte-oriented key/value cache.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Redis implements Cache on top of go-redis.
type Redis struct {
	client *redis.Client
}

// NewRedis creates a Redis cache. Call Ping to verify connectivity.
func NewRedis(addr string) *Redis {
	return &Redis{client: redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		PoolSize:     64,
	})}
}

// Ping checks that Redis is reachable.
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Get returns the cached value or ErrMiss.
func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return b, err
}

// Set stores value with the given TTL.
func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Close releases the connection pool.
func (r *Redis) Close() error { return r.client.Close() }
