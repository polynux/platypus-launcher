package stores

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/sanitize"
	"platypus/internal/types"
)

// Plugin is a store backed by a store plugin process.
type Plugin struct {
	m *plugins.Manager
	p plugins.Plugin
	// folders returns where the plugin's games may be launched from: its
	// install folder plus folders the user allowed.
	folders func() []string
	// moved says where core moved a game's folder, when it did (see
	// WithRelocations). Nil means core moved nothing.
	moved func(gameID string) (types.Relocation, bool)
}

func NewPlugin(m *plugins.Manager, p plugins.Plugin, folders func() []string) *Plugin {
	return &Plugin{m: m, p: p, folders: folders}
}

// WithRelocations tells the store where core moved its games' folders. A
// plugin that doesn't know about the move still reports the old folder; the
// library shows the new one and launches run from it.
func (s *Plugin) WithRelocations(moved func(gameID string) (types.Relocation, bool)) *Plugin {
	s.moved = moved
	return s
}

func (s *Plugin) relocation(gameID string) (types.Relocation, bool) {
	if s.moved == nil {
		return types.Relocation{}, false
	}
	return s.moved(gameID)
}

func (s *Plugin) ID() string { return s.p.Manifest.ID }

// has reports whether the plugin offers the capability right now: what its
// last initialize said, or the manifest's list before it has initialized.
func (s *Plugin) has(capability string) bool { return s.m.Has(s.ID(), capability) }

// gameFeatures resolves a library entry's `features` against the runtime
// capabilities. A plugin that says nothing offers every capability for the
// game; one that lists some offers those it really has, once each, in the
// registry's vocabulary of the plugin's own manifest.
func (s *Plugin) gameFeatures(listed []string) []string {
	runtime := s.m.Runtime(s.ID()).Capabilities
	if listed == nil {
		return slices.Clone(runtime)
	}
	out := []string{}
	for _, f := range listed {
		if slices.Contains(runtime, f) && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

func (s *Plugin) Library(ctx context.Context) ([]types.Game, error) {
	var res plugins.LibraryList
	if err := s.m.Call(ctx, s.ID(), "library.list", nil, &res); err != nil {
		return nil, err
	}
	folders := s.folders()
	games := make([]types.Game, 0, len(res.Games))
	seen := map[string]bool{}
	for _, lg := range res.Games {
		if lg.ID == "" || seen[lg.ID] {
			continue
		}
		seen[lg.ID] = true
		games = append(games, s.toGame(lg, folders))
	}
	return games, nil
}

func (s *Plugin) toGame(lg plugins.LibraryGame, folders []string) types.Game {
	title := lg.Title
	if title == "" {
		title = lg.ID
	}
	g := types.Game{
		ID:              s.gameID(lg.ID),
		Source:          s.ID(),
		Store:           types.StoreType(s.p.Manifest.Store),
		StoreAppID:      lg.ID,
		Title:           title,
		CoverURL:        s.artURL(lg.CoverURL),
		HeroURL:         s.artURL(lg.HeroURL),
		BannerURL:       s.artURL(lg.BannerURL),
		IsInstalled:     lg.Installed,
		SizeBytes:       lg.SizeBytes,
		Version:         lg.Version,
		UpdateAvailable: lg.Installed && lg.UpdateAvailable,
		Installable:     lg.Installable && s.has("install"),
		Note:            lg.Note,
		Native:          lg.Native,
		ClientLaunch:    lg.ClientLaunch,
		Offline:         normalizeOffline(lg.Offline, s.speaksOffline()),
		CloudSync:       lg.CloudSync && s.speaksOffline(),
		LastPlayed:      max(lg.LastPlayed, 0),
		StoreStatus:     storeStatus(lg.Status, s.speaksStatus()),
		StoreNotes:      storeNotes(lg.UserNotes, s.speaksStatus()),
		Features:        s.gameFeatures(lg.Features),
		Settings:        types.GameSettings{UseGameMode: true},
	}
	// A borrowed game is only marked while the game offers `family`: the
	// plugin's own list (or the runtime set) decides, as for every capability.
	if lg.SharedBy != nil && lg.SharedBy.ID != "" && slices.Contains(g.Features, "family") {
		g.SharedBy = &types.SharedBy{ID: lg.SharedBy.ID, Name: lg.SharedBy.Name}
		// Copies only mean something for a borrowed game: an own game has none.
		g.Copies, g.PreferredCopy = normalizeCopies(lg.Copies, lg.PreferredCopy)
		g.CopyCompare = compareCopies(g.Copies, g.PreferredCopy)
	}
	if lg.Installed && lg.InstallPath != "" {
		g.InstallDir = filepath.Clean(lg.InstallPath)
		if rel, ok := s.relocation(g.ID); ok && g.InstallDir == rel.From {
			g.InstallDir = rel.To
			folders = append(slices.Clip(folders), rel.To)
		}
		real := g.InstallDir
		if r, err := filepath.EvalSymlinks(real); err == nil {
			real = r
		}
		// A client launch hands the game to its store's client, which runs it
		// from the store's own library: there is no path Platypus runs itself,
		// so the install-folder guard has nothing to contain (issue #86).
		if !lg.ClientLaunch {
			g.OutsideFolder = !insideAny(real, folders)
		}
	}
	return g
}

// Details asks the plugin for the game page's metadata. Plugins that answer
// game.details (stores with a metadata source) fill the same fields Steam
// games show; others leave the details empty.
func (s *Plugin) Details(ctx context.Context, game types.Game) (*types.GameDetails, error) {
	var d plugins.GameDetails
	params := map[string]any{"id": game.StoreAppID}
	if err := s.m.Call(ctx, s.ID(), "game.details", params, &d); err != nil {
		var rpcErr *jsonrpc.Error
		// An older plugin without game.details shows the library entry only.
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound {
			return &types.GameDetails{}, nil
		}
		return nil, err
	}
	out := &types.GameDetails{
		Summary:     d.Summary,
		Genres:      d.Genres,
		Developers:  d.Developers,
		Publishers:  d.Publishers,
		ReleaseDate: d.ReleaseDate,
		StoreURL:    storeURL(d.StoreURL),
	}
	// The compat fields are protocol 1.10: an older plugin's answer has none.
	if s.m.Runtime(s.ID()).Speaks(plugins.CompatVersion) {
		out.SteamAppID = d.SteamAppID
		out.Platforms = d.Platforms
		if d.DRM != nil {
			out.DRM = &types.DRM{
				DRMFree: d.DRM.DRMFree, OnlineRequired: d.DRM.OnlineRequired,
				Launcher: d.DRM.Launcher, DenuvoLikely: d.DRM.DenuvoLikely, Notes: d.DRM.Notes,
			}
		}
	}
	for _, u := range d.Screenshots {
		if u = s.artURL(u); u != "" {
			out.Screenshots = append(out.Screenshots, u)
		}
		if len(out.Screenshots) >= maxScreenshots {
			break
		}
	}
	if d.HeroURL = s.artURL(d.HeroURL); d.HeroURL != "" {
		out.HeroURL = d.HeroURL
	}
	// The media and richer facts are protocol 1.12: an older plugin's answer
	// carries none. HTML is sanitized in core so the UI only renders
	// known-safe markup; URLs are https (or the local art server); lists and
	// lengths are bounded.
	if s.m.Runtime(s.ID()).Speaks(plugins.MediaVersion) {
		// Cut the input, not the output: cutting sanitized markup could end
		// inside a tag. The sanitizer closes whatever the cut left open.
		desc := d.DescriptionHTML
		if r := []rune(desc); len(r) > maxDescriptionRunes {
			desc = string(r[:maxDescriptionRunes])
		}
		out.DescriptionHTML = sanitize.HTML(desc)
		out.Website = sanitize.HTTPSURL(d.Website)
		out.Languages = sanitize.Strings(d.Languages, maxListItems, maxCopyText)
		switch d.Controller {
		case "full", "partial":
			out.Controller = d.Controller
		}
		if d.Requirements != nil {
			min := sanitize.Clip(sanitize.HTML(d.Requirements.Minimum), maxRequirementsRunes)
			rec := sanitize.Clip(sanitize.HTML(d.Requirements.Recommended), maxRequirementsRunes)
			if min != "" || rec != "" {
				out.Requirements = &types.GameRequirements{Minimum: min, Recommended: rec}
			}
		}
		out.AgeRating = sanitize.Clip(d.AgeRating, maxCopyText)
		out.Tags = sanitize.Strings(d.Tags, maxListItems, maxCopyText)
		for _, v := range d.Videos {
			if len(out.Videos) >= maxVideos {
				break
			}
			u := sanitize.MediaURL(v.URL)
			if u == "" {
				continue
			}
			mime := videoMIME(v.MIME, u)
			out.Videos = append(out.Videos, types.GameVideo{
				URL: u, Thumbnail: sanitize.MediaURL(v.Thumbnail),
				Title: sanitize.Clip(v.Title, maxCopyText), MIME: mime,
			})
		}
	}
	if d.Achievements != nil && d.Achievements.Total > 0 {
		list := make([]types.Achievement, 0, len(d.Achievements.List))
		for _, a := range d.Achievements.List {
			list = append(list, types.Achievement{
				ID: a.ID, Name: a.Name, Description: a.Description,
				Icon: s.artURL(a.Icon), GlobalPercent: a.GlobalPercent,
			})
		}
		// Progress is store-level data: unlocked stays false and UnlockedAt 0,
		// so the game page says "highlighted by the store".
		out.Achievements = &types.Achievements{Total: d.Achievements.Total, Unlocked: -1, List: list}
	}
	return out, nil
}

// familyRoles are the member roles the Families section names; any other
// role the plugin sends is shown as none.
var familyRoles = []string{"adult", "child"}

// FamilyMembers asks the plugin for the family group (protocol 1.8). Members
// without an id are dropped; roles and avatars are normalized like the rest
// of the plugin's data. Platypus shows them and enforces nothing.
func (s *Plugin) FamilyMembers(ctx context.Context) ([]types.FamilyMember, error) {
	var wire []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		AvatarURL string `json:"avatarUrl"`
		State     string `json:"state"`
	}
	if err := s.m.Call(ctx, s.ID(), "family.members", map[string]any{}, &wire); err != nil {
		return nil, err
	}
	out := []types.FamilyMember{}
	seen := map[string]bool{}
	for _, w := range wire {
		if w.ID == "" || seen[w.ID] {
			continue
		}
		seen[w.ID] = true
		m := types.FamilyMember{ID: w.ID, Name: w.Name, AvatarURL: webURL(w.AvatarURL), State: w.State}
		if m.Name == "" {
			m.Name = w.ID
		}
		if slices.Contains(familyRoles, w.Role) {
			m.Role = w.Role
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Plugin) Launch(ctx context.Context, game types.Game, preview bool) (*Launch, error) {
	var spec plugins.LaunchSpec
	params := map[string]any{"id": game.StoreAppID, "preview": preview}
	// The user's per-game executable override (protocol 1.15, issue #44):
	// the plugin answers with a direct spec for that file, instead of the
	// core bypassing the plugin.
	if game.Executable != "" && s.speaksOffline() {
		params["executable"] = game.Executable
	}
	if err := s.m.Call(ctx, s.ID(), "game.launchSpec", params, &spec); err != nil {
		return nil, err
	}
	folders := s.folders()
	if override, ok := params["executable"].(string); ok && sameFile(spec.Exe, override) {
		// The user picked this executable for the game: its folder is theirs
		// to launch from, even outside the plugin's install folders.
		folders = append(slices.Clip(folders), filepath.Dir(override))
	}
	if rel, ok := s.relocation(game.ID); ok {
		// The plugin may still name the folder core moved the game from.
		spec = rebaseSpec(spec, rel.From, rel.To)
		folders = append(slices.Clip(folders), rel.To)
	}
	return ValidateSpec(spec, s.p.Manifest, folders)
}

// sameFile says whether a and b name the same file, after resolving symlinks.
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	rb, err := filepath.EvalSymlinks(b)
	return err == nil && ra == rb
}

// rebaseSpec points the paths of a launch spec that are inside from at to.
// Arguments are left alone: they are the game's own.
func rebaseSpec(spec plugins.LaunchSpec, from, to string) plugins.LaunchSpec {
	spec.Exe = rebasePath(spec.Exe, from, to)
	spec.Cwd = rebasePath(spec.Cwd, from, to)
	if spec.Command != nil {
		command := slices.Clone(spec.Command)
		for i, c := range command {
			command[i] = rebasePath(c, from, to)
		}
		spec.Command = command
	}
	return spec
}

// rebasePath returns p with the from folder replaced by to, when p is inside
// from; any other path comes back as it is.
func rebasePath(p, from, to string) string {
	if p == from {
		return to
	}
	if rest, ok := strings.CutPrefix(p, from+string(filepath.Separator)); ok {
		return filepath.Join(to, rest)
	}
	return p
}

var unsafeIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// GameID is Platypus's id for a plugin game. It's used in file names (wine
// prefix, launch log), so the plugin's id is reduced to safe characters, with
// a hash suffix when that changed it so ids stay unique.
func GameID(source, id string) string {
	return GameIDWithPrefix("", source, id)
}

// GameIDWithPrefix is GameID with a manifest gameIdPrefix (protocol 1.15,
// issue #54 groundwork): empty means the default "p.<source>.". A custom
// prefix (e.g. "steam_") replaces it, so a bundled plugin's ids stay
// byte-identical across the move out of the core.
func GameIDWithPrefix(prefix, source, id string) string {
	safe := unsafeIDChars.ReplaceAllString(id, "_")
	base := "p." + source + "."
	if prefix != "" {
		base = prefix
	}
	out := base + safe
	if safe != id || safe == "" {
		sum := sha256.Sum256([]byte(id))
		out += "-" + hex.EncodeToString(sum[:4])
	}
	return out
}

// gameID is Platypus's id for one of this plugin's games, honouring its
// manifest gameIdPrefix when the plugin speaks protocol 1.15.
func (s *Plugin) gameID(storeID string) string {
	if s.speaksOffline() && s.p.Manifest.GameIDPrefix != "" {
		return GameIDWithPrefix(s.p.Manifest.GameIDPrefix, s.ID(), storeID)
	}
	return GameID(s.ID(), storeID)
}

// WireID is the expected Platypus id for a store id, honouring the plugin's
// manifest gameIdPrefix (protocol 1.15). The conformance suite uses it: a
// bundled plugin's ids stay byte-identical across the move out of the core.
func (s *Plugin) WireID(storeID string) string { return s.gameID(storeID) }

// speaksOffline reports whether the plugin negotiated protocol 1.15 or later,
// so the offline fields (offline, cloudSync, local-art, gameIdPrefix, the
// executable param) are read. Older plugins keep today's behaviour.
func (s *Plugin) speaksOffline() bool {
	return s.m.Runtime(s.ID()).Speaks(plugins.OfflineVersion)
}

// normalizeOffline validates a plugin's offline value (protocol 1.15):
// "ok", "network" or "unknown", absent meaning "unknown". Anything else is
// "unknown": the conformance suite rejects it, but the library never breaks
// on it. Older plugins (pre-1.15) always read "unknown".
func normalizeOffline(raw string, offline bool) string {
	if !offline {
		return "unknown"
	}
	switch raw {
	case "ok", "network":
		return raw
	case "", "unknown":
		return "unknown"
	default:
		return "unknown"
	}
}

// speaksStatus reports whether the plugin negotiated protocol 1.16 or later,
// so the status and notes hints (LibraryGame.status/userNotes) are read.
// Older plugins keep no hint: only the user's own values show.
func (s *Plugin) speaksStatus() bool {
	return s.m.Runtime(s.ID()).Speaks(plugins.StatusVersion)
}

// storeStatus normalizes a plugin's status hint (protocol 1.16, issue #51):
// one of the game statuses, "" meaning no suggestion. Anything else is "":
// the conformance suite rejects it, but the library never breaks on it.
// Plugins older than 1.16 always read "".
func storeStatus(raw string, speaks bool) string {
	if !speaks || !types.ValidGameStatus(raw) {
		return ""
	}
	return raw
}

// storeNotes normalizes a plugin's notes hint: one within the cap, else no
// suggestion — dropped like an art URL the core refuses: the library never
// breaks on what a plugin sends. Plugins older than 1.16 always read "".
func storeNotes(raw string, speaks bool) string {
	if !speaks || raw == "" || len(raw) > types.MaxGameNotes {
		return ""
	}
	return raw
}

// Bounds for the media and richer facts (protocol 1.12) of game.details.
const (
	maxScreenshots       = 30
	maxVideos            = 20
	maxListItems         = 100
	maxDescriptionRunes  = 200_000
	maxRequirementsRunes = 50_000
)

// videoMIME keeps known video types, guesses from the URL suffix, else ""
// (unknown: the player sniffs it). HLS (.m3u8) is dropped by the plugins,
// never served here.
func videoMIME(raw, u string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "video/mp4", "video/webm":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	lower := strings.ToLower(u)
	if i := strings.Index(lower, "?"); i >= 0 {
		lower = lower[:i]
	}
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	}
	return ""
}

// webURL keeps http(s) art URLs and drops anything else (file:, javascript:).
func webURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}

// GameArtURLPrefix is where the core serves a plugin's local art files (the
// local-art capability, protocol 1.15, issue #44): the core rewrites each
// file:// art URL to GameArtURLPrefix + <plugin-id> + "/" +
// base64url(absolute path), and serves it with http.ServeFile.
const GameArtURLPrefix = "/platypus-art/"

// localArtExts are the art file extensions the local-art capability serves.
var localArtExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// artURL keeps an art URL: http(s) passes, file:// is rewritten to the local
// art server when the plugin offers local-art (protocol 1.15) and the file
// qualifies, else dropped (as today without the capability).
func (s *Plugin) artURL(raw string) string {
	if u := webURL(raw); u != "" {
		return u
	}
	return s.localArtURL(raw)
}

// localArtURL rewrites a file:// art URL to the local art server, or "" when
// it doesn't qualify. It refuses non-absolute paths, extensions other than
// jpg/jpeg/png/webp, and paths outside the plugin's data folder and declared
// permissions.read folders (checked after EvalSymlinks). Without the local-art capability,
// file:// is dropped, as today.
func (s *Plugin) localArtURL(raw string) string {
	if !s.has("local-art") || !s.speaksOffline() {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" {
		return ""
	}
	path := u.Path
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	if !localArtExts[strings.ToLower(filepath.Ext(path))] {
		return ""
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	}
	if !insideAny(real, s.readFolders()) {
		return ""
	}
	return GameArtURLPrefix + s.ID() + "/" + base64.RawURLEncoding.EncodeToString([]byte(path))
}

// readFolders is where local art may live: the plugin's own data folder
// (PLATYPUS_PLUGIN_DATA) and its permissions.read folders (~ and ~/… expanded
// to the user's home), for the local-art containment check.
func (s *Plugin) readFolders() []string {
	home, _ := os.UserHomeDir()
	var out []string
	if d := s.m.DataDirOf(s.ID()); d != "" {
		out = append(out, d)
	}
	for _, p := range s.p.Manifest.Permissions.Read {
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") && home != "" {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// DecodeGameArt decodes a local art server path (/platypus-art/<plugin-id>/<b64>)
// back to the absolute file path, or "" when it isn't one.
func DecodeGameArt(pluginID, encoded string) string {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 || !filepath.IsAbs(string(raw)) {
		return ""
	}
	return string(raw)
}

// isLoopback reports whether host is a loopback address for store-URL
// purposes: 127.0.0.0/8, ::1, or localhost (the test plugin's local page).
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// storeURL keeps a store page link (issue #17): https, or an http loopback
// page for the test plugin. Anything else is dropped.
func storeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme == "https" {
		return u.String()
	}
	if u.Scheme == "http" && isLoopback(u.Hostname()) {
		return u.String()
	}
	return ""
}
