// Package session tracks launched games and reports real running state.
//
// A PID alone is meaningless for Steam games: `steam -applaunch` only forwards
// the request to the running client and exits. The game itself runs under
// Steam's reaper (`reaper SteamLaunch AppId=<id> -- ...`), so Steam sessions
// are detected by scanning /proc for that marker.
package session

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"hydra-launcher/internal/types"
)

const (
	pollEvery = 2 * time.Second
	// How long Steam gets to start the game (shader pre-caching and updates
	// can take a while) before we report that nothing happened.
	steamStartTimeout = 3 * time.Minute
	// A direct launch that exits faster than this is reported as a failure.
	quickExit = 5 * time.Second
)

type Emit func(types.SessionEvent)

type Tracker struct {
	emit   Emit
	mu     sync.Mutex
	active map[string]*exec.Cmd // gameID → direct-launch process (nil for Steam)
}

func NewTracker(emit Emit) *Tracker {
	return &Tracker{emit: emit, active: map[string]*exec.Cmd{}}
}

// Running reports whether a game launched by Hydra is still tracked.
func (t *Tracker) Running(gameID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.active[gameID]
	return ok
}

func (t *Tracker) set(gameID string, cmd *exec.Cmd) {
	t.mu.Lock()
	t.active[gameID] = cmd
	t.mu.Unlock()
}

func (t *Tracker) clear(gameID string) {
	t.mu.Lock()
	delete(t.active, gameID)
	t.mu.Unlock()
}

// StopAll kills direct-launch processes (on app shutdown). Steam games are
// owned by the Steam client and are left alone.
func (t *Tracker) StopAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, cmd := range t.active {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// WatchSteam follows a game started through the Steam client.
func (t *Tracker) WatchSteam(gameID, appID string) {
	t.set(gameID, nil)
	t.emit(types.SessionEvent{GameID: gameID, State: "starting", Message: "Waiting for Steam to start the game"})
	go func() {
		defer t.clear(gameID)
		deadline := time.Now().Add(steamStartTimeout)
		pid := 0
		for pid == 0 {
			if pid = FindSteamGame(appID); pid != 0 {
				break
			}
			if time.Now().After(deadline) {
				t.emit(types.SessionEvent{GameID: gameID, State: "failed", Message: "Steam never started the game. Check the Steam window for an update, a prompt or an error."})
				return
			}
			time.Sleep(pollEvery)
		}
		t.emit(types.SessionEvent{GameID: gameID, State: "running", PID: pid})
		for FindSteamGame(appID) != 0 {
			time.Sleep(pollEvery)
		}
		t.emit(types.SessionEvent{GameID: gameID, State: "stopped"})
	}()
}

// StartDirect runs cmd and follows it until exit. Output goes to a per-game
// log file under ~/.local/share/hydra/logs so failed launches can be diagnosed.
func (t *Tracker) StartDirect(gameID string, cmd *exec.Cmd) (int, error) {
	tail := &tailBuffer{}
	log, logPath, logErr := openLog(gameID)
	if logErr == nil {
		fmt.Fprintf(log, "\n=== %s · %s\n", time.Now().Format(time.RFC3339), strings.Join(cmd.Args, " "))
		cmd.Stdout = io.MultiWriter(log, tail)
	} else {
		cmd.Stdout = tail
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		if log != nil {
			_ = log.Close()
		}
		return 0, err
	}
	pid := cmd.Process.Pid
	t.set(gameID, cmd)
	t.emit(types.SessionEvent{GameID: gameID, State: "running", PID: pid})
	started := time.Now()
	go func() {
		// Wait also waits for the output copy to finish, so closing after is safe.
		err := cmd.Wait()
		if log != nil {
			_ = log.Close()
		}
		t.clear(gameID)
		if err != nil && time.Since(started) < quickExit {
			msg := strings.TrimSpace(tail.String())
			if msg == "" {
				msg = err.Error()
			}
			if logPath != "" {
				msg += " (log: " + logPath + ")"
			}
			t.emit(types.SessionEvent{GameID: gameID, State: "failed", Message: msg})
			return
		}
		t.emit(types.SessionEvent{GameID: gameID, State: "stopped"})
	}()
	return pid, nil
}

// tailBuffer keeps the last few KB written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const tailMax = 2048

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > tailMax {
		b.buf = b.buf[len(b.buf)-tailMax:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// FindSteamGame returns the PID of Steam's reaper for appID, or 0.
func FindSteamGame(appID string) int {
	marker := []byte("AppId=" + appID + "\x00")
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || !bytes.Contains(cmdline, []byte("SteamLaunch")) {
			continue
		}
		if bytes.Contains(cmdline, marker) {
			return pid
		}
	}
	return 0
}

func openLog(gameID string) (*os.File, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(home, ".local/share/hydra/logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, gameID+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	return f, path, err
}
