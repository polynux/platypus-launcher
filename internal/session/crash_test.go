package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"platypus/internal/types"
)

// waitFor drains events until one arrives in state, or fails the test.
func waitFor(t *testing.T, events <-chan types.SessionEvent, state string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case e := <-events:
			if e.State == state {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", state)
		}
	}
}

// A run that exits non-zero keeps its log for the crash hints (issue #46).
func TestCrashLogKeepsTheLogOfAnAbnormalExit(t *testing.T) {
	logDir := t.TempDir()
	events := make(chan types.SessionEvent, 16)
	tr := NewTracker(func(e types.SessionEvent) { events <- e })
	tr.SetLogDir(logDir)

	cmd := exec.Command("sh", "-c", "echo 'err:module:import_dll Library bar.dll not found'; exit 3")
	if _, err := tr.StartDirect("crashy", cmd, "sh", nil); err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	waitFor(t, events, "failed")

	want := filepath.Join(logDir, "crashy.log")
	if got := tr.CrashLog("crashy"); got != want {
		t.Fatalf("CrashLog = %q, want %q", got, want)
	}
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Library bar.dll not found") {
		t.Fatalf("log misses the game's stderr: %q", raw)
	}
	if tr.CrashLog("someone-else") != "" {
		t.Fatal("CrashLog reported a game that never ran")
	}
}

// The log holds one session, capped: a launch that writes far more than
// maxSessionLog keeps the first part and says it was cut.
func TestSessionLogIsCapped(t *testing.T) {
	logDir := t.TempDir()
	events := make(chan types.SessionEvent, 16)
	tr := NewTracker(func(e types.SessionEvent) { events <- e })
	tr.SetLogDir(logDir)

	size := int64(maxSessionLog + 1<<20)
	cmd := exec.Command("sh", "-c", "head -c "+strconv.FormatInt(size, 10)+" /dev/zero | tr '\\0' x; exit 1")
	if _, err := tr.StartDirect("chatty", cmd, "sh", nil); err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	waitFor(t, events, "failed")

	info, err := os.Stat(filepath.Join(logDir, "chatty.log"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > maxSessionLog+256 {
		t.Fatalf("log is %d bytes, want at most about %d", info.Size(), maxSessionLog)
	}
	raw, err := os.ReadFile(filepath.Join(logDir, "chatty.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "[log truncated]\n") {
		t.Fatalf("capped log lacks the truncation note: tail %q", raw[len(raw)-40:])
	}
}
