// Command bench is a small load generator that reports latency percentiles
// and how many calls actually reached the external API.
//
// Keys follow a Zipf distribution: a few contacts are requested very often
// and many are requested rarely — the realistic access pattern of
// messaging platforms, where active conversations hit the same profiles
// over and over.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8080", "API base URL")
	upstreamURL := flag.String("upstream", "http://localhost:8081", "upstream base URL (for call counting)")
	requests := flag.Int("n", 2000, "total requests")
	concurrency := flag.Int("c", 50, "concurrent workers")
	keys := flag.Uint64("keys", 500, "distinct contact ids")
	label := flag.String("label", "run", "label printed in the report")
	flag.Parse()

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: *concurrency * 2},
	}

	_, _ = client.Post(*upstreamURL+"/stats/reset", "", nil)

	ids := make(chan uint64, *requests)
	z := rand.NewZipf(rand.New(rand.NewPCG(1, 2)), 1.1, 1, *keys-1)
	for i := 0; i < *requests; i++ {
		ids <- z.Uint64()
	}
	close(ids)

	var (
		mu        sync.Mutex
		latencies = make([]time.Duration, 0, *requests)
		errs      atomic.Int64
		wg        sync.WaitGroup
	)
	start := time.Now()
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ids {
				t0 := time.Now()
				resp, err := client.Get(fmt.Sprintf("%s/contacts/%d", *apiURL, id))
				d := time.Since(t0)
				if err != nil || resp.StatusCode != http.StatusOK {
					errs.Add(1)
				}
				if resp != nil {
					resp.Body.Close()
				}
				mu.Lock()
				latencies = append(latencies, d)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	slices.Sort(latencies)
	pct := func(p float64) time.Duration { return latencies[int(float64(len(latencies)-1)*p)] }
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}

	var stats struct {
		Calls int64 `json:"calls"`
	}
	if resp, err := client.Get(*upstreamURL + "/stats"); err == nil {
		_ = json.NewDecoder(resp.Body).Decode(&stats)
		resp.Body.Close()
	}

	fmt.Fprintf(os.Stdout, "\n== %s ==\n", *label)
	fmt.Printf("requests:        %d (errors: %d)\n", len(latencies), errs.Load())
	fmt.Printf("total time:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput:      %.1f req/s\n", float64(len(latencies))/elapsed.Seconds())
	fmt.Printf("latency avg:     %s\n", (sum / time.Duration(len(latencies))).Round(100*time.Microsecond))
	fmt.Printf("latency p50:     %s\n", pct(0.50).Round(100*time.Microsecond))
	fmt.Printf("latency p95:     %s\n", pct(0.95).Round(time.Millisecond))
	fmt.Printf("latency p99:     %s\n", pct(0.99).Round(time.Millisecond))
	fmt.Printf("upstream calls:  %d (%.1f%% of requests)\n", stats.Calls, 100*float64(stats.Calls)/float64(len(latencies)))
}
