package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// clientOpenFetchTimeout bounds asking the plugin for its open spec: the
// plugin answers its cached spec, like the setup inspect path.
const clientOpenFetchTimeout = 30 * time.Second

// openRun is one standalone client opened from the plugin card, until its
// waiter has unwound.
type openRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// openTracker tracks the standalone clients opened from the plugin card
// (protocol 1.25), one run per source. The zero value is ready to use.
type openTracker struct {
	mu   sync.Mutex
	runs map[string]*openRun
}

func (t *openTracker) add(source string, r *openRun) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.runs == nil {
		t.runs = map[string]*openRun{}
	}
	t.runs[source] = r
}

func (t *openTracker) remove(source string, r *openRun) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.runs[source] == r {
		delete(t.runs, source)
	}
}

// stop cancels one run and waits, until ctx ends, for its waiter to unwind.
func (t *openTracker) stop(ctx context.Context, source string) {
	t.mu.Lock()
	r := t.runs[source]
	t.mu.Unlock()
	if r == nil {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-ctx.Done():
	}
}

// stopAll cancels every run and waits, until ctx ends, for the waiters: the
// shutdown stops the opened clients before the plugins (and the runner they
// need for the wineserver stop) go away.
func (t *openTracker) stopAll(ctx context.Context) {
	t.mu.Lock()
	runs := make([]*openRun, 0, len(t.runs))
	for _, r := range t.runs {
		runs = append(runs, r)
	}
	t.mu.Unlock()
	for _, r := range runs {
		r.cancel()
	}
	for _, r := range runs {
		select {
		case <-r.done:
		case <-ctx.Done():
			return
		}
	}
}

// ClientState reports what the plugin card shows for a wine-client store
// (protocol 1.25): whether its client is set up and what holds it. Supported
// means the plugin speaks client.openSpec, so it can name its standalone
// client exe; older wine-client stores keep the game-page setup path only.
// OffersProtocol reads the manifest, so a cold core (plugin not started
// yet) still shows the card. SetUp is the launch path's own answer: the
// pinned client prefix exists and the reported exe validates there, like a
// wine-client launch's client.notSetUp. A spec that doesn't validate reads
// as not set up; nothing here errors for that, so the card falls back to
// Set up (a repair).
func (a *App) ClientState(source string) (types.ClientState, error) {
	var st types.ClientState
	p, ok := a.plugins.Plugin(source)
	if !ok {
		return st, fmt.Errorf("unknown plugin %q", source)
	}
	if !a.plugins.Has(source, "launch") || !a.plugins.OffersProtocol(source, plugins.ClientOpenVersion) {
		return st, nil
	}
	st.Supported = true
	prefix := stores.ClientPrefixDir(source, a.effectivePrefixesFolder())
	if fi, err := os.Stat(prefix); err == nil && fi.IsDir() {
		ctx, cancel := context.WithTimeout(a.ctxOrBackground(), clientOpenFetchTimeout)
		var spec plugins.ClientOpenSpec
		err := a.plugins.Call(ctx, source, "client.openSpec", map[string]any{"prefix": prefix}, &spec)
		cancel()
		if err == nil {
			folders := append(a.plugins.Folders(source, a.settings.GamesFolder()), prefix)
			if _, verr := stores.ValidateClientOpen(spec, p.Manifest, folders, prefix); verr == nil {
				st.SetUp = true
			}
		}
	}
	st.Running = a.wineClientRunning(source)
	if j, ok := a.jobs.Holding(stores.ClientGameID(source), jobs.KindClientSetup); ok {
		j := j
		st.SetupJob = &j
	}
	st.ClientRunBusy = a.clientRunHolding(source)
	return st, nil
}

// OpenClient starts a wine-client store by itself (protocol 1.25), so the
// user can sign in from the plugin card with no game to launch. The client
// exe comes from the plugin's client.openSpec answer and runs under the
// runner in the pinned client prefix, validated like a wine-client launch:
// containment, Windows path resolution, and the spec's redaction on the log
// path. While a setup or clientRun job holds the client, or the client
// already runs, opening refuses with a readable error instead of starting a
// second copy.
//
// The opened client is not a session: no session event, no playlog row, no
// tray or auto-update hold. It is tracked in two cheaper places instead:
// the wine session (so a later game launch reuses the running client, a
// clientRun refuses while it runs, and the card reads as running) and the
// open tracker (so shutdown and the tests can stop it).
func (a *App) OpenClient(source string) (string, error) {
	p, ok := a.plugins.Plugin(source)
	name := source
	if ok {
		name = p.Manifest.Name
	}
	if !a.plugins.Has(source, "launch") {
		return "", fmt.Errorf("%s doesn't offer launches", name)
	}
	if !a.plugins.OffersProtocol(source, plugins.WineClientVersion) {
		return "", fmt.Errorf("%s speaks an older protocol and can't open its client", name)
	}
	if !a.plugins.OffersProtocol(source, plugins.ClientOpenVersion) {
		return "", fmt.Errorf("%s doesn't open its client on its own", name)
	}
	if _, ok := a.jobs.Holding(stores.ClientGameID(source), jobs.KindClientSetup); ok {
		return "", apperr.New("client.busy",
			fmt.Sprintf("%s is being set up; try again once that finishes", name),
			map[string]string{"client": name, "action": "setting up", "actionFr": "configuration"})
	}
	if kind := a.clientRunKind(source); kind != "" {
		enVerb, frVerb := clientRunVerb(kind)
		return "", apperr.New("client.busy",
			fmt.Sprintf("%s is %s; try again once it finishes", name, enVerb),
			map[string]string{"client": name, "action": enVerb, "actionFr": frVerb})
	}
	if a.wineClientRunning(source) {
		return "", apperr.New("client.running",
			fmt.Sprintf("%s is already running", name),
			map[string]string{"client": name})
	}
	prefix := stores.ClientPrefixDir(source, a.effectivePrefixesFolder())
	spec, err := a.fetchClientOpenSpec(source, prefix)
	if err != nil {
		return "", err
	}
	// Serialize validate → wrap → start on the client id, so two Opens (or
	// an Open racing a game launch's prefix creation) don't run the client
	// twice. The running check repeats under the lock.
	unlock := a.lockGame(stores.ClientGameID(source))
	defer unlock()
	if a.wineClientRunning(source) {
		return "", apperr.New("client.running",
			fmt.Sprintf("%s is already running", name),
			map[string]string{"client": name})
	}
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}
	folders := append(a.plugins.Folders(source, a.settings.GamesFolder()), prefix)
	open, err := stores.ValidateClientOpen(spec, p.Manifest, folders, prefix)
	if err != nil {
		return "", err
	}
	present, err := a.runnerPlugin()
	if err != nil {
		return "", err
	}
	if !present {
		return "", apperr.New("game.noRunner",
			fmt.Sprintf("%s needs the runner plugin to open its client", name),
			map[string]string{"title": name + " client"})
	}
	game := types.Game{
		ID:     stores.ClientGameID(source),
		Source: source,
		Title:  name + " client",
		Settings: types.GameSettings{
			WinePrefix: prefix,
		},
	}
	wrapped, err := a.wrapLaunch(game, append([]string{open.Exe}, open.Args...), nil, open.Dir, true, false, open.Redact)
	if err != nil {
		return "", err
	}
	lw, err := a.sessions.OpenSessionLog(game.ID, wrapped.shown(), open.Redact)
	if err != nil {
		lw = nullWriter{}
	}
	cmd := exec.Command(wrapped.args[0], wrapped.args[1:]...)
	cmd.Env = os.Environ()
	for k, v := range wrapped.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = wrapped.dir
	cmd.Stdout, cmd.Stderr = lw, lw
	detachProcess(cmd)
	// Note the wine session before the process can exit: an instant exit
	// still clears it in the waiter, never leaving Running stuck.
	a.noteWineSession(game.ID, source)
	if err := cmd.Start(); err != nil {
		a.clearWineSession(game.ID)
		_ = lw.Close()
		return "", fmt.Errorf("%s client: %w", name, err)
	}
	runCtx, cancel := context.WithCancel(a.ctx)
	run := &openRun{cancel: cancel, done: make(chan struct{})}
	a.clientOpens.add(source, run)
	go a.waitOpenClient(runCtx, run, source, game.ID, cmd, lw, wrapped)
	return fmt.Sprintf("Opened %s (PID %d)", name, cmd.Process.Pid), nil
}
func (a *App) waitOpenClient(ctx context.Context, run *openRun, source, gameID string, cmd *exec.Cmd, lw io.Closer, wrapped *prepared) {
	// Liveness first: a stop (or the shutdown) waits on done, so it
	// always closes, even if the lines below panic.
	defer close(run.done)
	defer run.cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		// The user closed the client (or it exited on its own).
	case <-ctx.Done():
		killProcessTree(cmd)
		<-done
	}
	// Exit always clears the note (even an instant one), before asking
	// whether anything else still runs in the prefix.
	a.clearWineSession(gameID)
	if !a.wineClientRunning(source) {
		a.stopClientWineserver(wrapped)
	}
	_ = lw.Close()
	a.clientOpens.remove(source, run)
}

// stopOpenClient stops the standalone client opened from the card, if one
// runs. The UI offers no stop for it; the tests use this.
func (a *App) stopOpenClient(source string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a.clientOpens.stop(ctx, source)
}

// fetchClientOpenSpec asks the plugin for its standalone client spec. A
// -32601 answer means the plugin has no client to open on its own.
func (a *App) fetchClientOpenSpec(source, prefix string) (plugins.ClientOpenSpec, error) {
	var spec plugins.ClientOpenSpec
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), clientOpenFetchTimeout)
	defer cancel()
	p, _ := a.plugins.Plugin(source)
	name := source
	if p.Manifest.Name != "" {
		name = p.Manifest.Name
	}
	if err := a.plugins.Call(ctx, source, "client.openSpec", map[string]any{"prefix": prefix}, &spec); err != nil {
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound {
			return plugins.ClientOpenSpec{}, fmt.Errorf("%s doesn't open its client on its own", name)
		}
		return plugins.ClientOpenSpec{}, err
	}
	return spec, nil
}
