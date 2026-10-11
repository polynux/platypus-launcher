package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

// Plugin calls made from the Plugins view: a login can wait on the store.
const authTimeout = time.Minute

func (a *App) storePlugin(p plugins.Plugin) types.StorePlugin {
	m := p.Manifest
	kind := m.Kind
	if kind == "" {
		kind = plugins.KindStore
	}
	folder, def := a.plugins.InstallFolder(m.ID, a.settings.GamesFolder())
	extra := a.plugins.Folders(m.ID, a.settings.GamesFolder())[1:]
	reqs := func(rs []plugins.Requirement) []types.PluginRequirement {
		out := make([]types.PluginRequirement, len(rs))
		for i, r := range rs {
			out[i] = types.PluginRequirement{Bin: r.Bin, Hint: r.Hint}
		}
		return out
	}
	nonNil := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	rt := a.plugins.Runtime(m.ID)
	offers := make([]types.PluginCapability, 0, len(m.Capabilities))
	for _, name := range m.Capabilities {
		c, known := plugins.LookupCapability(name)
		f := rt.Features[name]
		if f == nil {
			f = map[string]bool{}
		}
		offers = append(offers, types.PluginCapability{
			Name: name, On: slices.Contains(rt.Capabilities, name), Known: known, Orchestrated: c.Orchestrated, Features: f,
		})
	}
	versions, transport := "", ""
	if kind == plugins.KindStore {
		set, tr := m.Offered()
		versions, transport = set.String(), tr
	}
	return types.StorePlugin{
		ID:                   m.ID,
		Name:                 m.Name,
		Version:              m.Version,
		Author:               m.Author,
		Description:          m.Description,
		Store:                m.Store,
		Kind:                 kind,
		Status:               string(p.Status),
		Problem:              p.Problem,
		Missing:              reqs(p.Missing),
		Requires:             reqs(m.Requires),
		Capabilities:         nonNil(rt.Capabilities),
		Declared:             nonNil(m.Capabilities),
		Offers:               offers,
		Needs:                nonNil(m.Needs),
		Warnings:             nonNil(p.Warnings),
		Initialized:          rt.Initialized,
		Protocol:             rt.Protocol,
		ProtocolVersions:     versions,
		Transport:            transport,
		Dir:                  p.Dir,
		Exec:                 m.Exec,
		Hash:                 p.Hash,
		Network:              nonNil(m.Permissions.Network),
		Read:                 nonNil(m.Permissions.Read),
		Write:                nonNil(m.Permissions.Write),
		Running:              a.plugins.Running(m.ID) != 0,
		GaveUp:               a.plugins.GaveUp(m.ID),
		InstallFolder:        folder,
		DefaultInstallFolder: def,
		ExtraFolders:         nonNil(extra),
		IconURL:              a.plugins.IconURL(m.ID),
		StorefrontURL:        storefrontURL(m),
		StorefrontDomains:    storefrontDomains(m),
	}
}

// storefrontURL is the store's official website the launcher embeds (issue
// #17), "" when the plugin declares none.
func storefrontURL(m plugins.Manifest) string {
	if m.Storefront == nil {
		return ""
	}
	return m.Storefront.URL
}

// storefrontDomains are the hostnames the storefront belongs to. Never nil
// on the wire (Go sends nil slices as null; the UI normalizes at the edge,
// but an empty list here reads cleaner).
func storefrontDomains(m plugins.Manifest) []string {
	if m.Storefront == nil || m.Storefront.Domains == nil {
		return []string{}
	}
	return m.Storefront.Domains
}

// ListStorePlugins returns the plugins found by the last scan.
func (a *App) ListStorePlugins() []types.StorePlugin {
	list := a.plugins.Plugins()
	out := make([]types.StorePlugin, len(list))
	for i, p := range list {
		out[i] = a.storePlugin(p)
	}
	return out
}

// RescanPlugins scans the plugins folder again. Plugins that may no longer
// run are stopped, and their running jobs fail.
func (a *App) RescanPlugins() ([]types.StorePlugin, error) {
	found, err := a.plugins.Discover()
	if err != nil {
		return nil, err
	}
	// The plugin set itself may have changed: no cached listing survives it.
	a.invalidateLibraries()
	a.invalidateAuthAll()
	a.syncDataPlugins(found)
	a.forgetSettingsOfRemoved(found)
	a.failStoppedJobs()
	a.syncSocial()
	return a.ListStorePlugins(), nil
}

func (a *App) failStoppedJobs() {
	for _, j := range a.jobs.List() {
		if j.State != "running" {
			continue
		}
		if p, ok := a.plugins.Plugin(j.Source); !ok || p.Status != plugins.StatusReady {
			a.jobs.FailSource(j.Source, "the plugin was stopped ("+statusOr(p, "removed")+")")
		}
	}
}

func statusOr(p plugins.Plugin, fallback string) string {
	if p.Status == "" {
		return fallback
	}
	return string(p.Status)
}

// ApprovePlugin pins the plugin's current executable and enables it. The UI
// shows the manifest before calling this.
func (a *App) ApprovePlugin(id string) error {
	if err := a.plugins.Approve(id); err != nil {
		return err
	}
	// An approved plugin joins the library: list it again.
	a.invalidateLibrary(id)
	// Approval may restart the plugin: its cached login state is not its own.
	a.invalidateAuth(id)
	a.failStoppedJobs()
	a.syncSocial()
	return nil
}

// SetPluginEnabled turns an approved plugin on or off. Off stops it.
func (a *App) SetPluginEnabled(id string, enabled bool) error {
	if err := a.plugins.SetEnabled(id, enabled); err != nil {
		return err
	}
	// A disabled plugin leaves the library (pruned on the next scan); an
	// enabled one joins it. Either way its entry can't be served again.
	a.invalidateLibrary(id)
	// Enabling restarts the plugin: its cached login state is not its own.
	a.invalidateAuth(id)
	a.failStoppedJobs()
	a.syncSocial()
	return nil
}

// RetryPlugin clears a gave-up plugin's crash count so the next call
// restarts it. The Plugins view offers it after a give-up (CORE-10);
// UI wiring belongs to workstream D.
func (a *App) RetryPlugin(id string) error {
	if err := a.plugins.RetryPlugin(id); err != nil {
		return err
	}
	// The plugin starts again on the next call: a cached failure from the
	// crashed period must not be served.
	a.invalidateLibrary(id)
	// A restart drops whatever the old process answered about the login.
	a.invalidateAuth(id)
	return nil
}

// OpenPluginsFolder opens the plugins folder in the file manager.
func (a *App) OpenPluginsFolder() error {
	dir := a.plugins.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("xdg-open", dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (a *App) authCall(id, method string, params, result any) error {
	p, ok := a.plugins.Plugin(id)
	if !ok {
		return fmt.Errorf("unknown plugin %q", id)
	}
	if !a.plugins.Has(id, "auth") {
		return fmt.Errorf("%s has no login", p.Manifest.Name)
	}
	ctx, cancel := context.WithTimeout(a.ctx, authTimeout)
	defer cancel()
	return a.plugins.Call(ctx, id, method, params, result)
}

// PluginAuthBegin starts a login. The URL, if any, is opened by the UI in the
// user's browser; only http(s) URLs are passed through.
func (a *App) PluginAuthBegin(id string) (types.AuthPrompt, error) {
	var pr types.AuthPrompt
	if err := a.authCall(id, "auth.begin", nil, &pr); err != nil {
		return pr, err
	}
	if pr.URL != "" {
		u, err := url.Parse(pr.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return types.AuthPrompt{}, fmt.Errorf("plugin returned a login URL Platypus won't open: %q", pr.URL)
		}
	}
	var fields []types.AuthField
	for _, f := range pr.Fields {
		if f.ID != "" {
			if f.Label == "" {
				f.Label = f.ID
			}
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		fields = []types.AuthField{{ID: "code", Label: "Code"}}
	}
	pr.Fields = fields
	return pr, nil
}

// PluginAuthComplete finishes the login with what the user typed.
func (a *App) PluginAuthComplete(id string, fields map[string]string) (types.AuthStatus, error) {
	var st types.AuthStatus
	err := a.authCall(id, "auth.complete", map[string]any{"fields": fields}, &st)
	if err == nil && st.LoggedIn {
		// A login changes what the store lists: list it again.
		a.invalidateLibrary(id)
		a.socialLoggedIn(id)
	}
	if err == nil {
		// The plugin just said what the login state is: cache it fresh.
		a.setAuthStatus(id, st)
	}
	return st, err
}

// PluginLogout logs out of the plugin's store.
func (a *App) PluginLogout(id string) (types.AuthStatus, error) {
	var st types.AuthStatus
	err := a.authCall(id, "auth.logout", nil, &st)
	if err == nil {
		// A logout changes what the store lists: list it again.
		a.invalidateLibrary(id)
		// The plugin just said what the login state is: cache it fresh.
		a.setAuthStatus(id, st)
	}
	return st, err
}

// SetPluginInstallFolder overrides where the plugin installs games; "" restores
// <Games folder>/<plugin>.
func (a *App) SetPluginInstallFolder(id, path string) error {
	if err := a.plugins.SetInstallFolder(id, path); err != nil {
		return err
	}
	// The folders feed the library's launch checks: list it again.
	a.invalidateLibrary(id)
	return nil
}

// ChoosePluginInstallFolder opens a folder picker at the plugin's install
// folder. Returns "" if the user cancels.
func (a *App) ChoosePluginInstallFolder(id string) (string, error) {
	folder, _ := a.plugins.InstallFolder(id, a.settings.GamesFolder())
	return a.chooseFolder("Install folder", folder)
}

// AllowGameFolder lets a plugin's games launch from a folder outside its
// install folder (games its CLI installed before Platypus).
func (a *App) AllowGameFolder(pluginID, path string) error {
	if err := a.plugins.AllowFolder(pluginID, path); err != nil {
		return err
	}
	// The folders feed the library's launch checks: list it again.
	a.invalidateLibrary(pluginID)
	return nil
}
