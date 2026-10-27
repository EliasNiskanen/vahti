package metrics

import (
	"testing"
	"time"
)

// The summary gives the median, the 95th percentile, the cache share and the upstream
// median, and forgets measurements older than 15 minutes.
func TestSnapshot(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 30, 0, time.UTC)
	r := New()
	r.SetClock(func() time.Time { return now })
	if s := r.Snapshot(); s.Queries != 0 || s.MedianMs != 0 {
		t.Fatalf("empty: %+v", s)
	}
	for i := 0; i < 80; i++ {
		r.Observe(Cache, 50*time.Microsecond)
	}
	for i := 0; i < 15; i++ {
		r.Observe(Upstream, 15*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		r.Observe(Upstream, 300*time.Millisecond)
	}
	s := r.Snapshot()
	if s.Queries != 100 || s.MedianMs != 0.1 || s.P95Ms != 20 || s.UpstreamMs != 20 || s.CachePct != 80 {
		t.Fatalf("got %+v", s)
	}
	now = now.Add(14 * time.Minute)
	if s := r.Snapshot(); s.Queries != 100 {
		t.Fatalf("still within 15 minutes: %+v", s)
	}
	now = now.Add(time.Minute)
	r.Observe(Blocked, time.Millisecond)
	if s := r.Snapshot(); s.Queries != 1 || s.CachePct != 0 {
		t.Fatalf("old minutes should be forgotten: %+v", s)
	}
	var nilRec *Recorder
	nilRec.Observe(Cache, time.Second) // must not panic
}

// Very slow answers land in the last bucket and are reported as its lower limit.
func TestSlowAnswers(t *testing.T) {
	r := New()
	r.Observe(Upstream, 10*time.Second)
	if s := r.Snapshot(); s.MedianMs != 2000 {
		t.Fatalf("got %+v", s)
	}
}
