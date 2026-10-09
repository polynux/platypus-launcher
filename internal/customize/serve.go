package customize

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	keyPattern  = regexp.MustCompile(`^[0-9a-f]{16}$`)
	filePattern = regexp.MustCompile(`^(cover|banner|logo|icon|background)\.(png|jpg|gif|webp)$`)
)

var contentTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}

// Serve answers a request under URLPrefix with the stored image and reports
// whether the request was one of its own. Only the exact names the store writes
// are served, so no path in the request can leave the art folder.
func (s *Store) Serve(w http.ResponseWriter, r *http.Request) bool {
	rest, ok := strings.CutPrefix(r.URL.Path, URLPrefix)
	if !ok {
		return false
	}
	dir, name, found := strings.Cut(rest, "/")
	if !found || !keyPattern.MatchString(dir) || !filePattern.MatchString(name) {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Content-Type", contentTypes[filepath.Ext(name)])
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The URL carries the image's version, so a replacement is a new address.
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.artRoot(), dir, name))
	return true
}
