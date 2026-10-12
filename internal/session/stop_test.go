package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"platypus/internal/types"
)

type recorder struct{ ch chan types.SessionEvent }

func newRecorder() *recorder { return &recorder{ch: make(chan types.SessionEvent, 64)} }

func (r *recorder) emit(e types.SessionEvent) { r.ch <- e }

// waitState waits for the next event with the given state, skipping others.
func (r *recorder) waitState(t *testing.T, state string, within time.Duration) types.SessionEvent {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case e := <-r.ch:
			if e.State == state {
				return e
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q event", state)
		}
	}
}

// alive reports whether pid exists and is not a zombie.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	return i >= 0 && i+2 < len(s) && s[i+2] != 'Z' && s[i+2] != 'X'
}

func waitDead(t *testing.T, pid int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !alive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pid %d is still alive", pid)
}

// stubbornScript is a launcher that spawns a child; both ignore SIGTERM. The
// child's pid is written to pidFile.
func stubbornScript(t *testing.T) (script, pidFile string) {
	t.Helper()
	dir := t.TempDir()
	pidFile = filepath.Join(dir, "child.pid")
	script = filepath.Join(dir, "game.sh")
	body := "#!/bin/sh\ntrap '' TERM\n(trap '' TERM; echo $$ > " + pidFile + "; while :; do sleep 0.1; done) &\nwhile :; do sleep 0.1; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, pidFile
}

func readPid(t *testing.T, file string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(file); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pid file %s never appeared", file)
	return 0
}

func TestStopNotRunning(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())
	err := tr.Stop("nope", false)
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("want ErrNotRunning, got %v", err)
	}
}

// A well-behaved game exits on SIGTERM. Even though it dies within quickExit
// with a signal error, that is a stop by the user, not a failed launch.
func TestStopGracefulReportsStopped(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	pid, err := tr.StartDirect("g1", exec.Command("sleep", "30"), "sleep 30", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "running", 5*time.Second)

	if err := tr.Stop("g1", false); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	rec.waitState(t, "stopping", 5*time.Second)
	ev := rec.waitState(t, "stopped", 5*time.Second)
	if ev.GameID != "g1" || ev.Message == "" {
		t.Fatalf("stopped event should say it was stopped by the user: %+v", ev)
	}
	waitDead(t, pid, 2*time.Second)
	if tr.Running("g1") {
		t.Fatal("still tracked after stop")
	}
}

// The launcher and its child both ignore SIGTERM: after the grace period the
// whole process group gets SIGKILL.
func TestStopEscalatesToSIGKILLAndKillsTheGroup(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	tr.grace = 300 * time.Millisecond
	script, pidFile := stubbornScript(t)

	pid, err := tr.StartDirect("g2", exec.Command("/bin/sh", script), "game.sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pidFile)
	rec.waitState(t, "running", 5*time.Second)

	started := time.Now()
	if err := tr.Stop("g2", false); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// Idempotent: a second graceful request while stopping is a no-op.
	if err := tr.Stop("g2", false); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	// SIGTERM alone must not have been enough.
	time.Sleep(100 * time.Millisecond)
	if !alive(pid) || !alive(child) {
		t.Fatal("test script should survive SIGTERM")
	}
	rec.waitState(t, "stopped", 5*time.Second)
	if time.Since(started) < 250*time.Millisecond {
		t.Fatalf("SIGKILL fired before the grace period (%v)", time.Since(started))
	}
	waitDead(t, pid, 2*time.Second)
	waitDead(t, child, 2*time.Second)
}

func TestForceStopKillsImmediately(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	tr.grace = time.Hour
	script, pidFile := stubbornScript(t)

	pid, err := tr.StartDirect("g3", exec.Command("/bin/sh", script), "game.sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pidFile)
	rec.waitState(t, "running", 5*time.Second)

	if err := tr.Stop("g3", false); err != nil {
		t.Fatal(err)
	}
	if err := tr.Stop("g3", true); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "stopped", 5*time.Second)
	waitDead(t, pid, 2*time.Second)
	waitDead(t, child, 2*time.Second)
}

// Wine's wineserver daemonizes into its own session, outside the game's
// process group. The per-launch environment token still finds it on force.
func TestForceStopKillsDaemonizedDescendant(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not available")
	}
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "daemon.pid")
	script := filepath.Join(dir, "game.sh")
	body := "#!/bin/sh\nsetsid sh -c 'echo $$ > " + pidFile + "; while :; do sleep 0.1; done' >/dev/null 2>&1 &\nwhile :; do sleep 0.1; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	pid, err := tr.StartDirect("g4", exec.Command("/bin/sh", script), "game.sh", nil)
	if err != nil {
		t.Fatal(err)
	}
	daemon := readPid(t, pidFile)
	rec.waitState(t, "running", 5*time.Second)
	if pg, _ := syscall.Getpgid(daemon); pg == pid {
		t.Fatal("test daemon should live outside the game's process group")
	}

	if err := tr.Stop("g4", true); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "stopped", 5*time.Second)
	waitDead(t, daemon, 2*time.Second)
}

func TestStopRunsTheStopHook(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	var graceful, forced atomic.Int32
	hook := func(force bool) {
		if force {
			forced.Add(1)
		} else {
			graceful.Add(1)
		}
	}
	if _, err := tr.StartDirect("g5", exec.Command("sleep", "30"), "sleep", nil, WithStopHook(hook)); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "running", 5*time.Second)
	if err := tr.Stop("g5", false); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "stopped", 5*time.Second)
	if graceful.Load() != 1 || forced.Load() != 0 {
		t.Fatalf("hook calls: graceful=%d forced=%d", graceful.Load(), forced.Load())
	}
}

// Client launches: the watched game process (and what it spawned) is stopped,
// the process that started it (the client) is never touched.
func TestStopClientGameLeavesTheClientAlone(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	tr.grace = 300 * time.Millisecond

	client := exec.Command("sleep", "30")
	if err := client.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = client.Wait() }()
	defer func() { _ = client.Process.Kill() }()

	// The "game" ignores SIGTERM and has its own child.
	script, pidFile := stubbornScript(t)
	game := exec.Command("/bin/sh", script)
	game.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := game.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = game.Wait() }()
	gamePid := game.Process.Pid
	child := readPid(t, pidFile)

	find := func() int {
		if alive(gamePid) {
			return gamePid
		}
		return 0
	}
	if !tr.WatchClient("g6", "Steam", find, time.Minute) {
		t.Fatal("WatchClient refused")
	}
	rec.waitState(t, "running", 5*time.Second)

	if err := tr.Stop("g6", false); err != nil {
		t.Fatal(err)
	}
	ev := rec.waitState(t, "stopped", 10*time.Second)
	if ev.Message == "" {
		t.Fatalf("stopped event should carry the reason: %+v", ev)
	}
	waitDead(t, gamePid, 2*time.Second)
	waitDead(t, child, 2*time.Second)
	if !alive(client.Process.Pid) {
		t.Fatal("the client process must not be killed")
	}
}

// A stop requested while the client is still starting the game is delivered as
// soon as the game appears.
func TestStopWhileClientStillStarting(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(t.TempDir())
	tr.grace = 300 * time.Millisecond

	var gamePid atomic.Int32
	find := func() int {
		if pid := int(gamePid.Load()); alive(pid) {
			return pid
		}
		return 0
	}
	if !tr.WatchClient("g7", "Steam", find, time.Minute) {
		t.Fatal("WatchClient refused")
	}
	rec.waitState(t, "starting", 5*time.Second)
	if err := tr.Stop("g7", false); err != nil {
		t.Fatal(err)
	}

	game := exec.Command("sleep", "30")
	game.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := game.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = game.Wait() }()
	gamePid.Store(int32(game.Process.Pid))

	rec.waitState(t, "stopped", 10*time.Second)
	waitDead(t, game.Process.Pid, 2*time.Second)
}
