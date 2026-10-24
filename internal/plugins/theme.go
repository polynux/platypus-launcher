// Plugin themes (issue #105): a data plugin may carry a theme (theme.toml +
// layouts/*.xml + assets). The manifest names the toml in Theme; the manager
// loads it with internal/themes at discovery, and the app serves its images
// and fonts through the token-protected art route. A missing or broken theme
// never breaks the plugin's data: the theme is listed as invalid instead.
package plugins

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/themes"
)

// ThemeFile returns the on-disk file of one theme asset and its content type.
// ok is false when the plugin is unknown, carries no theme, or the file is
// missing or invalid: the art handler answers 404.
func (m *Manager) ThemeFile(id, rel string) (path, ctype string, ok bool) {
	m.mu.Lock()
	p, known := m.plugins[id]
	m.mu.Unlock()
	if !known || p.Theme == nil {
		return "", "", false
	}
	path, ctype, err := p.Manifest.ThemeAssetPath(p.Dir, rel)
	if err != nil {
		return "", "", false
	}
	return path, ctype, true
}

// ThemeURL is the address the UI fetches one theme asset from ("" when it has
// no valid file). The plugin hash versions it, so a replaced file is a new
// address and the response can be cached immutably.
func (m *Manager) ThemeURL(id, rel string) string {
	m.mu.Lock()
	p, known := m.plugins[id]
	m.mu.Unlock()
	if !known || p.Theme == nil {
		return ""
	}
	if _, _, err := p.Manifest.ThemeAssetPath(p.Dir, rel); err != nil {
		return ""
	}
	u := themes.ThemeURLPrefix + id + "/" + rel
	if len(p.Hash) >= 12 {
		u += "?v=" + p.Hash[:12]
	} else if p.Hash != "" {
		u += "?v=" + p.Hash
	}
	return u
}

// ThemeAssetPath resolves one theme asset inside the plugin folder, following
// symlinks, and checks it stays inside, is a small regular file and holds a
// theme type (png, webp, svg or woff2). Any failure is an error: the caller
// serves nothing for it.
func (m Manifest) ThemeAssetPath(dir, rel string) (path, ctype string, err error) {
	if rel == "" || !filepath.IsLocal(rel) {
		return "", "", fmt.Errorf("theme asset %q must be a path inside the plugin folder", rel)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", err
	}
	path, err = filepath.EvalSymlinks(filepath.Join(realDir, filepath.FromSlash(rel)))
	if err != nil {
		return "", "", fmt.Errorf("theme asset: %w", err)
	}
	if r, err := filepath.Rel(realDir, path); err != nil || !filepath.IsLocal(r) {
		return "", "", fmt.Errorf("theme asset %q resolves outside the plugin folder", rel)
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("theme asset: %w", err)
	}
	if !st.Mode().IsRegular() {
		return "", "", fmt.Errorf("theme asset %q is not a regular file", rel)
	}
	if st.Size() == 0 {
		return "", "", fmt.Errorf("theme asset %q is empty", rel)
	}
	if st.Size() > themes.MaxAssetBytes {
		return "", "", fmt.Errorf("theme asset %q is larger than %d KB", rel, themes.MaxAssetBytes>>10)
	}
	head := make([]byte, 512)
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("theme asset: %w", err)
	}
	n, rerr := f.Read(head)
	_ = f.Close()
	if rerr != nil && n == 0 {
		return "", "", fmt.Errorf("theme asset: can't read %q", rel)
	}
	head = head[:n]
	ext := strings.ToLower(filepath.Ext(rel))
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")) && ext == ".png":
		return path, themes.ThemePNG, nil
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP" && ext == ".webp":
		return path, themes.ThemeWEBP, nil
	case isThemeSVG(head) && ext == ".svg":
		return path, themes.ThemeSVG, nil
	case bytes.HasPrefix(head, []byte("wOF2")) && ext == ".woff2":
		return path, themes.ThemeWOFF, nil
	}
	return "", "", fmt.Errorf("theme asset %q is not a PNG, WebP, SVG or woff2 file", rel)
}

// isThemeSVG reports whether head looks like SVG data, like the theme
// validator does: an <svg> root, optionally after an XML declaration.
func isThemeSVG(head []byte) bool {
	s := strings.TrimSpace(string(head))
	if strings.HasPrefix(strings.ToLower(s), "<svg") {
		return true
	}
	if strings.HasPrefix(s, "<?xml") {
		lower := strings.ToLower(s)
		if i := strings.Index(lower, "<svg"); i >= 0 {
			rest := strings.TrimSpace(lower[:i])
			return strings.HasSuffix(rest, "?>") || strings.HasSuffix(rest, ">")
		}
	}
	return false
}
