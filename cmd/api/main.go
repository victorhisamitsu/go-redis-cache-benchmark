// Command api exposes GET /contacts/{id}, backed by the slow external API
// and (optionally) a Redis cache-aside layer.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/cache"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/contacts"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/metrics"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/upstream"
)

func main() {
	addr := flag.String("addr", envOr("API_ADDR", ":8080"), "listen address")
	upstreamURL := flag.String("upstream", envOr("UPSTREAM_URL", "http://localhost:8081"), "external API base URL")
	redisAddr := flag.String("redis", envOr("REDIS_ADDR", "localhost:6379"), "Redis address")
	cacheOn := flag.Bool("cache", envOr("CACHE_ENABLED", "true") == "true", "enable the Redis cache")
	ttl := flag.Duration("ttl", 5*time.Minute, "cache TTL")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	m := &metrics.Counters{}

	var c cache.Cache
	if *cacheOn {
		r := cache.NewRedis(*redisAddr)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := r.Ping(ctx); err != nil {
			log.Warn("redis unreachable at startup; requests will fall back to upstream", "err", err)
		}
		cancel()
		defer r.Close()
		c = r
	}

	svc := contacts.NewService(upstream.NewClient(*upstreamURL), c, *ttl, m, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		ct, err := svc.Get(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, upstream.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
			return
		case err != nil:
			log.Error("get contact", "err", err)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		writeJSON(w, ct)
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, m.Snapshot())
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("api listening", "addr", *addr, "cache", *cacheOn, "ttl", ttl.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
