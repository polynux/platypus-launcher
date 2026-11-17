package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/platform"
	"platypus/internal/plugins"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// clientRuns tracks the core-run client commands (protocol 1.23): one run
// per job id, like clientSetups. clientRunSource maps a running job to its
// plugin source, so launches can hold while the client's command runs.
func (a *App) clientRunBegin(id, source string, cancel context.CancelFunc) (*coreRun, error) {
	run, err := a.clientRuns.begin(id, cancel)
	if err != nil {
		return nil, err
	}
	a.clientRunMu.Lock()
	if a.clientRunSource == nil {
		a.clientRunSource = map[string]string{}
	}
	a.clientRunSource[id] = source
	a.clientRunMu.Unlock()
	return run, nil
}

func (a *App) clientRunEnd(id string, run *coreRun) {
	a.clientRunMu.Lock()
	delete(a.clientRunSource, id)
	a.clientRunMu.Unlock()
	a.clientRuns.end(id, run)
}

// clientRunHolding reports whether a core-run client command for source is
// running (protocol 1.23): the client's games refuse to launch meanwhile.
func (a *App) clientRunHolding(source string) bool {
	a.clientRunMu.Lock()
	defer a.clientRunMu.Unlock()
	for _, s := range a.clientRunSource {
		if s == source {
			return true
		}
	}
	return false
}

// startClientRun runs the plugin's clientRun answer as part of its
// install/update/uninstall job (protocol 1.23, issue #212). The answer was
// already validated and the job already sized: this only checks the holds,
// wraps the command for the pinned client prefix, and runs it. The plugin
// still owns the job and ends it with job.done / job.failed from its own
// watch; the core never ends it when the command exits.
func (a *App) startClientRun(j types.Job, answer plugins.InstallAnswer, run *stores.ClientRun) error {
	if !a.plugins.Has(j.Source, "launch") {
		return fmt.Errorf("%s doesn't offer launches", j.Source)
	}
	if !a.plugins.Runtime(j.Source).Speaks(plugins.WineClientVersion) {
		return fmt.Errorf("%s speaks an older protocol and can't run client commands", j.Source)
	}
	if _, ok := a.jobs.Holding(stores.ClientGameID(j.Source), jobs.KindClientSetup); ok {
		return fmt.Errorf("%s is being set up; try again once that finishes", j.Title)
	}
	if a.clientRunHolding(j.Source) {
		return fmt.Errorf("%s is busy with another client command; try again once it finishes", j.Title)
	}
	runCtx, cancel := context.WithCancel(a.ctx)
	cr, err := a.clientRunBegin(j.ID, j.Source, cancel)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.clientRunEnd(j.ID, cr)
		defer cancel()
		a.runClientRun(runCtx, j, answer, run)
	}()
	return nil
}

// runClientRun wraps the validated client command for the pinned client
// prefix and runs it to exit, logging to the client's session log. A cancel
// kills the command and stops the prefix's wineserver. The job itself is
// left running: the plugin ends it from its own watch.
func (a *App) runClientRun(ctx context.Context, j types.Job, answer plugins.InstallAnswer, run *stores.ClientRun) {
	defer func() {
		if r := recover(); r != nil {
			a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf("client command failed: %v", r)})
		}
	}()
	fail := func(err error) {
		if ctx.Err() != nil {
			return // canceled: the queue already marked it, later notifications are ignored
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
	}

	clientDir := stores.ClientPrefixDir(j.Source, a.effectivePrefixesFolder())
	unlock := a.lockGame(stores.ClientGameID(j.Source))
	var mkdirErr error
	if answer.ClientOwnsLocation {
		mkdirErr = os.MkdirAll(clientDir, 0o755)
	} else {
		mkdirErr = os.MkdirAll(j.InstallDir, 0o755)
	}
	unlock()
	if mkdirErr != nil {
		fail(mkdirErr)
		return
	}
	if ctx.Err() != nil {
		return
	}
	if answer.Size > 0 {
		dir := j.InstallDir
		if answer.ClientOwnsLocation {
			dir = clientDir
		}
		free, err := platform.FreeSpace(dir)
		if err != nil {
			fail(fmt.Errorf("free space check: %w", err))
			return
		}
		if free < uint64(answer.Size) {
			fail(fmt.Errorf("not enough space in %s: needs %s, %s free",
				dir, jobs.HumanBytes(answer.Size), jobs.HumanBytes(int64(free))))
			return
		}
	}

	p, ok := a.plugins.Plugin(j.Source)
	name := j.Source
	if ok {
		name = p.Manifest.Name
	} else {
		fail(fmt.Errorf("unknown plugin %q", j.Source))
		return
	}
	present, err := a.runnerPlugin()
	if err != nil {
		fail(err)
		return
	}
	if !present {
		fail(apperr.New("game.noRunner",
			fmt.Sprintf("%s needs the runner plugin to run its client", name),
			map[string]string{"title": name + " client"}))
		return
	}
	wrapped, err := a.wrapClientRun(j, clientDir, run)
	if err != nil {
		fail(err)
		return
	}
	a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: "installing"})
	if err := a.runClientCommand(ctx, j, run, wrapped); err != nil {
		fail(err)
		return
	}
	// The command exited; the job stays running until the plugin's watch
	// ends it with job.done / job.failed.
}

// wrapClientRun wraps the validated client command for the client's pinned
// prefix: the default runner and none of the game's settings, like a client
// setup (protocol 1.20, S1).
func (a *App) wrapClientRun(j types.Job, clientDir string, run *stores.ClientRun) (*prepared, error) {
	game := types.Game{
		ID:     stores.ClientGameID(j.Source),
		Source: j.Source,
		Title:  j.Title,
		Settings: types.GameSettings{
			WinePrefix: clientDir,
		},
	}
	return a.wrapLaunch(game, append([]string{run.Exe}, run.Args...),
		run.Env, filepath.Dir(run.Exe), true, false, run.Redact)
}

// runClientCommand runs the wrapped client command to exit, logging to the
// client's session log. A cancel kills the command tree and stops the
// prefix's wineserver.
func (a *App) runClientCommand(ctx context.Context, j types.Job, run *stores.ClientRun, wrapped *prepared) error {
	lw, err := a.sessions.OpenSessionLog(stores.ClientGameID(j.Source), wrapped.shown(), run.Redact)
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
		return fmt.Errorf("client command: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("client command: %w", err)
		}
		return nil
	case <-ctx.Done():
		killProcessTree(cmd)
		<-done
		a.stopClientWineserver(j, wrapped)
		return ctx.Err()
	}
}
