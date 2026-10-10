package social

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// A friends list turned off and on again must reach the UI as newer views.
// The UI keeps the view with the highest seq, so a restarted subscription
// that counts from 1 again would have every view dropped while the old
// subscription's higher seq is held: the list stays off.
func TestSeqRisesAcrossOffOnCycles(t *testing.T) {
	f := newFake()
	var mu sync.Mutex
	var seen []View
	m := NewManager(Options{
		Backend:    f,
		Enabled:    func(string) bool { return true },
		Emit:       func(v View) { mu.Lock(); seen = append(seen, v); mu.Unlock() },
		MinBackoff: time.Millisecond,
		MaxBackoff: 5 * time.Millisecond,
	})
	// Cycles that switch off while the store is still connecting, and cycles
	// that switch off once it is connected and has pushed its friends.
	connected := 0
	for cycle := 0; cycle < 6; cycle++ {
		m.Start("echo")
		if cycle%2 == 1 {
			m.Stop("echo")
			continue
		}
		connected++
		waitFor(t, "connected", func() bool { return f.count(MethodState) >= connected })
		m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"1","name":"Ana","state":"online"}]}`))
		m.Stop("echo")
	}
	m.Start("echo")
	defer m.StopAll()
	connected++
	waitFor(t, "connected again", func() bool { return f.count(MethodState) >= connected })

	mu.Lock()
	defer mu.Unlock()
	var last uint64
	for i, v := range seen {
		if v.Seq <= last {
			t.Fatalf("view %d has seq %d after %d: the UI would drop it (running=%v enabled=%v)", i, v.Seq, last, v.Running, v.Enabled)
		}
		last = v.Seq
	}
}
