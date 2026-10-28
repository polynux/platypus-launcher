package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"platypus/internal/apperr"
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
		Bundled:              plugins.BundledPluginIDs[m.ID],
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
	// Approving a data plugin applies its fixes, tweaks and theme.
	a.syncDataPlugins(a.plugins.Plugins())
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

// UninstallPlugin removes a plugin cleanly (issue #161): it stops the plugin
// process first, deletes its folder, and forgets what the core kept for it
// (approval, install folder, plugin settings, chat/social/cloud switches,
// login cache, library cache, its private data folder). Game files are never
// touched: installed games stay on disk and in the library, marked "store
// missing" (launchable where possible). removeGames also removes the library
// entries (the snapshot) and forgets the plugin's per-game settings values;
// otherwise those stay so a reinstall restores them. Bundled plugins can't be
// uninstalled, only disabled.
func (a *App) UninstallPlugin(id string, removeGames bool) error {
	if plugins.BundledPluginIDs[id] {
		return fmt.Errorf("the bundled %s plugin can't be uninstalled, only disabled", id)
	}
	if _, ok := a.plugins.Plugin(id); !ok {
		return fmt.Errorf("unknown plugin %q", id)
	}
	// A theme plugin may carry the picked theme: remember it, so the theme
	// falls back after the folder is gone. Resetting before a failed removal
	// would lose the user's theme while the plugin stays.
	themeFallback := false
	if picked := a.settings.Theme(); picked != "" {
		if t, ok := a.findTheme(picked); ok && t.Info.Source == id {
			themeFallback = true
		}
	}
	if err := a.plugins.Uninstall(id); err != nil {
		return err
	}
	if themeFallback {
		if err := a.settings.SetTheme(""); err != nil {
			return err
		}
	}
	if err := a.settings.ForgetPlugin(id, removeGames); err != nil {
		return err
	}
	a.removePluginData(id)
	if a.chat != nil {
		a.chat.Forget(id)
	}
	a.invalidateAuth(id)
	a.invalidateLibrary(id)
	a.dropArtCache(id)
	if removeGames {
		// The library entries go with the store; otherwise the snapshot
		// stays and its installed rows show as "store missing".
		a.dropSnapshot(id)
	}
	found, err := a.plugins.Discover()
	if err != nil {
		return err
	}
	a.invalidateLibraries()
	a.invalidateAuthAll()
	a.syncDataPlugins(found)
	// Like forgetSettingsOfRemoved, but the just-uninstalled plugin's
	// per-game settings stay unless the user asked to remove them: a
	// reinstall restores them, while approval and login still start clean.
	present := map[string]bool{}
	for _, p := range found {
		present[p.Manifest.ID] = true
	}
	keep := func(pid string) bool {
		if pid == id && !removeGames {
			return true
		}
		return present[pid]
	}
	if err := a.settings.PrunePlugins(keep); err != nil {
		a.warnf("plugin settings of removed plugins not forgotten: %v", err)
	}
	a.failStoppedJobs()
	a.syncSocial()
	return nil
}

// removePluginData deletes the plugin's private data folder and its log file.
// Failures are ignored: the data is the plugin's own (login tokens live
// there; no OS keyring is used), and a leftover file must not fail the
// uninstall.
func (a *App) removePluginData(id string) {
	if a.dataDir == "" || id == "" || id == "." || id == ".." {
		return
	}
	if strings.ContainsAny(id, `/\`) {
		return
	}
	data := filepath.Join(a.dataDir, "plugin-data", id)
	if rel, err := filepath.Rel(a.dataDir, data); err != nil || !filepath.IsLocal(rel) {
		return
	} else {
		_ = os.RemoveAll(data)
	}
	_ = os.Remove(filepath.Join(a.dataDir, "logs", "plugin-"+id+".log"))
}

// dropArtCache forgets the remembered online-art hits of one source.
func (a *App) dropArtCache(source string) {
	a.artCacheMu.Lock()
	defer a.artCacheMu.Unlock()
	for k := range a.artCache {
		if k == source || strings.HasPrefix(k, source+":") {
			delete(a.artCache, k)
		}
	}
}

// dropSnapshot deletes one source's library snapshot: the uninstall option
// that removes the library entries.
func (a *App) dropSnapshot(source string) {
	path := snapshotPath(a.library.snapDir, source)
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// ChoosePluginZip opens a file picker for a plugin zip. Returns "" when the
// user cancels.
func (a *App) ChoosePluginZip() (string, error) {
	return a.host.OpenFile(FileDialog{
		Title:   "Install a plugin",
		Filters: []FileFilter{{Name: "Plugin zip", Extensions: []string{"zip"}}},
	})
}

// InstallPluginZip installs a store or theme plugin from a zip file (issue
// #147). Without confirmReplace, an id that is already installed answers
// NeedsConfirm ("would replace vX with vY") and writes nothing; with it the
// whole folder is replaced atomically, never merged. Bundled plugins can't be
// replaced. The new plugin goes through the usual approval sheet: a zip
// install is never auto-approved.
func (a *App) InstallPluginZip(zipPath string, confirmReplace bool) (types.PluginInstallResult, error) {
	if zipPath == "" {
		return types.PluginInstallResult{}, fmt.Errorf("no zip file chosen")
	}
	res, err := a.plugins.InstallFromZip(zipPath, confirmReplace)
	if err != nil {
		return types.PluginInstallResult{}, err
	}
	if res.NeedsConfirm {
		return types.PluginInstallResult{
			ID: res.ID, Version: res.Version, NeedsConfirm: true,
			CurrentVersion: res.CurrentVersion, NewVersion: res.NewVersion,
		}, nil
	}
	a.invalidateLibraries()
	a.invalidateAuthAll()
	a.syncDataPlugins(a.plugins.Plugins())
	a.forgetSettingsOfRemoved(a.plugins.Plugins())
	a.failStoppedJobs()
	a.syncSocial()
	return types.PluginInstallResult{ID: res.ID, Version: res.Version, Replaced: res.Replaced}, nil
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
		return apperr.New("plugin.unknown", fmt.Sprintf("unknown plugin %q", id), map[string]string{"id": id})
	}
	if !a.plugins.Has(id, "auth") {
		return apperr.New("plugin.noLogin", fmt.Sprintf("%s has no login", p.Manifest.Name), map[string]string{"name": p.Manifest.Name})
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
		// ... and what it has unread: seed the badges again.
		go a.refreshChatUnread(id)
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
		// The account's chats end with it: its previews must not survive.
		a.forgetChatPlugin(id)
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
