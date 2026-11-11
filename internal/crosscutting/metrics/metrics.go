// Package metrics holds lightweight, lock-free counters exposed at /metrics.
package metrics

import "sync/atomic"

// Counters tracks how requests were served.
type Counters struct {
	Requests      atomic.Int64
	CacheHits     atomic.Int64
	CacheMisses   atomic.Int64
	CacheErrors   atomic.Int64
	UpstreamCalls atomic.Int64
	Coalesced     atomic.Int64
}

// Snapshot is a JSON-friendly copy of the counters.
type Snapshot struct {
	Requests      int64   `json:"requests"`
	CacheHits     int64   `json:"cache_hits"`
	CacheMisses   int64   `json:"cache_misses"`
	CacheErrors   int64   `json:"cache_errors"`
	UpstreamCalls int64   `json:"upstream_calls"`
	Coalesced     int64   `json:"coalesced_requests"`
	HitRatio      float64 `json:"hit_ratio"`
}

// Snapshot returns the current values.
func (c *Counters) Snapshot() Snapshot {
	s := Snapshot{
		Requests:      c.Requests.Load(),
		CacheHits:     c.CacheHits.Load(),
		CacheMisses:   c.CacheMisses.Load(),
		CacheErrors:   c.CacheErrors.Load(),
		UpstreamCalls: c.UpstreamCalls.Load(),
		Coalesced:     c.Coalesced.Load(),
	}
	if lookups := s.CacheHits + s.CacheMisses; lookups > 0 {
		s.HitRatio = float64(s.CacheHits) / float64(lookups)
	}
	return s
}
