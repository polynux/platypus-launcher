package savebackup

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode"

	"platypus/internal/steam"
	"platypus/internal/types"
)

// Context describes where a game's saves may live: the resolved roots every
// expanded path must stay inside, plus the Wine identity for placeholders.
type Context struct {
	// Roots maps root labels to absolute paths: prefix, install, home,
	// xdgData, xdgConfig, steam. Expansion records which root a file came
	// from so restores can rebase later (phase 2 stores the labels).
	Roots map[string]string
	// Windows says the game runs through Wine/Proton; Native games use the
	// linux side of when: filters.
	Windows bool
	// Proton says the prefix has the pfx/ layout; WinUser is the Windows
	// user name inside the prefix.
	Proton  bool
	WinUser string
	// SteamID / GogID are the matched store ids for <storeGameId>.
	SteamID string
	GogID   string
	// StoreUserID resolves <storeUserId> for Steam games: the single
	// userdata/<id> folder when there is exactly one, else "" (unresolved).
	StoreUserID string
	// OSUser is the login name for <osUserName>.
	OSUser string
}

// isSteamGame reports built-in Steam (local client) games: they have no
// plugin and run through the Steam client, with their Proton prefix at
// <steam library>/steamapps/compatdata/<appid>/pfx.
func isSteamGame(game types.Game) bool {
	return (game.Source == types.SourceSteam || game.Store == types.StoreSteam) && game.StoreAppID != ""
}

// Prefix is a game's Wine or Proton prefix as the runner plugin reported it
// for the launch: Dir is the prefix folder the game runs in, User the login
// inside it (Proton and umu name steamuser). Dir is "" for a native game, a
// game with no runner, or one launched through its store client.
type Prefix struct {
	Dir  string
	User string
}

// NewContextFor builds the resolution context for a game. The runner plugin
// decides the prefix (see Prefix); this package only maps it to save roots.
// steamRoot is the Steam client folder ("" = ~/.local/share/Steam).
//
// Built-in Steam games ignore prefix: their prefix is the Steam compatdata
// folder when it exists (Proton + steamuser), otherwise they are native (no
// prefix). Plugin games use the prefix the runner reported for the launch.
func NewContextFor(game types.Game, home, steamRoot string, prefix Prefix) Context {
	if steamRoot == "" {
		steamRoot = filepath.Join(home, ".local", "share", "Steam")
	}
	if isSteamGame(game) {
		roots := map[string]string{
			"home":      home,
			"xdgData":   xdgDirFor("XDG_DATA_HOME", home, ".local/share"),
			"xdgConfig": xdgDirFor("XDG_CONFIG_HOME", home, ".config"),
			"steam":     steamRoot,
		}
		if game.InstallDir != "" {
			roots["install"] = game.InstallDir
		}
		ctx := Context{
			Roots: roots, OSUser: currentLogin(),
			StoreUserID: steam.SingleUserID(steamRoot),
		}
		if prefix, ok := steam.CompatPrefix(steamRoot, game.StoreAppID); ok {
			ctx.Windows, ctx.Proton, ctx.WinUser = true, true, "steamuser"
			ctx.Roots["prefix"] = prefix
		}
		return ctx
	}
	windows := prefix.Dir != "" && !game.Native
	// A prefix with a user is a Proton or umu prefix: the runner names steamuser.
	proton := windows && prefix.User != ""
	login := currentLogin()
	if proton {
		login = prefix.User
	}
	prefixDir := ""
	if windows {
		prefixDir = prefix.Dir
	}
	roots := map[string]string{
		"home":      home,
		"xdgData":   xdgDirFor("XDG_DATA_HOME", home, ".local/share"),
		"xdgConfig": xdgDirFor("XDG_CONFIG_HOME", home, ".config"),
		"steam":     steamRoot,
	}
	if game.InstallDir != "" {
		roots["install"] = game.InstallDir
	}
	if prefixDir != "" {
		roots["prefix"] = prefixDir
	}
	return Context{
		Roots: roots, Windows: windows, Proton: proton, WinUser: login,
		OSUser: currentLogin(),
	}
}

func currentLogin() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if h := os.Getenv("USER"); h != "" {
		return h
	}
	return "user"
}

func xdgDirFor(env, home, fallback string) string {
	if d := os.Getenv(env); filepath.IsAbs(d) {
		return d
	}
	return filepath.Join(home, fallback)
}

// normalizeTitle folds a title for matching: lowercase, punctuation and
// spaces collapsed.
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevSpace := true // collapse leading separators too
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// Match finds the manifest entry for a game: by Steam id, then GOG id, then
// the normalised title (aliases resolved). Exact matches only; an ambiguous
// title (two entries normalising alike) means no match. steamID/gogID are the
// game's own ids (compat AppID or info_source gog:<id> for sideloaded games).
func Match(idx *Index, title, steamID, gogID string) *GameEntry {
	if idx == nil {
		return nil
	}
	if steamID != "" {
		var found *GameEntry
		for i := range idx.Games {
			if idx.Games[i].SteamID != "" && idx.Games[i].SteamID == steamID {
				if found != nil {
					return nil // ambiguous
				}
				found = &idx.Games[i]
			}
		}
		if found != nil {
			return found
		}
	}
	if gogID != "" {
		var found *GameEntry
		for i := range idx.Games {
			if idx.Games[i].GogID != "" && idx.Games[i].GogID == gogID {
				if found != nil {
					return nil
				}
				found = &idx.Games[i]
			}
		}
		if found != nil {
			return found
		}
	}
	want := normalizeTitle(title)
	if want == "" {
		return nil
	}
	var found *GameEntry
	for i := range idx.Games {
		g := &idx.Games[i]
		if normalizeTitle(g.Title) == want {
			if found != nil {
				return nil
			}
			found = g
			continue
		}
		for _, a := range g.Aliases {
			if normalizeTitle(a) == want {
				if found != nil {
					return nil
				}
				found = g
				break
			}
		}
	}
	return found
}

// whenOK applies the when: os filter (windows under Wine, linux for native
// games) and keeps store constraints: an entry naming a store only applies
// when the game has that store id.
func whenOK(fe FileEntry, ctx Context, steamID, gogID string) bool {
	if len(fe.WhenOS) > 0 {
		want := "linux"
		if ctx.Windows {
			want = "windows"
		}
		hit := false
		for _, o := range fe.WhenOS {
			lo := strings.ToLower(o)
			if lo == want || (want == "windows" && (lo == "win" || lo == "wine" || lo == "proton")) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if len(fe.WhenStore) > 0 {
		hit := false
		for _, s := range fe.WhenStore {
			ls := strings.ToLower(strings.TrimSpace(s))
			switch {
			case strings.HasPrefix(ls, "steam"):
				// "steam", "steam:123" or a bare id under a steam key.
				id := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(ls, "steam"), ":"))
				if steamID != "" && (id == "" || id == strings.ToLower(steamID)) {
					hit = true
				}
			case strings.HasPrefix(ls, "gog"):
				id := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(ls, "gog"), ":"))
				if gogID != "" && (id == "" || id == strings.ToLower(gogID)) {
					hit = true
				}
			default:
				if ls == strings.ToLower(steamID) || ls == strings.ToLower(gogID) {
					hit = true
				}
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// expandPattern expands one manifest pattern into absolute path patterns.
// It returns the patterns and the count of unresolvable placeholders (which
// skip the path, never an error).
func expandPattern(pattern string, ctx Context, game types.Game) ([]string, int) {
	out := []string{pattern}
	unresolved := 0
	expand := func(token, to string, ok bool) {
		if !strings.Contains(out[0], token) && !containsToken(out, token) {
			return
		}
		var next []string
		for _, p := range out {
			if !strings.Contains(p, token) {
				next = append(next, p)
				continue
			}
			if !ok {
				unresolved++
				continue // drop this branch: skip the path, count it
			}
			next = append(next, strings.ReplaceAll(p, token, to))
		}
		out = next
	}
	driveC := ""
	driveCUser := ""
	if p, ok := ctx.Roots["prefix"]; ok {
		driveC = filepath.Join(p, "drive_c")
		driveCUser = winUserDir(driveC, ctx.WinUser)
	}
	winUser := ctx.WinUser
	if winUser == "" {
		winUser = ctx.OSUser
	}
	get := func(label string) (string, bool) {
		v, ok := ctx.Roots[label]
		return v, ok && v != ""
	}
	home, _ := get("home")
	xdgData, _ := get("xdgData")
	xdgConfig, _ := get("xdgConfig")
	// Ludusavi's install placeholders: <base> is the game's install folder,
	// <root> the folder games are installed in and <game> the install
	// folder's name, so <root>/<game> is <base> again. Paths that leave the
	// install folder are dropped later by the containment check.
	install, hasInstall := get("install")
	gameDir, rootDir := "", ""
	if hasInstall {
		gameDir, rootDir = filepath.Base(install), filepath.Dir(install)
	}
	expand("<winAppData>", filepath.Join(driveCUser, "AppData", "Roaming"), driveCUser != "")
	expand("<winLocalAppData>", filepath.Join(driveCUser, "AppData", "Local"), driveCUser != "")
	expand("<winLocalAppDataLow>", filepath.Join(driveCUser, "AppData", "LocalLow"), driveCUser != "")
	expand("<winDocuments>", filepath.Join(driveCUser, "Documents"), driveCUser != "")
	expand("<winPublic>", filepath.Join(driveC, "users", "Public"), driveC != "")
	expand("<winProgramData>", filepath.Join(driveC, "ProgramData"), driveC != "")
	expand("<winDir>", filepath.Join(driveC, "windows"), driveC != "")
	expand("<home>", home, home != "")
	expand("<xdgData>", xdgData, xdgData != "")
	expand("<xdgConfig>", xdgConfig, xdgConfig != "")
	expand("<base>", install, hasInstall)
	expand("<root>", rootDir, hasInstall)
	expand("<game>", gameDir, hasInstall)
	expand("<osUserName>", firstNonEmpty(winUser, ctx.OSUser), firstNonEmpty(winUser, ctx.OSUser) != "")
	storeGameID := firstNonEmpty(ctx.SteamID, ctx.GogID)
	expand("<storeGameId>", storeGameID, storeGameID != "")
	// <storeUserId> resolves for Steam games to the single userdata/<id>
	// folder when there is exactly one; otherwise it stays unresolved.
	expand("<storeUserId>", ctx.StoreUserID, ctx.StoreUserID != "")
	// Normalise separators: the manifest uses Windows backslashes.
	for i, p := range out {
		out[i] = filepath.Clean(strings.ReplaceAll(p, "\\", string(filepath.Separator)))
	}
	// Drop empties left by skipped branches.
	kept := out[:0]
	for _, p := range out {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return kept, unresolved
}

// winUserDir picks the Windows user folder inside a prefix's drive_c:
// users/<winUser> when it exists (the prefix's own login→steamuser link
// resolves on its own), else the steamuser folder Proton/umu prefixes
// provide, else the wanted path so the pattern still expands and simply
// matches nothing (issue #60).
func winUserDir(driveC, winUser string) string {
	if driveC == "" || winUser == "" {
		return ""
	}
	want := filepath.Join(driveC, "users", winUser)
	if _, err := os.Lstat(want); err == nil {
		return want
	}
	if winUser != "steamuser" {
		if alt := filepath.Join(driveC, "users", "steamuser"); alt != want {
			if _, err := os.Lstat(alt); err == nil {
				return alt
			}
		}
	}
	return want
}

// globBase is a glob pattern's static parent for display: everything before
// the first metacharacter, cut back to its directory. "" when the pattern
// starts with a metacharacter (issue #60).
func globBase(pattern string) string {
	i := strings.IndexAny(pattern, "*?[")
	if i < 0 {
		return pattern
	}
	dir := pattern[:i]
	j := strings.LastIndex(dir, string(filepath.Separator))
	if j <= 0 {
		return ""
	}
	return filepath.Clean(dir[:j])
}

func containsToken(patterns []string, token string) bool {
	for _, p := range patterns {
		if strings.Contains(p, token) {
			return true
		}
	}
	return false
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// ResolvePaths resolves the game's save locations: user paths first, then the
// matched manifest entry. It returns the existing-or-not roots (for display),
// the file list will be enumerated at backup time. unresolved counts manifest
// paths skipped on unresolvable placeholders.
func ResolvePaths(game types.Game, userPaths []string, entry *GameEntry, ctx Context, steamID, gogID string) (roots []types.SavePath, patterns []resolvedPattern, unresolved int) {
	seen := map[string]bool{}
	addRoot := func(path, origin string) {
		if seen[path] {
			return
		}
		seen[path] = true
		roots = append(roots, types.SavePath{Path: path, Origin: origin, Exists: exists(path)})
	}
	for _, u := range userPaths {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		addRoot(filepath.Clean(u), "user")
		patterns = append(patterns, resolvedPattern{Pattern: filepath.Clean(u), Root: rootLabelFor(filepath.Clean(u), ctx), Origin: "user"})
	}
	if entry != nil {
		for _, fe := range entry.Files {
			if !whenOK(fe, ctx, steamID, gogID) {
				continue
			}
			expanded, un := expandPattern(fe.Pattern, ctx, game)
			unresolved += un
			for _, p := range expanded {
				patterns = append(patterns, resolvedPattern{Pattern: p, Root: rootLabelFor(p, ctx), Origin: "manifest"})
				// Record the literal parent for display when glob-free; for
				// glob patterns list the static parent so the Maintenance
				// tab shows the folder as found automatically (issue #60).
				if !strings.ContainsAny(p, "*?[") {
					addRoot(p, "manifest")
				} else if base := globBase(p); base != "" {
					addRoot(base, "manifest")
				}
			}
		}
	}
	return roots, patterns, unresolved
}

type resolvedPattern struct {
	Pattern string
	Root    string
	Origin  string
}

func rootLabelFor(path string, ctx Context) string {
	best, bestLen := "", -1
	for _, label := range sortedKeys(ctx.Roots) {
		root := ctx.Roots[label]
		if root == "" {
			continue
		}
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			if len(root) > bestLen {
				best, bestLen = label, len(root)
			}
		}
	}
	return best
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
