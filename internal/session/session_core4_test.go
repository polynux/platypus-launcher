package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hydra-launcher/internal/types"
)

// CORE-4: launch secrets must not leak through the game's own stdout into
// the per-game log or the failure message. The game's argv echo contains
// the raw token; both sinks must carry only the redacted form.
func TestStartDirectRedactsSecretsInLogAndFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	events := make(chan types.SessionEvent, 16)
	tr := NewTracker(func(e types.SessionEvent) { events <- e })

	secret := "-AUTH_PASSWORD=s3cret-xyz-123"
	cmd := exec.Command("sh", "-c", "echo 'echo game "+secret+"'; exit 1")
	pid, err := tr.StartDirect("game-secret", cmd, "sh -c ***", []string{"-AUTH_PASSWORD="})
	if err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	if pid == 0 {
		t.Fatal("no pid")
	}

	var failed *types.SessionEvent
	deadline := time.After(10 * time.Second)
drain:
	for {
		select {
		case e := <-events:
			if e.State == "failed" {
				e := e
				failed = &e
				break drain
			}
		case <-deadline:
			t.Fatal("timed out waiting for failed event")
		}
	}
	if failed == nil {
		t.Fatal("no failed event")
	}
	if strings.Contains(failed.Message, "s3cret-xyz-123") {
		t.Fatalf("failure message leaks secret: %q", failed.Message)
	}
	if !strings.Contains(failed.Message, "-AUTH_PASSWORD=***") {
		t.Fatalf("failure message missing redacted form: %q", failed.Message)
	}

	raw, err := os.ReadFile(filepath.Join(home, ".local/share/hydra/logs", "game-secret.log"))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if strings.Contains(string(raw), "s3cret-xyz-123") {
		t.Fatalf("log file leaks secret: %q", raw)
	}
	if !strings.Contains(string(raw), "-AUTH_PASSWORD=***") {
		t.Fatalf("log file missing redacted form: %q", raw)
	}
}
