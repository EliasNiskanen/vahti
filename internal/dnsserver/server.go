// Package dnsserver is Vahti's filtering DNS resolver. Every query is checked
// against the engine before anything is sent upstream, so a blocked name never
// leaves the home network.
package dnsserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/config"
	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
	"github.com/EliasNiskanen/vahti/internal/metrics"
	"github.com/EliasNiskanen/vahti/internal/netutil"
)

// maxRefreshes limits how many background refreshes (serve-stale and prefetch) run at once.
const maxRefreshes = 8

// SelfCheckName is answered locally and used by the watchdog.
const SelfCheckName = "selfcheck.vahti.invalid."

// Exchanger sends a query to an upstream resolver.
type Exchanger interface {
	Exchange(*dns.Msg) (*dns.Msg, error)
}

// ExchangeFunc adapts a function to Exchanger.
type ExchangeFunc func(*dns.Msg) (*dns.Msg, error)

// Exchange calls f.
func (f ExchangeFunc) Exchange(m *dns.Msg) (*dns.Msg, error) { return f(m) }

// Exchange makes the forwarder usable as an Exchanger.
func (f *forwarder) Exchange(m *dns.Msg) (*dns.Msg, error) { return f.exchange(m) }

// Server is the filtering resolver.
type Server struct {
	eng          *engine.Engine
	db           *intel.DB
	upstream     Exchanger
	local        Exchanger
	cache        *cache
	lim          *limiter
	allowed      netutil.PrefixSet
	localDomains []string
	hostnames    map[string]bool
	rebind       bool
	rebindAllow  []string
	macOf        func(netip.Addr) string
	localAddrFor func(netip.Addr) (netip.Addr, bool)
	now          func() time.Time
	speed        *metrics.Recorder

	refreshing sync.Map      // cache keys being refreshed in the background
	refreshSem chan struct{} // limits concurrent background refreshes

	ptrTried sync.Map
	servers  []*dns.Server
}

// Options configures a Server beyond the config file (mostly for tests and demo).
type Options struct {
	Upstream     Exchanger                           // overrides cfg.Upstreams
	MACOf        func(netip.Addr) string             // defaults to the kernel ARP table
	LocalAddrFor func(netip.Addr) (netip.Addr, bool) // defaults to interface lookup
}

// New builds a server from configuration.
func New(cfg config.DNS, hostnames []string, eng *engine.Engine, opt Options) (*Server, error) {
	allowed, err := netutil.ParsePrefixes(cfg.AllowedClients)
	if err != nil {
		return nil, err
	}
	s := &Server{
		eng: eng, db: eng.DB(), lim: newLimiter(cfg.RateLimitQPS),
		cache: newCache(cfg.CacheSize, time.Duration(cfg.ServeStaleMinutes)*time.Minute, cfg.Prefetch),
		speed: metrics.New(), refreshSem: make(chan struct{}, maxRefreshes),
		allowed: allowed, rebind: cfg.RebindProtection, hostnames: map[string]bool{},
		macOf: opt.MACOf, localAddrFor: opt.LocalAddrFor, now: time.Now, upstream: opt.Upstream,
	}
	for _, d := range cfg.LocalDomains {
		s.localDomains = append(s.localDomains, intel.Normalize(d))
	}
	for _, d := range cfg.RebindAllow {
		s.rebindAllow = append(s.rebindAllow, intel.Normalize(d))
	}
	for _, h := range hostnames {
		s.hostnames[intel.Normalize(h)] = true
	}
	if s.upstream == nil {
		f := &forwarder{}
		for _, u := range cfg.Upstreams {
			pu, err := config.ParseUpstream(u)
			if err != nil {
				return nil, err
			}
			f.ups = append(f.ups, newUpstream(pu))
		}
		s.upstream = f
	}
	if cfg.LocalResolver != "" {
		s.local = &forwarder{ups: []*upstream{newUpstream(config.Upstream{Net: "udp", Addr: cfg.LocalResolver})}}
	}
	if s.macOf == nil {
		arp := netutil.NewARP()
		s.macOf = arp.Lookup
	}
	if s.localAddrFor == nil {
		s.localAddrFor = netutil.LocalAddrFor
	}
	return s, nil
}

// Start listens on UDP and TCP for every address and returns once bound.
func (s *Server) Start(listen []string) error {
	for _, addr := range listen {
		for _, proto := range []string{"udp", "tcp"} {
			started := make(chan struct{})
			srv := &dns.Server{
				Addr: addr, Net: proto, Handler: s,
				ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
				NotifyStartedFunc: func() { close(started) },
			}
			if proto == "tcp" {
				srv.MaxTCPQueries = 100
				srv.IdleTimeout = func() time.Duration { return 10 * time.Second }
			}
			errc := make(chan error, 1)
			go func() { errc <- srv.ListenAndServe() }()
			select {
			case <-started:
				s.servers = append(s.servers, srv)
			case err := <-errc:
				s.Shutdown()
				return fmt.Errorf("dns %s %s: %w", proto, addr, err)
			case <-time.After(5 * time.Second):
				s.Shutdown()
				return fmt.Errorf("dns %s %s: timed out while starting", proto, addr)
			}
		}
	}
	return nil
}

// Addrs returns the bound addresses (useful when listening on port 0).
func (s *Server) Addrs() []string {
	var out []string
	for _, srv := range s.servers {
		if srv.PacketConn != nil {
			out = append(out, "udp:"+srv.PacketConn.LocalAddr().String())
		}
		if srv.Listener != nil {
			out = append(out, "tcp:"+srv.Listener.Addr().String())
		}
	}
	return out
}

// Shutdown stops all listeners.
func (s *Server) Shutdown() {
	for _, srv := range s.servers {
		_ = srv.Shutdown()
	}
	s.servers = nil
}

// ServeDNS implements dns.Handler.
func (s *Server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dns handler panic", "err", r)
			m := new(dns.Msg)
			m.SetRcode(req, dns.RcodeServerFailure)
			_ = w.WriteMsg(m)
		}
	}()
	client, ok := netutil.AddrFromNetAddr(w.RemoteAddr())
	if !ok {
		return
	}
	_, udp := w.RemoteAddr().(*net.UDPAddr)
	resp := s.Process(client, req)
	if resp == nil {
		return // silently dropped
	}
	if udp {
		size := dns.MinMsgSize
		if o := req.IsEdns0(); o != nil && int(o.UDPSize()) > size {
			size = int(o.UDPSize())
		}
		if size > 1232 {
			size = 1232
		}
		resp.Truncate(size)
	}
	_ = w.WriteMsg(resp)
}

// Speed returns how fast Vahti has answered over the last 15 minutes.
func (s *Server) Speed() metrics.Snapshot { return s.speed.Snapshot() }

// Process handles one query and returns the response, or nil to drop it. It also
// records how long the answer took and where it came from.
func (s *Server) Process(client netip.Addr, req *dns.Msg) *dns.Msg {
	start := time.Now()
	resp, src := s.process(client, req)
	if resp != nil {
		s.speed.Observe(src, time.Since(start))
	}
	return resp
}

// process does the work of Process and also reports where the answer came from.
func (s *Server) process(client netip.Addr, req *dns.Msg) (*dns.Msg, metrics.Source) {
	// Never answer outside the home network: an open resolver can be abused
	// for amplification attacks if the port is ever exposed by mistake.
	if !s.allowed.Contains(client) {
		return nil, metrics.Local
	}
	if !s.lim.allow(client, s.now()) {
		return reply(req, dns.RcodeRefused), metrics.Local
	}
	if req.Response || len(req.Question) != 1 {
		return reply(req, dns.RcodeFormatError), metrics.Local
	}
	if req.Opcode != dns.OpcodeQuery {
		return reply(req, dns.RcodeNotImplemented), metrics.Local
	}
	q := req.Question[0]
	if q.Qclass != dns.ClassINET {
		return reply(req, dns.RcodeRefused), metrics.Local
	}
	if q.Qtype == dns.TypeAXFR || q.Qtype == dns.TypeIXFR || q.Qtype == dns.TypeANY {
		return reply(req, dns.RcodeNotImplemented), metrics.Local
	}
	fqdn := strings.ToLower(q.Name)
	name := intel.Normalize(fqdn)

	if fqdn == SelfCheckName {
		m := reply(req, dns.RcodeSuccess)
		m.Answer = append(m.Answer, &dns.TXT{Hdr: hdr(fqdn, dns.TypeTXT, 0), Txt: []string{"ok"}})
		return m, metrics.Local
	}
	if s.hostnames[name] {
		return s.answerSelf(req, client, fqdn, q.Qtype), metrics.Local
	}
	if s.isLocal(name) {
		return s.forwardLocal(req), metrics.Local
	}

	devID := s.eng.Identify(client, s.macOf(client))
	s.maybeNameDevice(devID, client)
	qtype := dns.TypeToString[q.Qtype]
	if qtype == "" {
		qtype = fmt.Sprint(q.Qtype)
	}

	// Canary names make browsers and Apple devices stop bypassing Vahti.
	if isCanary(name) && s.eng.CategoryAction(intel.Bypass) == engine.Block {
		s.eng.Record(engine.LogEntry{Device: devID, Domain: name, QType: qtype, Category: intel.Bypass,
			Action: engine.Block, Reason: "Browser's own encrypted DNS is not allowed"})
		return reply(req, dns.RcodeNameError), metrics.Blocked
	}

	m := s.db.Lookup(name)
	v := s.eng.Decide(devID, name, m)
	le := engine.LogEntry{Device: devID, Domain: name, QType: qtype, Category: v.Category, Action: v.Action, Reason: v.Reason, Queued: v.Queued}
	if v.Action == engine.Block {
		s.eng.Record(le)
		return blocked(req, fqdn, q.Qtype), metrics.Blocked
	}

	resp, src, err := s.resolve(req, fqdn)
	if err != nil {
		le.Reason = "Upstream resolver did not answer"
		s.eng.Record(le)
		return reply(req, dns.RcodeServerFailure), metrics.Failed
	}

	// CNAME cloaking: a first-party name that points to a tracker.
	for _, rr := range resp.Answer {
		c, ok := rr.(*dns.CNAME)
		if !ok {
			continue
		}
		target := intel.Normalize(c.Target)
		tm := s.db.Lookup(target)
		if !tm.Found() {
			continue
		}
		tv := s.eng.DecideCNAME(devID, target, tm)
		if tv.Action == engine.Block {
			le.Action, le.Category, le.Via = engine.Block, tv.Category, target
			le.Reason = "Disguised tracking (CNAME)"
			s.eng.Record(le)
			return blocked(req, fqdn, q.Qtype), metrics.Blocked
		}
	}

	if s.rebind && !s.rebindAllowed(name) {
		for _, rr := range resp.Answer {
			var ip net.IP
			switch a := rr.(type) {
			case *dns.A:
				ip = a.A
			case *dns.AAAA:
				ip = a.AAAA
			}
			if ip == nil {
				continue
			}
			if addr, ok := netip.AddrFromSlice(ip); ok && netutil.IsInternal(addr) {
				le.Action = engine.Block
				le.Reason = "Protection against attacks on the home network (DNS rebinding)"
				s.eng.Record(le)
				return reply(req, dns.RcodeNameError), src
			}
		}
	}
	s.eng.Record(le)
	return resp, src
}

// resolve answers from the cache or the upstream and builds the client response. An
// expired cached answer is given straight away and refreshed in the background
// (serve-stale), and a popular answer about to expire is refreshed early (prefetch).
func (s *Server) resolve(req *dns.Msg, fqdn string) (*dns.Msg, metrics.Source, error) {
	q := req.Question[0]
	do := false
	if o := req.IsEdns0(); o != nil {
		do = o.Do()
	}
	key := cacheKey(dns.Question{Name: fqdn, Qtype: q.Qtype, Qclass: q.Qclass}, do, req.CheckingDisabled)
	up, kind := s.cache.get(key)
	src := metrics.Cache
	switch kind {
	case miss:
		var err error
		if up, err = s.fetch(key, fqdn, q.Qtype, do, req.CheckingDisabled); err != nil {
			return nil, metrics.Failed, err
		}
		src = metrics.Upstream
	case stale:
		s.refresh(key, fqdn, q.Qtype, do, req.CheckingDisabled)
		src = metrics.Stale
	case prefetch:
		s.refresh(key, fqdn, q.Qtype, do, req.CheckingDisabled)
	}
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.RecursionAvailable = true
	resp.Rcode = up.Rcode
	resp.AuthenticatedData = up.AuthenticatedData && do
	resp.Answer = up.Answer
	resp.Ns = up.Ns
	for _, rr := range up.Extra {
		if rr.Header().Rrtype != dns.TypeOPT {
			resp.Extra = append(resp.Extra, rr)
		}
	}
	if o := req.IsEdns0(); o != nil {
		resp.SetEdns0(1232, do)
	}
	return resp, src, nil
}

// fetch asks the upstream a fresh question, checks that the answer matches it and stores
// it in the cache. No client EDNS options (e.g. client subnet) are passed on.
func (s *Server) fetch(key, fqdn string, qtype uint16, do, cd bool) (*dns.Msg, error) {
	fq := new(dns.Msg)
	fq.SetQuestion(fqdn, qtype)
	fq.RecursionDesired = true
	fq.CheckingDisabled = cd
	fq.SetEdns0(1232, do)
	up, err := s.upstream.Exchange(fq)
	if err != nil {
		return nil, err
	}
	if up == nil || len(up.Question) != 1 || !strings.EqualFold(up.Question[0].Name, fqdn) || up.Question[0].Qtype != qtype {
		return nil, errors.New("upstream answered a different question")
	}
	s.cache.put(key, up)
	return up, nil
}

// refresh fetches a cached name again in the background. Each name is refreshed at most
// once at a time, and at most maxRefreshes refreshes run together; extra ones are skipped
// (the next query will try again). If the refresh fails, the old answer stays in the cache.
func (s *Server) refresh(key, fqdn string, qtype uint16, do, cd bool) {
	if _, busy := s.refreshing.LoadOrStore(key, true); busy {
		return
	}
	select {
	case s.refreshSem <- struct{}{}:
	default:
		s.refreshing.Delete(key)
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("dns refresh panic", "err", r)
			}
			<-s.refreshSem
			s.refreshing.Delete(key)
		}()
		if _, err := s.fetch(key, fqdn, qtype, do, cd); err != nil {
			slog.Debug("background refresh failed", "name", fqdn, "err", err)
		}
	}()
}

// answerSelf answers queries for Vahti's own name (e.g. vahti.lan) with the address the
// client can reach Vahti at.
func (s *Server) answerSelf(req *dns.Msg, client netip.Addr, fqdn string, qtype uint16) *dns.Msg {
	m := reply(req, dns.RcodeSuccess)
	m.Authoritative = true
	ip, ok := s.localAddrFor(client)
	if !ok {
		return m
	}
	switch {
	case qtype == dns.TypeA && ip.Is4():
		m.Answer = append(m.Answer, &dns.A{Hdr: hdr(fqdn, dns.TypeA, 60), A: ip.AsSlice()})
	case qtype == dns.TypeAAAA && ip.Is6():
		m.Answer = append(m.Answer, &dns.AAAA{Hdr: hdr(fqdn, dns.TypeAAAA, 60), AAAA: ip.AsSlice()})
	}
	return m
}

// isLocal reports names that must never be sent to the internet: local
// domains, single-label names and reverse lookups of private addresses.
func (s *Server) isLocal(name string) bool {
	if !strings.Contains(name, ".") {
		return true
	}
	for _, d := range s.localDomains {
		if name == d || strings.HasSuffix(name, "."+d) {
			return true
		}
	}
	if a, ok := reverseAddr(name); ok {
		return netutil.IsInternal(a) || a.IsUnspecified()
	}
	return false
}

// forwardLocal sends a local-network name (e.g. printer.lan) to the router's DNS, if one is
// configured. Otherwise it answers "no such name". Local names never go to the internet.
func (s *Server) forwardLocal(req *dns.Msg) *dns.Msg {
	if s.local == nil {
		return reply(req, dns.RcodeNameError)
	}
	fq := req.Copy()
	fq.Question[0].Name = strings.ToLower(fq.Question[0].Name)
	r, err := s.local.Exchange(fq)
	if err != nil || r == nil {
		return reply(req, dns.RcodeServerFailure)
	}
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.RecursionAvailable = true
	resp.Rcode, resp.Answer, resp.Ns = r.Rcode, r.Answer, r.Ns
	return resp
}

// maybeNameDevice asks the router (via PTR) for a new device's DHCP hostname.
func (s *Server) maybeNameDevice(devID string, client netip.Addr) {
	if s.local == nil || client.IsLoopback() {
		return
	}
	if _, loaded := s.ptrTried.LoadOrStore(devID, true); loaded {
		return
	}
	go func() {
		rev, err := dns.ReverseAddr(client.String())
		if err != nil {
			return
		}
		m := new(dns.Msg)
		m.SetQuestion(rev, dns.TypePTR)
		r, err := s.local.Exchange(m)
		if err != nil || r == nil {
			return
		}
		for _, rr := range r.Answer {
			if p, ok := rr.(*dns.PTR); ok {
				n := intel.Normalize(p.Ptr)
				if i := strings.IndexByte(n, '.'); i > 0 {
					n = n[:i]
				}
				if n != "" && len(n) <= 60 {
					s.eng.SetDeviceNameHint(devID, n)
				}
				return
			}
		}
	}()
}

// rebindAllowed reports whether a domain is on the list allowed to resolve to local
// addresses (DNS rebinding protection makes exceptions only for these).
func (s *Server) rebindAllowed(name string) bool {
	for _, d := range s.rebindAllow {
		if name == d || strings.HasSuffix(name, "."+d) {
			return true
		}
	}
	return false
}

// isCanary reports the "canary" domains that browsers and Apple devices query to ask whether
// they may bypass the network's DNS (Firefox DoH, iCloud Private Relay). Vahti answers no.
func isCanary(name string) bool {
	switch name {
	case "use-application-dns.net", "mask.icloud.com", "mask-h2.icloud.com":
		return true
	}
	return false
}

// reverseAddr turns a reverse-lookup name (x.x.x.x.in-addr.arpa or ip6.arpa) back into the
// IP address it asks about.
func reverseAddr(name string) (netip.Addr, bool) {
	switch {
	case strings.HasSuffix(name, ".in-addr.arpa"):
		parts := strings.Split(strings.TrimSuffix(name, ".in-addr.arpa"), ".")
		if len(parts) != 4 {
			return netip.Addr{}, false
		}
		for i, j := 0, 3; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		a, err := netip.ParseAddr(strings.Join(parts, "."))
		return a, err == nil
	case strings.HasSuffix(name, ".ip6.arpa"):
		nib := strings.Split(strings.TrimSuffix(name, ".ip6.arpa"), ".")
		if len(nib) != 32 {
			return netip.Addr{}, false
		}
		var b strings.Builder
		for i := 31; i >= 0; i-- {
			b.WriteString(nib[i])
			if i%4 == 0 && i != 0 {
				b.WriteByte(':')
			}
		}
		a, err := netip.ParseAddr(b.String())
		return a, err == nil
	}
	return netip.Addr{}, false
}

// hdr builds a DNS record header with the given name, type and TTL.
func hdr(name string, t uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{Name: name, Rrtype: t, Class: dns.ClassINET, Ttl: ttl}
}

// reply creates an empty answer to req with the given result code.
func reply(req *dns.Msg, rcode int) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(req, rcode)
	m.RecursionAvailable = true
	return m
}

// blocked answers with an unroutable address so apps fail fast.
func blocked(req *dns.Msg, fqdn string, qtype uint16) *dns.Msg {
	m := reply(req, dns.RcodeSuccess)
	switch qtype {
	case dns.TypeA:
		m.Answer = append(m.Answer, &dns.A{Hdr: hdr(fqdn, dns.TypeA, 10), A: net.IPv4zero.To4()})
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{Hdr: hdr(fqdn, dns.TypeAAAA, 10), AAAA: net.IPv6zero})
	}
	return m
}

// SelfCheck queries the server over UDP at addr and reports whether it answered.
func SelfCheck(addr string) error {
	c := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
	m := new(dns.Msg)
	m.SetQuestion(SelfCheckName, dns.TypeTXT)
	r, _, err := c.Exchange(m, addr)
	if err != nil {
		return err
	}
	if r.Rcode != dns.RcodeSuccess || len(r.Answer) == 0 {
		return errors.New("unexpected self-check answer")
	}
	return nil
}
