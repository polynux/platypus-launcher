package session

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"hydra-launcher/internal/types"
)

const (
	// How long a game gets to exit after SIGTERM before the whole process
	// tree is killed.
	defaultGrace = 6 * time.Second

	// Shown as the reason of a "stopped" event when the user asked for it.
	stoppedByUserMsg = "Stopped by user"

	// Every direct launch carries HYDRA_SESSION=<random token> in its
	// environment. Children inherit it, so a forced stop can find
	// descendants that daemonized out of the process group (wineserver)
	// without matching on process names.
	tokenEnv = "HYDRA_SESSION"
)

// ErrNotRunning is returned by Stop when the game isn't tracked.
var ErrNotRunning = errors.New("is not running")

// StartOption tweaks one direct launch.
type StartOption func(*entry)

// WithStopHook registers extra work for stopping the game, called once per
// kill attempt: force is false with the graceful SIGTERM and true with the
// final SIGKILL. It runs off the caller's goroutine. Wine games use it to ask
// the prefix's wineserver to terminate.
func WithStopHook(hook func(force bool)) StartOption {
	return func(e *entry) { e.hook = hook }
}

// ExitInfo says how a direct launch ended.
type ExitInfo struct {
	// Stopped: the user asked Hydra to stop it.
	Stopped bool
	// Failed: it died within moments of starting with an error, so it never
	// really ran.
	Failed bool
}

// WithOnExit registers a callback for when the launch's process has ended and
// the game is no longer tracked. It runs on the tracker's goroutine after the
// final session event, so it must return quickly (start a goroutine for real
// work). It does not run when the process never started.
func WithOnExit(fn func(ExitInfo)) StartOption {
	return func(e *entry) { e.onExit = fn }
}

// prepareForStop puts the launch in its own process group (so a stop reaches
// wrappers, Wine and the game, and never Hydra) and tags its environment.
func prepareForStop(cmd *exec.Cmd, e *entry) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return
	}
	token := hex.EncodeToString(b[:])
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, tokenEnv+"="+token)
	e.token = token
}

// Stop asks a tracked game to quit: SIGTERM to its processes, then SIGKILL
// once the grace period has passed. With force it kills right away. It returns
// as soon as the request is delivered; the "stopped" event reports the end.
// Stopping a game that is already stopping is a no-op (unless force is set).
// A game a store client is still starting is stopped as soon as it appears.
func (t *Tracker) Stop(gameID string, force bool) error {
	t.mu.Lock()
	e, ok := t.active[gameID]
	if !ok {
		t.mu.Unlock()
		return fmt.Errorf("game %s %w", gameID, ErrNotRunning)
	}
	first := !e.stopping
	e.stopping = true
	e.state = "stopping"
	if force {
		e.forceReq = true
	}
	pid := e.pid
	t.mu.Unlock()

	if first {
		t.emit(types.SessionEvent{GameID: gameID, State: "stopping", PID: pid})
	}
	switch {
	case pid == 0:
		// The process isn't known yet; its starter delivers the request.
	case force:
		go t.kill(e, true)
	case first:
		t.deliver(e, false)
	}
	return nil
}

// deliverPending sends a stop that was requested before the pid was known.
func (t *Tracker) deliverPending(e *entry) {
	t.mu.Lock()
	force := e.forceReq
	t.mu.Unlock()
	if force {
		go t.kill(e, true)
		return
	}
	t.deliver(e, false)
}

// deliver sends SIGTERM and schedules the SIGKILL fallback.
func (t *Tracker) deliver(e *entry, force bool) {
	go func() {
		t.kill(e, force)
		if force {
			return
		}
		select {
		case <-e.done:
		case <-time.After(t.grace):
			t.kill(e, true)
		}
	}()
}

// kill signals the game's processes: the whole process group for direct
// launches, the watched process and its descendants for client launches. The
// store client itself is never part of either.
func (t *Tracker) kill(e *entry, force bool) {
	select {
	case <-e.done:
		return
	default:
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	t.mu.Lock()
	pid, direct, token, hook := e.pid, e.cmd != nil, e.token, e.hook
	t.mu.Unlock()
	if pid == 0 {
		return
	}
	if direct {
		killGroup(pid, sig)
	} else {
		for _, p := range descendants(pid) {
			_ = syscall.Kill(p, sig)
		}
		_ = syscall.Kill(pid, sig)
	}
	if force && token != "" {
		for _, p := range pidsWithToken(token) {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}
	if hook != nil {
		hook(force)
	}
}

// userStopped reports whether the user asked to stop this launch.
func (t *Tracker) userStopped(e *entry) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return e.stopping
}

// emitStopped reports the end of a client-watched game.
func (t *Tracker) emitStopped(gameID string, e *entry) {
	ev := types.SessionEvent{GameID: gameID, State: "stopped"}
	if t.userStopped(e) {
		ev.Message = stoppedByUserMsg
	}
	t.emit(ev)
}

// killGroup signals pid's process group, but only when pid leads its own group
// (never Hydra's); otherwise just pid.
func killGroup(pid int, sig syscall.Signal) {
	if pg, err := syscall.Getpgid(pid); err == nil && pg == pid && pg != syscall.Getpgrp() {
		_ = syscall.Kill(-pid, sig)
		return
	}
	_ = syscall.Kill(pid, sig)
}

// parentOf reads /proc/<pid>/stat and returns the parent pid, or 0.
func parentOf(pid int) int {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	// pid (comm) state ppid ...; comm may contain spaces and parentheses.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0
	}
	f := bytes.Fields(b[i+1:])
	if len(f) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(string(f[1]))
	return ppid
}

// descendants returns every process below root, found through parent links.
func descendants(root int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	children := map[int][]int{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if pp := parentOf(pid); pp > 0 {
			children[pp] = append(children[pp], pid)
		}
	}
	var out []int
	queue := []int{root}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, c := range children[p] {
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}

// pidsWithToken lists the processes whose environment carries this launch's
// session token.
func pidsWithToken(token string) []int {
	needle := []byte(tokenEnv + "=" + token + "\x00")
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		if err != nil {
			continue
		}
		if bytes.Contains(env, needle) {
			out = append(out, pid)
		}
	}
	return out
}
