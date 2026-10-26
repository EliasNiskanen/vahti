package dnsserver

import (
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/config"
	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
)

// fakeUpstream is a real DNS server on loopback that records what it receives.
type fakeUpstream struct {
	addr    string
	queries atomic.Int32
	mu      sync.Mutex
	sawECS  bool
	srv     *dns.Server
}

func startUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	f := &fakeUpstream{}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.addr = pc.LocalAddr().String()
	f.srv = &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		f.queries.Add(1)
		if o := r.IsEdns0(); o != nil {
			for _, opt := range o.Option {
				if _, ok := opt.(*dns.EDNS0_SUBNET); ok {
					f.mu.Lock()
					f.sawECS = true
					f.mu.Unlock()
				}
			}
		}
		m := new(dns.Msg)
		m.SetReply(r)
		q := r.Question[0]
		switch {
		case q.Name == "shop.example.":
			m.Answer = append(m.Answer, &dns.CNAME{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300}, Target: "shop.criteo.com."})
			m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: "shop.criteo.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.IPv4(198, 51, 100, 7)})
		case q.Name == "rebind.example.":
			m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.IPv4(192, 168, 1, 1)})
		case q.Qtype == dns.TypeA:
			m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.IPv4(198, 51, 100, 1)})
		}
		_ = w.WriteMsg(m)
	})}
	go func() { _ = f.srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = f.srv.Shutdown() })
	return f
}

func newServer(t *testing.T, up *fakeUpstream) (*Server, *engine.Engine) {
	t.Helper()
	cfg := config.Default().DNS
	cfg.Upstreams = []string{"udp://" + up.addr}
	cfg.RateLimitQPS = 1000
	eng := engine.New(nil, intel.New(), 1000)
	s, err := New(cfg, []string{"vahti.lan"}, eng, Options{
		MACOf:        func(netip.Addr) string { return "" },
		LocalAddrFor: func(netip.Addr) (netip.Addr, bool) { return netip.MustParseAddr("192.168.1.2"), true },
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, eng
}

var lan = netip.MustParseAddr("192.168.1.50")

func query(s *Server, client netip.Addr, name string, qt uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qt)
	return s.Process(client, m)
}

func firstA(t *testing.T, m *dns.Msg) string {
	t.Helper()
	for _, rr := range m.Answer {
		if a, ok := rr.(*dns.A); ok {
			return a.A.String()
		}
	}
	t.Fatalf("no A record in %v", m)
	return ""
}

func TestForwardAndBlock(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	if got := firstA(t, query(s, lan, "www.yle.fi", dns.TypeA)); got != "198.51.100.1" {
		t.Fatalf("allowed name: got %s", got)
	}
	before := up.queries.Load()
	if got := firstA(t, query(s, lan, "ad.doubleclick.net", dns.TypeA)); got != "0.0.0.0" {
		t.Fatalf("tracker: got %s", got)
	}
	if up.queries.Load() != before {
		t.Fatal("a blocked name must never be sent upstream")
	}
}

func TestCacheAndNoClientSubnet(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	m := new(dns.Msg)
	m.SetQuestion("Cached.Example.", dns.TypeA)
	o := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	o.SetUDPSize(1232)
	o.Option = append(o.Option, &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET, Family: 1, SourceNetmask: 24, Address: net.IPv4(85, 76, 1, 0)})
	m.Extra = append(m.Extra, o)
	r := s.Process(lan, m)
	if r.Question[0].Name != "Cached.Example." {
		t.Fatal("response must echo the client's question")
	}
	query(s, lan, "cached.example", dns.TypeA)
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("second query should be cached, upstream saw %d", n)
	}
	up.mu.Lock()
	defer up.mu.Unlock()
	if up.sawECS {
		t.Fatal("client subnet (location hint) must not be forwarded")
	}
}

func TestCNAMECloakingBlocked(t *testing.T) {
	up := startUpstream(t)
	s, eng := newServer(t, up)
	if got := firstA(t, query(s, lan, "shop.example", dns.TypeA)); got != "0.0.0.0" {
		t.Fatalf("cloaked tracker should be blocked, got %s", got)
	}
	l := eng.Log(engine.LogFilter{Limit: 1})[0]
	if l.Via != "shop.criteo.com" || l.Action != engine.Block {
		t.Fatalf("log should name the CNAME target: %+v", l)
	}
}

func TestRebindProtection(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	if r := query(s, lan, "rebind.example", dns.TypeA); r.Rcode != dns.RcodeNameError {
		t.Fatalf("public name resolving to a LAN address must be refused, rcode %d", r.Rcode)
	}
}

func TestOnlyHomeNetworkIsServed(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	if r := query(s, netip.MustParseAddr("8.8.8.8"), "www.yle.fi", dns.TypeA); r != nil {
		t.Fatal("queries from the internet must be dropped")
	}
	if r := query(s, netip.MustParseAddr("100.64.1.1"), "www.yle.fi", dns.TypeA); r != nil {
		t.Fatal("carrier-grade NAT addresses are outside the home network")
	}
}

func TestLocalNamesStayHome(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	for _, n := range []string{"printer.lan", "nas", "10.1.168.192.in-addr.arpa", "router.home.arpa"} {
		if r := query(s, lan, n, dns.TypeA); r.Rcode != dns.RcodeNameError {
			t.Errorf("%s: rcode %d", n, r.Rcode)
		}
	}
	if up.queries.Load() != 0 {
		t.Fatal("local names must never be sent to the internet")
	}
	if got := firstA(t, query(s, lan, "vahti.lan", dns.TypeA)); got != "192.168.1.2" {
		t.Fatalf("vahti.lan should resolve to the device, got %s", got)
	}
}

func TestBypassCanary(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	if r := query(s, lan, "use-application-dns.net", dns.TypeA); r.Rcode != dns.RcodeNameError {
		t.Fatal("Firefox canary must return NXDOMAIN")
	}
	if r := query(s, lan, "dns.google", dns.TypeA); firstA(t, r) != "0.0.0.0" {
		t.Fatal("public DoH resolvers should be blocked")
	}
}

func TestMalformedQueries(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	m := new(dns.Msg)
	if r := s.Process(lan, m); r.Rcode != dns.RcodeFormatError {
		t.Fatal("empty question")
	}
	m.SetQuestion("example.com.", dns.TypeANY)
	if r := s.Process(lan, m); r.Rcode != dns.RcodeNotImplemented {
		t.Fatal("ANY should be refused (amplification)")
	}
	m.SetQuestion("example.com.", dns.TypeAXFR)
	if r := s.Process(lan, m); r.Rcode != dns.RcodeNotImplemented {
		t.Fatal("zone transfer")
	}
}

func TestRateLimit(t *testing.T) {
	l := newLimiter(10)
	now := time.Now()
	ok := 0
	for i := 0; i < 100; i++ {
		if l.allow(lan, now) {
			ok++
		}
	}
	if ok != 40 {
		t.Fatalf("burst should be 4x rate, allowed %d", ok)
	}
	if !l.allow(lan, now.Add(time.Second)) {
		t.Fatal("tokens should refill")
	}
}

func TestOverRealSockets(t *testing.T) {
	up := startUpstream(t)
	s, _ := newServer(t, up)
	if err := s.Start([]string{"127.0.0.1:0"}); err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown()
	var udp, tcp string
	for _, a := range s.Addrs() {
		if strings.HasPrefix(a, "udp:") {
			udp = strings.TrimPrefix(a, "udp:")
		}
		if strings.HasPrefix(a, "tcp:") {
			tcp = strings.TrimPrefix(a, "tcp:")
		}
	}
	if err := SelfCheck(udp); err != nil {
		t.Fatal(err)
	}
	c := &dns.Client{Net: "tcp", Timeout: 2 * time.Second}
	m := new(dns.Msg)
	m.SetQuestion("www.google-analytics.com.", dns.TypeA)
	r, _, err := c.Exchange(m, tcp)
	if err != nil {
		t.Fatal(err)
	}
	if firstA(t, r) != "0.0.0.0" {
		t.Fatal("tracker over TCP should be blocked")
	}
}

func TestRateLimiterSurvivesSpoofing(t *testing.T) {
	l := newLimiter(5)
	now := time.Now()
	for i := 0; i < 10000; i++ {
		l.allow(netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)}), now)
	}
	if !l.allow(lan, now) {
		t.Fatal("a real device must not be locked out by spoofed sources")
	}
}
