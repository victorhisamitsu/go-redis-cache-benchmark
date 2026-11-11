// Package cache is the Redis-backed contact cache repository.
//
// It owns key naming and serialization, so the domain only deals with
// model.Contact values.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/model"
)

const keyPrefix = "contact:v1:"

// ContactRedis implements service.ContactCache on top of Redis.
type ContactRedis struct {
	client *redis.Client
}

// NewContactRedis creates the repository from an existing Redis client.
func NewContactRedis(client *redis.Client) *ContactRedis {
	return &ContactRedis{client: client}
}

// Get returns the cached contact. found is false on a cache miss.
func (r *ContactRedis) Get(ctx context.Context, id string) (c model.Contact, found bool, err error) {
	b, err := r.client.Get(ctx, keyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return model.Contact{}, false, nil
	}
	if err != nil {
		return model.Contact{}, false, fmt.Errorf("redis get: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return model.Contact{}, false, fmt.Errorf("decode cached contact: %w", err)
	}
	return c, true, nil
}

// Set stores the contact with the given TTL.
func (r *ContactRedis) Set(ctx context.Context, c model.Contact, ttl time.Duration) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode contact: %w", err)
	}
	if err := r.client.Set(ctx, keyPrefix+c.ID, b, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}
