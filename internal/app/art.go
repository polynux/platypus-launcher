package app

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"platypus/internal/plugins"
	"platypus/internal/steam"
)

var pluginIconID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// SteamArt serves images from Steam's local library cache under
// steam.ArtURLPrefix. Newer titles only have hashed CDN URLs, but the Steam
// client has already downloaded their art.
func SteamArt(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel, ok := strings.CutPrefix(r.URL.Path, steam.ArtURLPrefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		root := steam.Root()
		clean := filepath.Clean("/" + rel) // blocks ../ escapes
		ext := strings.ToLower(filepath.Ext(clean))
		if root == "" || (ext != ".jpg" && ext != ".png") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "max-age=86400")
		http.ServeFile(w, r, filepath.Join(root, "appcache/librarycache", clean))
	})
}

// CustomArt serves the user's images to the shell under customize.URLPrefix,
// falling through to next for anything else. The shells chain it after
// SteamArt (see app_customize.go).
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
