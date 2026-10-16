// Package session tracks launched games and reports real running state.
//
// A PID alone is meaningless for Steam games: `steam -applaunch` only forwards
// the request to the running client and exits. The game itself runs under
// Steam's reaper (`reaper SteamLaunch AppId=<id> -- ...`), so Steam sessions
// are detected by scanning /proc for that marker.
package session

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"platypus/internal/datadir"
	"platypus/internal/types"
)

const (
	pollEvery = 2 * time.Second
	// How long a store client gets to start the game (Steam's shader
	// pre-caching and updates can take a while) before we report that nothing
	// happened.
	clientStartTimeout = 3 * time.Minute
	// A direct launch that exits faster than this is reported as a failure.
	quickExit = 5 * time.Second
)

type Emit func(types.SessionEvent)

type Tracker struct {
	emit   Emit
	grace  time.Duration // how long a game gets to exit after SIGTERM before SIGKILL
	mu     sync.Mutex
	active map[string]*entry // gameID → launch entry (direct process or store-client watch)
	// logDir overrides the session-log folder (issue #41: the App passes
	// <data>/logs so tests need no HOME). Empty means the default.
	logDir string
	// crashed maps a game to the log of its last direct run when that run
	// ended abnormally (issue #46). A clean run or a new launch clears it.
	crashed map[string]string
}

// entry is one tracked launch. cmd is the direct-launch process (nil for
// store-client launches); state/pid/message mirror the last emitted event
// so late subscribers can hydrate via Snapshot (CORE-1).
type entry struct {
	cmd     *exec.Cmd
	state   string
	pid     int
	message string

	done     chan struct{} // closed when the launch is no longer tracked
	doneOnce sync.Once
	token    string // PLATYPUS_SESSION value in the game's environment (direct launches)
	hook     func(force bool)
	onExits  []func(ExitInfo)
	stopping bool // the user asked to stop it
	forceReq bool // ... and asked for SIGKILL before a pid was known
}

func NewTracker(emit Emit) *Tracker {
	return &Tracker{emit: emit, grace: defaultGrace, active: map[string]*entry{}, crashed: map[string]string{}}
}

// CrashLog returns the log file of gameID's last direct launch when that run
// ended abnormally (a non-zero exit or a very short run, issue #46), "" otherwise.
// The file holds one session: the next launch replaces it.
func (t *Tracker) CrashLog(gameID string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.crashed[gameID]
}

// SetLogDir overrides the folder session logs are written to (issue #41).
// Empty restores the default (<data>/logs under the user's home).
func (t *Tracker) SetLogDir(dir string) {
	t.mu.Lock()
	t.logDir = dir
	t.mu.Unlock()
}

// Running reports whether a game launched by Platypus is still tracked.
func (t *Tracker) Running(gameID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.active[gameID]
	return ok
}

// Snapshot returns the currently tracked sessions for late subscribers
// (second windows, reloads). Entries are copies; the slice is never nil.
func (t *Tracker) Snapshot() []types.SessionEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]types.SessionEvent, 0, len(t.active))
	for gameID, e := range t.active {
		out = append(out, types.SessionEvent{
			GameID:  gameID,
			State:   e.state,
			PID:     e.pid,
			Message: e.message,
		})
	}
	return out
}

// ErrAlreadyRunning is returned when a launch is claimed while the game is
// already tracked. LaunchGame maps it to a titled message.
var ErrAlreadyRunning = errors.New("already running")

// tryClaim atomically claims gameID for one launch (CORE-3). It returns nil
// when the game is already tracked, so concurrent launches can't both start.
func (t *Tracker) tryClaim(gameID string, cmd *exec.Cmd) *entry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.active[gameID]; ok {
		return nil
	}
	e := &entry{cmd: cmd, state: "starting", done: make(chan struct{})}
	if cmd != nil && cmd.Process != nil {
		e.state = "running"
		e.pid = cmd.Process.Pid
	}
	t.active[gameID] = e
	return e
}

// clearEntry removes gameID only if it still holds this launch's entry, so
// one run's exit can't drop another run's tracking (CORE-3).
func (t *Tracker) clearEntry(gameID string, e *entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cur, ok := t.active[gameID]; ok && cur == e {
		delete(t.active, gameID)
	}
	e.doneOnce.Do(func() { close(e.done) })
}

// setRunning records the pid of an already-claimed game once a store client
// has handed it over. It reports whether the user asked to stop the game in
// the meantime; the state then stays "stopping".
func (t *Tracker) setRunning(gameID string, pid int) (stopping bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.active[gameID]; ok {
		e.pid = pid
		if e.stopping {
			return true
		}
		e.state = "running"
		e.message = ""
	}
	return false
}

// setMessage updates the stored message for a tracked game.
func (t *Tracker) setMessage(gameID, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.active[gameID]; ok {
		e.message = message
	}
}

// StopAll kills direct-launch processes and their groups (on app shutdown).
// Steam games are owned by the Steam client and are left alone.
func (t *Tracker) StopAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.active {
		if e.cmd != nil && e.cmd.Process != nil {
			killGroup(e.cmd.Process.Pid, syscall.SIGKILL)
			_ = e.cmd.Process.Kill()
		}
	}
}

// WatchClient follows a game started through a store's own client (Steam).
// client names it in messages; find returns the game's PID, or 0 while it
// isn't running. It reports false when the game is already tracked. opts
// register on-exit hooks like StartDirect: they run after the final stopped
// event when the game actually ran (not when the client never started it),
// so after-session backups fire for client launches too.
func (t *Tracker) WatchClient(gameID, client string, find func() int, startTimeout time.Duration, opts ...StartOption) bool {
	if startTimeout <= 0 {
		startTimeout = clientStartTimeout
	}
	e := t.tryClaim(gameID, nil)
	if e == nil {
		return false
	}
	t.mu.Lock()
	for _, o := range opts {
		o(e)
	}
	t.mu.Unlock()
	msg := "Waiting for " + client + " to start the game"
	t.setMessage(gameID, msg)
	t.emit(types.SessionEvent{GameID: gameID, State: "starting", Message: msg})
	go func() {
		defer t.clearEntry(gameID, e)
		deadline := time.Now().Add(startTimeout)
		pid := 0
		for pid == 0 {
			if pid = find(); pid != 0 {
				break
			}
			if time.Now().After(deadline) {
				if t.userStopped(e) {
					t.emit(types.SessionEvent{GameID: gameID, State: "stopped", Message: stoppedByUserMsg})
					return
				}
				t.emit(types.SessionEvent{GameID: gameID, State: "failed", Message: client + " never started the game. Check the " + client + " window for an update, a prompt or an error."})
				return
			}
			time.Sleep(pollEvery)
		}
		if t.setRunning(gameID, pid) {
			// Stop was requested while the client was still starting the game.
			t.emit(types.SessionEvent{GameID: gameID, State: "stopping", PID: pid})
			t.deliverPending(e)
		} else {
			t.emit(types.SessionEvent{GameID: gameID, State: "running", PID: pid})
		}
		for find() != 0 {
			time.Sleep(pollEvery)
		}
		stoppedByUser := t.userStopped(e)
		t.mu.Lock()
		hooks := append([]func(ExitInfo){}, e.onExits...)
		t.mu.Unlock()
		t.emitStopped(gameID, e)
		for _, h := range hooks {
			h(ExitInfo{Stopped: stoppedByUser})
		}
	}()
	return true
}

// StartDirect runs cmd and follows it until exit. Output goes to a per-game
// log file under ~/.local/share/platypus/logs so failed launches can be diagnosed.
// shown is the command line written to the log, with secrets redacted.
// redact holds the launch spec's secret prefixes; the game's own stdout is
// filtered with the same prefixes so echoed secrets reach neither the log
// file nor the failure message (CORE-4).
// It claims the game before starting so concurrent launches can't both run;
// losers get ErrAlreadyRunning and never start a process.
func (t *Tracker) StartDirect(gameID string, cmd *exec.Cmd, shown string, redact []string, opts ...StartOption) (int, error) {
	e := t.tryClaim(gameID, cmd)
	if e == nil {
		return 0, fmt.Errorf("game %s %w", gameID, ErrAlreadyRunning)
	}
	t.mu.Lock()
	for _, o := range opts {
		o(e)
	}
	delete(t.crashed, gameID)
	t.mu.Unlock()
	prepareForStop(cmd, e)
	tail := &tailBuffer{}
	log, logPath, logErr := t.openLog(gameID)
	var out io.Writer
	var flush func()
	if len(redact) == 0 {
		if logErr == nil {
			fmt.Fprintf(log, "\n=== %s · %s\n", time.Now().Format(time.RFC3339), shown)
			out = io.MultiWriter(&limitWriter{w: log, left: maxSessionLog}, tail)
		} else {
			out = tail
		}
	} else {
		var lw, tw *redactWriter
		if logErr == nil {
			fmt.Fprintf(log, "\n=== %s · %s\n", time.Now().Format(time.RFC3339), shown)
			lw = &redactWriter{w: &limitWriter{w: log, left: maxSessionLog}, prefixes: redact}
			tw = &redactWriter{w: tail, prefixes: redact}
			out = io.MultiWriter(lw, tw)
		} else {
			tw = &redactWriter{w: tail, prefixes: redact}
			out = tw
		}
		flush = func() {
			if lw != nil {
				lw.Flush()
			}
			tw.Flush()
		}
	}
	cmd.Stdout = out
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		if log != nil {
			_ = log.Close()
		}
		t.clearEntry(gameID, e)
		return 0, err
	}
	pid := cmd.Process.Pid
	t.mu.Lock()
	e.cmd = cmd
	e.pid = pid
	if !e.stopping {
		e.state = "running"
	}
	stopping := e.stopping
	t.mu.Unlock()
	if stopping {
		// Stop arrived between the claim and the process start.
		t.emit(types.SessionEvent{GameID: gameID, State: "stopping", PID: pid})
		t.deliverPending(e)
	} else {
		t.emit(types.SessionEvent{GameID: gameID, State: "running", PID: pid})
	}
	started := time.Now()
	go func() {
		// Wait also waits for the output copy to finish, so closing after is safe.
		err := cmd.Wait()
		if flush != nil {
			flush()
		}
		if log != nil {
			_ = log.Close()
		}
		stoppedByUser := t.userStopped(e)
		t.clearEntry(gameID, e)
		failed := !stoppedByUser && err != nil && time.Since(started) < quickExit
		// Abnormal for the crash hints (issue #46): a non-zero exit or a very
		// short run that the user didn't stop.
		if abnormal := !stoppedByUser && (err != nil || time.Since(started) < quickExit); abnormal && logPath != "" {
			t.mu.Lock()
			t.crashed[gameID] = logPath
			t.mu.Unlock()
		}
		t.mu.Lock()
		hooks := append([]func(ExitInfo){}, e.onExits...)
		t.mu.Unlock()
		if len(hooks) > 0 {
			info := ExitInfo{Stopped: stoppedByUser, Failed: failed}
			defer func() {
				for _, h := range hooks {
					h(info)
				}
			}()
		}
		if stoppedByUser {
			// A stop by the user is never a failed launch, however fast the exit.
			t.emit(types.SessionEvent{GameID: gameID, State: "stopped", Message: stoppedByUserMsg})
			return
		}
		if failed {
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

// redactWriter filters secret values out of a game's output stream. Every
// occurrence of a prefix is kept, but the token that follows it (up to the
// next whitespace or quote) is replaced with "***", mirroring RedactArgs.
type redactWriter struct {
	w        io.Writer
	prefixes []string
	pending  []byte
}

func (r *redactWriter) Write(p []byte) (int, error) {
	r.pending = append(r.pending, p...)
	for {
		i := indexByte(r.pending, '\n')
		if i < 0 {
			break
		}
		line := redactTokens(string(r.pending[:i+1]), r.prefixes)
		if _, err := io.WriteString(r.w, line); err != nil {
			return len(p), err
		}
		r.pending = append([]byte(nil), r.pending[i+1:]...)
	}
	return len(p), nil
}

// Flush redacts and writes any trailing output without a newline.
func (r *redactWriter) Flush() {
	if len(r.pending) == 0 {
		return
	}
	_, _ = io.WriteString(r.w, redactTokens(string(r.pending), r.prefixes))
	r.pending = nil
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// redactTokens replaces prefix+token with prefix+"***" for every prefix
// occurrence with a non-empty token after it.
func redactTokens(s string, prefixes []string) string {
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		var b strings.Builder
		b.Grow(len(s))
		rest := s
		for {
			i := strings.Index(rest, p)
			if i < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:i+len(p)])
			rest = rest[i+len(p):]
			j := 0
			for j < len(rest) && !isTokenEnd(rest[j]) {
				j++
			}
			if j > 0 {
				b.WriteString("***")
			}
			rest = rest[j:]
		}
		s = b.String()
	}
	return s
}

func isTokenEnd(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '"', '\'', '`':
		return true
	}
	return false
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
	return FindByCmdline("SteamLaunch", "AppId="+appID+"\x00")
}

// FindByCmdline returns the PID of a process whose command line contains
// every part, or 0. Arguments in /proc/<pid>/cmdline are NUL-separated.
func FindByCmdline(parts ...string) int {
	if len(parts) == 0 {
		return 0
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
next:
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		for _, p := range parts {
			if !bytes.Contains(cmdline, []byte(p)) {
				continue next
			}
		}
		return pid
	}
	return 0
}

func openLog(gameID string) (*os.File, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	return openLogIn(filepath.Join(datadir.Path(home), "logs"), gameID)
}

// openLog is the tracker's session log: openLogIn(t.logDir) when set (issue
// #41), else the default folder.
func (t *Tracker) openLog(gameID string) (*os.File, string, error) {
	t.mu.Lock()
	dir := t.logDir
	t.mu.Unlock()
	if dir == "" {
		return openLog(gameID)
	}
	return openLogIn(dir, gameID)
}

// maxSessionLog bounds one session's log (issue #46). Wine's stderr can grow
// without limit; the crash hints read the whole file, so it stays small.
const maxSessionLog = 4 << 20

// limitWriter passes writes to w until left bytes are written, then drops the
// rest and says so once.
type limitWriter struct {
	w    io.Writer
	left int64
	cut  bool
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.left <= 0 {
		if !l.cut {
			l.cut = true
			fmt.Fprintln(l.w, "[log truncated]")
		}
		return len(p), nil
	}
	chunk := p
	if int64(len(chunk)) > l.left {
		chunk = chunk[:l.left]
	}
	n, err := l.w.Write(chunk)
	l.left -= int64(n)
	if err != nil {
		return n, err
	}
	return len(p), nil
}

// openLogIn opens gameID's log for one session: it starts empty, so the file
// is the last launch's output only (issue #46).
func openLogIn(dir, gameID string) (*os.File, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, gameID+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	return f, path, err
}
