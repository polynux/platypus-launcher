package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// ErrExited is returned for calls the plugin never answered because it exited.
var ErrExited = errors.New("exited")

// Lifecycle timings. Tests override them through Options.Timings.
type Timings struct {
	BackoffBase time.Duration // wait before the first restart after a crash
	BackoffMax  time.Duration
	StableAfter time.Duration // a run this long resets the crash count
	MaxCrashes  int           // consecutive crashes before giving up
	StopGrace   time.Duration // time to exit after stdin closes, before SIGKILL
	InitTimeout time.Duration
}

// timings is the internal view of Timings with defaults applied.
type timings struct {
	backoffBase time.Duration // wait before the first restart after a crash
	backoffMax  time.Duration
	stableAfter time.Duration // a run this long resets the crash count
	maxCrashes  int           // consecutive crashes before giving up
	stopGrace   time.Duration // time to exit after stdin closes, before SIGKILL
	initTimeout time.Duration
}

var defaultTimings = timings{
	backoffBase: 500 * time.Millisecond,
	backoffMax:  30 * time.Second,
	stableAfter: time.Minute,
	maxCrashes:  5,
	stopGrace:   2 * time.Second,
	initTimeout: 10 * time.Second,
}

// InitResult is the plugin's answer to initialize.
type InitResult struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	API          int           `json:"api"`
	Capabilities []string      `json:"capabilities"`
	Missing      []Requirement `json:"missing"`
}

// process runs one plugin executable. It starts on the first call, restarts
// after a crash with exponential backoff, and gives up after maxCrashes quick
// crashes in a row.
type process struct {
	id      string
	dir     string
	exec    string
	hash    string       // pinned at approval
	verify  func() error // re-checks the pinned hash before every start
	logPath string
	dataDir string      // HYDRA_PLUGIN_DATA
	onExit  func(error) // unexpected exits only
	notify  jsonrpc.NotifyFunc
	t       timings

	mu        sync.Mutex
	cmd       *exec.Cmd
	conn      *jsonrpc.Conn
	stdin     *os.File
	ready     chan struct{} // closed once initialize succeeded
	exited    chan struct{} // closed when the current run is over
	dead      chan struct{} // closed right after cmd.Wait returns, before the drain (CORE-2)
	info      *InitResult
	started   time.Time
	crashes   int
	retryAt   time.Time
	lastErr   error
	lastCrash time.Time // when the latest crash was recorded (for decay)
	initErr   error     // why initialize failed, reported instead of "killed"
	stopping  bool
}

// call sends a request exactly once, starting the plugin first if needed.
// If the run died before anything was sent (its dead channel is already
// closed — the reaped process cannot have read the request), it resolves a
// fresh connection and sends there. A death during or after the send is
// reported as ErrExited without resending: plugin methods are not
// idempotent (install/update/uninstall, auth.complete, job.cancel), so a
// request the dead process may already have acted on is never replayed.
func (p *process) call(ctx context.Context, method string, params, result any) error {
	conn, dead, err := p.ensure(ctx)
	if err != nil {
		return err
	}
	select {
	case <-dead:
		conn, _, err = p.ensure(ctx)
		if err != nil {
			return err
		}
	default:
	}
	err = conn.Call(ctx, method, params, result)
	if errors.Is(err, jsonrpc.ErrClosed) {
		return fmt.Errorf("plugin %s %w during %s", p.id, ErrExited, method)
	}
	return err
}

// ensure returns a connection to a running, initialized plugin, plus the
// run's dead channel (closed once its process has been reaped). Concurrent
// callers share one start and all wait for initialize. After a crash, the next
// call waits out the backoff before starting the plugin again.
func (p *process) ensure(ctx context.Context) (*jsonrpc.Conn, <-chan struct{}, error) {
	for {
		p.mu.Lock()
		if p.stopping {
			p.mu.Unlock()
			return nil, nil, fmt.Errorf("plugin %s is stopped", p.id)
		}
		if p.conn != nil {
			conn, ready, exited, dead := p.conn, p.ready, p.exited, p.dead
			p.mu.Unlock()
			select {
			case <-ready:
				// The process died but cleanup hasn't finished (CORE-2
				// drain window): never hand out the stale conn. Wait for
				// the reaper, then loop to restart or report give-up.
				select {
				case <-dead:
					select {
					case <-exited:
						continue
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					}
				default:
				}
				select {
				case <-conn.Done():
					// The plugin just died; wait for cleanup, then restart.
					select {
					case <-exited:
						continue
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					}
				default:
					return conn, dead, nil
				}
			case <-exited:
				p.mu.Lock()
				err := p.lastErr
				p.mu.Unlock()
				if err == nil {
					err = errors.New("stopped")
				}
				return nil, nil, fmt.Errorf("plugin %s: %w", p.id, err)
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		// A breakage fixed long ago shouldn't look permanent: crashes older
		// than a stable run decay away (CORE-10).
		if p.crashes >= p.t.maxCrashes && !p.lastCrash.IsZero() && time.Since(p.lastCrash) >= p.t.stableAfter {
			p.crashes = 0
		}
		if p.crashes >= p.t.maxCrashes {
			err := fmt.Errorf("plugin %s crashed %d times in a row, last: %v", p.id, p.crashes, p.lastErr)
			p.mu.Unlock()
			return nil, nil, err
		}
		wait := time.Until(p.retryAt)
		if wait <= 0 {
			conn, err := p.start()
			p.mu.Unlock()
			if err != nil {
				return nil, nil, err
			}
			// Not tied to ctx: a caller giving up must not kill a healthy plugin.
			go p.initialize(conn)
			continue
		}
		p.mu.Unlock()
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}

// start launches the executable. Callers hold p.mu.
func (p *process) start() (*jsonrpc.Conn, error) {
	if err := p.verify(); err != nil {
		return nil, err
	}
	// Plain pipes rather than cmd.StdoutPipe: Wait must not close stdout
	// under the reader, and a helper the plugin spawned may still hold it.
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, err
	}
	cmd := exec.Command(p.exec)
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("HYDRA_PLUGIN_API=%d", APIVersion), "HYDRA_PLUGIN_DATA="+p.dataDir, "PYTHONDONTWRITEBYTECODE=1")
	cmd.Stdin = inR
	cmd.Stdout = outW
	var logFile *os.File
	if p.logPath != "" {
		if logFile = openLogFile(p.logPath); logFile != nil {
			fmt.Fprintf(logFile, "\n=== %s · start %s\n", time.Now().Format(time.RFC3339), p.exec)
			cmd.Stderr = logFile
		}
	}
	// Own process group, so the plugin's helpers (legendary, …) die with it,
	// and SIGKILL if Hydra dies without a clean shutdown.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	err = cmd.Start()
	inR.Close()
	outW.Close()
	if err != nil {
		inW.Close()
		outR.Close()
		if logFile != nil {
			logFile.Close()
		}
		p.recordCrash(err)
		return nil, fmt.Errorf("plugin %s: %w", p.id, err)
	}

	conn := jsonrpc.NewConn(outR, inW, p.notify)
	exited := make(chan struct{})
	dead := make(chan struct{})
	p.cmd, p.conn, p.stdin, p.exited, p.dead, p.info = cmd, conn, inW, exited, dead, nil
	p.ready = make(chan struct{})
	p.started = time.Now()

	go func() {
		waitErr := cmd.Wait()
		// Mark the run dead before the reader-drain below (CORE-2): from
		// here on ensure() must not hand out this conn.
		close(dead)
		// The leader is gone; take any helpers it left behind with it.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		// Let the reader drain what the plugin wrote before exiting.
		select {
		case <-conn.Done():
		case <-time.After(time.Second):
		}
		outR.Close()
		inW.Close()
		conn.Close()
		if logFile != nil {
			fmt.Fprintf(logFile, "=== exit: %v\n", waitErr)
			logFile.Close()
		}
		p.mu.Lock()
		crashed := false
		if p.cmd == cmd {
			p.cmd, p.conn, p.stdin, p.dead, p.info = nil, nil, nil, nil, nil
			if p.initErr != nil {
				waitErr, p.initErr = p.initErr, nil
			}
			if !p.stopping {
				if waitErr == nil {
					waitErr = errors.New("exited")
				}
				if time.Since(p.started) >= p.t.stableAfter {
					p.crashes = 0
				}
				p.recordCrash(waitErr)
				crashed = true
			}
		}
		p.mu.Unlock()
		close(exited)
		if crashed && p.onExit != nil {
			p.onExit(waitErr)
		}
	}()
	return conn, nil
}

// recordCrash schedules the next start. Callers hold p.mu.
func (p *process) recordCrash(err error) {
	now := time.Now()
	p.crashes++
	p.lastErr = err
	p.lastCrash = now
	backoff := p.t.backoffBase << (p.crashes - 1)
	if backoff > p.t.backoffMax || backoff <= 0 {
		backoff = p.t.backoffMax
	}
	p.retryAt = now.Add(backoff)
}

// initialize performs the handshake on a fresh process. A plugin that fails it
// is killed and counts as a crash.
func (p *process) initialize(conn *jsonrpc.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), p.t.initTimeout)
	defer cancel()
	var res InitResult
	err := conn.Call(ctx, "initialize", map[string]any{"api": APIVersion}, &res)
	if err == nil && res.API != APIVersion {
		err = fmt.Errorf("plugin speaks api %d, Hydra speaks %d", res.API, APIVersion)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != conn {
		return // already exited; the wait goroutine recorded why
	}
	if err != nil {
		p.initErr = fmt.Errorf("initialize: %w", err)
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		return
	}
	p.info = &res
	close(p.ready)
}

// stop closes the plugin's stdin, which asks it to exit, and kills it if it
// is still running after the grace period. A stopped process never restarts.
func (p *process) stop() {
	p.mu.Lock()
	p.stopping = true
	cmd, stdin, exited := p.cmd, p.stdin, p.exited
	p.mu.Unlock()
	if cmd == nil {
		return
	}
	stdin.Close()
	select {
	case <-exited:
	case <-time.After(p.t.stopGrace):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	}
}

// pid of the running plugin, or 0.
func (p *process) pid() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// notifyToLog writes `log` notifications to the plugin's log file and passes
// everything else to next.
func notifyToLog(path string, next func(method string, params json.RawMessage)) jsonrpc.NotifyFunc {
	return func(method string, params json.RawMessage) {
		if method == "log" && path != "" {
			var l struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(params, &l)
			appendPluginLog(path, fmt.Sprintf("[%s] %s\n", l.Level, l.Message))
			return
		}
		if next != nil {
			next(method, params)
		}
	}
}

// maxPluginLogBytes bounds one plugin log file; older output moves to a
// single ".prev" backup, so the pair stays under ~2x the cap (CORE-13).
const maxPluginLogBytes = 1 << 20

// rotateLogFile moves an over-cap log aside before the next write.
func rotateLogFile(path string) {
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxPluginLogBytes {
		return
	}
	_ = os.Remove(path + ".prev")
	_ = os.Rename(path, path+".prev")
}

// openLogFile opens the plugin log for appending: owner-only like the
// plugin state files, rotated when over the cap (CORE-13).
func openLogFile(path string) *os.File {
	if path == "" {
		return nil
	}
	rotateLogFile(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	_ = os.Chmod(path, 0o600) // tighten logs written 0644 before the fix
	return f
}

// appendPluginLog appends one log line, rotating first when over the cap.
func appendPluginLog(path, text string) {
	if path == "" {
		return
	}
	rotateLogFile(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_ = os.Chmod(path, 0o600)
	_, _ = io.WriteString(f, text)
	f.Close()
}
