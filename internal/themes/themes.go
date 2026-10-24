// Package themes reads the launcher's themes (issue #93). A theme is a
// theme.toml that names token values (colours, radii, fonts) and the
// background wash. The core parses and validates it and hands the UI resolved
// values; a theme never carries code, CSS text or URLs.
package themes

import (
	"embed"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"platypus/internal/types"
)

//go:embed builtin
var builtin embed.FS

// Schema is the newest theme.toml schema this build reads. A theme that asks
// for a newer one is listed as "newer" and can't be picked.
const Schema = 1

// DefaultThemeCatalog is assumed when an older theme omits its catalog target.
const DefaultThemeCatalog = "1.0"

// DefaultID is the built-in theme the launcher starts with. Every token a
// theme leaves out, or sets badly, takes its value from here.
const DefaultID = "warm"

// The statuses of a theme (types.ThemeInfo.Status).
const (
	StatusOK      = "ok"
	StatusNewer   = "newer"
	StatusInvalid = "invalid"
)

// supportsTokens is the level a theme declares for its tokens to apply.
const supportsTokens = "tokens"

// The token set v1 (issue #90), by kind. The names match the @theme variables
// in apps/ui/src/style.css where those exist.
var (
	colorTokens  = []string{"ground", "raised", "surface", "surface-strong", "hairline", "text", "text-body", "text-muted", "text-faint", "accent", "accent-strong", "on-accent", "ok", "attention", "info", "dl", "danger", "focus"}
	radiusTokens = []string{"radius-chip", "radius-card", "radius-panel"}
	fontTokens   = []string{"font-sans", "font-mono"}
	allTokens    = slices.Concat(colorTokens, radiusTokens, fontTokens)
)

// bundledFonts are the families the app ships (style.css @font-face): the only
// names a font token may use. genericFonts are the fallbacks it may end with.
var (
	bundledFonts = []string{"Hanken Grotesk", "JetBrains Mono"}
	genericFonts = []string{"system-ui", "ui-sans-serif", "sans-serif", "serif", "ui-monospace", "monospace"}
)

const maxWashLayers = 4

var (
	idRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	// A number in a colour function, then the separators CSS allows between
	// them. Nothing else gets into a colour value: no names of functions, no
	// variables, no quotes.
	number    = `[-+]?(?:\d+\.?\d*|\.\d+)(?:%|deg)?`
	separator = `(?:\s*[,/]\s*|\s+)`
	colorRe   = regexp.MustCompile(`^(?:#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})|transparent|(?:rgba?|hsla?|oklch)\(\s*` + number + `(?:` + separator + number + `){2,3}\s*\))$`)
	// Radii take px or rem; wash sizes and fades also take percentages; wash
	// positions may be negative.
	lengthRe    = regexp.MustCompile(`^\d+(?:\.\d+)?(?:px|rem)$`)
	lengthPctRe = regexp.MustCompile(`^\d+(?:\.\d+)?(?:px|rem|%)$`)
	positionRe  = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:px|rem|%)$`)
)

// Theme is one parsed theme.toml. Tokens and Wash hold only the values that
// passed validation; Resolve fills in the rest from the default.
type Theme struct {
	Info     types.ThemeInfo
	Tokens   map[string]string
	Wash     []types.WashLayer
	Variants map[string]string
}

// file is the theme.toml shape. Tokens and wash layers decode as loose maps so
// one bad value costs only that token, not the whole file.
type file struct {
	Theme struct {
		ID       string   `toml:"id"`
		Name     string   `toml:"name"`
		Schema   int      `toml:"schema"`
		Catalog  string   `toml:"catalog"`
		Supports []string `toml:"supports"`
	} `toml:"theme"`
	Tokens   map[string]any   `toml:"tokens"`
	Wash     []map[string]any `toml:"wash"`
	Variants map[string]any   `toml:"variants"`
}

// Parse reads one theme.toml. It never fails: a file it can't use comes back
// with status invalid and the reason in Warnings.
func Parse(src []byte) Theme {
	return parseTheme(src, true)
}

// parseTheme is Parse with the contrast check optional: reading the built-in
// default theme skips it, since that read supplies the values the check falls
// back to.
func parseTheme(src []byte, checkContrast bool) Theme {
	t := Theme{
		Info:     types.ThemeInfo{Status: StatusInvalid, Supports: []string{}, Warnings: []string{}},
		Tokens:   map[string]string{},
		Variants: map[string]string{},
	}
	var f file
	md, err := toml.Decode(string(src), &f)
	if err != nil {
		t.warn("theme.toml can't be read: %v", err)
		return t
	}
	for _, k := range md.Undecoded() {
		t.warn("unknown key %s, ignored", k)
	}
	t.Info.ID = f.Theme.ID
	if !idRe.MatchString(f.Theme.ID) {
		t.warn("theme.id must be lowercase letters, digits and dashes, got %q", f.Theme.ID)
		return t
	}
	t.Info.Name = strings.TrimSpace(f.Theme.Name)
	if t.Info.Name == "" {
		t.Info.Name = f.Theme.ID
	}
	t.Info.Schema = f.Theme.Schema
	t.Info.Supports = append([]string{}, f.Theme.Supports...)
	switch {
	case f.Theme.Schema < 1:
		t.warn("theme.schema is missing")
		return t
	case f.Theme.Schema > Schema:
		t.Info.Status = StatusNewer
		t.warn("needs schema %d, this Platypus reads schema %d", f.Theme.Schema, Schema)
		return t
	}
	catalog := f.Theme.Catalog
	if catalog == "" {
		catalog = DefaultThemeCatalog
	}
	targetMajor, _, validCatalog := catalogVersion(catalog)
	currentMajor, _, _ := catalogVersion(CatalogVersion)
	if !validCatalog {
		t.warn("theme.catalog must be a major.minor version, assuming %s", DefaultThemeCatalog)
		catalog = DefaultThemeCatalog
		targetMajor, _, _ = catalogVersion(catalog)
	}
	if targetMajor > currentMajor {
		t.Info.Status = StatusNewer
		t.warn("needs catalog %s, this Platypus reads catalog %s", catalog, CatalogVersion)
		return t
	}
	t.Info.Status = StatusOK
	for _, name := range slices.Sorted(maps.Keys(f.Tokens)) {
		t.setToken(name, f.Tokens[name])
	}
	if checkContrast {
		t.checkContrast()
	}
	if len(f.Wash) > 0 {
		t.setWash(f.Wash)
	}
	for _, key := range slices.Sorted(maps.Keys(f.Variants)) {
		t.setVariant(key, f.Variants[key])
	}
	return t
}

func catalogVersion(version string) (major, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, false
	}
	return major, minor, true
}

func (t *Theme) setVariant(key string, raw any) {
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		t.warn("unknown variant axis %q, ignored", key)
		return
	}
	for _, entry := range Catalog {
		if entry.Entry != parts[0] {
			continue
		}
		for _, axis := range entry.Axes {
			if axis.Name != parts[1] {
				continue
			}
			value, ok := raw.(string)
			if !ok || !slices.Contains(axis.Values, value) {
				t.warn("unknown value %q for variant %s, using the built-in value", raw, key)
				return
			}
			t.Variants[key] = value
			return
		}
	}
	t.warn("unknown variant axis %q, ignored", key)
}

func (t *Theme) warn(format string, args ...any) {
	t.Info.Warnings = append(t.Info.Warnings, fmt.Sprintf(format, args...))
}

func (t *Theme) setToken(name string, raw any) {
	if !slices.Contains(allTokens, name) {
		t.warn("unknown token %q, ignored", name)
		return
	}
	v, ok := raw.(string)
	if !ok {
		t.warn("token %s must be a string, using the built-in value", name)
		return
	}
	val, err := checkToken(name, strings.TrimSpace(v))
	if err != nil {
		t.warn("token %s: %v, using the built-in value", name, err)
		return
	}
	t.Tokens[name] = val
}

// checkToken validates one token value and returns it in canonical form.
func checkToken(name, v string) (string, error) {
	if strings.ContainsAny(v, ";{}") || strings.Contains(strings.ToLower(v), "url(") {
		return "", fmt.Errorf("%q has a character a theme may not use", v)
	}
	switch {
	case slices.Contains(colorTokens, name):
		if colorRe.MatchString(v) {
			return v, nil
		}
		return "", fmt.Errorf("%q is not a colour (hex, rgb(), hsl() or oklch())", v)
	case slices.Contains(radiusTokens, name):
		if lengthRe.MatchString(v) {
			return v, nil
		}
		return "", fmt.Errorf("%q is not a length in px or rem", v)
	default:
		return fontStack(v)
	}
}

// fontStack checks a font list: bundled family names in double quotes and
// generic families, comma separated. It returns the list in canonical form.
func fontStack(v string) (string, error) {
	var out []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if len(item) >= 2 && strings.HasPrefix(item, `"`) && strings.HasSuffix(item, `"`) {
			name := item[1 : len(item)-1]
			if !slices.Contains(bundledFonts, name) {
				return "", fmt.Errorf("font %q is not one the app ships", name)
			}
			out = append(out, `"`+name+`"`)
			continue
		}
		if !slices.Contains(genericFonts, item) {
			return "", fmt.Errorf("font %q is neither a bundled family in quotes nor a generic family", item)
		}
		out = append(out, item)
	}
	return strings.Join(out, ", "), nil
}

// setWash validates the wash layers. One bad layer costs the whole wash, which
// then keeps the built-in one.
func (t *Theme) setWash(raw []map[string]any) {
	if len(raw) > maxWashLayers {
		t.warn("wash has %d layers, at most %d, using the built-in wash", len(raw), maxWashLayers)
		return
	}
	layers := make([]types.WashLayer, 0, len(raw))
	for i, m := range raw {
		layer, err := t.washLayer(m)
		if err != nil {
			t.warn("wash layer %d: %v, using the built-in wash", i+1, err)
			return
		}
		layers = append(layers, layer)
	}
	t.Wash = layers
}

func (t *Theme) washLayer(m map[string]any) (types.WashLayer, error) {
	var l types.WashLayer
	fields := map[string]*string{"at": &l.At, "size": &l.Size, "color": &l.Color, "fade": &l.Fade}
	for _, key := range slices.Sorted(maps.Keys(m)) {
		dst, ok := fields[key]
		if !ok {
			t.warn("unknown wash key %q, ignored", key)
			continue
		}
		s, ok := m[key].(string)
		if !ok {
			return l, fmt.Errorf("%s must be a string", key)
		}
		*dst = strings.Join(strings.Fields(s), " ")
	}
	if l.At == "" || l.Size == "" || l.Color == "" || l.Fade == "" {
		return l, fmt.Errorf("needs at, size, color and fade")
	}
	if !pair(l.At, positionRe) || !pair(l.Size, lengthPctRe) || !colorRe.MatchString(l.Color) || !lengthPctRe.MatchString(l.Fade) {
		return l, fmt.Errorf("%q is not a valid gradient layer", l.Color)
	}
	return l, nil
}

// pair reports whether s is two values, each matching re.
func pair(s string, re *regexp.Regexp) bool {
	parts := strings.Fields(s)
	return len(parts) == 2 && re.MatchString(parts[0]) && re.MatchString(parts[1])
}

// All returns the built-in themes: the default first, then by name.
func All() []Theme {
	entries, err := builtin.ReadDir("builtin")
	if err != nil {
		return nil
	}
	var out []Theme
	for _, e := range entries {
		src, err := builtin.ReadFile(path.Join("builtin", e.Name(), "theme.toml"))
		if err != nil {
			continue
		}
		out = append(out, Parse(src))
	}
	slices.SortFunc(out, func(a, b Theme) int {
		switch {
		case a.Info.ID == DefaultID && b.Info.ID != DefaultID:
			return -1
		case b.Info.ID == DefaultID && a.Info.ID != DefaultID:
			return 1
		}
		return strings.Compare(a.Info.Name, b.Info.Name)
	})
	return out
}

// Infos lists the built-in themes for the Settings picker.
func Infos() []types.ThemeInfo {
	var out []types.ThemeInfo
	for _, t := range All() {
		out = append(out, t.Info)
	}
	return out
}

// Find returns the built-in theme with the given id.
func Find(id string) (Theme, bool) {
	for _, t := range All() {
		if t.Info.ID == id {
			return t, true
		}
	}
	return Theme{}, false
}

// Resolve returns the tokens to apply for the theme with the given id, "" for
// the default. A theme that is missing, unusable, or doesn't declare the
// tokens level gets the built-in values. Otherwise each token the theme sets
// validly wins, and the rest keep the built-in value.
func Resolve(id string) types.ThemeTokens {
	t, ok := Find(id)
	return resolve(t, ok)
}

func resolve(t Theme, found bool) types.ThemeTokens {
	def, ok := Find(DefaultID)
	if !ok {
		panic("themes: the built-in default " + DefaultID + " is missing")
	}
	variants := map[string]string{}
	for _, entry := range Catalog {
		for _, axis := range entry.Axes {
			key := entry.Entry + "." + axis.Name
			variants[key] = axis.Default
		}
	}
	out := types.ThemeTokens{ID: def.Info.ID, Tokens: map[string]string{}, Wash: def.Wash}
	out.Variants = types.ThemeVariants{
		Settings: types.ThemeSettingsVariants{
			Presentation: variants["settings.presentation"],
			Navigation:   variants["settings.navigation"],
		},
		Section: types.ThemeSectionVariants{Chrome: variants["section.chrome"]},
	}
	for _, name := range allTokens {
		out.Tokens[name] = def.Tokens[name]
	}
	if !found || t.Info.Status != StatusOK || !slices.Contains(t.Info.Supports, supportsTokens) {
		return out
	}
	out.ID = t.Info.ID
	for name, v := range t.Tokens {
		out.Tokens[name] = v
	}
	if t.Wash != nil {
		out.Wash = t.Wash
	}
	if slices.Contains(t.Info.Supports, "variants") {
		for key, value := range t.Variants {
			variants[key] = value
		}
	}
	out.Variants = types.ThemeVariants{
		Settings: types.ThemeSettingsVariants{
			Presentation: variants["settings.presentation"],
			Navigation:   variants["settings.navigation"],
		},
		Section: types.ThemeSectionVariants{Chrome: variants["section.chrome"]},
	}
	return out
}
