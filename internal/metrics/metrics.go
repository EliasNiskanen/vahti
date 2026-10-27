// Package metrics measures how fast Vahti answers DNS queries, so the admin can see
// whether Vahti slows the connection down. Only timings and counts are kept, never
// names or addresses, and only for the last few minutes.
package metrics

import (
	"sync"
	"time"
)

// Source says where an answer came from.
type Source int

const (
	Cache    Source = iota // fresh answer from Vahti's cache
	Stale                  // expired answer from the cache, refreshed in the background
	Upstream               // asked from the upstream resolver
	Blocked                // answered by Vahti itself because the name is blocked
	Local                  // Vahti's own names, local names and checks
	Failed                 // the upstream resolver did not answer
	numSources
)

// bounds are the upper limits of the latency buckets in microseconds. The last
// bucket holds everything slower.
var bounds = [...]int64{100, 250, 500, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000, 2000000}

const (
	numBuckets = len(bounds) + 1
	slotLen    = time.Minute
	numSlots   = 15
)

// slot holds one minute of measurements.
type slot struct {
	start  time.Time
	counts [numSources][numBuckets]uint64
}

// Recorder collects answer times per source over the last 15 minutes, in one-minute
// slots. It uses a fixed amount of memory no matter how many queries there are.
type Recorder struct {
	mu    sync.Mutex
	slots [numSlots]slot
	now   func() time.Time
}

// New returns an empty recorder.
func New() *Recorder { return &Recorder{now: time.Now} }

// SetClock replaces the clock (for tests).
func (r *Recorder) SetClock(now func() time.Time) { r.now = now }

// bucket returns the index of the latency bucket for d.
func bucket(d time.Duration) int {
	us := d.Microseconds()
	for i, b := range bounds {
		if us <= b {
			return i
		}
	}
	return len(bounds)
}

// Observe records one answer that took d and came from src. A nil recorder ignores it.
func (r *Recorder) Observe(src Source, d time.Duration) {
	if r == nil || src < 0 || src >= numSources {
		return
	}
	now := r.now()
	start := now.Truncate(slotLen)
	i := int(start.Unix()/int64(slotLen/time.Second)) % numSlots
	r.mu.Lock()
	s := &r.slots[i]
	if !s.start.Equal(start) {
		*s = slot{start: start}
	}
	s.counts[src][bucket(d)]++
	r.mu.Unlock()
}

// Snapshot is the summary shown in the web interface. Times are in milliseconds and are
// upper estimates: a median of 0.25 means half of the answers took at most 0.25 ms.
type Snapshot struct {
	Minutes    int     `json:"minutes"`    // how many minutes the numbers cover
	Queries    uint64  `json:"queries"`    // answered queries in that time
	MedianMs   float64 `json:"medianMs"`   // median answer time over all queries
	P95Ms      float64 `json:"p95Ms"`      // 95 % of all answers were at least this fast
	CachePct   float64 `json:"cachePct"`   // share answered from the cache (fresh or stale), of answers that needed a lookup
	UpstreamMs float64 `json:"upstreamMs"` // median time of the answers that went upstream
	Stale      uint64  `json:"stale"`      // answers served stale while refreshing
	Failed     uint64  `json:"failed"`     // queries the upstream did not answer
}

// Snapshot summarises the last 15 minutes.
func (r *Recorder) Snapshot() Snapshot {
	var all, up [numBuckets]uint64
	var per [numSources]uint64
	cutoff := r.now().Truncate(slotLen).Add(-(numSlots - 1) * slotLen)
	r.mu.Lock()
	for i := range r.slots {
		s := &r.slots[i]
		if s.start.IsZero() || s.start.Before(cutoff) {
			continue
		}
		for src := range s.counts {
			for b, n := range s.counts[src] {
				all[b] += n
				per[src] += n
				if Source(src) == Upstream {
					up[b] += n
				}
			}
		}
	}
	r.mu.Unlock()
	out := Snapshot{Minutes: numSlots, Stale: per[Stale], Failed: per[Failed]}
	for _, n := range per {
		out.Queries += n
	}
	out.MedianMs = quantile(all, 0.5)
	out.P95Ms = quantile(all, 0.95)
	out.UpstreamMs = quantile(up, 0.5)
	if lookups := per[Cache] + per[Stale] + per[Upstream]; lookups > 0 {
		out.CachePct = float64(per[Cache]+per[Stale]) * 100 / float64(lookups)
	}
	return out
}

// quantile returns the upper bound (in ms) of the bucket that holds quantile q, or 0
// if there are no measurements.
func quantile(h [numBuckets]uint64, q float64) float64 {
	var total uint64
	for _, n := range h {
		total += n
	}
	if total == 0 {
		return 0
	}
	want := uint64(q*float64(total) + 0.5)
	if want < 1 {
		want = 1
	}
	var seen uint64
	for i, n := range h {
		seen += n
		if seen >= want {
			if i < len(bounds) {
				return float64(bounds[i]) / 1000
			}
			break
		}
	}
	return float64(bounds[len(bounds)-1]) / 1000 // slower than the last bound
}
