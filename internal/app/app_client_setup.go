package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/platform"
	"platypus/internal/plugins"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// clientSetupFetchTimeout bounds fetching the installer spec: the plugin may
// download the installer before answering.
const clientSetupFetchTimeout = 30 * time.Minute

// startClientSetup queues a client.setup job (protocol 1.20). The core runs
// the installer's spec itself in runClientSetup, as a core-run job: the
// queue only tracks it.
func (a *App) startClientSetup(j types.Job) error {
	if !a.plugins.Has(j.Source, "launch") {
		return fmt.Errorf("%s doesn't offer launches", j.Source)
	}
	if !a.plugins.Runtime(j.Source).Speaks(plugins.WineClientVersion) {
		return fmt.Errorf("%s speaks an older protocol and can't set up its client", j.Source)
	}
	runCtx, cancel := context.WithCancel(a.ctx)
	run, err := a.clientSetups.begin(j.ID, cancel)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.clientSetups.end(j.ID, run)
		defer cancel()
		a.runClientSetup(runCtx, j)
	}()
	return nil
}

// runClientSetup fetches the installer spec, enforces the download
// confirmation, and runs the installer under the runner in the client's
// pinned prefix. Progress is indeterminate (a phase, no byte counts): the
// installer is a black box, like a client download. Outcomes and failures go
// through the queue; a cancel kills the installer and stops the prefix's
// wineserver.
func (a *App) runClientSetup(ctx context.Context, j types.Job) {
	defer func() {
		if r := recover(); r != nil {
			a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf("client setup failed: %v", r)})
		}
	}()
	fail := func(err error) {
		if ctx.Err() != nil {
			return // canceled: the queue already marked it, later notifications are ignored
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
	}

	spec, err := a.fetchClientSetupSpec(ctx, j.Source, j.InstallDir, j.ID)
	if err != nil {
		fail(err)
		return
	}
	if spec.Confirm != "" && !j.Confirmed {
		fail(apperr.New("client.confirmDownload", spec.Confirm,
			map[string]string{"message": spec.Confirm, "source": j.Source}))
		return
	}

	// The core owns the prefix lifecycle: concurrent launches and setups of
	// one client serialize on the client's id while the folder is created
	// (the installer run itself is not serialized: the job queue holds the
	// client to one setup at a time, and launches check that hold).
	unlock := a.lockGame(stores.ClientGameID(j.Source))
	mkdirErr := os.MkdirAll(j.InstallDir, 0o755)
	unlock()
	if mkdirErr != nil {
		fail(mkdirErr)
		return
	}
	if ctx.Err() != nil {
		return
	}
	if spec.Size > 0 {
		free, err := platform.FreeSpace(j.InstallDir)
		if err != nil {
			fail(fmt.Errorf("free space check: %w", err))
			return
		}
		if free < uint64(spec.Size) {
			fail(fmt.Errorf("not enough space in %s: needs %s, %s free",
				j.InstallDir, jobs.HumanBytes(spec.Size), jobs.HumanBytes(int64(free))))
			return
		}
	}

	p, ok := a.plugins.Plugin(j.Source)
	if !ok {
		fail(fmt.Errorf("unknown plugin %q", j.Source))
		return
	}
	folders := append(a.plugins.Folders(j.Source, a.settings.GamesFolder()), j.InstallDir)
	setup, err := stores.ValidateClientSetup(spec, p.Manifest, folders, j.InstallDir)
	if err != nil {
		fail(err)
		return
	}
	present, err := a.runnerPlugin()
	if err != nil {
		fail(err)
		return
	}
	if !present {
		fail(apperr.New("game.noRunner",
			fmt.Sprintf("%s needs the runner plugin to set up its client", p.Manifest.Name),
			map[string]string{"title": p.Manifest.Name + " client"}))
		return
	}
	// The installer runs with the default runner and none of the game's
	// settings (protocol 1.20, S1): wrappers, env and bindings are per-game
	// choices, and the client belongs to no game.
	wrapped, err := a.wrapClientSetup(j, setup)
	if err != nil {
		fail(err)
		return
	}
	a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: "installing"})
	if err := a.runClientInstaller(ctx, j, setup, wrapped); err != nil {
		fail(err)
		return
	}
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
}

// fetchClientSetupSpec asks the plugin for its installer spec. jobID is empty
// when inspecting (no job queued yet); the plugin answers its cached spec.
func (a *App) fetchClientSetupSpec(ctx context.Context, source, prefix, jobID string) (plugins.ClientSetupSpec, error) {
	var spec plugins.ClientSetupSpec
	fetchCtx, cancel := context.WithTimeout(ctx, clientSetupFetchTimeout)
	defer cancel()
	if err := a.plugins.Call(fetchCtx, source, "client.setup",
		map[string]any{"jobId": jobID, "prefix": prefix}, &spec); err != nil {
		return plugins.ClientSetupSpec{}, err
	}
	return spec, nil
}

// wrapClientSetup wraps the validated installer for the client's pinned
// prefix: the default runner ("auto" resolves while installed, else the
// automatic pick) and no per-game settings.
func (a *App) wrapClientSetup(j types.Job, setup *stores.ClientSetup) (*prepared, error) {
	game := types.Game{
		ID:     stores.ClientGameID(j.Source),
		Source: j.Source,
		Title:  j.Title,
		Settings: types.GameSettings{
			WinePrefix: j.InstallDir,
		},
	}
	return a.wrapLaunch(game, append([]string{setup.Exe}, setup.Args...),
		setup.Env, filepath.Dir(setup.Exe), true, false, setup.Redact)
}

// runClientInstaller runs the wrapped installer to exit, logging to the
// client's session log. A cancel kills the installer tree and stops the
// prefix's wineserver, so no half-installed client keeps running.
func (a *App) runClientInstaller(ctx context.Context, j types.Job, setup *stores.ClientSetup, wrapped *prepared) error {
	lw, err := a.sessions.OpenSessionLog(stores.ClientGameID(j.Source), wrapped.shown(), setup.Redact)
	if err != nil {
		lw = nullWriter{}
	}
	defer func() { _ = lw.Close() }()
	cmd := exec.Command(wrapped.args[0], wrapped.args[1:]...)
	cmd.Env = os.Environ()
	for k, v := range wrapped.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = wrapped.dir
	cmd.Stdout, cmd.Stderr = lw, lw
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("client installer: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("client installer: %w", err)
		}
		return nil
	case <-ctx.Done():
		killProcessTree(cmd)
		<-done
		a.stopClientWineserver(j, wrapped)
		return ctx.Err()
	}
}

// stopClientWineserver asks the prefix's wineserver to terminate after a
// setup cancel, like the stop hook does for direct launches.
func (a *App) stopClientWineserver(j types.Job, wrapped *prepared) {
	launch := map[string]any{"env": wrapped.env, "runner": wrapped.runner}
	stopCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	_ = a.plugins.Call(stopCtx, "runner", "runner.runAction", map[string]any{
		"id": "stop", "force": false, "launch": launch,
	}, nil)
}

// nullWriter drops the installer output when the session log can't open.
type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nullWriter) Close() error                { return nil }

// SetupClient queues installing or repairing a store client's own prefix
// (protocol 1.20). Without confirmed, a spec that asks for download
// confirmation fails with client.confirmDownload carrying the plugin's
// prompt, and nothing is queued to run: confirm first, then call again with
// confirmed.
func (a *App) SetupClient(source string, confirmed bool) (types.Job, error) {
	p, ok := a.plugins.Plugin(source)
	name := source
	if ok {
		name = p.Manifest.Name
	}
	if !a.plugins.Has(source, "launch") {
		return types.Job{}, fmt.Errorf("%s doesn't offer launches", name)
	}
	if !a.plugins.Runtime(source).Speaks(plugins.WineClientVersion) {
		return types.Job{}, fmt.Errorf("%s speaks an older protocol and can't set up its client", name)
	}
	if a.wineClientRunning(source) {
		return types.Job{}, apperr.New("client.inUse",
			fmt.Sprintf("%s is running; stop the game before setting it up", name),
			map[string]string{"client": name, "source": source})
	}
	prefix := stores.ClientPrefixDir(source, a.effectivePrefixesFolder())
	if !confirmed {
		spec, err := a.fetchClientSetupSpec(a.ctxOrBackground(), source, prefix, "")
		if err != nil {
			return types.Job{}, err
		}
		if spec.Confirm != "" {
			return types.Job{}, apperr.New("client.confirmDownload", spec.Confirm,
				map[string]string{"message": spec.Confirm, "source": source})
		}
	}
	return a.jobs.AddClientSetup(source, stores.ClientGameID(source),
		name+" client", prefix, confirmed)
}

// ClientSetupJob returns the client's setup job while one holds it (queued,
// running or paused), so the UI can show "Setting up <client>…" instead of
// another setup button.
func (a *App) ClientSetupJob(source string) (types.Job, bool) {
	return a.jobs.Holding(stores.ClientGameID(source), jobs.KindClientSetup)
}

// noteWineSession records a running wine-client launch (gameID → source), so
// SetupClient refuses while the client runs. Cleared when the session ends.
func (a *App) noteWineSession(gameID, source string) {
	a.wineMu.Lock()
	defer a.wineMu.Unlock()
	if a.wineSessions == nil {
		a.wineSessions = map[string]string{}
	}
	a.wineSessions[gameID] = source
}

func (a *App) clearWineSession(gameID string) {
	a.wineMu.Lock()
	defer a.wineMu.Unlock()
	delete(a.wineSessions, gameID)
}

func (a *App) wineClientRunning(source string) bool {
	a.wineMu.Lock()
	defer a.wineMu.Unlock()
	for _, s := range a.wineSessions {
		if s == source {
			return true
		}
	}
	return false
}
