package playlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) *Log {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func at(unix int64) time.Time { return time.Unix(unix, 0) }

func TestClosedSessionCountsMinutesAndLastPlayed(t *testing.T) {
	l := openTest(t)
	l.StartAt("g", at(1000))
	dur, ok := l.FinishAt("g", at(1000+125))
	if !ok || dur != 125*time.Second {
		t.Fatalf("finish = %v, %v; want 2m5s, true", dur, ok)
	}
	if got := l.Minutes("g"); got != 2 {
		t.Fatalf("minutes = %d, want 2", got)
	}
	if got := l.LastPlayed("g"); got != 1125 {
		t.Fatalf("last played = %d, want 1125", got)
	}
	if got := l.Recent("g", 5); len(got) != 1 || got[0].Seconds != 125 {
		t.Fatalf("recent = %+v, want one 125 s session", got)
	}
}

func TestShortSessionIsIgnoredForTotalsButKeepsLastPlayed(t *testing.T) {
	l := openTest(t)
	// A crash on launch: 5 s is under MinSessionSec.
	l.StartAt("g", at(2000))
	if _, ok := l.FinishAt("g", at(2005)); !ok {
		t.Fatal("no session was closed")
	}
	if got := l.Minutes("g"); got != 0 {
		t.Fatalf("minutes = %d, want 0", got)
	}
	if got := l.LastPlayed("g"); got != 2005 {
		t.Fatalf("last played = %d, want 2005", got)
	}
	if got := l.Recent("g", 5); len(got) != 0 {
		t.Fatalf("a short session was kept: %+v", got)
	}

	// Exactly the minimum counts.
	l.StartAt("g", at(3000))
	l.FinishAt("g", at(3000+MinSessionSec))
	if got := l.TotalSec("g"); got != MinSessionSec {
		t.Fatalf("total = %d s, want %d", got, MinSessionSec)
	}
}

func TestFinishWithoutAStartIsANoOp(t *testing.T) {
	l := openTest(t)
	if _, ok := l.Finish("nobody"); ok {
		t.Fatal("a game that never started closed a session")
	}
	// A failure the tracker reports without a running state (e.g. the store
	// client never started the game) must not invent playtime either.
	if got := l.Minutes("nobody"); got != 0 || l.LastPlayed("nobody") != 0 {
		t.Fatalf("minutes %d, last %d; want 0, 0", got, l.LastPlayed("nobody"))
	}
}

func TestHeartbeatRecoveryAfterASimulatedCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "play-sessions.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// The game runs; heartbeats land every minute; then the core dies
	// mid-session without a finish.
	l.StartAt("g", at(10000))
	l.HeartbeatAt("g", at(10060))
	l.HeartbeatAt("g", at(10120))

	crashed, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := crashed.Recovered(); got != 1 {
		t.Fatalf("recovered %d sessions, want 1", got)
	}
	// The session closes at the last heartbeat: 120 s, not the 180 s the
	// clock would say if it ran on.
	if got := crashed.TotalSec("g"); got != 120 {
		t.Fatalf("total = %d s, want 120", got)
	}
	if got := crashed.Minutes("g"); got != 2 {
		t.Fatalf("minutes = %d, want 2", got)
	}
	if got := crashed.LastPlayed("g"); got != 10120 {
		t.Fatalf("last played = %d, want 10120", got)
	}
	if _, ok := crashed.FinishAt("g", at(10200)); ok {
		t.Fatal("the recovered session is still open")
	}

	// A restart with nothing dangling recovers nothing and keeps the totals.
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Recovered(); got != 0 {
		t.Fatalf("recovered %d sessions, want 0", got)
	}
	if got := again.TotalSec("g"); got != 120 {
		t.Fatalf("total = %d s, want 120", got)
	}
}

func TestDanglingShortSessionKeepsLastPlayedOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "play-sessions.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	l.StartAt("g", at(5000))
	// No heartbeat past the start: the crash loses everything but the fact
	// that the game ran.
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Recovered(); got != 1 {
		t.Fatalf("recovered %d, want 1", got)
	}
	if got := again.TotalSec("g"); got != 0 {
		t.Fatalf("total = %d s, want 0", got)
	}
	if got := again.LastPlayed("g"); got != 5000 {
		t.Fatalf("last played = %d, want 5000", got)
	}
}

func TestRecentSessionsAreBoundedButTotalsKeepEverything(t *testing.T) {
	l := openTest(t)
	l.maxRecent = 3
	base := int64(100000)
	for i := 0; i < 10; i++ {
		l.StartAt("g", at(base+int64(i*1000)))
		l.FinishAt("g", at(base+int64(i*1000+60)))
	}
	if got := l.TotalSec("g"); got != 600 {
		t.Fatalf("total = %d s, want 600", got)
	}
	got := l.Recent("g", 100)
	if len(got) != 3 {
		t.Fatalf("kept %d sessions, want 3", len(got))
	}
	if got[0].Start != base+7*1000 || got[2].Start != base+9*1000 {
		t.Fatalf("kept the wrong sessions: %+v", got)
	}
	// The bound survives a reload.
	again, err := Open(l.path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.TotalSec("g"); got != 600 {
		t.Fatalf("reloaded total = %d s, want 600", got)
	}
	if got := len(again.Recent("g", 100)); got != 3 {
		t.Fatalf("reloaded %d sessions, want 3", got)
	}
}

func TestGamesAreBoundedByOldestLastPlayed(t *testing.T) {
	l := openTest(t)
	l.maxGames = 3
	for i, g := range []string{"a", "b", "c", "d"} {
		start := int64(1000 + i*100)
		l.StartAt(g, at(start))
		l.FinishAt(g, at(start+60))
	}
	if got := l.Minutes("a"); got != 0 {
		t.Fatalf("the oldest game survived the bound: %d minutes", got)
	}
	for _, g := range []string{"b", "c", "d"} {
		if got := l.Minutes(g); got != 1 {
			t.Fatalf("%s minutes = %d, want 1", g, got)
		}
	}
}

func TestConcurrentStartsAndFinishes(t *testing.T) {
	l := openTest(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g := fmt.Sprintf("g%d", i%4)
			for j := 0; j < 25; j++ {
				l.Start(g)
				l.Heartbeat(g)
				l.Finish(g)
			}
		}(i)
	}
	wg.Wait()
}

func TestCorruptFileIsMovedAsideNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "play-sessions.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err == nil {
		t.Fatal("a corrupt log should be reported")
	}
	l.StartAt("g", at(1000))
	if _, ok := l.FinishAt("g", at(1005)); !ok {
		t.Fatal("the fresh log recorded nothing")
	}
	if got := l.TotalSec("g"); got != 0 {
		t.Fatalf("a 5 s session counted %d s", got)
	}
	moved, _ := filepath.Glob(path + ".bad-*")
	if len(moved) != 1 {
		t.Fatalf("corrupt file not moved aside: %v", moved)
	}
	if raw, _ := os.ReadFile(moved[0]); string(raw) != "{broken" {
		t.Fatalf("moved file changed: %q", raw)
	}
}

func TestAnUnreadableLogIsNeverWrittenOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "play-sessions.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err == nil {
		t.Fatal("an unreadable log should be reported")
	}
	l.StartAt("g", at(1000))
	if _, ok := l.FinishAt("g", at(1100)); ok {
		t.Fatal("the read-only log recorded a session")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("the log file was replaced: %v %v", info, err)
	}
}
