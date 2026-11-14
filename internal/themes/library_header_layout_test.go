// Library header layouts (issue #118): the built-in arrangement parses
// clean, a layout that leaves out the required warnings gets them back in
// the sub region, and a layout that leaves out the chips stays without them.
package themes

import (
	"strings"
	"testing"
)

func TestParseLibraryHeaderBuiltinLayout(t *testing.T) {
	src, ok := builtinLayoutXML["library-header"]
	if !ok {
		t.Fatal("no built-in XML for view \"library-header\"")
	}
	layout, warnings := parseLayout("library-header", []byte(src))
	if layout == nil || len(warnings) != 0 {
		t.Fatalf("built-in library-header: layout = %+v, warnings = %v, want clean", layout, warnings)
	}
	if len(layout.Regions) != 2 || layout.Regions[0].Name != "bar" || layout.Regions[1].Name != "sub" {
		t.Fatalf("built-in library-header regions = %+v, want bar and sub", layout.Regions)
	}
	bar := []string{"search", "sort"}
	var gotBar []string
	for _, node := range layout.Regions[0].Nodes {
		gotBar = append(gotBar, node.Block)
	}
	if strings.Join(gotBar, ",") != strings.Join(bar, ",") {
		t.Fatalf("built-in bar blocks = %v, want %v", gotBar, bar)
	}
	sub := []string{"warnings", "chips"}
	var gotSub []string
	for _, node := range layout.Regions[1].Nodes {
		gotSub = append(gotSub, node.Block)
	}
	if strings.Join(gotSub, ",") != strings.Join(sub, ",") {
		t.Fatalf("built-in sub blocks = %v, want %v", gotSub, sub)
	}
}

// The warnings tell the truth about the stores, so leaving them out warns
// and restores them in the sub region.
func TestParseLibraryHeaderLayoutRestoresWarnings(t *testing.T) {
	src := `
<layout view="library-header" catalog="1.2">
  <region name="bar">
    <block name="search" />
    <block name="sort" />
  </region>
  <region name="sub">
    <block name="chips" />
  </region>
</layout>`
	layout, warnings := parseLayout("library-header", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, "warnings") && strings.Contains(w, "required") {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for warnings", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the warnings block restored")
	}
	found := false
	for _, region := range layout.Regions {
		if region.Name != "sub" {
			continue
		}
		for _, node := range region.Nodes {
			if node.Block == "warnings" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the warnings block restored in sub", layout)
	}
}

// The chips are decoration: leaving them out warns about nothing and keeps
// the layout without them, so a theme can hide them.
func TestParseLibraryHeaderLayoutHidesChips(t *testing.T) {
	src := `
<layout view="library-header" catalog="1.2">
  <region name="bar">
    <block name="search" />
    <block name="sort" />
  </region>
  <region name="sub">
    <block name="warnings" />
  </region>
</layout>`
	layout, warnings := parseLayout("library-header", []byte(src))
	if layout == nil || len(warnings) != 0 {
		t.Fatalf("chips-free library-header: layout = %+v, warnings = %v, want clean", layout, warnings)
	}
	for _, region := range layout.Regions {
		for _, node := range region.Nodes {
			if node.Block == "chips" {
				t.Fatalf("layout = %+v, want no chips block", layout)
			}
		}
	}
}
