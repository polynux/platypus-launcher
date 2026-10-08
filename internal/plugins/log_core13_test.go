package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CORE-13: plugin logs may carry tokens, so they must be owner-only (0600,
// like plugin state files) and bounded by size-based rotation.
func TestPluginLogPrivateAndRotates(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	logPath := filepath.Join(e.logs, "plugin-echo.log")
	if err := os.MkdirAll(e.logs, 0o755); err != nil {
		t.Fatal(err)
	}
	// A stale oversized world-readable log, as written before the fix.
	if err := os.WriteFile(logPath, []byte(strings.Repeat("x", maxPluginLogBytes+100)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	// The initialize handshake sends a `log` notification; wait for it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		data, _ := os.ReadFile(logPath)
		if strings.Contains(string(data), "[info] echo initialized") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("log notification never landed after rotation")
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("plugin log is group/other accessible: %o", st.Mode().Perm())
	}
	if st.Size() >= maxPluginLogBytes {
		t.Fatalf("oversized log was not rotated: %d bytes", st.Size())
	}
	if prev, err := os.Stat(logPath + ".prev"); err != nil || prev.Size() == 0 {
		t.Fatalf("rotated backup missing: %v", prev)
	}
}

func TestPluginLogStaysUnderCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin-test.log")
	msg := strings.Repeat("m", 200) + "\n"
	for i := 0; i < 10000; i++ {
		appendPluginLog(path, msg)
	}
	cur, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Mode().Perm()&0o077 != 0 {
		t.Fatalf("plugin log is group/other accessible: %o", cur.Mode().Perm())
	}
	total := cur.Size()
	if prev, err := os.Stat(path + ".prev"); err == nil {
		total += prev.Size()
	}
	if total > 2*maxPluginLogBytes+1<<17 {
		t.Fatalf("10k log writes grew past rotation: %d bytes", total)
	}
}
