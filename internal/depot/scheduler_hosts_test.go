package depot

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// STEAM-6: growth must respect host slots and report real hosts.
// 2 hosts x MaxPerHost 4 = 8 slots; workers must never exceed 8 even with
// MaxConns 32, and Hosts must be hosts actually used (<=2), not total.
func TestSchedulerCapsConnectionsByHosts(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 8, 8) // 64 chunks
	hosts, _ := startHosts(t, m, encMap, []time.Duration{5 * time.Millisecond, 5 * time.Millisecond}, -1, -1, "")
	if len(hosts) != 2 {
		t.Fatalf("want 2 hosts, got %d", len(hosts))
	}
	s := NewScheduler(NewClient(), SchedulerConfig{InitialConns: 8, MaxConns: 32, MaxPerHost: 4, GrowEvery: 10 * time.Millisecond})
	var peakConns int32
	var peakHosts int
	onProg := func(p Progress) {
		for {
			old := atomic.LoadInt32(&peakConns)
			if int32(p.Connections) <= old || atomic.CompareAndSwapInt32(&peakConns, old, int32(p.Connections)) {
				break
			}
		}
		if p.Hosts > peakHosts {
			peakHosts = p.Hosts
		}
		if p.Connections > 8 {
			t.Errorf("connections %d exceed host slots 8", p.Connections)
		}
		if p.Hosts > 2 {
			t.Errorf("hosts %d exceed real hosts 2", p.Hosts)
		}
	}
	if err := s.DownloadStream(context.Background(), m, 11, hosts, key, nil, func(id string, data []byte) error { return nil }, onProg); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&peakConns); got > 8 {
		t.Fatalf("peak connections %d > 8 host slots", got)
	}
	if peakHosts > 2 {
		t.Fatalf("peak hosts %d > 2", peakHosts)
	}
}
