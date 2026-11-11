// Command upstream simulates a slow, metered external CRM API.
//
// Every call costs latency (and, in real life, money), which is exactly
// what the cache layer in cmd/api is designed to avoid.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", envOr("UPSTREAM_ADDR", ":8081"), "listen address")
	latency := flag.Duration("latency", envDur("UPSTREAM_LATENCY", 1500*time.Millisecond), "base latency per call")
	jitter := flag.Duration("jitter", envDur("UPSTREAM_JITTER", 300*time.Millisecond), "random extra latency")
	flag.Parse()

	var calls atomic.Int64
	mux := http.NewServeMux()

	mux.HandleFunc("GET /contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		delay := *latency
		if *jitter > 0 {
			delay += time.Duration(rand.Int64N(int64(*jitter)))
		}
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		id := r.PathValue("id")
		n, _ := strconv.Atoi(id)
		segments := []string{"retail", "enterprise", "smb"}
		writeJSON(w, map[string]any{
			"id":         id,
			"name":       fmt.Sprintf("Contact %s", id),
			"phone":      fmt.Sprintf("+55 11 9%08d", n),
			"segment":    segments[n%len(segments)],
			"opt_in":     n%2 == 0,
			"updated_at": time.Now().UTC(),
		})
	})

	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]int64{"calls": calls.Load()})
	})
	mux.HandleFunc("POST /stats/reset", func(w http.ResponseWriter, _ *http.Request) {
		calls.Store(0)
		w.WriteHeader(http.StatusNoContent)
	})

	slog.Info("upstream listening", "addr", *addr, "latency", *latency, "jitter", *jitter)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
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

func envDur(k string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(k)); err == nil {
		return d
	}
	return def
}
