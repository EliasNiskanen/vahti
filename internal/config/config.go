// Package config loads Vahti's JSON configuration file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strings"

	"github.com/EliasNiskanen/vahti/internal/intel"
	"github.com/EliasNiskanen/vahti/internal/netutil"
)

// Config is the on-disk configuration (/etc/vahti/config.json).
type Config struct {
	DataDir string `json:"dataDir"`
	DNS     DNS    `json:"dns"`
	Web     Web    `json:"web"`
	Lists   []List `json:"lists"`
	// LogEntries is how many recent DNS decisions are kept in memory.
	// The log is never written to disk.
	LogEntries int `json:"logEntries"`
}

// DNS configures the resolver.
type DNS struct {
	Listen []string `json:"listen"`
	// Upstreams: "tls://9.9.9.9:853#dns.quad9.net", "udp://1.1.1.1:53", "tcp://..."
	Upstreams        []string `json:"upstreams"`
	AllowedClients   []string `json:"allowedClients"`
	LocalDomains     []string `json:"localDomains"`
	LocalResolver    string   `json:"localResolver"`
	RebindProtection bool     `json:"rebindProtection"`
	RebindAllow      []string `json:"rebindAllow"`
	CacheSize        int      `json:"cacheSize"`
	RateLimitQPS     int      `json:"rateLimitQPS"`
}

// Web configures the management interface.
type Web struct {
	Listen         string   `json:"listen"`
	RedirectListen string   `json:"redirectListen"`
	TLS            bool     `json:"tls"`
	Hostnames      []string `json:"hostnames"`
	AllowedClients []string `json:"allowedClients"`
}

// List is a downloadable blocklist.
type List struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

// Default returns production defaults for a Raspberry Pi on a home LAN.
func Default() Config {
	return Config{
		DataDir: "/var/lib/vahti",
		DNS: DNS{
			Listen:           []string{":53"},
			Upstreams:        []string{"tls://9.9.9.9:853#dns.quad9.net", "tls://149.112.112.112:853#dns.quad9.net"},
			AllowedClients:   netutil.DefaultLocalPrefixes,
			LocalDomains:     []string{"lan", "home.arpa", "local", "internal", "localdomain"},
			RebindProtection: true,
			CacheSize:        10000,
			RateLimitQPS:     50,
		},
		Web: Web{
			Listen:         ":443",
			RedirectListen: ":80",
			TLS:            true,
			Hostnames:      []string{"vahti.lan", "vahti.home.arpa"},
			AllowedClients: netutil.DefaultLocalPrefixes,
		},
		Lists: []List{
			{Name: "stevenblack", URL: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts", Category: intel.Ads},
			{Name: "urlhaus", URL: "https://urlhaus.abuse.ch/downloads/hostfile/", Category: intel.Threat},
		},
		LogEntries: 20000,
	}
}

// Load reads path over the defaults. A missing file yields defaults.
func Load(path string) (Config, bool, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, false, c.Validate()
	}
	if err != nil {
		return c, false, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, true, fmt.Errorf("%s: %w", path, err)
	}
	return c, true, c.Validate()
}

// Upstream is a parsed upstream resolver.
type Upstream struct {
	Net        string // udp, tcp, tcp-tls
	Addr       string // host:port
	ServerName string
}

// ParseUpstream parses "tls://ip:port#name", "udp://ip:port", "tcp://ip:port".
func ParseUpstream(s string) (Upstream, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Upstream{}, err
	}
	var up Upstream
	switch u.Scheme {
	case "udp":
		up.Net = "udp"
	case "tcp":
		up.Net = "tcp"
	case "tls":
		up.Net = "tcp-tls"
		up.ServerName = u.Fragment
		if up.ServerName == "" {
			return up, fmt.Errorf("upstream %q: tls needs #servername for certificate checks", s)
		}
	default:
		return up, fmt.Errorf("upstream %q: scheme must be udp, tcp or tls", s)
	}
	ap, err := netip.ParseAddrPort(u.Host)
	if err != nil {
		return up, fmt.Errorf("upstream %q: use an IP address and port: %v", s, err)
	}
	up.Addr = ap.String()
	return up, nil
}

// Validate checks the configuration.
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return errors.New("dataDir is empty")
	}
	if len(c.DNS.Upstreams) == 0 {
		return errors.New("dns.upstreams is empty")
	}
	for _, s := range c.DNS.Upstreams {
		if _, err := ParseUpstream(s); err != nil {
			return err
		}
	}
	if _, err := netutil.ParsePrefixes(c.DNS.AllowedClients); err != nil {
		return fmt.Errorf("dns.allowedClients: %w", err)
	}
	if _, err := netutil.ParsePrefixes(c.Web.AllowedClients); err != nil {
		return fmt.Errorf("web.allowedClients: %w", err)
	}
	if c.DNS.LocalResolver != "" {
		if _, err := netip.ParseAddrPort(c.DNS.LocalResolver); err != nil {
			return fmt.Errorf("dns.localResolver must be ip:port: %w", err)
		}
	}
	for _, h := range c.Web.Hostnames {
		if !intel.ValidName(intel.Normalize(h)) {
			return fmt.Errorf("web.hostnames: invalid name %q", h)
		}
	}
	for _, l := range c.Lists {
		u, err := url.Parse(l.URL)
		if err != nil || u.Scheme != "https" {
			return fmt.Errorf("list %q: url must use https", l.Name)
		}
		if !intel.ValidCategory(l.Category) {
			return fmt.Errorf("list %q: unknown category %q", l.Name, l.Category)
		}
		if l.Name == "" || strings.ContainsAny(l.Name, `/\.`) {
			return fmt.Errorf("list name %q must be a simple word", l.Name)
		}
	}
	if c.DNS.CacheSize < 0 || c.DNS.RateLimitQPS < 0 || c.LogEntries < 0 {
		return errors.New("negative sizes are not allowed")
	}
	return nil
}
