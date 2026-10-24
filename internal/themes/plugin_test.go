package themes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFixturePluginThemeLoads(t *testing.T) {
	dir := filepath.Join("testdata", "fixture-theme")
	th := LoadFS(os.DirFS(dir), "theme.toml", "fixture-theme")
	if th.Info.Status != StatusOK {
		t.Fatalf("fixture status %q warnings %v, want ok", th.Info.Status, th.Info.Warnings)
	}
	if len(th.Info.Warnings) != 0 {
		t.Fatalf("fixture warnings %v, want none", th.Info.Warnings)
	}
	if th.Info.Source != "fixture-theme" {
		t.Fatalf("source = %q, want the plugin id", th.Info.Source)
	}
	if th.Info.ID != "fixture" {
		t.Fatalf("id = %q, want fixture", th.Info.ID)
	}
	if th.Fonts["Fixture Sans"] != "fonts/fixture.woff2" {
		t.Fatalf("fonts = %v, want the shipped family", th.Fonts)
	}
	if len(th.Wash) != 1 || th.Wash[0].Image != "assets/bg.png" {
		t.Fatalf("wash = %+v, want the image reference", th.Wash)
	}
	if _, ok := th.Layouts["dashboard"]; !ok {
		t.Fatalf("layouts = %v, want dashboard", th.Layouts)
	}
	got := ResolveTheme(th, true)
	if got.Fonts["Fixture Sans"] != "fonts/fixture.woff2" {
		t.Fatalf("resolved fonts = %v", got.Fonts)
	}
	if len(got.Wash) != 1 || got.Wash[0].Image != "assets/bg.png" {
		t.Fatalf("resolved wash = %+v", got.Wash)
	}
}

// Contrast and catalog validation apply to plugin themes exactly as to the
// built-ins: a bad pair falls back with a warning, and a newer catalog needs
// a newer Platypus.
func TestPluginThemeContrastAndCatalog(t *testing.T) {
	base := map[string]string{
		"theme.toml":    "[theme]\nid = \"x\"\nname = \"X\"\nschema = 1\nsupports = [\"tokens\"]\n\n[tokens]\naccent = \"#7dd3fc\"\n",
		"assets/bg.png": string(append([]byte("\x89PNG\r\n\x1a\n"), []byte("body")...)),
	}
	bad := fstest.MapFS{
		"theme.toml": &fstest.MapFile{Data: []byte(base["theme.toml"] + "text = \"#0b1118\"\n")},
	}
	th := LoadFS(bad, "theme.toml", "p")
	if th.Info.Status != StatusOK || !warningsContain(th, "text on ground") {
		t.Fatalf("contrast status %q warnings %v, want a fallback warning", th.Info.Status, th.Info.Warnings)
	}
	if _, set := th.Tokens["text"]; set {
		t.Error("a failing token was kept, want it dropped")
	}
	// A wash image that fails the sniff costs the whole wash, like any bad layer.
	withImage := fstest.MapFS{
		"theme.toml":    &fstest.MapFile{Data: []byte(base["theme.toml"] + "\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.png\"\n")},
		"assets/bg.png": &fstest.MapFile{Data: []byte("not an image")},
	}
	th = LoadFS(withImage, "theme.toml", "p")
	if !warningsContain(th, "not a PNG") || th.Wash != nil {
		t.Fatalf("bad wash image reached the UI: %+v (%v)", th.Wash, th.Info.Warnings)
	}
	// A newer catalog lists the theme as newer, exactly like a built-in.
	newer := fstest.MapFS{
		"theme.toml": &fstest.MapFile{Data: []byte("[theme]\nid = \"x\"\nname = \"X\"\nschema = 1\ncatalog = \"2.0\"\nsupports = [\"tokens\"]\n")},
	}
	th = LoadFS(newer, "theme.toml", "p")
	if th.Info.Status != StatusNewer || !warningsContain(th, "needs catalog 2.0") {
		t.Fatalf("catalog status %q warnings %v", th.Info.Status, th.Info.Warnings)
	}
	_ = base
}

func TestPluginThemeAssetRefsRejected(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("body")...)
	woff := append([]byte("wOF2"), make([]byte, 100)...)
	big := append(png, make([]byte, MaxAssetBytes)...)
	write := func(files fstest.MapFS, themeExtra string) Theme {
		src := "[theme]\nid = \"x\"\nname = \"X\"\nschema = 1\nsupports = [\"tokens\"]\n" + themeExtra
		files["theme.toml"] = &fstest.MapFile{Data: []byte(src)}
		return LoadFS(files, "theme.toml", "p")
	}
	cases := map[string]struct {
		files fstest.MapFS
		extra string
		want  string
		check func(Theme) bool
	}{
		"traversal wash": {
			files: fstest.MapFS{},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"../outside.png\"\n",
			want:  "inside the theme folder",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"traversal font": {
			files: fstest.MapFS{},
			extra: "\n[fonts]\n\"X\" = \"../outside.woff2\"\n",
			want:  "inside the theme folder",
			check: func(th Theme) bool { return len(th.Fonts) == 0 },
		},
		"quote in path": {
			files: fstest.MapFS{},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = 'assets/a\"b.png'\n",
			want:  "may not use",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"oversize": {
			files: fstest.MapFS{"assets/big.png": &fstest.MapFile{Data: big}},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/big.png\"\n",
			want:  "larger than",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"wrong type png as text": {
			files: fstest.MapFS{"assets/bg.png": &fstest.MapFile{Data: []byte("just text")}},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.png\"\n",
			want:  "not a PNG",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"png content under svg name": {
			files: fstest.MapFS{"assets/bg.svg": &fstest.MapFile{Data: png}},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.svg\"\n",
			want:  "must end in .png",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"text under png name": {
			files: fstest.MapFS{"assets/bg.png": &fstest.MapFile{Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")}},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.png\"\n",
			want:  "must end in .svg",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"font text as woff2": {
			files: fstest.MapFS{"fonts/x.woff2": &fstest.MapFile{Data: []byte("just text")}},
			extra: "\n[fonts]\n\"X\" = \"fonts/x.woff2\"\n",
			want:  "not a woff2",
			check: func(th Theme) bool { return len(th.Fonts) == 0 },
		},
		"font png as woff2": {
			files: fstest.MapFS{"fonts/x.woff2": &fstest.MapFile{Data: png}},
			extra: "\n[fonts]\n\"X\" = \"fonts/x.woff2\"\n",
			want:  "not a woff2",
			check: func(th Theme) bool { return len(th.Fonts) == 0 },
		},
		"missing file": {
			files: fstest.MapFS{},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/missing.png\"\n",
			want:  "can't be read",
			check: func(th Theme) bool { return th.Wash == nil },
		},
		"unknown font in stack": {
			files: fstest.MapFS{"fonts/x.woff2": &fstest.MapFile{Data: woff}},
			extra: "\n[tokens]\naccent = \"#7dd3fc\"\nfont-sans = '\"Y\", sans-serif'\n\n[fonts]\n\"X\" = \"fonts/x.woff2\"\n",
			want:  "not one the app ships",
			check: func(th Theme) bool { return th.Tokens["font-sans"] == "" },
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			th := write(c.files, c.extra)
			if !warningsContain(th, c.want) || !c.check(th) {
				t.Fatalf("warnings %v tokens %v fonts %v wash %+v, want %q",
					th.Info.Warnings, th.Tokens, th.Fonts, th.Wash, c.want)
			}
		})
	}
	// A shipped family is usable in a font token.
	th := write(fstest.MapFS{"fonts/x.woff2": &fstest.MapFile{Data: woff}},
		"\n[tokens]\naccent = \"#7dd3fc\"\nfont-sans = '\"X\", sans-serif'\n\n[fonts]\n\"X\" = \"fonts/x.woff2\"\n")
	if th.Tokens["font-sans"] != `"X", sans-serif` {
		t.Fatalf("shipped font not accepted: %v (%v)", th.Tokens, th.Info.Warnings)
	}
	// An svg image validates by content.
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="7"/></svg>`)
	th = write(fstest.MapFS{"assets/bg.svg": &fstest.MapFile{Data: svg}},
		"\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.svg\"\n")
	if len(th.Wash) != 1 || th.Wash[0].Image != "assets/bg.svg" {
		t.Fatalf("svg wash rejected: %+v (%v)", th.Wash, th.Info.Warnings)
	}
	// HTML pretending to be svg is refused.
	th = write(fstest.MapFS{"assets/bg.svg": &fstest.MapFile{Data: []byte("<html><script>alert(1)</script></html>")}},
		"\n[tokens]\naccent = \"#7dd3fc\"\n\n[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.svg\"\n")
	if th.Wash != nil || !warningsContain(th, "not a PNG") {
		t.Fatalf("html svg accepted: %+v (%v)", th.Wash, th.Info.Warnings)
	}
}

func TestLoadFSMissingFileIsInvalid(t *testing.T) {
	th := LoadFS(fstest.MapFS{}, "theme.toml", "p")
	if th.Info.Status != StatusInvalid || !warningsContain(th, "can't be read") {
		t.Fatalf("status %q warnings %v", th.Info.Status, th.Info.Warnings)
	}
	if th.Info.Source != "p" {
		t.Fatalf("source = %q, want the plugin id even when invalid", th.Info.Source)
	}
}

func TestLoadFSLayoutsWarnLikeBuiltins(t *testing.T) {
	fsys := fstest.MapFS{
		"theme.toml":            &fstest.MapFile{Data: []byte("[theme]\nid = \"x\"\nname = \"X\"\nschema = 1\nsupports = [\"tokens\", \"layouts\"]\n")},
		"layouts/broken.xml":    &fstest.MapFile{Data: []byte("<layout>")},
		"layouts/note.txt":      &fstest.MapFile{Data: []byte("not xml")},
		"layouts/dashboard.xml": &fstest.MapFile{Data: []byte("<layout view=\"dashboard\" catalog=\"1.0\"><region name=\"main\"><block name=\"hero\" /></region></layout>")},
	}
	th := LoadFS(fsys, "theme.toml", "p")
	if th.Info.Status != StatusOK {
		t.Fatalf("status %q warnings %v", th.Info.Status, th.Info.Warnings)
	}
	joined := strings.Join(th.Info.Warnings, "\n")
	if !strings.Contains(joined, "note.txt") || !strings.Contains(joined, "broken") {
		t.Fatalf("layout warnings = %v, want both files reported", th.Info.Warnings)
	}
	if _, ok := th.Layouts["dashboard"]; !ok {
		t.Fatalf("layouts = %v, want dashboard", th.Layouts)
	}
}
