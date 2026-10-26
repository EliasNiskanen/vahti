package dnsserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/config"
	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
)

// FuzzProcess feeds arbitrary packets from a home-network client through the
// whole pipeline. The upstream echoes a CNAME and an internal address so the
// CNAME and rebinding paths are exercised too.
func FuzzProcess(f *testing.F) {
	for _, n := range []string{"example.com.", "ad.doubleclick.net.", "use-application-dns.net.", "1.1.168.192.in-addr.arpa.", "vahti.lan.", "."} {
		m := new(dns.Msg)
		m.SetQuestion(n, dns.TypeA)
		m.SetEdns0(4096, true)
		b, _ := m.Pack()
		f.Add(b)
	}
	cfg := config.Default().DNS
	cfg.RateLimitQPS = 0
	eng := engine.New(nil, intel.New(), 200)
	s, err := New(cfg, []string{"vahti.lan"}, eng, Options{
		Upstream: ExchangeFunc(func(q *dns.Msg) (*dns.Msg, error) {
			r := new(dns.Msg)
			r.SetReply(q)
			n := q.Question[0].Name
			r.Answer = append(r.Answer,
				&dns.CNAME{Hdr: dns.RR_Header{Name: n, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60}, Target: "x.criteo.com."},
				&dns.A{Hdr: dns.RR_Header{Name: "x.criteo.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.IPv4(10, 0, 0, 1)})
			return r, nil
		}),
		MACOf:        func(netip.Addr) string { return "" },
		LocalAddrFor: func(netip.Addr) (netip.Addr, bool) { return netip.MustParseAddr("192.168.1.2"), true },
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		m := new(dns.Msg)
		if err := m.Unpack(b); err != nil {
			return
		}
		r := s.Process(netip.MustParseAddr("192.168.1.9"), m)
		if r != nil {
			if _, err := r.Pack(); err != nil {
				t.Fatalf("response cannot be packed: %v", err)
			}
		}
	})
}
