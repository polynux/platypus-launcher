// Package plugins discovers store plugins in ~/.local/share/hydra/plugins,
// decides which ones may run (manifest, pinned hash, user approval) and runs
// them as JSON-RPC subprocesses. See docs/plans/launcher-plugins.md.
package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"hydra-launcher/internal/settings"
)

// Status says whether a discovered plugin may run.
type Status string

const (
	StatusReady      Status = "ready"
	StatusInvalid    Status = "invalid"    // manifest or executable is broken
	StatusUnapproved Status = "unapproved" // never approved by the user
	StatusChanged    Status = "changed"    // executable changed since approval
	StatusDisabled   Status = "disabled"   // approved, then turned off
	StatusMissing    Status = "missing-requirements"
)

// Plugin is one folder under the plugins directory.
type Plugin struct {
	Dir      string        `json:"dir"`
	Manifest Manifest      `json:"manifest"`
	Hash     string        `json:"hash"` // SHA-256 of the executable
	Status   Status        `json:"status"`
	Problem  string        `json:"problem,omitempty"`
	Missing  []Requirement `json:"missing,omitempty"`
}

// NotifyFunc receives notifications other than `log` from a running plugin.
type NotifyFunc func(pluginID, method string, params json.RawMessage)

type Manager struct {
	dir      string
	logDir   string
	trust    *settings.Store
	onNotify NotifyFunc
	lookPath func(string) (string, error)
	t        timings

	mu      sync.Mutex
	plugins map[string]*Plugin
	procs   map[string]*process
	closed  bool
}

// NewManager manages plugins in dir. Approvals live in trust; stderr and `log`
// notifications go to logDir/plugin-<id>.log.
func NewManager(dir, logDir string, trust *settings.Store, onNotify NotifyFunc) *Manager {
	return &Manager{
		dir:      dir,
		logDir:   logDir,
		trust:    trust,
		onNotify: onNotify,
		lookPath: exec.LookPath,
		t:        defaultTimings,
		plugins:  map[string]*Plugin{},
		procs:    map[string]*process{},
	}
}

// Dir is the plugins folder.
func (m *Manager) Dir() string { return m.dir }

// Discover rescans the plugins folder and returns every plugin found, sorted
// by id. Running plugins that may no longer run (removed, changed, disabled)
// are stopped.
func (m *Manager) Discover() ([]Plugin, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	found := map[string]*Plugin{}
	for _, e := range entries {
		dir := filepath.Join(m.dir, e.Name())
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		p := m.inspect(dir)
		found[p.Manifest.ID] = p
	}

	m.mu.Lock()
	m.plugins = found
	var stale []*process
	for id, proc := range m.procs {
		if p, ok := found[id]; !ok || p.Status != StatusReady || p.Hash != proc.hash {
			stale = append(stale, proc)
			delete(m.procs, id)
		}
	}
	m.mu.Unlock()
	for _, proc := range stale {
		proc.stop()
	}
	return m.Plugins(), nil
}

// inspect validates one plugin folder and works out its status.
func (m *Manager) inspect(dir string) *Plugin {
	p := &Plugin{Dir: dir}
	man, err := LoadManifest(dir)
	p.Manifest = man
	if err != nil {
		// Key broken plugins by folder name so each still shows up, once.
		p.Manifest.ID = filepath.Base(dir)
		p.Status, p.Problem = StatusInvalid, err.Error()
		return p
	}
	execPath, _ := man.ExecPath(dir) // checked by LoadManifest
	if p.Hash, err = hashFile(execPath); err != nil {
		p.Status, p.Problem = StatusInvalid, err.Error()
		return p
	}
	p.Missing = man.MissingRequirements(m.lookPath)

	trust, ok := m.trust.PluginTrust(man.ID)
	switch {
	case !ok:
		p.Status = StatusUnapproved
	case trust.Hash != p.Hash:
		p.Status, p.Problem = StatusChanged, "the executable changed since you approved it"
	case !trust.Enabled:
		p.Status = StatusDisabled
	case len(p.Missing) > 0:
		p.Status, p.Problem = StatusMissing, fmt.Sprintf("%d required tool(s) not found", len(p.Missing))
	default:
		p.Status = StatusReady
	}
	return p
}

// Plugins returns the result of the last Discover, sorted by id.
func (m *Manager) Plugins() []Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}

// Approve pins the plugin's current executable hash and enables it. The UI
// must have shown the manifest (what it runs, network hosts, paths, required
// tools) before calling this.
func (m *Manager) Approve(id string) error {
	m.mu.Lock()
	p, ok := m.plugins[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown plugin %q", id)
	}
	// Hash again: what gets approved is what is on disk now.
	p = m.inspect(p.Dir)
	if p.Status == StatusInvalid {
		return fmt.Errorf("plugin %s is invalid: %s", id, p.Problem)
	}
	if err := m.trust.SetPluginTrust(id, settings.PluginTrust{Hash: p.Hash, Enabled: true}); err != nil {
		return err
	}
	_, err := m.Discover()
	return err
}

// SetEnabled turns an approved plugin on or off. A plugin whose executable
// changed must be approved again instead.
func (m *Manager) SetEnabled(id string, enabled bool) error {
	trust, ok := m.trust.PluginTrust(id)
	if !ok {
		return fmt.Errorf("plugin %s was never approved", id)
	}
	m.mu.Lock()
	p, known := m.plugins[id]
	m.mu.Unlock()
	if enabled && known && p.Hash != trust.Hash {
		return fmt.Errorf("plugin %s changed since approval; approve it again", id)
	}
	trust.Enabled = enabled
	if err := m.trust.SetPluginTrust(id, trust); err != nil {
		return err
	}
	_, err := m.Discover()
	return err
}

// Call sends a request to a ready plugin, starting it on demand.
func (m *Manager) Call(ctx context.Context, id, method string, params, result any) error {
	proc, err := m.process(id)
	if err != nil {
		return err
	}
	return proc.call(ctx, method, params, result)
}

// Initialize starts the plugin if needed and returns its initialize answer.
func (m *Manager) Initialize(ctx context.Context, id string) (*InitResult, error) {
	proc, err := m.process(id)
	if err != nil {
		return nil, err
	}
	if _, err := proc.ensure(ctx); err != nil {
		return nil, err
	}
	proc.mu.Lock()
	defer proc.mu.Unlock()
	if proc.info == nil {
		return nil, fmt.Errorf("plugin %s exited", id)
	}
	info := *proc.info
	return &info, nil
}

// process returns the plugin's process, creating it if the plugin may run.
func (m *Manager) process(id string) (*process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("plugin manager is shut down")
	}
	p, ok := m.plugins[id]
	if !ok {
		return nil, fmt.Errorf("unknown plugin %q", id)
	}
	if p.Status != StatusReady {
		msg := fmt.Sprintf("plugin %s is %s", id, p.Status)
		if p.Problem != "" {
			msg += ": " + p.Problem
		}
		return nil, errors.New(msg)
	}
	if proc, ok := m.procs[id]; ok {
		return proc, nil
	}
	execPath, err := p.Manifest.ExecPath(p.Dir)
	if err != nil {
		return nil, err
	}
	logPath := ""
	if m.logDir != "" {
		if err := os.MkdirAll(m.logDir, 0o755); err == nil {
			logPath = filepath.Join(m.logDir, "plugin-"+id+".log")
		}
	}
	var next func(string, json.RawMessage)
	if m.onNotify != nil {
		next = func(method string, params json.RawMessage) { m.onNotify(id, method, params) }
	}
	pinned := p.Hash
	proc := &process{
		id:      id,
		dir:     p.Dir,
		exec:    execPath,
		hash:    pinned,
		logPath: logPath,
		notify:  notifyToLog(logPath, next),
		t:       m.t,
		// The file may change between discovery and any later (re)start.
		verify: func() error {
			h, err := hashFile(execPath)
			if err != nil {
				return err
			}
			if h != pinned {
				return fmt.Errorf("plugin %s: the executable changed since you approved it; approve it again", id)
			}
			return nil
		},
	}
	m.procs[id] = proc
	return proc, nil
}

// Running reports the PID of a running plugin, or 0.
func (m *Manager) Running(id string) int {
	m.mu.Lock()
	proc := m.procs[id]
	m.mu.Unlock()
	if proc == nil {
		return 0
	}
	return proc.pid()
}

// Shutdown stops every plugin (on app exit). The manager can't be used after.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	procs := m.procs
	m.procs = map[string]*process{}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, proc := range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proc.stop()
		}()
	}
	wg.Wait()
}
