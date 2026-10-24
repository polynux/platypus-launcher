package themes

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"platypus/internal/types"
)

// withCatalog runs t with a temporary catalog, for layouts no built-in view
// exercises (required-block restoration has no dashboard case: none of the
// dashboard blocks is required).
func withCatalog(t *testing.T, entries []types.ThemeCatalogEntry) {
	t.Helper()
	old := Catalog
	Catalog = entries
	t.Cleanup(func() { Catalog = old })
}

func dashboardTestCatalog() []types.ThemeCatalogEntry {
	return append([]types.ThemeCatalogEntry{}, Catalog...)
}

func layoutWarnings(layout *types.ThemeLayout, warnings []string) string {
	return strings.Join(warnings, "\n")
}

func TestParseDashboardLayout(t *testing.T) {
	src := `
<layout view="dashboard" catalog="1.0">
  <region name="main">
    <block name="hero" />
    <row>
      <block name="continue" />
      <block name="waiting" />
    </row>
    <block name="recent" />
  </region>
  <region name="side">
    <block name="activity" limit="6" />
  </region>
</layout>`
	layout, warnings := parseLayout("dashboard", []byte(src))
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if layout == nil || layout.View != "dashboard" || len(layout.Regions) != 2 {
		t.Fatalf("layout = %+v", layout)
	}
	main := layout.Regions[0]
	if main.Name != "main" || len(main.Nodes) != 3 {
		t.Fatalf("main = %+v", main)
	}
	if main.Nodes[0].Block != "hero" || main.Nodes[0].Row != nil {
		t.Fatalf("node 0 = %+v, want the hero block", main.Nodes[0])
	}
	row := main.Nodes[1]
	if len(row.Row) != 2 || row.Row[0].Block != "continue" || row.Row[1].Block != "waiting" {
		t.Fatalf("row = %+v, want continue and waiting side by side", row)
	}
	if main.Nodes[2].Block != "recent" {
		t.Fatalf("node 2 = %+v, want the recent block", main.Nodes[2])
	}
	if got := layout.Regions[1].Nodes[0].Attrs["limit"]; got != "6" {
		t.Fatalf("activity limit = %q, want 6", got)
	}
}

// Unknown regions, blocks, attributes and elements are ignored with a
// warning: a theme from a newer catalog never breaks the app.
func TestParseLayoutUnknownNamesAreWarnings(t *testing.T) {
	src := `
<layout view="dashboard" catalog="1.0" mood="dark">
  <region name="main">
    <block name="continue" size="big" />
    <block name="ticker" />
    <banner />
    <row>
      <block name="waiting" />
      <row>
        <block name="waiting" />
      </row>
    </row>
  </region>
  <region name="attic">
    <block name="continue" />
  </region>
  <shelf />
</layout>`
	layout, warnings := parseLayout("dashboard", []byte(src))
	if layout == nil {
		t.Fatalf("warnings = %v, want a usable layout", warnings)
	}
	for _, want := range []string{
		`unknown layout attribute "mood"`,
		`unknown attribute "size" on block "continue"`,
		`unknown block "ticker"`,
		`unknown layout element <banner>`,
		`unknown layout element <row> in a row`,
		`unknown region "attic"`,
		`unknown layout element <shelf>`,
	} {
		found := false
		for _, w := range warnings {
			if strings.Contains(w, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no warning containing %q in %v", want, warnings)
		}
	}
	main := layout.Regions[0]
	if len(main.Nodes) != 2 || main.Nodes[0].Block != "continue" || len(main.Nodes[1].Row) != 1 {
		t.Fatalf("main = %+v, want continue and a one-block row", main)
	}
	if main.Nodes[0].Attrs != nil {
		t.Fatalf("bad attribute reached the UI: %+v", main.Nodes[0])
	}
}

// A layout that leaves out a required block still gets it, in its built-in
// region, with a warning.
func TestParseLayoutRestoresRequiredBlocks(t *testing.T) {
	withCatalog(t, append(dashboardTestCatalog(), types.ThemeCatalogEntry{
		Entry: "dashboard.ticker", Since: "1.0", Kind: "block", Required: true, Region: "side",
	}))
	layout, warnings := parseLayout("dashboard", []byte(`
<layout view="dashboard" catalog="1.0">
  <region name="main">
    <block name="continue" />
  </region>
</layout>`))
	if layout == nil {
		t.Fatalf("warnings = %v, want a layout with the block restored", warnings)
	}
	if !strings.Contains(layoutWarnings(layout, warnings), `leaves out required block "ticker"`) {
		t.Fatalf("warnings = %v, want the required-block warning", warnings)
	}
	var side *types.ThemeLayoutRegion
	for i := range layout.Regions {
		if layout.Regions[i].Name == "side" {
			side = &layout.Regions[i]
		}
	}
	if side == nil || len(side.Nodes) != 1 || side.Nodes[0].Block != "ticker" {
		t.Fatalf("layout = %+v, want ticker restored in a side region", layout)
	}
}

// A bad limit costs only the attribute; an unknown view or a newer catalog
// costs the whole layout.
func TestParseLayoutLimitsAndVersions(t *testing.T) {
	for _, bad := range []string{"0", "51", "many"} {
		layout, warnings := parseLayout("dashboard", []byte(
			`<layout view="dashboard"><region name="main"><block name="activity" limit="`+bad+`" /></region></layout>`))
		if layout == nil {
			t.Fatalf("limit %q: layout = nil, want the block without the attribute", bad)
		}
		if got := layout.Regions[0].Nodes[0].Attrs["limit"]; got != "" {
			t.Fatalf("limit %q reached the UI", bad)
		}
		if !strings.Contains(layoutWarnings(layout, warnings), "is not valid") {
			t.Fatalf("limit %q: warnings = %v, want the invalid-attribute warning", bad, warnings)
		}
	}
	if layout, warnings := parseLayout("attic", []byte(`<layout view="attic" />`)); layout != nil {
		t.Fatalf("unknown view: layout = %+v, want nil", layout)
	} else if !strings.Contains(strings.Join(warnings, "\n"), `unknown layout view "attic"`) {
		t.Fatalf("unknown view: warnings = %v", warnings)
	}
	if layout, warnings := parseLayout("dashboard", []byte(`<layout view="dashboard" catalog="2.0" />`)); layout != nil {
		t.Fatalf("newer catalog: layout = %+v, want nil", layout)
	} else if !strings.Contains(strings.Join(warnings, "\n"), "needs catalog 2.0") {
		t.Fatalf("newer catalog: warnings = %v", warnings)
	}
	if _, warnings := parseLayout("dashboard", []byte(`<layout view="dashboard"><region>`)); warnings == nil {
		t.Fatal("malformed XML parsed without a warning")
	}
}

// Every built-in layout parses with zero warnings; warm ships none and keeps
// the built-in arrangement.
func TestBuiltinLayouts(t *testing.T) {
	for _, th := range All() {
		if len(th.Info.Warnings) != 0 {
			t.Errorf("%s: warnings %v, want none", th.Info.ID, th.Info.Warnings)
		}
	}
	warm, ok := Find(DefaultID)
	if !ok {
		t.Fatal("no default theme")
	}
	if len(warm.Layouts) != 0 {
		t.Fatalf("warm ships layouts %v, want none", warm.Layouts)
	}
	cool, ok := Find("cool")
	if !ok {
		t.Fatal("no cool theme")
	}
	dashboard, ok := cool.Layouts["dashboard"]
	if !ok {
		t.Fatalf("cool ships layouts %v, want a dashboard layout", cool.Layouts)
	}
	if len(dashboard.Regions) != 2 || dashboard.Regions[1].Name != "side" {
		t.Fatalf("cool dashboard = %+v", dashboard)
	}
	if LayoutFor("warm", "dashboard") != nil {
		t.Fatal("warm resolves a layout, want the built-in arrangement")
	}
	if LayoutFor("cool", "dashboard") == nil {
		t.Fatal("cool resolves no layout, want its dashboard layout")
	}
	if LayoutFor("missing", "dashboard") != nil || LayoutFor("cool", "attic") != nil {
		t.Fatal("a missing theme or view resolved a layout")
	}
}

// The layout file a theme's folder holds but its XML can't name a view for is
// still reported, not silent.
func TestThemeLayoutFileWarnings(t *testing.T) {
	th := Parse([]byte(minimal))
	th.parseLayouts(func(name string) ([]byte, error) {
		return []byte(`<layout view="dashboard"><region name="attic" /></layout>`), nil
	}, []string{"dashboard.xml", "notes.txt"})
	if len(th.Layouts) != 1 {
		t.Fatalf("layouts = %+v", th.Layouts)
	}
	for _, want := range []string{`unknown region "attic"`, "is not an XML layout"} {
		found := false
		for _, w := range th.Info.Warnings {
			if strings.Contains(w, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no warning containing %q in %v", want, th.Info.Warnings)
		}
	}
}

// The XSD is generated from the catalog, so editor validation can't drift
// from what the core accepts.
func TestLayoutXSDCurrent(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate layout test")
	}
	path := filepath.Join(filepath.Dir(source), "layout.xsd")
	data, err := os.ReadFile(path)
	if err != nil && os.IsNotExist(err) && *updateCatalogSnapshot {
		if err := os.WriteFile(path, []byte(LayoutXSD()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read layout XSD: %v (refresh with go test ./internal/themes -run TestLayoutXSDCurrent -update)", err)
	}
	if got, want := LayoutXSD(), string(data); got != want {
		if *updateCatalogSnapshot {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		t.Fatal("layout.xsd is out of date; refresh with go test ./internal/themes -run TestLayoutXSDCurrent -update")
	}
	for _, want := range []string{`<xs:enumeration value="dashboard" />`, `<xs:enumeration value="side" />`, `<xs:enumeration value="activity" />`, `name="limit"`} {
		if !strings.Contains(LayoutXSD(), want) {
			t.Errorf("XSD misses %q", want)
		}
	}
}
