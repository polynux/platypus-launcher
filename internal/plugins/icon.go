// Plugin logos (issue #21): a store plugin may ship a logo (SVG preferred,
// PNG accepted) referenced from its manifest ("icon": "icon.svg"). The core
// validates it and serves it locally; the UI draws it in the sidebar Sources
// and the Plugins view, with a monogram tile when there is none.

package plugins

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxIconBytes caps a plugin logo. Logos are tiny marks; this leaves
	// room for a detailed SVG without letting a stray file fill memory.
	MaxIconBytes = 256 << 10

	// IconURLPrefix is where the asset server serves plugin logos (see
	// App.PluginArt). It lives under customize.URLPrefix's route so the
	// Electron shell's /platypus-art/ proxy passes it through; the first
	// segment ("plugin") can never be a customize image key (16 hex
	// chars), so the two never collide.
	IconURLPrefix = "/platypus-art/plugin/"
)

// IconContentTypes are the sniffed content types a logo may be served as.
const (
	IconSVG = "image/svg+xml"
	IconPNG = "image/png"
)

// IconPath resolves the manifest's icon, following symlinks, and checks it
// stays inside the plugin folder, is a small regular file and holds SVG or
// PNG data. Any failure (missing file included) is an error: the caller
// serves no logo, and the plugin itself stays valid.
func (m Manifest) IconPath(dir string) (path, ctype string, err error) {
	if m.Icon == "" {
		return "", "", fmt.Errorf("no icon")
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", err
	}
	path, err = filepath.EvalSymlinks(filepath.Join(realDir, m.Icon))
	if err != nil {
		return "", "", fmt.Errorf("icon: %w", err)
	}
	if rel, err := filepath.Rel(realDir, path); err != nil || !filepath.IsLocal(rel) {
		return "", "", fmt.Errorf("icon %q resolves outside the plugin folder", m.Icon)
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("icon: %w", err)
	}
	if !st.Mode().IsRegular() {
		return "", "", fmt.Errorf("icon %q is not a regular file", m.Icon)
	}
	if st.Size() == 0 {
		return "", "", fmt.Errorf("icon %q is empty", m.Icon)
	}
	if st.Size() > MaxIconBytes {
		return "", "", fmt.Errorf("icon %q is larger than %d KB", m.Icon, MaxIconBytes>>10)
	}
	head := make([]byte, 512)
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("icon: %w", err)
	}
	n, rerr := f.Read(head)
	_ = f.Close()
	if rerr != nil && n == 0 {
		return "", "", fmt.Errorf("icon: can't read %q", m.Icon)
	}
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return path, IconPNG, nil
	case isSVG(head):
		return path, IconSVG, nil
	}
	return "", "", fmt.Errorf("icon %q is not an SVG or PNG image", m.Icon)
}

// isSVG reports whether head looks like SVG data: an <svg> root, optionally
// after an XML declaration or doctype. The type comes from the content, never
// from the file's name.
func isSVG(head []byte) bool {
	s := strings.TrimSpace(string(head))
	if strings.HasPrefix(strings.ToLower(s), "<svg") {
		return true
	}
	if strings.HasPrefix(s, "<?xml") {
		lower := strings.ToLower(s)
		if i := strings.Index(lower, "<svg"); i >= 0 {
			// The root follows the declaration; anything else (HTML, a
			// script) is refused.
			rest := strings.TrimSpace(lower[:i])
			return strings.HasSuffix(rest, "?>") || strings.HasSuffix(rest, ">")
		}
	}
	return false
}

// IconFile returns the on-disk logo of a known plugin and its content type.
// ok is false when the plugin is unknown or its logo is missing or invalid:
// the UI shows its monogram tile instead.
func (m *Manager) IconFile(id string) (path, ctype string, ok bool) {
	m.mu.Lock()
	p, known := m.plugins[id]
	m.mu.Unlock()
	if !known || p.Manifest.Icon == "" {
		return "", "", false
	}
	path, ctype, err := p.Manifest.IconPath(p.Dir)
	if err != nil {
		return "", "", false
	}
	return path, ctype, true
}

// IconURL is the address the UI draws the plugin's logo from ("" when it has
// no valid one). The plugin hash versions it, so a replaced logo is a new
// address and the response can be cached immutably.
func (m *Manager) IconURL(id string) string {
	m.mu.Lock()
	p, known := m.plugins[id]
	m.mu.Unlock()
	if !known || p.Manifest.Icon == "" {
		return ""
	}
	if _, _, err := p.Manifest.IconPath(p.Dir); err != nil {
		return ""
	}
	u := IconURLPrefix + id
	if len(p.Hash) >= 12 {
		u += "?v=" + p.Hash[:12]
	} else if p.Hash != "" {
		u += "?v=" + p.Hash
	}
	return u
}
