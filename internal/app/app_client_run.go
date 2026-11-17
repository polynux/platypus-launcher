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

// clientRunTarget is one running core-run client command: its plugin source
// and job kind, so holds can name what the client is doing.
type clientRunTarget struct {
	source string
	kind   string
}

// clientRuns tracks the core-run client commands (protocol 1.23): one run
// per job id, like clientSetups. clientRunSource maps a running job to its
// target, so launches can hold while the client's command runs.
func (a *App) clientRunBegin(id, source, kind string, cancel context.CancelFunc) (*coreRun, error) {
	run, err := a.clientRuns.begin(id, cancel)
	if err != nil {
		return nil, err
	}
	a.clientRunMu.Lock()
	if a.clientRunSource == nil {
		a.clientRunSource = map[string]clientRunTarget{}
	}
	a.clientRunSource[id] = clientRunTarget{source: source, kind: kind}
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
	return a.clientRunKind(source) != ""
}

// clientRunKind is the kind of the client's running clientRun job, or "".
func (a *App) clientRunKind(source string) string {
	a.clientRunMu.Lock()
	defer a.clientRunMu.Unlock()
	for _, t := range a.clientRunSource {
		if t.source == source {
			return t.kind
		}
	}
	return ""
}

// clientRunJobs lists the running clientRun jobs of one source.
func (a *App) clientRunJobs(source string) []types.Job {
	a.clientRunMu.Lock()
	ids := map[string]bool{}
	for id, t := range a.clientRunSource {
		if t.source == source {
			ids[id] = true
		}
	}
	a.clientRunMu.Unlock()
	var out []types.Job
	for _, j := range a.jobs.List() {
		if ids[j.ID] {
			out = append(out, j)
		}
	}
	return out
}

// stopClientRuns stops every core-run client command of source (the jobs
// themselves are failed by the caller).
func (a *App) stopClientRuns(source string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, j := range a.clientRunJobs(source) {
		a.clientRuns.stop(ctx, j.ID)
	}
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
	if a.wineClientRunning(j.Source) {
		// Killing the client's wineserver would take the running game
		// with it: refuse to start while the client runs a game.
		p, ok := a.plugins.Plugin(j.Source)
		name := j.Source
		if ok {
			name = p.Manifest.Name
		}
		enAction, frAction := clientRunNoun(j.Kind)
		return apperr.New("client.gameRunning",
			fmt.Sprintf("%s is running a game; stop it before %s.", name, enAction),
			map[string]string{"client": name, "action": enAction, "actionFr": frAction})
	}
	runCtx, cancel := context.WithCancel(a.ctx)
	cr, err := a.clientRunBegin(j.ID, j.Source, j.Kind, cancel)
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

// clientRunVerb names what a clientRun job does, for hold messages.
func clientRunVerb(kind string) (en, fr string) {
	switch kind {
	case jobs.KindUpdate:
		return "updating", "mise à jour"
	case jobs.KindUninstall:
		return "uninstalling", "désinstallation"
	default:
		return "installing", "installation"
	}
}

// clientRunNoun names the job for "stop it before ..." messages.
func clientRunNoun(kind string) (en, fr string) {
	switch kind {
	case jobs.KindUpdate:
		return "the update", "la mise à jour"
	case jobs.KindUninstall:
		return "the uninstall", "la désinstallation"
	default:
		return "the install", "l'installation"
	}
}

// failClientRuns fails every running clientRun job of a plugin that exited,
// and stops the core-run commands (issue #212): the dead plugin's watch can
// never end them, so requeueing would leave the jobs in limbo.
func (a *App) failClientRuns(source string) {
	live := a.clientRunJobs(source)
	for _, j := range live {
		a.clientRuns.stop(a.ctx, j.ID)
	}
	for _, j := range live {
		p, ok := a.plugins.Plugin(source)
		name := source
		if ok {
			name = p.Manifest.Name
		}
		enNoun, _ := clientRunNoun(j.Kind)
		a.jobs.Notify(source, "job.failed", plugins.JobUpdate{
			JobID: j.ID, Error: fmt.Sprintf("%s stopped during %s; try again", name, enNoun),
		})
	}
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
	phase := "installing"
	if j.Kind == jobs.KindUninstall {
		phase = "uninstalling"
	}
	a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: phase})
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
		// Never stop the prefix's wineserver while a client session is
		// live: it would kill the running game along with the command.
		if !a.wineClientRunning(j.Source) {
			a.stopClientWineserver(j, wrapped)
		}
		return ctx.Err()
	}
}
