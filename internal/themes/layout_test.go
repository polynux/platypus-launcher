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

// Every theme parses with zero warnings; warm ships no layouts and keeps
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
	library, ok := cool.Layouts["library"]
	if !ok {
		t.Fatalf("cool ships layouts %v, want a library layout", cool.Layouts)
	}
	if len(library.Regions) != 2 || library.Regions[1].Name != "side" {
		t.Fatalf("cool library = %+v", library)
	}
	if LayoutFor("warm", "dashboard") == nil || LayoutFor("warm", "library") == nil {
		t.Fatal("warm resolves no layout, want the built-in arrangement")
	}
	if LayoutFor("cool", "dashboard") == nil || LayoutFor("cool", "library") == nil {
		t.Fatal("cool resolves no layout, want its dashboard and library layouts")
	}
	if LayoutFor("missing", "dashboard") == nil {
		t.Fatal("a missing theme resolved no layout, want the built-in arrangement")
	}
	if LayoutFor("cool", "attic") != nil {
		t.Fatal("an unknown view resolved a layout")
	}
}

// The built-in arrangements (issue #108) pass the same validation as theme
// layouts: they parse with zero warnings, and resolving a view with blocks
// never returns nothing.
func TestBuiltinLayoutsPassValidation(t *testing.T) {
	for _, view := range []string{"dashboard", "library"} {
		src, ok := builtinLayoutXML[view]
		if !ok {
			t.Fatalf("no built-in XML for view %q", view)
		}
		layout, warnings := parseLayout(view, []byte(src))
		if layout == nil || len(warnings) != 0 {
			t.Fatalf("built-in %s: layout = %+v, warnings = %v, want clean", view, layout, warnings)
		}
		got := BuiltinLayout(view)
		if got == nil || got.View != view || len(got.Regions) == 0 {
			t.Fatalf("BuiltinLayout(%q) = %+v", view, got)
		}
		for _, region := range got.Regions {
			if !layoutRegions(view)[region.Name] {
				t.Fatalf("built-in %s uses unknown region %q", view, region.Name)
			}
			for _, node := range region.Nodes {
				for _, child := range node.Row {
					if _, ok := layoutBlock(view, child.Block); !ok {
						t.Fatalf("built-in %s uses unknown block %q", view, child.Block)
					}
				}
				if node.Block != "" {
					if _, ok := layoutBlock(view, node.Block); !ok {
						t.Fatalf("built-in %s uses unknown block %q", view, node.Block)
					}
				}
			}
		}
	}
	if BuiltinLayout("attic") != nil {
		t.Fatal("an unknown view resolved a built-in layout")
	}
}

// The library layout (issue #98) orders the header blocks in the main column
// and moves the last-played strip to the side, with a tighter activity feed.
// Plugin warnings are required, so leaving them out restores them with a
// warning; a bad limit costs only the attribute.
func TestParseLibraryLayout(t *testing.T) {
	src := `
<layout view="library" catalog="1.0">
  <region name="main">
    <block name="title" />
    <block name="mode" />
    <block name="all-titles" />
    <block name="table" />
  </region>
  <region name="side">
    <block name="last-played" />
    <block name="activity" limit="4" />
    <block name="recent" />
  </region>
</layout>`
	layout, warnings := parseLayout("library", []byte(src))
	for _, w := range warnings {
		if !strings.Contains(w, "leaves out required block") {
			continue
		}
		// The warnings block is required: leaving it out warns and restores it.
		goto restored
	}
	t.Fatalf("warnings = %v, want the required-block warning for warnings", warnings)
restored:
	if layout == nil || len(layout.Regions) != 3 {
		t.Fatalf("layout = %+v, want main, side and the restored top regions", layout)
	}
	var top *types.ThemeLayoutRegion
	for i := range layout.Regions {
		if layout.Regions[i].Name == "top" {
			top = &layout.Regions[i]
		}
	}
	if top == nil {
		t.Fatalf("layout = %+v, want a top region", layout)
	}
	found := false
	for _, n := range top.Nodes {
		if n.Block == "warnings" {
			found = true
		}
	}
	if !found {
		t.Fatalf("top = %+v, want the warnings block restored", top)
	}
	var side *types.ThemeLayoutRegion
	for i := range layout.Regions {
		if layout.Regions[i].Name == "side" {
			side = &layout.Regions[i]
		}
	}
	if side == nil || len(side.Nodes) != 3 || side.Nodes[1].Attrs["limit"] != "4" {
		t.Fatalf("side = %+v, want last-played, activity limit 4 and recent", side)
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
		} else {
			t.Fatal("layout.xsd is out of date; refresh with go test ./internal/themes -run TestLayoutXSDCurrent -update")
		}
	}
	for _, want := range []string{`<xs:enumeration value="dashboard" />`, `<xs:enumeration value="side" />`, `<xs:enumeration value="activity" />`, `name="limit"`} {
		if !strings.Contains(LayoutXSD(), want) {
			t.Errorf("XSD misses %q", want)
		}
	}
	dir := filepath.Join(filepath.Dir(source), "layouts")
	for _, view := range LayoutXSDViews() {
		path := filepath.Join(dir, view+".xsd")
		data, err := os.ReadFile(path)
		if err != nil && os.IsNotExist(err) && *updateCatalogSnapshot {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(LayoutXSDForView(view)), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("read %s XSD: %v (refresh with go test ./internal/themes -run TestLayoutXSDCurrent -update)", view, err)
		}
		if got := LayoutXSDForView(view); got != string(data) {
			if *updateCatalogSnapshot {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			t.Fatalf("layouts/%s.xsd is out of date; refresh with go test ./internal/themes -run TestLayoutXSDCurrent -update", view)
		}
	}
}

// A layout that places the same block twice warns and keeps only the first
// (issue #110 item 2): the second placement is dropped, never merged.
func TestParseLayoutDuplicateBlocksWarn(t *testing.T) {
	src := `
<layout view="dashboard" catalog="1.0">
  <region name="main">
    <block name="continue" />
    <block name="continue" />
  </region>
  <region name="side">
    <row>
      <block name="waiting" />
      <block name="waiting" />
    </row>
  </region>
</layout>`
	layout, warnings := parseLayout("dashboard", []byte(src))
	if layout == nil {
		t.Fatalf("warnings = %v, want a usable layout", warnings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, `duplicate block "continue"`) || !strings.Contains(joined, `duplicate block "waiting"`) {
		t.Fatalf("warnings = %v, want duplicate-block warnings", warnings)
	}
	if len(layout.Regions[0].Nodes) != 1 || layout.Regions[0].Nodes[0].Block != "continue" {
		t.Fatalf("main = %+v, want a single continue block", layout.Regions[0])
	}
	if len(layout.Regions[1].Nodes) != 1 || len(layout.Regions[1].Nodes[0].Row) != 1 {
		t.Fatalf("side = %+v, want a one-block row", layout.Regions[1])
	}
}

// Typed block attributes come from the catalog (issue #110 item 4): int
// attributes enforce their min/max, enum attributes their values, and the
// canonical form is what reaches the UI.
func TestParseLayoutTypedBlockAttrs(t *testing.T) {
	withCatalog(t, []types.ThemeCatalogEntry{
		{Entry: "dashboard.main", Since: "1.0", Kind: "region"},
		{Entry: "dashboard.count", Since: "1.0", Kind: "block", Region: "main", Attrs: []types.ThemeCatalogAttr{{Name: "limit", Type: "int", Min: 2, Max: 5}}},
		{Entry: "dashboard.align", Since: "1.0", Kind: "block", Region: "main", Attrs: []types.ThemeCatalogAttr{{Name: "side", Type: "enum", Values: []string{"left", "right"}}}},
	})
	for _, bad := range []string{"1", "6", "many"} {
		layout, warnings := parseLayout("dashboard", []byte(
			`<layout view="dashboard"><region name="main"><block name="count" limit="`+bad+`" /></region></layout>`))
		if layout == nil {
			t.Fatalf("limit %q: layout = nil, want the block without the attribute", bad)
		}
		if got := layout.Regions[0].Nodes[0].Attrs["limit"]; got != "" {
			t.Fatalf("limit %q reached the UI", bad)
		}
		if !strings.Contains(strings.Join(warnings, "\n"), "is not valid") {
			t.Fatalf("limit %q: warnings = %v, want the invalid-attribute warning", bad, warnings)
		}
	}
	layout, warnings := parseLayout("dashboard", []byte(
		`<layout view="dashboard"><region name="main"><block name="count" limit=" 3 " /></region></layout>`))
	if len(warnings) != 0 || layout.Regions[0].Nodes[0].Attrs["limit"] != "3" {
		t.Fatalf("limit 3 with spaces: layout = %+v, warnings = %v, want canonical 3", layout, warnings)
	}
	layout, warnings = parseLayout("dashboard", []byte(
		`<layout view="dashboard"><region name="main"><block name="align" side="left" /></region></layout>`))
	if len(warnings) != 0 || layout.Regions[0].Nodes[0].Attrs["side"] != "left" {
		t.Fatalf("enum left: layout = %+v, warnings = %v", layout, warnings)
	}
	layout, warnings = parseLayout("dashboard", []byte(
		`<layout view="dashboard"><region name="main"><block name="align" side="middle" /></region></layout>`))
	if layout == nil {
		t.Fatalf("enum middle: layout = nil, want the block without the attribute")
	}
	if got := layout.Regions[0].Nodes[0].Attrs["side"]; got != "" {
		t.Fatalf("enum middle reached the UI")
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "is not valid") {
		t.Fatalf("enum middle: warnings = %v, want the invalid-attribute warning", warnings)
	}
}

// Per-view XSDs are strict plain XSD 1.0 (issue #110 item 3): each view's
// file lists only its regions and blocks, with the view fixed, while the
// union layout.xsd stays for compatibility.
func TestLayoutXSDPerView(t *testing.T) {
	dashboard := LayoutXSDForView("dashboard")
	library := LayoutXSDForView("library")
	shell := LayoutXSDForView("shell")
	for _, want := range []string{`<xs:enumeration value="hero" />`, `<xs:enumeration value="main" />`, `<xs:enumeration value="dashboard" />`} {
		if !strings.Contains(dashboard, want) {
			t.Errorf("dashboard XSD misses %q", want)
		}
	}
	for _, unwanted := range []string{`<xs:enumeration value="header" />`, `<xs:enumeration value="sidebar" />`} {
		if strings.Contains(dashboard, unwanted) {
			t.Errorf("dashboard XSD contains %q, want only dashboard names", unwanted)
		}
	}
	if !strings.Contains(library, `<xs:enumeration value="header" />`) || strings.Contains(library, `<xs:enumeration value="hero" />`) {
		t.Errorf("library XSD mixes dashboard blocks")
	}
	if !strings.Contains(shell, `<xs:enumeration value="sidebar" />`) || strings.Contains(shell, `<xs:enumeration value="main" />`) {
		t.Errorf("shell XSD mixes dashboard regions")
	}
	// Attribute types come from the catalog: limit is an int 1..50, not a
	// bare positiveInteger, and enum attrs render as enumerations.
	for _, xsd := range []string{dashboard, LayoutXSD()} {
		if !strings.Contains(xsd, `name="limit"`) || !strings.Contains(xsd, `<xs:minInclusive value="1" />`) || !strings.Contains(xsd, `<xs:maxInclusive value="50" />`) {
			t.Errorf("XSD misses the typed limit attribute")
		}
		if strings.Contains(xsd, `name="limit" type="xs:positiveInteger"`) {
			t.Errorf("XSD still types limit as positiveInteger")
		}
	}
	withCatalog(t, []types.ThemeCatalogEntry{
		{Entry: "dashboard.main", Since: "1.0", Kind: "region"},
		{Entry: "dashboard.align", Since: "1.0", Kind: "block", Region: "main", Attrs: []types.ThemeCatalogAttr{{Name: "side", Type: "enum", Values: []string{"left", "right"}}}},
	})
	xsd := LayoutXSDForView("dashboard")
	if !strings.Contains(xsd, `name="side"`) || !strings.Contains(xsd, `<xs:enumeration value="left" />`) || !strings.Contains(xsd, `<xs:enumeration value="right" />`) {
		t.Errorf("enum attr missing from XSD:\n%s", xsd)
	}
}
