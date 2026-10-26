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

type cacheEntry struct {
	msg     *dns.Msg
	stored  time.Time
	expires time.Time
}

// cache is a small TTL cache of upstream answers. Decisions are never cached:
// filtering runs on every query before the cache is consulted.
type cache struct {
	mu   sync.Mutex
	max  int
	data map[string]cacheEntry
	now  func() time.Time
}

func newCache(max int) *cache {
	return &cache{max: max, data: map[string]cacheEntry{}, now: time.Now}
}

func cacheKey(q dns.Question, do bool, cd bool) string {
	return q.Name + "|" + strconv.Itoa(int(q.Qtype)) + "|" + strconv.Itoa(int(q.Qclass)) + "|" + strconv.FormatBool(do) + strconv.FormatBool(cd)
}

func (c *cache) get(key string) *dns.Msg {
	if c == nil || c.max == 0 {
		return nil
	}
	now := c.now()
	c.mu.Lock()
	e, ok := c.data[key]
	if ok && !now.Before(e.expires) {
		delete(c.data, key)
		ok = false
	}
	c.mu.Unlock()
	if !ok {
		return nil
	}
	m := e.msg.Copy()
	elapsed := uint32(now.Sub(e.stored) / time.Second)
	for _, sec := range [][]dns.RR{m.Answer, m.Ns, m.Extra} {
		for _, rr := range sec {
			h := rr.Header()
			if h.Rrtype == dns.TypeOPT {
				continue
			}
			if h.Ttl > elapsed {
				h.Ttl -= elapsed
			} else {
				h.Ttl = 1
			}
		}
	}
	return m
}

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
		// evict expired entries, then arbitrary ones (map order) until 10% is free
		for k, e := range c.data {
			if !now.Before(e.expires) {
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
	c.data[key] = cacheEntry{msg: m.Copy(), stored: now, expires: now.Add(time.Duration(ttl) * time.Second)}
}

func (c *cache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}
