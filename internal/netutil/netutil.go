// Package netutil holds small network helpers: client address checks,
// ARP-table lookups and finding the local address a client can reach.
package netutil

import (
	"bufio"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultLocalPrefixes are the address ranges treated as "inside the home network".
var DefaultLocalPrefixes = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"fc00::/7", "fe80::/10",
}

// PrefixSet is an immutable list of prefixes.
type PrefixSet []netip.Prefix

// ParsePrefixes parses CIDR strings. Invalid entries return an error.
func ParsePrefixes(in []string) (PrefixSet, error) {
	out := make(PrefixSet, 0, len(in))
	for _, s := range in {
		p, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// Contains reports whether addr is inside any prefix.
func (ps PrefixSet) Contains(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// AddrFromNetAddr extracts the IP from a net.Addr (UDP or TCP).
func AddrFromNetAddr(na net.Addr) (netip.Addr, bool) {
	switch v := na.(type) {
	case *net.UDPAddr:
		a, ok := netip.AddrFromSlice(v.IP)
		return a.Unmap(), ok
	case *net.TCPAddr:
		a, ok := netip.AddrFromSlice(v.IP)
		return a.Unmap(), ok
	}
	ap, err := netip.ParseAddrPort(na.String())
	if err != nil {
		return netip.Addr{}, false
	}
	return ap.Addr().Unmap(), true
}

// IsInternal reports whether an upstream DNS answer address points inside a
// private network. Used for DNS rebinding protection.
func IsInternal(a netip.Addr) bool {
	a = a.Unmap()
	if a.IsUnspecified() {
		return false // 0.0.0.0 is what blocklists return; harmless
	}
	return a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() ||
		netip.MustParsePrefix("100.64.0.0/10").Contains(a)
}

// ARP reads the kernel neighbour table (/proc/net/arp) with a short cache.
type ARP struct {
	Path string
	mu   sync.Mutex
	at   time.Time
	tab  map[netip.Addr]string
}

// NewARP returns an ARP reader for the Linux kernel table.
func NewARP() *ARP { return &ARP{Path: "/proc/net/arp"} }

// Lookup returns the MAC address for ip, or "" when unknown.
func (a *ARP) Lookup(ip netip.Addr) string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if mac, ok := a.tab[ip]; ok && time.Since(a.at) < 30*time.Second {
		return mac
	}
	if time.Since(a.at) < 2*time.Second && a.tab != nil {
		return a.tab[ip]
	}
	a.tab = readARP(a.Path)
	a.at = time.Now()
	return a.tab[ip]
}

func readARP(path string) map[netip.Addr]string {
	out := map[netip.Addr]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fs := strings.Fields(sc.Text())
		if len(fs) < 4 || fs[2] == "0x0" {
			continue
		}
		ip, err := netip.ParseAddr(fs[0])
		if err != nil {
			continue
		}
		mac := strings.ToLower(fs[3])
		if _, err := net.ParseMAC(mac); err != nil || mac == "00:00:00:00:00:00" {
			continue
		}
		out[ip] = mac
	}
	return out
}

// LocalAddrFor returns this machine's address on the same subnet as client,
// falling back to the first non-loopback address of the same family.
func LocalAddrFor(client netip.Addr) (netip.Addr, bool) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return netip.Addr{}, false
	}
	var fallback netip.Addr
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(ipn.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		ones, _ := ipn.Mask.Size()
		p := netip.PrefixFrom(ip, ones).Masked()
		if p.Contains(client) {
			return ip, true
		}
		if !fallback.IsValid() && !ip.IsLoopback() && ip.Is4() == client.Is4() && !ip.IsLinkLocalUnicast() {
			fallback = ip
		}
	}
	if client.IsLoopback() {
		return client, true
	}
	return fallback, fallback.IsValid()
}
