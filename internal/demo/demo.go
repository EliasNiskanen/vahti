// Package demo generates simulated home-network traffic for trying Vahti on
// a desktop. Every simulated query goes through the real DNS pipeline
// (filtering, queue, CNAME checks); only the devices and the upstream
// resolver are fake. Nothing is sent to the internet in demo mode.
package demo

import (
	"context"
	"math/rand/v2"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/EliasNiskanen/vahti/internal/engine"
	"github.com/EliasNiskanen/vahti/internal/intel"
)

type dev struct {
	ip, mac, name, kind string
	mode                engine.Mode
	dests               []dest
}

type dest struct {
	name   string
	weight int
}

// The fictional companies below are marked as such in the intel database.
var devices = []dev{
	{"192.168.1.14", "02:00:00:00:00:14", "Living room TV", "tv", engine.Locked, []dest{
		{"api-global.netflix.com", 5}, {"samsungacr.com", 3}, {"log-config.samsungacr.com", 2}, {"samsungads.com", 2},
		{"doubleclick.net", 2}, {"fw.tv-vendor-cloud.example", 1}, {"beacon.channel-stats.example", 1}}},
	{"192.168.1.21", "02:00:00:00:00:21", "My phone", "phone", engine.Open, []dest{
		{"g.whatsapp.net", 4}, {"app-measurement.com", 3}, {"graph.facebook.com", 2}, {"connect.facebook.net", 2},
		{"firebaselogging-pa.googleapis.com", 2}, {"sdk.shopmetrics.example", 2}, {"mobile.bank.example", 2}, {"api.adjust.com", 1}, {"dns.google", 1}}},
	{"192.168.1.33", "02:00:00:00:00:33", "Laptop", "computer", engine.Open, []dest{
		{"www.yle.fi", 3}, {"www.google-analytics.com", 3}, {"cdn.cart-tracker.example", 2}, {"metrics.shop.example", 2},
		{"bat.bing.com", 1}, {"upd-cdn-sync.example", 1}, {"use-application-dns.net", 1}, {"vortex.data.microsoft.com", 1}}},
	{"192.168.1.40", "02:00:00:00:00:40", "Robot vacuum", "appliance", engine.Locked, []dest{
		{"api.robovac-cloud.example", 3}, {"map-upload.robovac-cloud.example", 2}, {"log.robovac-cloud.example", 2}}},
	{"192.168.1.52", "02:00:00:00:00:52", "Doorbell camera", "camera", engine.Locked, []dest{
		{"stream.doorcam-cloud.example", 3}, {"events.doorcam-cloud.example", 2}, {"face-match.doorcam-cloud.example", 1}}},
	{"192.168.1.61", "02:00:00:00:00:61", "Smart speaker", "speaker", engine.Learning, []dest{
		{"voice.speaker-cloud.example", 3}, {"metrics.speaker-cloud.example", 2}, {"device-metrics-us.amazon.com", 2}}},
}

// cname simulates first-party names that are really trackers (CNAME cloaking).
var cname = map[string]string{
	"metrics.shop.example": "shop.cart-tracker.example",
}

// SeedIntel adds the fictional demo companies to the intel database.
func SeedIntel(db *intel.DB) {
	db.Add("shopmetrics.example", intel.Entry{Category: intel.Analytics, Owner: "ShopMetrics (fictional)", Note: "Shopping-behaviour analytics SDK embedded in store apps", Source: "Demo data"})
	db.Add("cart-tracker.example", intel.Entry{Category: intel.Analytics, Owner: "ShopMetrics (fictional)", Note: "Tracking script on online stores", Source: "Demo data"})
	db.Add("channel-stats.example", intel.Entry{Category: intel.Telemetry, Owner: "Channel Stats (fictional)", Note: "TV viewing measurement", Source: "Demo data"})
	db.Add("upd-cdn-sync.example", intel.Entry{Category: intel.Threat, Note: "Known malware command server (example)", Source: "Demo data"})
}

// Upstream answers every query with a documentation address (RFC 5737).
func Upstream() func(*dns.Msg) (*dns.Msg, error) {
	return func(q *dns.Msg) (*dns.Msg, error) {
		m := new(dns.Msg)
		m.SetReply(q)
		m.RecursionAvailable = true
		name := q.Question[0].Name
		n := strings.TrimSuffix(name, ".")
		if t, ok := cname[n]; ok {
			m.Answer = append(m.Answer, &dns.CNAME{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300}, Target: t + "."})
			name = t + "."
		}
		if q.Question[0].Qtype == dns.TypeA {
			ip := net.IPv4(203, 0, 113, byte(1+rand.IntN(250)))
			m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 30}, A: ip})
		}
		return m, nil
	}
}

// MACOf maps demo IPs to fake MAC addresses.
func MACOf(a netip.Addr) string {
	for _, d := range devices {
		if d.ip == a.String() {
			return d.mac
		}
	}
	return ""
}

// Processor runs a query through the real pipeline.
type Processor interface {
	Process(client netip.Addr, req *dns.Msg) *dns.Msg
}

// Prepare names the demo devices and sets their modes.
func Prepare(eng *engine.Engine) {
	for _, d := range devices {
		ip := netip.MustParseAddr(d.ip)
		id := eng.Identify(ip, d.mac)
		name, kind, mode := d.name, d.kind, d.mode
		u := engine.DeviceUpdate{Name: &name, Kind: &kind, Mode: &mode}
		if mode == engine.Learning {
			h := 1
			u.LearnHours = &h
		}
		if dv, ok := eng.Device(id); ok && dv.Name == d.name {
			continue // keep the user's changes between demo runs
		}
		_, _ = eng.UpdateDevice(id, u)
	}
}

// Run generates traffic until ctx ends. history queries are sent at once.
func Run(ctx context.Context, p Processor, history int) {
	send := func() {
		d := devices[rand.IntN(len(devices))]
		total := 0
		for _, x := range d.dests {
			total += x.weight
		}
		r := rand.IntN(total)
		var name string
		for _, x := range d.dests {
			r -= x.weight
			if r < 0 {
				name = x.name
				break
			}
		}
		q := new(dns.Msg)
		q.SetQuestion(dns.Fqdn(name), dns.TypeA)
		p.Process(netip.MustParseAddr(d.ip), q)
	}
	for i := 0; i < history; i++ {
		send()
	}
	t := time.NewTicker(1500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			send()
		}
	}
}
