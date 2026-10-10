package app

import (
	"net/http"
	"path/filepath"
	"strings"

	"platypus/internal/steam"
)

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
