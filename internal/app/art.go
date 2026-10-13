package app

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"platypus/internal/plugins"
	"platypus/internal/stores"
)

var pluginIconID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// CustomArt serves the user's images to the shell under customize.URLPrefix,
// falling through to next for anything else.
func CustomArt(a *App, next http.Handler) http.Handler { return a.customArt(next) }

// PluginArt serves store plugin logos to the shell under
// plugins.IconURLPrefix, falling through to next for anything else. Chain it
// before CustomArt: a "/platypus-art/plugin/..." path would otherwise reach
// the customize store first, which answers 404 for names it doesn't know and
// never falls through. Logos are served from the plugin folder, never
// fetched from the network. SVG runs no script when drawn through <img>, and
// the response carries a restrictive content type plus a scriptless CSP so a
// logo opened directly can't run anything either.
func PluginArt(a *App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, plugins.IconURLPrefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		// One path segment (the plugin id): anything deeper is refused, so no
		// request path can name a file.
		if !pluginIconID.MatchString(rest) || strings.Contains(rest, "/") {
			http.NotFound(w, r)
			return
		}
		path, ctype, found := "", "", false
		if a.plugins != nil {
			path, ctype, found = a.plugins.IconFile(rest)
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		// Scripts in SVG don't run in <img>; this locks the direct view too.
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// The URL carries the plugin hash, so a replaced logo is a new address.
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		http.ServeFile(w, r, path)
	})
}

// pluginGameArtAllowed reports whether the local art file may be served: the
// plugin is known and the path (after EvalSymlinks) sits inside its own
// data folder (PLATYPUS_PLUGIN_DATA) or one of its declared permissions.read
// folders. Non-absolute paths never pass (DecodeGameArt already refused them).
func (a *App) pluginGameArtAllowed(id, path string) bool {
	p, ok := a.plugins.Plugin(id)
	if !ok {
		return false
	}
	real := path
	if r, err := filepath.EvalSymlinks(path); err == nil {
		real = r
	}
	home := a.homeDir()
	folders := append([]string{a.plugins.DataDirOf(id)}, p.Manifest.Permissions.Read...)
	for _, folder := range folders {
		f := folder
		if f == "~" {
			f = home
		} else if strings.HasPrefix(f, "~/") && home != "" {
			f = filepath.Join(home, strings.TrimPrefix(f, "~/"))
		}
		if f == "" {
			continue
		}
		if realFolder, err := filepath.EvalSymlinks(f); err == nil {
			f = realFolder
		}
		if rel, err := filepath.Rel(f, real); err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return true
		}
	}
	return false
}

// PluginGameArt serves a store plugin's local art files under
// /platypus-art/<plugin-id>/<base64url(abs path)> (the local-art capability,
// protocol 1.15, issue #44). The core rewrites each file:// art URL to one of
// these addresses (see stores.Plugin.artURL); this handler serves the file
// with http.ServeFile. Chain it before CustomArt: the customize store answers
// 404 for names it doesn't know and never falls through. The bundled steam
// plugin's art rides this path too (its gameIdPrefix keeps the ids, its
// local-art capability the files); only the logo prefix
// (/platypus-art/plugin/...) falls through to its handler.
func PluginGameArt(a *App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, stores.GameArtURLPrefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		id, encoded, found := strings.Cut(rest, "/")
		if !found || id == "" || encoded == "" || strings.Contains(encoded, "/") {
			next.ServeHTTP(w, r)
			return
		}
		if id == "plugin" || !pluginIconID.MatchString(id) {
			next.ServeHTTP(w, r)
			return
		}
		path := stores.DecodeGameArt(id, encoded)
		if path == "" {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp":
		default:
			http.NotFound(w, r)
			return
		}
		if a.plugins == nil || !a.pluginGameArtAllowed(id, path) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, path)
	})
}
