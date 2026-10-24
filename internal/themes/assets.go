// Theme assets for data-plugin themes (issue #105, ADR 0002 §8). A theme may
// ship images (png, webp, svg) and fonts (woff2) inside its own folder. The
// wash may reference an image by relative path and font tokens may use a
// shipped family; the core turns those paths into art URLs at serve time.
// Only the relative paths are stored here; the app maps them to URLs.
package themes

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// MaxAssetBytes caps one theme asset. Backgrounds can be larger than a plugin
// logo, but a stray file must not fill memory.
const MaxAssetBytes = 2 << 20

// ThemeURLPrefix is where the asset server serves plugin theme files (see
// App.ThemeArt). It lives under customize.URLPrefix's route so the Electron
// shell's /platypus-art/ proxy passes it through; the first segment ("theme")
// can never be a game-art plugin id the way custom art hashes can, and the
// logo prefix ("plugin") is already taken, so the three never collide.
const ThemeURLPrefix = "/platypus-art/theme/"

// themeContentTypes are the sniffed types a theme asset may be served as.
const (
	ThemePNG  = "image/png"
	ThemeWEBP = "image/webp"
	ThemeSVG  = "image/svg+xml"
	ThemeWOFF = "font/woff2"
)

// assetKind is what a relative path was asked for: an image or a font.
type assetKind int

const (
	assetImage assetKind = iota
	assetFont
)

// checkAssetRef validates one relative asset path and reads it from the theme
// folder. It returns the cleaned relative path. A reference that leaves the
// folder, names a missing or oversized file, or holds the wrong content is
// refused with a warning, and the caller drops that token or layer.
func (t *Theme) checkAssetRef(fsys fs.FS, themeDir, raw string, kind assetKind) (string, error) {
	rel := strings.TrimSpace(raw)
	if rel == "" || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%q must be a path inside the theme folder", raw)
	}
	// A quote, backslash or control character would break the url("...") the
	// core builds from this path, so it never reaches the UI.
	if strings.ContainsAny(rel, "\"'\\") || strings.ContainsAny(rel, "\n\r\t") {
		return "", fmt.Errorf("%q has a character a theme may not use", raw)
	}
	// The FS joins with slashes; an OS-absolute or backslash escape never
	// passes IsLocal, but a leading "./" would read the same file under two
	// names, so clean it.
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("%q must be a path inside the theme folder", raw)
	}
	if fsys == nil {
		return "", fmt.Errorf("%q can't be checked without the theme folder", raw)
	}
	name := rel
	if themeDir != "" && themeDir != "." {
		name = path.Join(themeDir, rel)
	}
	f, err := fsys.Open(name)
	if err != nil {
		return "", fmt.Errorf("%q can't be read: %v", raw, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("%q can't be read: %v", raw, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a file", raw)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("%q is empty", raw)
	}
	if info.Size() > MaxAssetBytes {
		return "", fmt.Errorf("%q is larger than %d KB", raw, MaxAssetBytes>>10)
	}
	head := make([]byte, 512)
	n, rerr := io.ReadFull(f, head)
	if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
		return "", fmt.Errorf("%q can't be read: %v", raw, rerr)
	}
	head = head[:n]
	var ctype string
	switch kind {
	case assetImage:
		ctype = sniffThemeImage(head)
		if ctype == "" {
			return "", fmt.Errorf("%q is not a PNG, WebP or SVG image", raw)
		}
		ext := strings.ToLower(path.Ext(rel))
		switch ctype {
		case ThemePNG:
			if ext != ".png" {
				return "", fmt.Errorf("%q must end in .png", raw)
			}
		case ThemeWEBP:
			if ext != ".webp" {
				return "", fmt.Errorf("%q must end in .webp", raw)
			}
		case ThemeSVG:
			if ext != ".svg" {
				return "", fmt.Errorf("%q must end in .svg", raw)
			}
		}
	case assetFont:
		if !bytes.HasPrefix(head, []byte("wOF2")) {
			return "", fmt.Errorf("%q is not a woff2 font", raw)
		}
		if strings.ToLower(path.Ext(rel)) != ".woff2" {
			return "", fmt.Errorf("%q must end in .woff2", raw)
		}
		ctype = ThemeWOFF
		_ = ctype
	}
	return rel, nil
}

// sniffThemeImage reports the content type of a theme image header, or "" for
// anything else. The type comes from the content, never from the file's name.
func sniffThemeImage(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return ThemePNG
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return ThemeWEBP
	case isThemeSVG(head):
		return ThemeSVG
	}
	return ""
}

// isThemeSVG reports whether head looks like SVG data: an <svg> root,
// optionally after an XML declaration. Anything else (HTML, a script) is
// refused.
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
