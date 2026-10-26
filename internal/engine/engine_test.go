package engine

import (
	"net/netip"
	"testing"
	"time"

	"github.com/EliasNiskanen/vahti/internal/intel"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTest(t *testing.T) (*Engine, *clock, string) {
	t.Helper()
	db := intel.New()
	e := New(nil, db, 1000)
	c := &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	e.SetClock(c.now)
	id := e.Identify(netip.MustParseAddr("192.168.1.10"), "aa:bb:cc:dd:ee:ff")
	return e, c, id
}

func decide(e *Engine, dev, name string) Verdict {
	return e.Decide(dev, name, e.DB().Lookup(name))
}

func TestKnownTrackerBlockedByDefault(t *testing.T) {
	e, _, dev := newTest(t)
	v := decide(e, dev, "stats.g.doubleclick.net")
	if v.Action != Block || v.Category != intel.Ads {
		t.Fatalf("got %+v, want ads block", v)
	}
	if v := decide(e, dev, "www.yle.fi"); v.Action != Allow {
		t.Fatalf("unknown name on open device should be allowed, got %+v", v)
	}
}

func TestThreatCannotBeAllowed(t *testing.T) {
	e, _, dev := newTest(t)
	e.DB().Add("evil.example", intel.Entry{Category: intel.Threat})
	if _, err := e.AddRule(dev, "evil.example", Allow, 0, false, "user"); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "x.evil.example"); v.Action != Block {
		t.Fatalf("threat must stay blocked even with an allow rule, got %+v", v)
	}
	if err := e.UpdateSettings(SettingsUpdate{Categories: map[string]Action{intel.Threat: Allow}}); err == nil {
		t.Fatal("allowing the threat category must be rejected")
	}
	if _, err := e.Pause(30); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "evil.example"); v.Action != Block {
		t.Fatal("threats must be blocked during a pause")
	}
}

func TestRulePrecedence(t *testing.T) {
	e, _, dev := newTest(t)
	mustRule(t, e, "*", "example.com", Block)
	mustRule(t, e, dev, "example.com", Allow)
	if v := decide(e, dev, "a.example.com"); v.Action != Allow {
		t.Fatalf("device rule should beat global rule, got %+v", v)
	}
	mustRule(t, e, dev, "ads.example.com", Block)
	if v := decide(e, dev, "x.ads.example.com"); v.Action != Block {
		t.Fatalf("more specific rule should win, got %+v", v)
	}
	if v := decide(e, dev, "notexample.com"); v.Action != Allow {
		t.Fatalf("suffix match must respect label boundaries, got %+v", v)
	}
	// a user allow rule overrides a category block
	mustRule(t, e, dev, "doubleclick.net", Allow)
	if v := decide(e, dev, "doubleclick.net"); v.Action != Allow {
		t.Fatalf("allow rule should override category, got %+v", v)
	}
}

func mustRule(t *testing.T, e *Engine, dev, dom string, a Action) {
	t.Helper()
	if _, err := e.AddRule(dev, dom, a, 0, false, "user"); err != nil {
		t.Fatal(err)
	}
}

func TestOnceAndTimedRules(t *testing.T) {
	e, c, dev := newTest(t)
	locked := Locked
	if _, err := e.UpdateDevice(dev, DeviceUpdate{Mode: &locked}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddRule(dev, "cloud.example", Allow, 0, true, "queue"); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "cloud.example"); v.Action != Allow || v.Reason != "Kertalupa" {
		t.Fatalf("first use of once rule: %+v", v)
	}
	if v := decide(e, dev, "cloud.example"); v.Action != Block || !v.Queued {
		t.Fatalf("once rule must be consumed, got %+v", v)
	}
	if _, err := e.AddRule(dev, "share.example", Allow, 1, false, "share"); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "share.example"); v.Action != Allow {
		t.Fatalf("timed rule active: %+v", v)
	}
	c.t = c.t.Add(61 * time.Minute)
	if v := decide(e, dev, "share.example"); v.Action != Block {
		t.Fatalf("timed rule must expire, got %+v", v)
	}
}

func TestLockedQueueAndResolve(t *testing.T) {
	e, _, dev := newTest(t)
	locked := Locked
	_, _ = e.UpdateDevice(dev, DeviceUpdate{Mode: &locked})
	for i := 0; i < 3; i++ {
		decide(e, dev, "map-upload.vac.example")
	}
	decide(e, dev, "api.vac.example")
	q := e.Queue()
	var item QueueItem
	for _, it := range q {
		if it.Domain == "map-upload.vac.example" {
			item = it
		}
	}
	if len(q) != 2 || item.Count != 3 {
		t.Fatalf("unexpected queue %+v", q)
	}
	if _, err := e.Resolve(item.ID, "block", "site"); err != nil {
		t.Fatal(err)
	}
	if e.QueueLen() != 0 {
		t.Fatal("site decision should clear sibling queue items")
	}
	if v := decide(e, dev, "api.vac.example"); v.Action != Block || v.Queued {
		t.Fatalf("site rule should block without queueing, got %+v", v)
	}
	if _, err := e.Resolve("nope", "allow", "name"); err == nil {
		t.Fatal("unknown queue id must fail")
	}
}

func TestLearningLocksAfterwards(t *testing.T) {
	e, c, dev := newTest(t)
	m := Learning
	h := 2
	if _, err := e.UpdateDevice(dev, DeviceUpdate{Mode: &m, LearnHours: &h}); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "api.vendor.example"); v.Action != Allow {
		t.Fatal(v)
	}
	c.t = c.t.Add(3 * time.Hour)
	if v := decide(e, dev, "eu.api.vendor.example"); v.Action != Allow || v.Reason != "Opittu kohde" {
		t.Fatalf("learned site should stay allowed: %+v", v)
	}
	if v := decide(e, dev, "new.other.example"); v.Action != Block || !v.Queued {
		t.Fatalf("device should be locked after learning: %+v", v)
	}
	if d, _ := e.Device(dev); d.Mode != Locked {
		t.Fatalf("mode = %s", d.Mode)
	}
}

func TestCategoryOverrideAndCNAME(t *testing.T) {
	e, _, dev := newTest(t)
	allow := Allow
	if _, err := e.UpdateDevice(dev, DeviceUpdate{CatOverride: map[string]*Action{intel.Analytics: &allow}}); err != nil {
		t.Fatal(err)
	}
	if v := decide(e, dev, "google-analytics.com"); v.Action != Allow {
		t.Fatalf("override should allow: %+v", v)
	}
	if v := e.DecideCNAME(dev, "unknown.example", intel.Match{}); v.Action != Allow || e.QueueLen() != 0 {
		t.Fatal("unknown CNAME targets are never queued")
	}
	if _, err := e.UpdateDevice(dev, DeviceUpdate{CatOverride: map[string]*Action{intel.Threat: &allow}}); err == nil {
		t.Fatal("threat override must be rejected")
	}
}

func TestValidation(t *testing.T) {
	e, _, dev := newTest(t)
	bad := []string{"", "localhost", "a..b", "-x.com", "x.com/evil", "<script>.com", "a b.com"}
	for _, d := range bad {
		if _, err := e.AddRule(dev, d, Block, 0, false, "user"); err == nil {
			t.Errorf("AddRule(%q) should fail", d)
		}
	}
	long := string(make([]rune, 61))
	if _, err := e.UpdateDevice(dev, DeviceUpdate{Name: &long}); err == nil {
		t.Error("too long name accepted")
	}
	k := "toaster"
	if _, err := e.UpdateDevice(dev, DeviceUpdate{Kind: &k}); err == nil {
		t.Error("unknown kind accepted")
	}
	if _, err := e.Pause(-1); err == nil {
		t.Error("negative pause accepted")
	}
}

func TestLogRingAndSnapshot(t *testing.T) {
	db := intel.New()
	e := New(nil, db, 100)
	for i := 0; i < 250; i++ {
		e.Record(LogEntry{Device: "d", Domain: "x.example", Action: Allow})
	}
	if got := len(e.Log(LogFilter{Limit: 1000})); got != 100 {
		t.Fatalf("ring should hold 100 entries, got %d", got)
	}
	if l := e.Log(LogFilter{Limit: 1}); l[0].Seq != 250 {
		t.Fatalf("newest first, got seq %d", l[0].Seq)
	}
	e.Identify(netip.MustParseAddr("10.0.0.2"), "")
	s, ok := e.Snapshot(false)
	if !ok || len(s.Devices) != 1 {
		t.Fatal("snapshot after change")
	}
	if _, ok := e.Snapshot(false); ok {
		t.Fatal("snapshot without changes should report unchanged")
	}
}

func TestStateIsBounded(t *testing.T) {
	e, _, _ := newTest(t)
	for i := 0; i < MaxDevices+50; i++ {
		a := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		e.Identify(a, "")
	}
	if n := len(e.Devices()); n != MaxDevices {
		t.Fatalf("devices = %d, want cap %d", n, MaxDevices)
	}
	if id := e.Identify(netip.MustParseAddr("10.200.0.1"), ""); id != OverflowDevice {
		t.Fatalf("got %s", id)
	}
	if v := decide(e, OverflowDevice, "doubleclick.net"); v.Action != Block {
		t.Fatal("overflow clients are still filtered")
	}
}
