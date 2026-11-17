package app

import (
	"context"
	"fmt"
	"os"

	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// startClientSetup runs a client.setup job (protocol 1.19): the core owns the
// prefix lifecycle (it creates the folder), the plugin installs or repairs
// its store client in it and reports progress like any other job. Cancel is
// job.cancel, like installs.
func (a *App) startClientSetup(ctx context.Context, j types.Job) error {
	if !a.plugins.Runtime(j.Source).Speaks(plugins.WineClientVersion) {
		return fmt.Errorf("%s speaks an older protocol and can't set up its client", j.Source)
	}
	if err := os.MkdirAll(j.InstallDir, 0o755); err != nil {
		return err
	}
	return a.plugins.Call(ctx, j.Source, "client.setup",
		map[string]any{"jobId": j.ID, "prefix": j.InstallDir}, nil)
}

// SetupClient queues installing or repairing a store client's own prefix
// (protocol 1.19). The plugin must offer launches over protocol 1.19 or
// later; the job shows in the queue like any other job.
func (a *App) SetupClient(source string) (types.Job, error) {
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
	return a.jobs.AddClientSetup(source, stores.ClientGameID(source),
		name+" client", stores.ClientPrefixDir(source, a.effectivePrefixesFolder()))
}

// ClientSetupJob returns the client's setup job while one holds it (queued,
// running or paused), so the UI can show "Setting up <client>…" instead of
// another setup button.
func (a *App) ClientSetupJob(source string) (types.Job, bool) {
	return a.jobs.Holding(stores.ClientGameID(source), jobs.KindClientSetup)
}
