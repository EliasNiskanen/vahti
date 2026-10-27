package dnsserver

import (
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/config"
	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
)

// answer builds an upstream answer for name with one A record and the given TTL.
func answer(name string, ttl uint32, ip net.IP) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)
	m.Response = true
	m.Answer = []dns.RR{&dns.A{Hdr: hdr(name, dns.TypeA, ttl), A: ip}}
	return m
}

// An entry is fresh until its TTL, then prefetch-worthy in the last 10%, then served
// stale with a 30-second TTL until the stale window ends, and then gone.
func TestCacheStaleAndPrefetch(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := newCache(100, time.Hour, true)
	c.now = func() time.Time { return now }
	c.put("k", answer("a.example.", 600, net.IPv4(198, 51, 100, 1)))

	steps := []struct {
		after time.Duration
		want  lookup
		ttl   uint32
	}{
		{0, fresh, 600},
		{500 * time.Second, fresh, 100},
		{550 * time.Second, prefetch, 50},
		{600 * time.Second, stale, staleTTL},
		{600*time.Second + 59*time.Minute, stale, staleTTL},
		{600*time.Second + time.Hour, miss, 0},
	}
	start := now
	for _, st := range steps {
		now = start.Add(st.after)
		m, kind := c.get("k")
		if kind != st.want {
			t.Fatalf("after %v: kind %d, want %d", st.after, kind, st.want)
		}
		if m != nil && m.Answer[0].Header().Ttl != st.ttl {
			t.Fatalf("after %v: ttl %d, want %d", st.after, m.Answer[0].Header().Ttl, st.ttl)
		}
	}
}

// With serve-stale and prefetch off, an expired entry is simply gone, and short-lived
// entries are never prefetched.
func TestCacheWithoutStale(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := newCache(100, 0, false)
	c.now = func() time.Time { return now }
	c.put("k", answer("a.example.", 100, net.IPv4(198, 51, 100, 1)))
	now = now.Add(95 * time.Second)
	if _, kind := c.get("k"); kind != fresh {
		t.Fatalf("prefetch is off, got %d", kind)
	}
	now = now.Add(5 * time.Second)
	if _, kind := c.get("k"); kind != miss {
		t.Fatalf("no stale window, got %d", kind)
	}

	p := newCache(100, time.Hour, true)
	p.now = func() time.Time { return now }
	p.put("short", answer("b.example.", 30, net.IPv4(198, 51, 100, 2)))
	now = now.Add(29 * time.Second)
	if _, kind := p.get("short"); kind != fresh {
		t.Fatalf("names with TTL under a minute are not prefetched, got %d", kind)
	}
}

// An expired name is answered from the cache at once, while one background refresh
// fetches the new address; the next query gets the new address. The filtering still
// applies to the stale answer.
func TestServeStaleRefreshesInBackground(t *testing.T) {
	var calls atomic.Int32
	ip := atomic.Pointer[net.IP]{}
	first := net.IPv4(198, 51, 100, 1)
	ip.Store(&first)
	up := ExchangeFunc(func(m *dns.Msg) (*dns.Msg, error) {
		calls.Add(1)
		r := answer(m.Question[0].Name, 60, *ip.Load())
		r.Id = m.Id
		return r, nil
	})
	cfg := config.Default().DNS
	cfg.RateLimitQPS = 0
	eng := engine.New(nil, intel.New(), 1000)
	s, err := New(cfg, nil, eng, Options{Upstream: up, MACOf: func(netip.Addr) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.cache.now = func() time.Time { return now }

	if got := firstA(t, query(s, lan, "moved.example", dns.TypeA)); got != "198.51.100.1" {
		t.Fatalf("first answer %s", got)
	}
	second := net.IPv4(198, 51, 100, 2)
	ip.Store(&second)
	now = now.Add(2 * time.Minute) // expired, inside the stale window

	if got := firstA(t, query(s, lan, "moved.example", dns.TypeA)); got != "198.51.100.1" {
		t.Fatalf("stale answer should come at once from the cache, got %s", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	for i := 0; i < 200; i++ { // wait for the refresh to be stored
		if _, busy := s.refreshing.Load(cacheKey(dns.Question{Name: "moved.example.", Qtype: dns.TypeA, Qclass: dns.ClassINET}, false, false)); !busy {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := firstA(t, query(s, lan, "moved.example", dns.TypeA)); got != "198.51.100.2" {
		t.Fatalf("after the refresh the new address should be served, got %s", got)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("upstream asked %d times, want 2", n)
	}

	sp := s.Speed()
	if sp.Queries != 3 || sp.Stale != 1 || sp.CachePct < 66 || sp.CachePct > 67 {
		t.Fatalf("speed snapshot %+v", sp)
	}
	if query(s, lan, "ad.doubleclick.net", dns.TypeA); s.Speed().Queries != 4 {
		t.Fatal("blocked answers are measured too")
	}
}
