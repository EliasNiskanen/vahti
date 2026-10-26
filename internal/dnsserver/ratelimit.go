package dnsserver

import (
	"net/netip"
	"sync"
	"time"
)

// limiter is a per-client token bucket.
type limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[netip.Addr]*bucket
	last    time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

// newLimiter creates a per-client rate limiter allowing qps queries per second with bursts
// of four times that. qps <= 0 turns limiting off (nil limiter).
func newLimiter(qps int) *limiter {
	if qps <= 0 {
		return nil
	}
	return &limiter{rate: float64(qps), burst: float64(qps) * 4, buckets: map[netip.Addr]*bucket{}}
}

// allow reports whether this client may make another query now (token bucket). Buckets
// idle for over a minute are cleaned up so spoofed addresses cannot fill memory.
func (l *limiter) allow(a netip.Addr, now time.Time) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.last) > time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.at) > time.Minute {
				delete(l.buckets, k)
			}
		}
		l.last = now
	}
	b := l.buckets[a]
	if b == nil {
		if len(l.buckets) >= 4096 {
			// Many (possibly spoofed) sources: drop idle buckets. If the table is
			// still full, serve the query rather than lock out real devices.
			for k, ob := range l.buckets {
				if now.Sub(ob.at) > 5*time.Second {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= 4096 {
				return true
			}
		}
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[a] = b
	}
	b.tokens += now.Sub(b.at).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
