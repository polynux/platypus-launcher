//go:build windows

package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// The Windows tree tests drive the test binary itself as the game: a helper
// mode (parent) spawns a second copy (child) and both block until killed,
// which proves a stop reaches the whole job-object tree. A third mode
// (mark) prints one line and exits, proving stdio pipes survive
// NoInheritHandles.
func TestMain(m *testing.M) {
	switch os.Getenv("PLATYPUS_SESSION_HELPER") {
	case "child":
		writeHelperPid()
		select {} // killed by the job close or TerminateProcess
	case "parent":
		writeHelperPid()
		runHelperChild()
		select {} // killed by the job close or TerminateProcess
	case "mark":
		fmt.Println("platypus-windows-stdio-mark")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func writeHelperPid() {
	path := os.Getenv("PLATYPUS_SESSION_PIDFILE")
	if path == "" {
		os.Exit(2)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		os.Exit(2)
	}
}

func runHelperChild() {
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(),
		"PLATYPUS_SESSION_HELPER=child",
		"PLATYPUS_SESSION_PIDFILE="+os.Getenv("PLATYPUS_SESSION_CHILD_PIDFILE"))
	// Stdout stays connected: the child must not hold the protocol stream
	// open, but here it only proves spawn works under a job member.
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "helper child:", err)
		os.Exit(1)
	}
}

var procGetExitCode = kernel32.NewProc("GetExitCodeProcess")

const (
	testQueryAccess = 0x1000
	testStillActive = 259
)

// windowsAlive reports whether pid is a running process.
func windowsAlive(pid int) bool {
	h, _, _ := procOpenProcess.Call(uintptr(testQueryAccess), 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)
	var code uint32
	if ok, _, _ := procGetExitCode.Call(h, uintptr(unsafe.Pointer(&code))); ok == 0 {
		return false
	}
	return code == testStillActive
}

func readHelperPid(t *testing.T, file string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
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

func waitWindowsDead(t *testing.T, pid int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !windowsAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pid %d is still alive", pid)
}

func startHelperGame(t *testing.T, gameID, dir string) (tr *Tracker, rec *recorder, parent, child int) {
	t.Helper()
	rec = newRecorder()
	tr = NewTracker(rec.emit)
	tr.SetLogDir(dir)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"PLATYPUS_SESSION_HELPER=parent",
		"PLATYPUS_SESSION_PIDFILE="+filepath.Join(dir, gameID+".parent.pid"),
		"PLATYPUS_SESSION_CHILD_PIDFILE="+filepath.Join(dir, gameID+".child.pid"))
	pid, err := tr.StartDirect(gameID, cmd, "helper parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	parent = readHelperPid(t, filepath.Join(dir, gameID+".parent.pid"))
	if parent != pid {
		t.Fatalf("parent pid file = %d, StartDirect returned %d", parent, pid)
	}
	child = readHelperPid(t, filepath.Join(dir, gameID+".child.pid"))
	rec.waitState(t, "running", 10*time.Second)
	return tr, rec, parent, child
}

func gameJobsEmpty() bool {
	gameJobs.Lock()
	defer gameJobs.Unlock()
	return len(gameJobs.m) == 0
}

// A forced stop terminates the leader and the child it spawned (the job
// object closes), never the launcher itself.
func TestWindowsStopForceKillsTree(t *testing.T) {
	dir := t.TempDir()
	tr, rec, parent, child := startHelperGame(t, "g1", dir)
	if err := tr.Stop("g1", true); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "stopped", 10*time.Second)
	waitWindowsDead(t, parent, 10*time.Second)
	waitWindowsDead(t, child, 10*time.Second)
	if !gameJobsEmpty() {
		t.Fatal("the game's job handle leaked")
	}
}

// A graceful stop escalates after the grace period: the helpers ignore
// CTRL_BREAK (there is no console to deliver it to), so the job close ends
// them, mirroring the Unix SIGTERM-then-SIGKILL escalation.
func TestWindowsStopGracefulEscalatesToJobKill(t *testing.T) {
	dir := t.TempDir()
	tr, rec, parent, child := startHelperGame(t, "g2", dir)
	tr.grace = 500 * time.Millisecond
	started := time.Now()
	if err := tr.Stop("g2", false); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "stopping", 10*time.Second)
	rec.waitState(t, "stopped", 15*time.Second)
	if time.Since(started) < 400*time.Millisecond {
		t.Fatal("the stop escalated before the grace period")
	}
	waitWindowsDead(t, parent, 10*time.Second)
	waitWindowsDead(t, child, 10*time.Second)
	if !gameJobsEmpty() {
		t.Fatal("the game's job handle leaked")
	}
}

// The game's stdout reaches its session log: NoInheritHandles didn't close
// the stdio pipes the tracker set up.
func TestWindowsGameStdioReachesLog(t *testing.T) {
	logDir := t.TempDir()
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	tr.SetLogDir(logDir)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "PLATYPUS_SESSION_HELPER=mark")
	if _, err := tr.StartDirect("gmark", cmd, "helper mark", nil); err != nil {
		t.Fatal(err)
	}
	rec.waitState(t, "running", 10*time.Second)
	rec.waitState(t, "stopped", 10*time.Second)
	raw, err := os.ReadFile(filepath.Join(logDir, "gmark.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "platypus-windows-stdio-mark") {
		t.Fatalf("session log lacks the game's stdout: %q", raw)
	}
}
