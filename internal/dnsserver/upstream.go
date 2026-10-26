package dnsserver

import (
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/config"
)

// upstream forwards queries to one resolver, reusing TCP/TLS connections.
type upstream struct {
	cfg    config.Upstream
	client *dns.Client
	pool   chan *dns.Conn
}

func newUpstream(u config.Upstream) *upstream {
	c := &dns.Client{Net: u.Net, Timeout: 3 * time.Second, UDPSize: 1232}
	if u.Net == "tcp-tls" {
		c.TLSConfig = &tls.Config{ServerName: u.ServerName, MinVersion: tls.VersionTLS12}
	}
	return &upstream{cfg: u, client: c, pool: make(chan *dns.Conn, 4)}
}

func (u *upstream) exchange(m *dns.Msg) (*dns.Msg, error) {
	if u.cfg.Net == "udp" {
		r, _, err := u.client.Exchange(m, u.cfg.Addr)
		if err == nil && r.Truncated {
			tcp := &dns.Client{Net: "tcp", Timeout: u.client.Timeout}
			r, _, err = tcp.Exchange(m, u.cfg.Addr)
		}
		return r, err
	}
	// Stream transports: try a pooled connection first, then a fresh one.
	for attempt := 0; attempt < 2; attempt++ {
		var conn *dns.Conn
		fresh := false
		select {
		case conn = <-u.pool:
		default:
		}
		if conn == nil {
			var err error
			conn, err = u.client.Dial(u.cfg.Addr)
			if err != nil {
				return nil, err
			}
			fresh = true
		}
		r, _, err := u.client.ExchangeWithConn(m, conn)
		if err != nil {
			conn.Close()
			if fresh {
				return nil, err
			}
			continue // stale pooled connection; retry
		}
		select {
		case u.pool <- conn:
		default:
			conn.Close()
		}
		return r, nil
	}
	return nil, errors.New("upstream unavailable")
}

// forwarder tries upstreams in order.
type forwarder struct {
	ups []*upstream
	mu  sync.Mutex
	pos int // index of the last upstream that worked
}

func (f *forwarder) exchange(m *dns.Msg) (*dns.Msg, error) {
	f.mu.Lock()
	start := f.pos
	f.mu.Unlock()
	var lastErr error
	for i := 0; i < len(f.ups); i++ {
		idx := (start + i) % len(f.ups)
		q := m.Copy()
		q.Id = dns.Id()
		r, err := f.ups[idx].exchange(q)
		if err != nil {
			lastErr = err
			continue
		}
		if r.Id != q.Id {
			lastErr = errors.New("upstream answered with a wrong id")
			continue
		}
		f.mu.Lock()
		f.pos = idx
		f.mu.Unlock()
		return r, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no upstreams")
	}
	return nil, lastErr
}
