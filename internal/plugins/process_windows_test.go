//go:build windows

package plugins

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The Windows plugin tests drive the test binary itself as the plugin: one
// mode prints a marker on stdout (proving the child inherits the protocol
// pipes the manager sets up) and a parent/child pair blocks until killed
// (proving the job object cleans up the whole tree).
func TestMain(m *testing.M) {
	switch os.Getenv("PLATYPUS_PLUGIN_HELPER") {
	case "pipechild":
		fmt.Println("platypus-plugin-pipe-mark")
		os.Exit(0)
	case "child":
		if err := os.WriteFile(os.Getenv("PLATYPUS_PLUGIN_PIDFILE"), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
			os.Exit(2)
		}
		select {} // killed by the job close or TerminateProcess
	case "parent":
		if err := os.WriteFile(os.Getenv("PLATYPUS_PLUGIN_PIDFILE"), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
			os.Exit(2)
		}
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(),
			"PLATYPUS_PLUGIN_HELPER=child",
			"PLATYPUS_PLUGIN_PIDFILE="+os.Getenv("PLATYPUS_PLUGIN_CHILD_PIDFILE"))
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "helper child:", err)
			os.Exit(1)
		}
		select {} // killed by the job close or TerminateProcess
	}
	os.Exit(m.Run())
}

func readPluginHelperPid(t *testing.T, file string) int {
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

func waitPluginHelperDead(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !testProcessAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pid %d is still alive", pid)
}

// The protocol pipes stay connected through configurePluginProcess: the
// child inherits exactly the stdio handles start() sets up for it.
func TestWindowsPluginProtocolPipeSurvivesConfigure(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "PLATYPUS_PLUGIN_HELPER=pipechild")
	configurePluginProcess(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(out); done <- b }()
	select {
	case b := <-done:
		if string(b) != "platypus-plugin-pipe-mark\n" {
			t.Fatalf("plugin stdout = %q", b)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timed out reading the helper's stdout")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

// Killing a plugin ends the child it spawned: closing its job object
// terminates the whole tree, and the job handle doesn't leak.
func TestWindowsPluginJobKillsTree(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "parent.pid")
	childFile := filepath.Join(dir, "child.pid")
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"PLATYPUS_PLUGIN_HELPER=parent",
		"PLATYPUS_PLUGIN_PIDFILE="+parentFile,
		"PLATYPUS_PLUGIN_CHILD_PIDFILE="+childFile)
	configurePluginProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	afterPluginStart(cmd)
	parent := readPluginHelperPid(t, parentFile)
	if parent != cmd.Process.Pid {
		t.Fatalf("parent pid file = %d, started pid = %d", parent, cmd.Process.Pid)
	}
	child := readPluginHelperPid(t, childFile)
	killPluginProcess(cmd)
	_ = cmd.Wait()
	waitPluginHelperDead(t, parent)
	waitPluginHelperDead(t, child)
	pluginJobs.Lock()
	leaked := len(pluginJobs.m)
	pluginJobs.Unlock()
	if leaked != 0 {
		t.Fatalf("%d plugin job handles leaked", leaked)
	}
}
