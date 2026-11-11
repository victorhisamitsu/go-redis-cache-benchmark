// Package config loads the API configuration from environment variables,
// with flags as overrides for local runs.
package config

import (
	"flag"
	"os"
	"time"
)

// Config holds every setting the API needs.
type Config struct {
	Addr            string
	UpstreamURL     string
	UpstreamTimeout time.Duration
	RedisAddr       string
	CacheEnabled    bool
	CacheTTL        time.Duration
	LogLevel        string
}

// Load reads env vars (12-factor style) and lets flags override them.
func Load() Config {
	c := Config{
		Addr:            env("API_ADDR", ":8080"),
		UpstreamURL:     env("UPSTREAM_URL", "http://localhost:8081"),
		UpstreamTimeout: envDuration("UPSTREAM_TIMEOUT", 30*time.Second),
		RedisAddr:       env("REDIS_ADDR", "localhost:6379"),
		CacheEnabled:    env("CACHE_ENABLED", "true") == "true",
		CacheTTL:        envDuration("CACHE_TTL", 5*time.Minute),
		LogLevel:        env("LOG_LEVEL", "info"),
	}
	flag.StringVar(&c.Addr, "addr", c.Addr, "listen address")
	flag.StringVar(&c.UpstreamURL, "upstream", c.UpstreamURL, "external API base URL")
	flag.StringVar(&c.RedisAddr, "redis", c.RedisAddr, "Redis address")
	flag.BoolVar(&c.CacheEnabled, "cache", c.CacheEnabled, "enable the Redis cache")
	flag.DurationVar(&c.CacheTTL, "ttl", c.CacheTTL, "cache TTL")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "debug | info | warn | error")
	flag.Parse()
	return c
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return d
	}
	return def
}
