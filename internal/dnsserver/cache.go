package dnsserver

import (
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	minTTL = 10
	maxTTL = 3600
	negTTL = 60
)

// staleTTL is the TTL given to an expired answer served from the cache (RFC 8767
// recommends 30 seconds), so the client asks again soon and gets the refreshed one.
const staleTTL = 30

type cacheEntry struct {
	msg     *dns.Msg
	stored  time.Time
	expires time.Time
	ttl     time.Duration // the lifetime the entry was stored with
}

// lookup tells the caller what kind of cache hit it got.
type lookup int

const (
	miss     lookup = iota // nothing usable: ask the upstream
	fresh                  // a valid answer
	prefetch               // a valid answer that expires soon: refresh it in the background
	stale                  // an expired answer: serve it now, refresh in the background
)

// cache is a small TTL cache of upstream answers. Decisions are never cached:
// filtering runs on every query before the cache is consulted, and the CNAME and
// rebinding checks run on every answer, cached or not.
type cache struct {
	mu       sync.Mutex
	max      int
	staleFor time.Duration // how long expired answers may still be served (0 = never)
	prefetch bool          // report popular entries that are about to expire
	data     map[string]cacheEntry
	now      func() time.Time
}

// newCache creates an answer cache that holds at most max entries (0 turns caching off).
// Expired answers are kept and served for staleFor while they are refreshed, and with
// prefetch on, entries asked for in the last 10% of their lifetime are refreshed early.
func newCache(max int, staleFor time.Duration, prefetch bool) *cache {
	return &cache{max: max, staleFor: staleFor, prefetch: prefetch, data: map[string]cacheEntry{}, now: time.Now}
}

// cacheKey builds the cache key from the question and the DNSSEC flags (DO and CD),
// because answers differ with them.
func cacheKey(q dns.Question, do bool, cd bool) string {
	return q.Name + "|" + strconv.Itoa(int(q.Qtype)) + "|" + strconv.Itoa(int(q.Qclass)) + "|" + strconv.FormatBool(do) + strconv.FormatBool(cd)
}

// get returns a copy of a cached answer and what kind of hit it is. A fresh answer has
// its TTLs reduced by the time it has spent in the cache. An expired answer within the
// stale window gets a short TTL of 30 seconds. Without a usable answer it returns nil, miss.
func (c *cache) get(key string) (*dns.Msg, lookup) {
	if c == nil || c.max == 0 {
		return nil, miss
	}
	now := c.now()
	c.mu.Lock()
	e, ok := c.data[key]
	if ok && !now.Before(e.expires.Add(c.staleFor)) {
		delete(c.data, key)
		ok = false
	}
	c.mu.Unlock()
	if !ok {
		return nil, miss
	}
	kind := fresh
	if !now.Before(e.expires) {
		kind = stale
	} else if c.prefetch && e.ttl >= time.Minute && e.expires.Sub(now) < e.ttl/10 {
		kind = prefetch
	}
	m := e.msg.Copy()
	elapsed := uint32(now.Sub(e.stored) / time.Second)
	for _, sec := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range sec {
			h := rr.Header()
			switch {
			case h.Rrtype == dns.TypeOPT:
			case kind == stale:
				h.Ttl = staleTTL
			case h.Ttl > elapsed:
				h.Ttl -= elapsed
			default:
				h.Ttl = 1
			}
		}
	}
	return m, kind
}

// put stores a successful or "no such name" answer for as long as its shortest TTL allows
// (within the cache's limits). When the cache is full, entries past their stale window and
// then arbitrary entries are dropped until 10% is free.
func (c *cache) put(key string, m *dns.Msg) {
	if c == nil || c.max == 0 || m == nil || m.Truncated {
		return
	}
	if m.Rcode != dns.RcodeSuccess && m.Rcode != dns.RcodeNameError {
		return
	}
	ttl := uint32(maxTTL)
	found := false
	for _, rr := range m.Answer {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
		found = true
	}
	if !found {
		ttl = negTTL
		for _, rr := range m.Ns {
			if soa, ok := rr.(*dns.SOA); ok && soa.Minttl < ttl {
				ttl = soa.Minttl
			}
		}
	}
	if ttl < minTTL {
		ttl = minTTL
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) >= c.max {
		// evict entries past their stale window, then arbitrary ones (map order) until 10% is free
		for k, e := range c.data {
			if !now.Before(e.expires.Add(c.staleFor)) {
				delete(c.data, k)
			}
		}
		for k := range c.data {
			if len(c.data) < c.max*9/10 {
				break
			}
			delete(c.data, k)
		}
	}
	life := time.Duration(ttl) * time.Second
	c.data[key] = cacheEntry{msg: m.Copy(), stored: now, expires: now.Add(life), ttl: life}
}
