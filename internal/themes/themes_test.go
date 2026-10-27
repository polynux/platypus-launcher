package themes

import (
	"slices"
	"strings"
	"testing"

	"platypus/internal/types"
)

// minimal is a valid theme that sets one token, so tests can add lines to it.
// Its accent has to be light enough for the on-accent it inherits, or the
// contrast check would warn.
const minimal = `
[theme]
id = "test"
name = "Test"
schema = 1
supports = ["tokens"]

[tokens]
accent = "#7dd3fc"
`

func parse(t *testing.T, src string) Theme {
	t.Helper()
	return Parse([]byte(src))
}

func warningsContain(th Theme, sub string) bool {
	for _, w := range th.Info.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestBuiltinThemesLoad(t *testing.T) {
	all := All()
	if len(all) < 2 {
		t.Fatalf("All() = %d themes, want the built-in warm and cool", len(all))
	}
	if all[0].Info.ID != DefaultID {
		t.Fatalf("first theme = %q, want the default %q", all[0].Info.ID, DefaultID)
	}
	for _, th := range all {
		if th.Info.Status != StatusOK {
			t.Errorf("%s: status %q, want ok (warnings %v)", th.Info.ID, th.Info.Status, th.Info.Warnings)
		}
		if len(th.Info.Warnings) != 0 {
			t.Errorf("%s: built-in theme has warnings %v", th.Info.ID, th.Info.Warnings)
		}
	}
	if _, ok := Find("cool"); !ok {
		t.Fatal("Find(cool) = false, want the cool theme")
	}
}

// The default must set every token and the wash: Resolve fills gaps from it.
func TestDefaultSetsEveryToken(t *testing.T) {
	def, ok := Find(DefaultID)
	if !ok {
		t.Fatal("no default theme")
	}
	for _, name := range allTokens {
		if def.Tokens[name] == "" {
			t.Errorf("default theme does not set %s", name)
		}
	}
	if len(def.Wash) == 0 {
		t.Error("default theme has no wash")
	}
}

func TestResolveDefaultsAndPicked(t *testing.T) {
	def := Resolve("")
	if def.ID != DefaultID || def.Tokens["accent"] != "oklch(82.8% 0.189 84.429)" {
		t.Fatalf("Resolve(\"\") = %s %q, want the warm default", def.ID, def.Tokens["accent"])
	}
	if got := Resolve("missing"); got.ID != DefaultID {
		t.Fatalf("Resolve(missing) = %s, want the default", got.ID)
	}
	cool := Resolve("cool")
	if cool.ID != "cool" || cool.Tokens["ground"] != "#0b1118" || len(cool.Wash) != 3 {
		t.Fatalf("Resolve(cool) = %+v", cool)
	}
	if cool.Variants["settings.presentation"] != "sheet" || cool.Variants["settings.navigation"] != "tabs" || cool.Variants["section.chrome"] != "framed" {
		t.Fatalf("cool variants = %v", cool.Variants)
	}
	if cool.Variants["dialog.presentation"] != "dialog" || cool.Variants["dialog.size"] != "auto" {
		t.Fatalf("cool dialog variants = %v", cool.Variants)
	}
	if cool.Variants["game-settings.presentation"] != "sheet" || cool.Variants["game-settings.tabs"] != "horizontal" {
		t.Fatalf("cool game-settings variants = %v", cool.Variants)
	}
	if def.Variants["settings.presentation"] != "page" || def.Variants["settings.navigation"] != "scroll" || def.Variants["section.chrome"] != "hairline" {
		t.Fatalf("warm variants = %v", def.Variants)
	}
	if def.Variants["dialog.presentation"] != "dialog" || def.Variants["dialog.size"] != "auto" {
		t.Fatalf("warm dialog variants = %v", def.Variants)
	}
	if def.Variants["game-settings.presentation"] != "dialog" || def.Variants["game-settings.tabs"] != "vertical" {
		t.Fatalf("warm game-settings variants = %v", def.Variants)
	}
	if cool.Variants["game-tile.aspect"] != "16:9" || cool.Variants["game-tile.compat"] != "off" || cool.Variants["game-tile.overlay"] != "off" || cool.Variants["game-tile.density"] != "compact" {
		t.Fatalf("cool game-tile variants = %v", cool.Variants)
	}
	if cool.Variants["game-row.columns"] != "minimal" || cool.Variants["game-row.mode"] != "compact" {
		t.Fatalf("cool game-row variants = %v", cool.Variants)
	}
	if def.Variants["game-tile.aspect"] != "3:4" || def.Variants["game-tile.compat"] != "on" || def.Variants["game-tile.overlay"] != "on" || def.Variants["game-tile.density"] != "comfortable" {
		t.Fatalf("warm game-tile variants = %v", def.Variants)
	}
	if def.Variants["game-row.columns"] != "full" || def.Variants["game-row.mode"] != "full" {
		t.Fatalf("warm game-row variants = %v", def.Variants)
	}
}

func TestVariantsWarnAndFallBack(t *testing.T) {
	src := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["tokens", "variants"]`, 1) + `
[variants]
"settings.presentation" = "drawer"
"settings.navigation" = "tabs"
"unknown.axis" = "x"
`
	th := parse(t, src)
	if !warningsContain(th, "unknown value") || !warningsContain(th, "unknown variant axis") {
		t.Fatalf("warnings = %v", th.Info.Warnings)
	}
	got := resolve(th, true)
	if got.Variants["settings.presentation"] != "page" || got.Variants["settings.navigation"] != "tabs" {
		t.Fatalf("variants = %v", got.Variants)
	}
}

// The game hero follows the global hero choice unless the theme says
// otherwise for the game itself (issue #101, like game-settings follows
// dialog): warm keeps the dashboard's tall glass banner and the game page's
// regular solid one from the defaults alone.
func TestGameHeroFollowsHero(t *testing.T) {
	varianted := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["tokens", "variants"]`, 1)
	builtin, _ := resolveSrc(t, varianted)
	if builtin.Variants["hero.height"] != "tall" || builtin.Variants["hero.chips"] != "glass" {
		t.Fatalf("hero variants = %v, want the dashboard defaults", builtin.Variants)
	}
	if builtin.Variants["game-hero.height"] != "regular" || builtin.Variants["game-hero.chips"] != "solid" {
		t.Fatalf("game-hero variants = %v, want the game page defaults", builtin.Variants)
	}
	globalOnly, _ := resolveSrc(t, varianted+`
[variants]
"hero.height" = "compact"
"hero.gradient" = "neutral"
`)
	if globalOnly.Variants["game-hero.height"] != "compact" || globalOnly.Variants["game-hero.gradient"] != "neutral" {
		t.Fatalf("game-hero variants = %v, want the global hero choice", globalOnly.Variants)
	}
	useSite, _ := resolveSrc(t, varianted+`
[variants]
"hero.height" = "compact"
"game-hero.height" = "regular"
`)
	if useSite.Variants["hero.height"] != "compact" || useSite.Variants["game-hero.height"] != "regular" {
		t.Fatalf("hero variants = %v, want the use-site override to win", useSite.Variants)
	}
}

// A use-site presentation wins over the global dialog one, which wins over
// the built-in (ADR §2): game settings follow dialog.presentation unless the
// theme says otherwise for game-settings itself.
func TestGameSettingsPresentationFallsBackToDialog(t *testing.T) {
	varianted := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["tokens", "variants"]`, 1)
	globalOnly, _ := resolveSrc(t, varianted+`
[variants]
"dialog.presentation" = "sheet"
`)
	if globalOnly.Variants["dialog.presentation"] != "sheet" {
		t.Fatalf("dialog presentation = %q, want sheet", globalOnly.Variants["dialog.presentation"])
	}
	if globalOnly.Variants["game-settings.presentation"] != "sheet" {
		t.Fatalf("game-settings presentation = %q, want the global sheet", globalOnly.Variants["game-settings.presentation"])
	}
	useSite, _ := resolveSrc(t, varianted+`
[variants]
"dialog.presentation" = "sheet"
"game-settings.presentation" = "dialog"
"game-settings.tabs" = "horizontal"
`)
	if useSite.Variants["game-settings.presentation"] != "dialog" || useSite.Variants["game-settings.tabs"] != "horizontal" {
		t.Fatalf("game-settings variants = %v, want the use-site values", useSite.Variants)
	}
	if useSite.Variants["dialog.presentation"] != "sheet" {
		t.Fatalf("dialog presentation = %q, want sheet", useSite.Variants["dialog.presentation"])
	}
	builtin, _ := resolveSrc(t, varianted)
	if builtin.Variants["game-settings.presentation"] != "dialog" || builtin.Variants["game-settings.tabs"] != "vertical" {
		t.Fatalf("game-settings variants = %v, want the built-in values", builtin.Variants)
	}
	if builtin.Variants["dialog.size"] != "auto" {
		t.Fatalf("dialog size = %q, want the built-in auto", builtin.Variants["dialog.size"])
	}
}

func TestValidThemeResolvesItsTokens(t *testing.T) {
	th := parse(t, minimal)
	if th.Info.Status != StatusOK || len(th.Info.Warnings) != 0 {
		t.Fatalf("status %q warnings %v, want ok and none", th.Info.Status, th.Info.Warnings)
	}
	if th.Info.Name != "Test" || th.Info.Schema != 1 || !slices.Equal(th.Info.Supports, []string{"tokens"}) {
		t.Fatalf("info = %+v", th.Info)
	}
	if th.Tokens["accent"] != "#7dd3fc" {
		t.Fatalf("accent = %q", th.Tokens["accent"])
	}
}

func TestMissingCatalogVersionAssumesOneWithoutWarning(t *testing.T) {
	th := parse(t, minimal)
	if th.Info.Status != StatusOK || len(th.Info.Warnings) != 0 {
		t.Fatalf("status %q warnings %v, want ok and no warning", th.Info.Status, th.Info.Warnings)
	}
}

func TestNewerMajorCatalogNeedsNewerPlatypus(t *testing.T) {
	src := strings.Replace(minimal, "schema = 1", "schema = 1\ncatalog = \"2.0\"", 1)
	th := parse(t, src)
	if th.Info.Status != StatusNewer || !warningsContain(th, "needs catalog 2.0") {
		t.Fatalf("status %q warnings %v, want newer catalog status", th.Info.Status, th.Info.Warnings)
	}
	if got := resolve(th, true); got.ID != DefaultID {
		t.Fatalf("newer catalog theme resolved as %q, want default", got.ID)
	}
}

func TestNewerMinorCatalogIsUsableAndUnknownVariantsFallBack(t *testing.T) {
	src := strings.Replace(minimal, "schema = 1", "schema = 1\ncatalog = \"1.1\"", 1)
	src = strings.Replace(src, `supports = ["tokens"]`, `supports = ["tokens", "variants"]`, 1) + `
[variants]
"settings.navigation" = "tabs"
"settings.presentation" = "drawer"
`
	th := parse(t, src)
	if th.Info.Status != StatusOK || !warningsContain(th, "unknown value") {
		t.Fatalf("status %q warnings %v, want usable theme and unknown-value warning", th.Info.Status, th.Info.Warnings)
	}
	got := resolve(th, true)
	if got.Variants["settings.navigation"] != "tabs" || got.Variants["settings.presentation"] != "page" {
		t.Fatalf("variants = %v, want known 1.0 value applied and unknown value defaulted", got.Variants)
	}
}

func TestUnknownKeysAreWarnings(t *testing.T) {
	th := parse(t, "extra = \"x\"\n"+minimal+"\n[theme.more]\nx = 1\n")
	if th.Info.Status != StatusOK {
		t.Fatalf("status %q, want ok: unknown keys don't break a theme", th.Info.Status)
	}
	if !warningsContain(th, "unknown key extra") {
		t.Fatalf("warnings %v, want the unknown top-level key", th.Info.Warnings)
	}
}

func TestUnknownTokenIsWarning(t *testing.T) {
	th := parse(t, minimal+`glow = "#ffffff"`+"\n")
	if !warningsContain(th, `unknown token "glow"`) || th.Info.Status != StatusOK {
		t.Fatalf("status %q warnings %v", th.Info.Status, th.Info.Warnings)
	}
}

// A bad token keeps the built-in value, and only that token.
func TestBadTokenFallsBackPerToken(t *testing.T) {
	src := `
[theme]
id = "test"
schema = 1
supports = ["tokens"]

[tokens]
accent = "not-a-colour"
ground = "#010203"
radius-card = "10em"
font-sans = '"Comic Sans", sans-serif'
`
	th := parse(t, src)
	if th.Info.Status != StatusOK {
		t.Fatalf("status %q, want ok", th.Info.Status)
	}
	for _, name := range []string{"accent", "radius-card", "font-sans"} {
		if _, set := th.Tokens[name]; set {
			t.Errorf("%s was accepted, want it dropped", name)
		}
		if !warningsContain(th, name) {
			t.Errorf("no warning for %s: %v", name, th.Info.Warnings)
		}
	}
	res, _ := resolveSrc(t, src)
	def := Resolve("")
	if res.Tokens["accent"] != def.Tokens["accent"] || res.Tokens["radius-card"] != def.Tokens["radius-card"] {
		t.Fatalf("bad tokens did not keep the built-in values: %q %q", res.Tokens["accent"], res.Tokens["radius-card"])
	}
	if res.Tokens["ground"] != "#010203" {
		t.Fatalf("ground = %q, want the valid value kept", res.Tokens["ground"])
	}
}

func TestTokenValueGrammar(t *testing.T) {
	good := map[string]string{
		"ground": "#abc", "raised": "#aabbcc", "surface": "rgba(255, 255, 255, 0.06)",
		"text": "rgb(10 20 30 / 50%)", "info": "hsl(200deg 80% 60%)", "dl": "oklch(0.8 0.1 280)",
		"ok": "transparent", "danger": "#aabbccdd",
	}
	for name, v := range good {
		if _, err := checkToken(name, v); err != nil {
			t.Errorf("%s = %q rejected: %v", name, v, err)
		}
	}
	bad := map[string]string{
		"ground": "url(evil.png)", "raised": "red;background:url(x)", "surface": "rgb(0,0,0) }",
		"text": "var(--x)", "info": "calc(1px)", "dl": "rgb(0, 0, 0", "ok": "{}", "attention": "oklch(0.5 0.1 h)",
	}
	for name, v := range bad {
		if _, err := checkToken(name, v); err == nil {
			t.Errorf("%s = %q accepted, want it rejected", name, v)
		}
	}
	for _, v := range []string{"10px", "0.75rem"} {
		if _, err := checkToken("radius-chip", v); err != nil {
			t.Errorf("radius %q rejected: %v", v, err)
		}
	}
	for _, v := range []string{"2.5REM", "10em", "calc(1rem)"} {
		if _, err := checkToken("radius-chip", v); err == nil {
			t.Errorf("radius %q accepted, want it rejected", v)
		}
	}
}

func TestFontStacks(t *testing.T) {
	got, err := fontStack(`"JetBrains Mono" , monospace`)
	if err != nil || got != `"JetBrains Mono", monospace` {
		t.Fatalf("fontStack = %q, %v", got, err)
	}
	for _, v := range []string{`"Comic Sans"`, `Hanken Grotesk`, `url(font.woff2)`, ``} {
		if _, err := fontStack(v); err == nil {
			t.Errorf("fontStack(%q) accepted", v)
		}
	}
}

func TestForbiddenCharactersRejectedWholeToken(t *testing.T) {
	th := parse(t, minimal+`ground = "#000000; background: url(https://x)"`+"\n")
	if _, set := th.Tokens["ground"]; set || !warningsContain(th, "ground") {
		t.Fatalf("ground accepted: %v", th.Info.Warnings)
	}
}

func TestWrongTypeIsWarning(t *testing.T) {
	th := parse(t, minimal+"ground = 12\n")
	if _, set := th.Tokens["ground"]; set || !warningsContain(th, "must be a string") {
		t.Fatalf("ground = %v, warnings %v", th.Tokens["ground"], th.Info.Warnings)
	}
}

func TestNewerSchemaIsListedNotPicked(t *testing.T) {
	th := parse(t, strings.Replace(minimal, "schema = 1", "schema = 2", 1))
	if th.Info.Status != StatusNewer || !warningsContain(th, "needs schema 2") {
		t.Fatalf("status %q warnings %v", th.Info.Status, th.Info.Warnings)
	}
	if len(th.Tokens) != 0 {
		t.Fatalf("a newer theme's tokens were read: %v", th.Tokens)
	}
}

func TestInvalidThemes(t *testing.T) {
	cases := map[string]string{
		"garbage":     "[theme\nid = ",
		"no id":       "[theme]\nschema = 1\n",
		"bad id":      "[theme]\nid = \"Bad Id\"\nschema = 1\n",
		"no schema":   "[theme]\nid = \"x\"\n",
		"wrong shape": "[theme]\nid = \"x\"\nschema = 1\nsupports = \"tokens\"\n",
	}
	for name, src := range cases {
		th := Parse([]byte(src))
		if th.Info.Status != StatusInvalid || len(th.Info.Warnings) == 0 {
			t.Errorf("%s: status %q warnings %v, want invalid with a reason", name, th.Info.Status, th.Info.Warnings)
		}
	}
}

// resolveSrc resolves a theme given as source, as if it were picked.
func resolveSrc(t *testing.T, src string) (types.ThemeTokens, Theme) {
	t.Helper()
	th := parse(t, src)
	return resolve(th, true), th
}

func TestNewerSchemaFallsBackToDefault(t *testing.T) {
	src := strings.Replace(minimal, "schema = 1", "schema = 2", 1)
	if got, _ := resolveSrc(t, src); got.ID != DefaultID || got.Tokens["accent"] != "oklch(82.8% 0.189 84.429)" {
		t.Fatalf("a newer theme was applied: %s %q", got.ID, got.Tokens["accent"])
	}
}

func TestUnsupportedTokensLevelKeepsDefault(t *testing.T) {
	src := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["layouts"]`, 1)
	got, _ := resolveSrc(t, src)
	if got.ID != "test" || got.Tokens["accent"] != "oklch(82.8% 0.189 84.429)" {
		t.Fatalf("tokens applied without the tokens level: %s %q", got.ID, got.Tokens["accent"])
	}
}

// Each level applies on its own (ADR §2): a theme that only declares variants
// keeps its id and its variants, and only its tokens fall back (issue #155).
func TestVariantsOnlyThemeKeepsItsIdAndVariants(t *testing.T) {
	src := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["variants"]`, 1) + `
[variants]
"settings.navigation" = "tabs"
`
	got, th := resolveSrc(t, src)
	if got.ID != th.Info.ID {
		t.Fatalf("resolve kept %q, want the theme id %q", got.ID, th.Info.ID)
	}
	if got.Variants["settings.navigation"] != "tabs" {
		t.Fatalf("variants = %v, want the theme's tabs", got.Variants)
	}
	if got.Tokens["accent"] != Resolve("").Tokens["accent"] {
		t.Errorf("accent = %q, want the built-in value without the tokens level", got.Tokens["accent"])
	}
}

func TestWashLayers(t *testing.T) {
	layer := func(color string) string {
		return "\n[[wash]]\nat = \"10% 20%\"\nsize = \"500px 300px\"\ncolor = \"" + color + "\"\nfade = \"50%\"\n"
	}
	ok, _ := resolveSrc(t, minimal+layer("rgba(1, 2, 3, 0.5)"))
	if len(ok.Wash) != 1 || ok.Wash[0].Color != "rgba(1, 2, 3, 0.5)" || ok.Wash[0].At != "10% 20%" {
		t.Fatalf("wash = %+v", ok.Wash)
	}
	def := Resolve("")
	bad, th := resolveSrc(t, minimal+layer("url(x)"))
	if !warningsContain(th, "built-in wash") || !slices.Equal(washColors(bad), washColors(def)) {
		t.Fatalf("bad wash reached the UI: %+v (warnings %v)", bad.Wash, th.Info.Warnings)
	}
	tooMany, th := resolveSrc(t, minimal+strings.Repeat(layer("#000"), maxWashLayers+1))
	if !warningsContain(th, "at most") || !slices.Equal(washColors(tooMany), washColors(def)) {
		t.Fatalf("too many layers reached the UI: %+v", tooMany.Wash)
	}
}

func washColors(t types.ThemeTokens) []string {
	var out []string
	for _, l := range t.Wash {
		out = append(out, l.Color)
	}
	return out
}

// Unknown supports values warn and are ignored (issue #110 item 1): a theme
// naming "bogus" still loads, but only the known levels apply.
func TestUnknownSupportsValueWarns(t *testing.T) {
	src := strings.Replace(minimal, `supports = ["tokens"]`, `supports = ["tokens", "bogus"]`, 1)
	th := parse(t, src)
	if !warningsContain(th, `unknown supports value "bogus"`) {
		t.Fatalf("warnings = %v, want the unknown-supports warning", th.Info.Warnings)
	}
	if !slices.Equal(th.Info.Supports, []string{"tokens"}) {
		t.Fatalf("supports = %v, want only the known levels", th.Info.Supports)
	}
}
