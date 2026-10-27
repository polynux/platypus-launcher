package themes

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"
	"testing"
)

// closeColor reports whether two colours match to eps on every channel.
func closeColor(a, b rgba, eps float64) bool {
	return math.Abs(a.r-b.r) <= eps && math.Abs(a.g-b.g) <= eps &&
		math.Abs(a.b-b.b) <= eps && math.Abs(a.a-b.a) <= eps
}

func formatColor(c rgba) string {
	return fmt.Sprintf("(%.4f %.4f %.4f / %.4f)", c.r, c.g, c.b, c.a)
}

func TestParseColorFormats(t *testing.T) {
	tests := []struct {
		in   string
		want rgba
	}{
		{"#fff", rgba{1, 1, 1, 1}},
		{"#0008", rgba{0, 0, 0, 0x88 / 255.0}},
		{"#ff8000", rgba{1, 128 / 255.0, 0, 1}},
		{"#FF800080", rgba{1, 128 / 255.0, 0, 128 / 255.0}},
		{"transparent", rgba{0, 0, 0, 0}},
		{"rgb(255 128 0)", rgba{1, 128 / 255.0, 0, 1}},
		{"rgb(255, 128, 0)", rgba{1, 128 / 255.0, 0, 1}},
		{"rgba(255, 128, 0, 0.5)", rgba{1, 128 / 255.0, 0, 0.5}},
		{"rgb(100% 50% 0% / 25%)", rgba{1, 0.5, 0, 0.25}},
		{"hsl(0 100% 50%)", rgba{1, 0, 0, 1}},
		{"hsl(120deg 100% 25%)", rgba{0, 0.5, 0, 1}},
		{"hsla(240, 100%, 50%, 0.5)", rgba{0, 0, 1, 0.5}},
		{"oklch(0.5 0 0)", rgba{0.3886, 0.3886, 0.3886, 1}},
		{"oklch(1 0 0)", rgba{1, 1, 1, 1}},
		{"oklch(0 0 0)", rgba{0, 0, 0, 1}},
	}
	for _, tc := range tests {
		got, ok := parseColor(tc.in)
		if !ok {
			t.Errorf("parseColor(%q) = not a colour", tc.in)
			continue
		}
		if !closeColor(got, tc.want, 1.0/255) {
			t.Errorf("parseColor(%q) = %s, want %s", tc.in, formatColor(got), formatColor(tc.want))
		}
	}
	for _, in := range []string{"", "red", "#12", "#12345", "rgb(1 2)", "rgb(10deg 20 30)", "oklch(0.5 0.1 h)", "hsl(0 100% 50deg)", "transparent-ish", "url(#fff)"} {
		if got, ok := parseColor(in); ok {
			t.Errorf("parseColor(%q) = %s, want it rejected", in, formatColor(got))
		}
	}
}

// The OKLCH conversion must land where a browser paints it, and clamp what
// sRGB can't show.
func TestOKLCHToSRGB(t *testing.T) {
	tests := []struct {
		in   string
		want rgba
	}{
		{"oklch(62.8% 0.2577 29.23)", rgba{1, 0, 0, 1}},                                // sRGB red
		{"oklch(45.2% 0.313 264.05)", rgba{0, 0, 1, 1}},                                // sRGB blue
		{"oklch(86.06% 0.1731 91.936)", rgba{250 / 255.0, 204 / 255.0, 21 / 255.0, 1}}, // #facc15
		{"oklch(0.7 0 0)", rgba{0.6205, 0.6205, 0.6205, 1}},                            // the achromatic axis is grey
	}
	for _, tc := range tests {
		got, ok := parseColor(tc.in)
		if !ok {
			t.Errorf("parseColor(%q) = not a colour", tc.in)
			continue
		}
		if !closeColor(got, tc.want, 0.01) {
			t.Errorf("parseColor(%q) = %s, want %s", tc.in, formatColor(got), formatColor(tc.want))
		}
	}
	// The warm accent sits past the sRGB gamut at its lightness: its channels
	// are clamped to what the screen can show.
	got, ok := parseColor("oklch(82.8% 0.189 84.429)")
	if !ok {
		t.Fatal("the warm accent does not parse")
	}
	for _, v := range []float64{got.r, got.g, got.b} {
		if v < 0 || v > 1 {
			t.Errorf("out-of-gamut channel %v was not clamped: %s", v, formatColor(got))
		}
	}
}

// Translucent colours must be composited before they are measured: raised sits
// on the ground, and a foreground sits on the result.
func TestCompositingOverGroundAndRaised(t *testing.T) {
	ground, ok := parseColor("#130f0a")
	if !ok {
		t.Fatal("warm ground does not parse")
	}
	raised, ok := parseColor("rgba(255, 255, 255, 0.06)")
	if !ok {
		t.Fatal("warm raised does not parse")
	}
	if got := raised.over(ground); !closeColor(got, rgba{33 / 255.0, 29 / 255.0, 25 / 255.0, 1}, 1.0/255) {
		t.Errorf("raised over ground = %s, want the composited #211d19", formatColor(got))
	}
	clear := rgba{r: 1, g: 0, b: 0, a: 0}
	if got := clear.over(ground); !closeColor(got, ground, 1e-9) {
		t.Errorf("transparent over ground = %s, want the ground %s", formatColor(got), formatColor(ground))
	}
	white, _ := parseColor("#ffffff")
	if got := white.over(rgba{r: 0.5, g: 0.5, b: 0.5, a: 1}); !closeColor(got, white, 1e-9) {
		t.Errorf("opaque white over grey = %s, want white", formatColor(got))
	}
}

func TestContrastKnownPairs(t *testing.T) {
	white, _ := parseColor("#ffffff")
	blackColor, _ := parseColor("#000000")
	if got := contrast(white, blackColor); math.Abs(got-21) > 0.01 {
		t.Errorf("contrast(white, black) = %.2f, want 21", got)
	}
	tests := []struct {
		in   string
		want float64
	}{
		{"#767676", 4.54},
		{"#777777", 4.48},
	}
	for _, tc := range tests {
		c, ok := parseColor(tc.in)
		if !ok {
			t.Fatalf("parseColor(%q) = not a colour", tc.in)
		}
		if got := contrast(c, white); math.Abs(got-tc.want) > 0.01 {
			t.Errorf("contrast(%q on white) = %.2f, want %.2f", tc.in, got, tc.want)
		}
	}
	if got := contrast(blackColor, blackColor); got != 1 {
		t.Errorf("contrast of a colour with itself = %.2f, want 1", got)
	}
}

// themeSource builds a theme.toml that sets every colour token of the built-in
// default, then the overrides, so a test names only the token it is about.
// A case that lightens the ground has to set text and text-body dark too: the
// pair is what must pass, and an inherited light text on a light ground would
// cost the theme its ground before the pair under test is reached.
func themeSource(t *testing.T, overrides map[string]string) string {
	t.Helper()
	tokens := Resolve("").Tokens
	var b strings.Builder
	b.WriteString("[theme]\nid = \"test\"\nschema = 1\nsupports = [\"tokens\"]\n\n[tokens]\n")
	for _, name := range colorTokens {
		v, ok := overrides[name]
		if !ok {
			v = tokens[name]
		}
		b.WriteString(name + " = " + strconv.Quote(v) + "\n")
	}
	return b.String()
}

// lightBase is a coherent light theme: the built-in tokens flipped so the
// pairs a case isn't about still pass on a white ground. A case then moves
// only the pair it tests, since the check drops whatever pair it can't fix.
func lightBase(overrides map[string]string) map[string]string {
	out := map[string]string{
		"ground": "#ffffff", "raised": "#ffffff",
		"text": "#000000", "text-body": "#000000",
		"text-muted": "#000000", "text-faint": "#000000", "focus": "#000000",
	}
	maps.Copy(out, overrides)
	return out
}

// contrastWarning returns the warning for one pair, "" when the pair passed.
func contrastWarning(th Theme, fg, bg string) string {
	prefix := "token " + fg + " on " + bg + ":"
	for _, w := range th.Info.Warnings {
		if strings.HasPrefix(w, prefix) {
			return w
		}
	}
	return ""
}

// Each threshold has a known passing and a known failing pair.
func TestThresholdsPerPair(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		fg, bg    string
		fails     bool
	}{
		{"text at 4.54:1", lightBase(map[string]string{"text": "#767676"}), "text", "ground", false},
		{"text at 4.48:1", lightBase(map[string]string{"text": "#787878"}), "text", "ground", true},
		{"body at 4.48:1", lightBase(map[string]string{"text-body": "#787878"}), "text-body", "ground", true},
		{"muted at 3.03:1", lightBase(map[string]string{"text-muted": "#949494"}), "text-muted", "ground", false},
		{"muted at 2.92:1", lightBase(map[string]string{"text-muted": "#979797"}), "text-muted", "ground", true},
		{"faint at 2.92:1", lightBase(map[string]string{"text-faint": "#979797"}), "text-faint", "ground", true},
		{"on-accent at 4.54:1", map[string]string{"accent": "#767676", "on-accent": "#ffffff"}, "on-accent", "accent", false},
		{"on-accent at 4.48:1", map[string]string{"accent": "#787878", "on-accent": "#ffffff"}, "on-accent", "accent", true},
		{"focus at 3.03:1", lightBase(map[string]string{"focus": "#949494"}), "focus", "ground", false},
		{"focus at 2.92:1", lightBase(map[string]string{"focus": "#979797"}), "focus", "ground", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th := parse(t, themeSource(t, tc.overrides))
			got := contrastWarning(th, tc.fg, tc.bg)
			if tc.fails && got == "" {
				t.Fatalf("%s passed, want a warning (warnings %v)", tc.name, th.Info.Warnings)
			}
			if !tc.fails && got != "" {
				t.Fatalf("%s failed: %q", tc.name, got)
			}
		})
	}
}

// A failing token is dropped like any other invalid one, and the warning says
// which two colours and which ratio.
func TestFailedTokenFallsBackWithItsWarning(t *testing.T) {
	src := themeSource(t, map[string]string{"text": "#1a1a1a"})
	th := parse(t, src)
	if _, set := th.Tokens["text"]; set {
		t.Fatalf("text kept its failing value %q", th.Tokens["text"])
	}
	warn := contrastWarning(th, "text", "ground")
	for _, want := range []string{"#1a1a1a", "#130f0a", "needs 4.5:1", "using the built-in value"} {
		if !strings.Contains(warn, want) {
			t.Errorf("warning %q does not mention %q", warn, want)
		}
	}
	res, _ := resolveSrc(t, src)
	if got, want := res.Tokens["text"], Resolve("").Tokens["text"]; got != want {
		t.Errorf("resolved text = %q, want the built-in %q", got, want)
	}
	if res.Tokens["ground"] != Resolve("").Tokens["ground"] {
		t.Errorf("ground = %q, want an untouched token", res.Tokens["ground"])
	}
}

// A theme that only moves the ground still has the text it inherits checked:
// the pair is what must be readable, not the token's own value.
func TestInheritedForegroundIsCheckedAgainstANewGround(t *testing.T) {
	th := parse(t, themeSource(t, map[string]string{"ground": "#ffffff"}))
	if contrastWarning(th, "text", "ground") == "" {
		t.Fatalf("light ground kept the inherited white text (warnings %v)", th.Info.Warnings)
	}
}

// A theme that only moves a background must not keep a failing pair: the
// inherited foreground has nothing to drop, so the background it moved falls
// back with it (issue #155).
func TestThemeThatOnlyMovesABackgroundFallsBackWithIt(t *testing.T) {
	// The built-in on-accent is dark: a dark accent leaves it unreadable.
	src := strings.Replace(minimal, `accent = "#7dd3fc"`, `accent = "#130f0a"`, 1)
	th := parse(t, src)
	if contrastWarning(th, "on-accent", "accent") == "" {
		t.Fatalf("a dark accent kept the inherited dark on-accent (warnings %v)", th.Info.Warnings)
	}
	got, _ := resolveSrc(t, src)
	if want := Resolve("").Tokens["accent"]; got.Tokens["accent"] != want {
		t.Errorf("resolved accent = %q, want the built-in %q", got.Tokens["accent"], want)
	}
}

// The same over a scheme: dropping the inherited scheme foreground alone
// would leave the scheme accent the theme set, so it falls back too.
func TestSchemeThatOnlyMovesABackgroundFallsBackWithIt(t *testing.T) {
	src := minimal + "\n[schemes.light]\naccent = \"#130f0a\"\n"
	th := parse(t, src)
	if !warningsContain(th, "token on-accent in scheme light on accent") {
		t.Fatalf("a dark light-scheme accent kept the inherited dark on-accent (warnings %v)", th.Info.Warnings)
	}
	if v, kept := th.Schemes["light"]["accent"]; kept {
		t.Errorf("light scheme kept accent %q, want the dark value", v)
	}
}

// Raised is measured composited over the ground, so a translucent raised counts.
func TestTranslucentRaisedIsComposited(t *testing.T) {
	th := parse(t, themeSource(t, map[string]string{"text": "#ffffff", "raised": "rgba(255, 255, 255, 0.85)"}))
	if got := contrastWarning(th, "text", "raised"); got == "" {
		t.Fatalf("white on a near-white raised passed (warnings %v)", th.Info.Warnings)
	}
	if got := contrastWarning(th, "text", "ground"); got != "" {
		t.Fatalf("the same text failed on the dark ground: %q", got)
	}
}

// The built-in themes must clear every threshold with no warning. This is the
// report: it prints each ratio and names the token when one fails.
func TestBuiltinThemesMeetContrast(t *testing.T) {
	for _, th := range All() {
		tokens := Resolve(th.Info.ID).Tokens
		for _, p := range contrastPairs {
			bg, ok := backdrop(tokens, p.bg)
			if !ok {
				t.Errorf("%s: %s does not parse", th.Info.ID, tokens[p.bg])
				continue
			}
			fg, ok := parseColor(tokens[p.fg])
			if !ok {
				t.Errorf("%s: %s does not parse", th.Info.ID, tokens[p.fg])
				continue
			}
			got := contrast(fg.over(bg), bg)
			t.Logf("%s: %s on %s = %.2f:1 (needs %.1f:1)", th.Info.ID, p.fg, p.bg, got, p.min)
			if got < p.min {
				t.Errorf("%s: %s %q on %s %q is %.2f:1, needs %.1f:1",
					th.Info.ID, p.fg, tokens[p.fg], p.bg, tokens[p.bg], got, p.min)
			}
		}
		if len(th.Info.Warnings) != 0 {
			t.Errorf("%s: warnings %v, want none", th.Info.ID, th.Info.Warnings)
		}
	}
}
