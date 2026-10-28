// Package plugins discovers store plugins in ~/.local/share/platypus/plugins,
// decides which ones may run (manifest, pinned hash, user approval) and runs
// them as JSON-RPC subprocesses. See docs/plans/launcher-plugins.md.
package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"platypus/internal/dataplug"
	"platypus/internal/settings"
	"platypus/internal/themes"
)

// Status says whether a discovered plugin may run.
type Status string

const (
	StatusReady      Status = "ready"
	StatusInvalid    Status = "invalid"    // manifest or executable is broken
	StatusUnapproved Status = "unapproved" // never approved by the user
	StatusChanged    Status = "changed"    // plugin folder changed since approval
	StatusDisabled   Status = "disabled"   // approved, then turned off
	StatusMissing    Status = "missing-requirements"
	// StatusNeedsNewer: approved, but the manifest `needs` a capability this
	// Platypus doesn't know. It is listed, never started.
	StatusNeedsNewer Status = "needs-newer-platypus"
	// StatusTooOld: the plugin only speaks protocol versions older than any
	// this Platypus still supports. Listed, never started.
	StatusTooOld Status = "too-old"
)

// Plugin is one folder under the plugins directory.
type Plugin struct {
	Dir      string        `json:"dir"`
	Manifest Manifest      `json:"manifest"`
	Hash     string        `json:"hash"` // SHA-256 of the plugin folder
	Status   Status        `json:"status"`
	Problem  string        `json:"problem,omitempty"`
	Missing  []Requirement `json:"missing,omitempty"`
	// Warnings are things the user should know that don't stop the plugin:
	// capabilities this Platypus ignores because it doesn't know them.
	Warnings []string `json:"warnings,omitempty"`
	// Data is a data plugin's content, loaded at discovery. Store plugins
	// have none.
	Data *dataplug.Data `json:"-"`
	// Theme is a data plugin's theme (issue #105), loaded at discovery with
	// internal/themes. Nil when the plugin carries none. An invalid theme
	// stays here as invalid: the picker lists it with its problem, and the
	// plugin's data still applies.
	Theme *themes.Theme `json:"-"`
}

// NotifyFunc receives notifications other than `log` from a running plugin.
type NotifyFunc func(pluginID, method string, params json.RawMessage)

// Options configure a Manager.
type Options struct {
	Dir      string          // plugins folder, one subfolder per plugin
	LogDir   string          // stderr and `log` notifications: LogDir/plugin-<id>.log
	DataDir  string          // private data: DataDir/<id>, passed as PLATYPUS_PLUGIN_DATA
	Settings *settings.Store // approvals and per-plugin folders
	// Home is the user's home for the "not the home folder itself" guard
	// (issue #41); empty reads the process home.
	Home     string
	OnNotify NotifyFunc
	// OnExit is called when a plugin exits without being asked to (crash),
	// so work in flight can be restarted.
	OnExit func(pluginID string, err error)
	// OnReady is called when a plugin has initialized (its protocol and
	// capabilities are settled) and is about to take requests. It runs on
	// its own goroutine, so it may call the plugin.
	OnReady func(pluginID string)
	// Protocols are the protocol versions Platypus offers plugins; nil means
	// SupportedProtocols. Tests narrow it to play an older Platypus.
	Protocols []Version
	// Timings overrides the crash/restart timings for tests. Zero fields
	// keep the defaults.
	Timings *Timings
	// Env adds variables to every plugin process's environment (issue #41:
	// tests pass fake-store knobs like ECHO_STEP_MS directly instead of
	// setting process-global state, so tests can run parallel).
	Env map[string]string
}

type Manager struct {
	dir       string
	logDir    string
	dataDir   string
	trust     *settings.Store
	home      string
	env       map[string]string
	onNotify  NotifyFunc
	onExit    func(string, error)
	onReady   func(string)
	lookPath  func(string) (string, error)
	t         timings
	protocols []Version

	mu      sync.Mutex
	plugins map[string]*Plugin
	procs   map[string]*process
	closed  bool
	// locale is the BCP 47 tag sent to plugins as initialize {locale} and
	// locale.changed {locale} (protocol 1.14, issue #39). Empty means English.
	locale string
}

// NewManager manages the plugins in o.Dir. Call Discover before use.
func NewManager(o Options) *Manager {
	home := o.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	m := &Manager{
		dir:       o.Dir,
		logDir:    o.LogDir,
		dataDir:   o.DataDir,
		trust:     o.Settings,
		home:      home,
		env:       maps.Clone(o.Env),
		onNotify:  o.OnNotify,
		onExit:    o.OnExit,
		onReady:   o.OnReady,
		lookPath:  exec.LookPath,
		t:         defaultTimings,
		protocols: slices.Clone(o.Protocols),
		plugins:   map[string]*Plugin{},
		procs:     map[string]*process{},
	}
	if len(m.protocols) == 0 {
		m.protocols = slices.Clone(SupportedProtocols)
	}
	if o.Timings != nil {
		t := *o.Timings
		if t.BackoffBase > 0 {
			m.t.backoffBase = t.BackoffBase
		}
		if t.BackoffMax > 0 {
			m.t.backoffMax = t.BackoffMax
		}
		if t.StableAfter > 0 {
			m.t.stableAfter = t.StableAfter
		}
		if t.MaxCrashes > 0 {
			m.t.maxCrashes = t.MaxCrashes
		}
		if t.StopGrace > 0 {
			m.t.stopGrace = t.StopGrace
		}
		if t.InitTimeout > 0 {
			m.t.initTimeout = t.InitTimeout
		}
	}
	return m
}

// DataDirOf is the private data folder a plugin gets as PLATYPUS_PLUGIN_DATA
// ("" when the manager has none).
func (m *Manager) DataDirOf(id string) string {
	if m == nil || m.dataDir == "" || id == "" {
		return ""
	}
	return filepath.Join(m.dataDir, id)
}

// Dir is the plugins folder.
func (m *Manager) Dir() string { return m.dir }

// Locale reports the BCP 47 tag sent to plugins (protocol 1.14). Empty means
// the default; callers send "en" on the wire for it.
func (m *Manager) Locale() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locale == "" {
		return "en"
	}
	return m.locale
}

// SetLocale records the effective locale and notifies every running plugin
// that negotiated protocol 1.14 or later with locale.changed {locale}.
// Plugins on older protocols get the new locale on their next start; the core
// never restarts them for this.
func (m *Manager) SetLocale(locale string) {
	type target struct {
		conn Conn
		prot string
	}
	m.mu.Lock()
	if locale == "" {
		locale = "en"
	}
	m.locale = locale
	var targets []target
	for _, proc := range m.procs {
		proc.mu.Lock()
		conn, rt := proc.conn, proc.runtime
		ready := proc.ready
		proc.mu.Unlock()
		if conn == nil || rt == nil {
			continue
		}
		select {
		case <-ready:
		default:
			continue // not initialized yet: initialize carries the locale
		}
		targets = append(targets, target{conn: conn, prot: rt.Protocol})
	}
	m.mu.Unlock()
	for _, t := range targets {
		v, err := ParseVersion(t.prot)
		if err != nil || v.Less(Version{1, 14}) {
			continue
		}
		_ = t.conn.Notify("locale.changed", map[string]any{"locale": locale})
	}
}

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

// inspect validates one plugin folder and works out its status. A data
// plugin without an approval record is ready when its JSON loads; one with
// a record (a zip install, issue #147) follows it like a store plugin.
func (m *Manager) inspect(dir string) *Plugin {
	p := &Plugin{Dir: dir}
	man, err := LoadManifest(dir)
	p.Manifest = man
	if err != nil {
		// Key broken plugins by folder name so each still shows up, once.
		p.Manifest.ID = filepath.Base(dir)
		p.Status, p.Problem = StatusInvalid, err.Error()
		var inc *IncompatibleError
		if errors.As(err, &inc) {
			p.Status = inc.Status
		}
		return p
	}
	if inc := man.Compatible(m.protocols); inc != nil {
		p.Status, p.Problem = inc.Status, inc.Reason
		return p
	}
	if man.IsData() {
		data, err := dataplug.Load(os.DirFS(dir), man.Data)
		if err != nil {
			// A theme-only plugin carries no fixes or tweaks: the missing
			// data files are not an error when it names a theme. Any other
			// data failure still breaks the plugin.
			if man.Theme == "" || !errors.Is(err, dataplug.ErrNoDataFiles) {
				p.Status, p.Problem = StatusInvalid, err.Error()
				return p
			}
		} else {
			p.Data = data
		}
		if p.Hash, err = hashPlugin(dir); err != nil {
			p.Status, p.Problem = StatusInvalid, err.Error()
			return p
		}
		// The theme loads whatever the approval state: only the status
		// decides whether it applies or picks (issue #147).
		if man.Theme != "" {
			th := themes.LoadFS(os.DirFS(dir), man.Theme, man.ID)
			p.Theme = &th
		}
		// A data plugin with an approval record follows it (issue #147): a
		// zip install files an empty record first, so it waits for approval
		// instead of applying its fixes, tweaks and theme at once. Without
		// a record it is a hand-installed folder, active as before.
		if trust, ok := m.trust.PluginState(man.ID); ok {
			switch {
			case trust.Hash == "":
				p.Status, p.Problem = StatusUnapproved, "waiting for approval after a zip install"
			case trust.Hash != p.Hash:
				p.Status, p.Problem = StatusChanged, "the plugin changed since you approved it"
			case !trust.Enabled:
				p.Status = StatusDisabled
			default:
				p.Status = StatusReady
			}
			return p
		}
		p.Status = StatusReady
		return p
	}
	if p.Hash, err = hashPlugin(dir); err != nil {
		p.Status, p.Problem = StatusInvalid, err.Error()
		return p
	}
	p.Missing = man.MissingRequirements(m.lookPath)
	unmet := man.UnmetNeeds()
	for _, c := range man.UnknownCapabilities() {
		if slices.Contains(unmet, c) {
			p.Warnings = append(p.Warnings, fmt.Sprintf("needs %q, which this Platypus doesn't know: a newer Platypus is required", c))
		} else {
			p.Warnings = append(p.Warnings, fmt.Sprintf("offers %q, which this Platypus doesn't know; it is ignored", c))
		}
	}

	trust, ok := m.trust.PluginState(man.ID)
	switch {
	case !ok:
		p.Status = StatusUnapproved
	case trust.Hash != p.Hash:
		p.Status, p.Problem = StatusChanged, "the plugin changed since you approved it"
	case !trust.Enabled:
		p.Status = StatusDisabled
	case len(unmet) > 0:
		p.Status, p.Problem = StatusNeedsNewer, fmt.Sprintf("needs a newer Platypus (unknown capability %s)", quoteList(unmet))
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
// tools) before calling this. Data plugins approve their folder hash instead:
// a zip-installed data plugin waits for approval before its fixes, tweaks
// and theme apply (issue #147).
func (m *Manager) Approve(id string) error {
	m.mu.Lock()
	p, ok := m.plugins[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown plugin %q", id)
	}
	// Hash again: what gets approved is what is on disk now.
	p = m.inspect(p.Dir)
	switch p.Status {
	case StatusInvalid:
		return fmt.Errorf("plugin %s is invalid: %s", id, p.Problem)
	case StatusNeedsNewer, StatusTooOld:
		if p.Hash == "" { // incompatible with this Platypus: there is nothing it could run
			return fmt.Errorf("plugin %s can't run on this Platypus: %s", id, p.Problem)
		}
	}
	if p.Manifest.IsData() && p.Hash == "" {
		return fmt.Errorf("plugin %s has nothing to approve", id)
	}
	state, _ := m.trust.PluginState(id) // keeps folder choices across re-approval
	state.Hash, state.Enabled = p.Hash, true
	if err := m.trust.SetPluginState(id, state); err != nil {
		return err
	}
	if _, err := m.Discover(); err != nil {
		return err
	}
	// Re-approving a gave-up plugin gives it another chance (CORE-10).
	m.resetCrashes(id)
	return nil
}

// SetEnabled turns an approved plugin on or off. A plugin whose executable
// changed must be approved again instead.
func (m *Manager) SetEnabled(id string, enabled bool) error {
	if p, ok := m.Plugin(id); ok && p.Manifest.IsData() {
		return fmt.Errorf("%s is a data plugin: nothing to enable or disable", id)
	}
	trust, ok := m.trust.PluginState(id)
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
	if err := m.trust.SetPluginState(id, trust); err != nil {
		return err
	}
	if _, err := m.Discover(); err != nil {
		return err
	}
	if enabled {
		// Re-enabling a gave-up plugin gives it another chance (CORE-10).
		m.resetCrashes(id)
	}
	return nil
}

// resetCrashes clears the crash count so the next call restarts the plugin
// instead of repeating the give-up error. Unknown plugins and plugins with
// no recorded run are a no-op.
func (m *Manager) resetCrashes(id string) {
	m.mu.Lock()
	proc := m.procs[id]
	m.mu.Unlock()
	if proc == nil {
		return
	}
	proc.mu.Lock()
	proc.crashes, proc.retryAt, proc.lastErr = 0, time.Time{}, nil
	proc.mu.Unlock()
}

// RetryPlugin clears a gave-up plugin's crash count so the next call
// restarts it immediately. The Plugins view offers it after a give-up
// (CORE-10).
func (m *Manager) RetryPlugin(id string) error {
	m.mu.Lock()
	_, known := m.plugins[id]
	proc := m.procs[id]
	m.mu.Unlock()
	if !known {
		return fmt.Errorf("unknown plugin %q", id)
	}
	if proc == nil {
		return nil // nothing recorded; the next call starts fresh anyway
	}
	proc.mu.Lock()
	proc.crashes, proc.retryAt, proc.lastErr = 0, time.Time{}, nil
	proc.mu.Unlock()
	return nil
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
	if _, _, err := proc.ensure(ctx); err != nil {
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

// Runtime says what the plugin offers right now: what its last initialize
// returned, or the manifest's capabilities until it has initialized. It never
// starts the plugin.
func (m *Manager) Runtime(id string) Runtime {
	m.mu.Lock()
	p, known := m.plugins[id]
	proc := m.procs[id]
	m.mu.Unlock()
	if proc != nil {
		proc.mu.Lock()
		rt := proc.runtime
		proc.mu.Unlock()
		if rt != nil {
			return Runtime{Initialized: true, Protocol: rt.Protocol, Capabilities: slices.Clone(rt.Capabilities), Features: rt.Features}
		}
	}
	if !known {
		return Runtime{}
	}
	caps := slices.Clone(p.Manifest.Capabilities)
	if caps == nil {
		caps = []string{}
	}
	return Runtime{Capabilities: caps}
}

// Has reports whether the plugin offers the capability right now (see Runtime).
func (m *Manager) Has(id, capability string) bool {
	return slices.Contains(m.Runtime(id).Capabilities, capability)
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
	dataDir := ""
	if m.dataDir != "" {
		dataDir = filepath.Join(m.dataDir, id)
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return nil, err
		}
	}
	var onExit func(error)
	if m.onExit != nil {
		onExit = func(err error) { m.onExit(id, err) }
	}
	var onReady func()
	if m.onReady != nil {
		onReady = func() { go m.onReady(id) }
	}
	pinned := p.Hash
	offered, transportName := p.Manifest.Offered()
	transport, ok := LookupTransport(transportName)
	if !ok { // inspect refuses these; a plugin can't change its manifest under us
		return nil, fmt.Errorf("plugin %s: unknown transport %q", id, transportName)
	}
	proc := &process{
		dataDir:   dataDir,
		declared:  p.Manifest.Capabilities,
		protocols: m.protocols,
		offered:   offered,
		transport: transport,
		env:       maps.Clone(m.env),
		onExit:    onExit,
		onReady:   onReady,
		locale:    m.Locale,
		id:        id,
		dir:       p.Dir,
		exec:      execPath,
		hash:      pinned,
		logPath:   logPath,
		notify:    notifyToLog(logPath, next),
		t:         m.t,
		// The file may change between discovery and any later (re)start.
		verify: func() error {
			h, err := hashPlugin(p.Dir)
			if err != nil {
				return err
			}
			if h != pinned {
				return fmt.Errorf("plugin %s: the plugin changed since you approved it; approve it again", id)
			}
			return nil
		},
	}
	m.procs[id] = proc
	return proc, nil
}

// Plugin returns one plugin from the last Discover.
func (m *Manager) Plugin(id string) (Plugin, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[id]
	if !ok {
		return Plugin{}, false
	}
	return *p, true
}

// DataPlugins returns every valid data plugin from the last Discover, sorted
// by id.
func (m *Manager) DataPlugins() []Plugin {
	var out []Plugin
	for _, p := range m.Plugins() {
		if p.Manifest.IsData() && p.Data != nil {
			out = append(out, p)
		}
	}
	return out
}

// InstallFolder is where the plugin installs games: the user's override, or
// <gamesFolder>/<install_dir>. It also returns the default.
func (m *Manager) InstallFolder(id, gamesFolder string) (folder, def string) {
	p, _ := m.Plugin(id)
	def = p.Manifest.InstallPath(gamesFolder)
	if st, ok := m.trust.PluginState(id); ok && st.InstallDir != "" {
		return st.InstallDir, def
	}
	return def, def
}

// SetInstallFolder overrides the plugin's install folder; "" restores the default.
func (m *Manager) SetInstallFolder(id, path string) error {
	if path != "" && !filepath.IsAbs(path) {
		return fmt.Errorf("install folder must be an absolute path, got %q", path)
	}
	st, ok := m.trust.PluginState(id)
	if !ok {
		return fmt.Errorf("plugin %s was never approved", id)
	}
	if path != "" {
		path = filepath.Clean(path)
		// The install folder is always a valid launch location, so it
		// gets the same system-folder guard as AllowFolder (CORE-6).
		if err := m.checkAllowable(path); err != nil {
			return err
		}
	}
	st.InstallDir = path
	return m.trust.SetPluginState(id, st)
}

// AllowFolder lets the plugin's games launch from an extra folder, for games
// its CLI installed before Platypus managed it.
func (m *Manager) AllowFolder(id, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("folder must be an absolute path, got %q", path)
	}
	st, ok := m.trust.PluginState(id)
	if !ok {
		return fmt.Errorf("plugin %s was never approved", id)
	}
	path = filepath.Clean(path)
	if err := m.checkAllowable(path); err != nil {
		return err
	}
	for _, f := range st.Folders {
		if f == path {
			return nil
		}
	}
	st.Folders = append(st.Folders, path)
	return m.trust.SetPluginState(id, st)
}

// systemDirs can't be allowed as game folders: launching "games" from there
// would let a plugin run any system binary.
var systemDirs = []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/proc", "/run", "/sbin", "/sys", "/usr", "/var"}

func checkAllowable(path string) error {
	home, _ := os.UserHomeDir()
	return checkAllowableFor(path, home)
}

// checkAllowableFor is checkAllowable with an explicit home (issue #41).
func checkAllowableFor(path, home string) error {
	if path == "/" {
		return errors.New("refusing to allow the whole filesystem")
	}
	if home != "" && path == filepath.Clean(home) {
		return errors.New("refusing to allow your whole home folder; pick the game's folder")
	}
	for _, d := range systemDirs {
		if path == d || strings.HasPrefix(path, d+"/") {
			return fmt.Errorf("refusing to allow system folder %s", path)
		}
	}
	return nil
}

// checkAllowable is the manager's guard with its configured home.
func (m *Manager) checkAllowable(path string) error {
	return checkAllowableFor(path, m.home)
}

// Folders are the folders the plugin's games may be launched from: its
// install folder first, then the ones the user allowed.
func (m *Manager) Folders(id, gamesFolder string) []string {
	folder, _ := m.InstallFolder(id, gamesFolder)
	out := []string{folder}
	if st, ok := m.trust.PluginState(id); ok {
		out = append(out, st.Folders...)
	}
	return out
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

// GaveUp reports whether a plugin gave up: it crashed maxCrashes times in a
// row without a stable run in between, so every call fails fast until
// RetryPlugin (or re-approval, or re-enabling) resets the count (CORE-10).
// The Plugins view offers a Retry action when this is true.
func (m *Manager) GaveUp(id string) bool {
	m.mu.Lock()
	proc := m.procs[id]
	m.mu.Unlock()
	if proc == nil {
		return false
	}
	proc.mu.Lock()
	defer proc.mu.Unlock()
	return proc.crashes >= proc.t.maxCrashes
}

// Uninstall stops the plugin and removes its folder. It refuses anything
// outside the plugins folder and never touches a missing manager dir. The
// caller re-discovers afterwards; approval and settings are the caller's to
// forget.
func (m *Manager) Uninstall(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("refusing to uninstall %q: not a plugin id", id)
	}
	if m.dir == "" {
		return fmt.Errorf("refusing to uninstall %q: no plugins folder", id)
	}
	target := filepath.Join(m.dir, id)
	rel, err := filepath.Rel(m.dir, target)
	if err != nil || rel != id || !filepath.IsLocal(rel) {
		return fmt.Errorf("refusing to uninstall %q: outside the plugins folder", id)
	}
	m.mu.Lock()
	proc := m.procs[id]
	delete(m.procs, id)
	// Forget the plugin before stopping it, so a concurrent refresh can't
	// restart it from the folder being removed (it would recreate its
	// private data after the caller deleted it).
	delete(m.plugins, id)
	m.mu.Unlock()
	if proc != nil {
		proc.stop()
	}
	if _, err := os.Lstat(target); err != nil {
		return fmt.Errorf("unknown plugin %q", id)
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return nil
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

func quoteList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(q, ", ")
}
