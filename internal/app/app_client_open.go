package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// clientOpenFetchTimeout bounds asking the plugin for its open spec: the
// plugin answers its cached spec, like the setup inspect path.
const clientOpenFetchTimeout = 30 * time.Second

// ClientState reports what the plugin card shows for a wine-client store
// (protocol 1.20): whether its client is set up and what holds it. SetUp is
// the launch path's own answer: the pinned client prefix exists and the
// client exe the plugin reports (client.openSpec, protocol 1.25) validates
// there, like a wine-client launch's client.notSetUp. A plugin that doesn't
// speak 1.25, or whose spec doesn't validate, reads as not set up; nothing
// here errors for that, so the card falls back to Set up (a repair).
func (a *App) ClientState(source string) (types.ClientState, error) {
	var st types.ClientState
	p, ok := a.plugins.Plugin(source)
	if !ok {
		return st, fmt.Errorf("unknown plugin %q", source)
	}
	if !a.plugins.Has(source, "launch") || !a.plugins.Runtime(source).Speaks(plugins.WineClientVersion) {
		return st, nil
	}
	st.Supported = true
	prefix := stores.ClientPrefixDir(source, a.effectivePrefixesFolder())
	// Platypus sends client.openSpec only to a plugin that settled on 1.25
	// or later; without it the core has no client exe to check.
	if !a.plugins.Runtime(source).Speaks(plugins.ClientOpenVersion) {
		st.Running = a.wineClientRunning(source) || a.isRunning(stores.ClientGameID(source))
		if j, ok := a.jobs.Holding(stores.ClientGameID(source), jobs.KindClientSetup); ok {
			j := j
			st.SetupJob = &j
		}
		st.ClientRunBusy = a.clientRunHolding(source)
		return st, nil
	}
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
	st.Running = a.wineClientRunning(source) || a.isRunning(stores.ClientGameID(source))
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
// containment, the env denylist (there is no env to deny: the spec carries
// none) and Windows path resolution. While a setup or clientRun job holds
// the client, or the client already runs, opening refuses with a readable
// error instead of starting a second copy. The process is tracked under the
// client's pseudo id like a client session, so a later game launch reuses
// the running client and a clientRun refuses while it runs.
func (a *App) OpenClient(source string) (string, error) {
	p, ok := a.plugins.Plugin(source)
	name := source
	if ok {
		name = p.Manifest.Name
	}
	if !a.plugins.Has(source, "launch") {
		return "", fmt.Errorf("%s doesn't offer launches", name)
	}
	if !a.plugins.Runtime(source).Speaks(plugins.WineClientVersion) {
		return "", fmt.Errorf("%s speaks an older protocol and can't open its client", name)
	}
	if !a.plugins.Runtime(source).Speaks(plugins.ClientOpenVersion) {
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
	if a.wineClientRunning(source) || a.isRunning(stores.ClientGameID(source)) {
		return "", apperr.New("client.running",
			fmt.Sprintf("%s is already running", name),
			map[string]string{"client": name})
	}
	prefix := stores.ClientPrefixDir(source, a.effectivePrefixesFolder())
	spec, err := a.fetchClientOpenSpec(source, prefix)
	if err != nil {
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
	wrapped, err := a.wrapLaunch(game, append([]string{open.Exe}, open.Args...), nil, open.Dir, true, false, nil)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(wrapped.args[0], wrapped.args[1:]...)
	cmd.Env = os.Environ()
	for k, v := range wrapped.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = wrapped.dir
	pid, err := a.sessions.StartDirect(game.ID, cmd, wrapped.shown(), nil,
		session.WithOnExit(func(session.ExitInfo) { a.clearWineSession(game.ID) }))
	if err != nil {
		if errors.Is(err, session.ErrAlreadyRunning) {
			return "", apperr.New("client.running",
				fmt.Sprintf("%s is already running", name),
				map[string]string{"client": name})
		}
		return "", fmt.Errorf("%s client: %w", name, err)
	}
	a.noteWineSession(game.ID, source)
	return fmt.Sprintf("Opened %s (PID %d)", name, pid), nil
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
