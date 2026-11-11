// Command api exposes GET /contacts/{id}, backed by a slow external CRM and
// (optionally) a Redis cache-aside layer.
//
// main only wires dependencies; all behavior lives in internal/.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/handler"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/controller/usecase"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/domain/service"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/repository/cache"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/app/contact/repository/integration"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/config"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/httpx"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/logger"
	"github.com/victorhisamitsu/go-redis-cache-benchmark/internal/crosscutting/metrics"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.LogLevel)
	counters := &metrics.Counters{}

	// Repositories
	crm := integration.NewCRMClient(cfg.UpstreamURL, cfg.UpstreamTimeout)

	var contactCache service.ContactCache
	if cfg.CacheEnabled {
		rdb := redis.NewClient(&redis.Options{
			Addr:         cfg.RedisAddr,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  200 * time.Millisecond,
			WriteTimeout: 200 * time.Millisecond,
			PoolSize:     64,
		})
		defer rdb.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Warn("redis unreachable at startup; requests will fall back to the crm", "err", err)
		}
		cancel()
		contactCache = cache.NewContactRedis(rdb)
	}

	// Domain → use case → handler
	contactService := service.NewContactService(crm, contactCache, cfg.CacheTTL, counters, log)
	getContact := usecase.NewGetContact(contactService)
	contactHandler := handler.NewContact(getContact, log)

	// Routes
	mux := http.NewServeMux()
	controller.RegisterRoutes(mux, contactHandler)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, counters.Snapshot())
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("api listening", "addr", cfg.Addr, "cache", cfg.CacheEnabled, "ttl", cfg.CacheTTL.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
