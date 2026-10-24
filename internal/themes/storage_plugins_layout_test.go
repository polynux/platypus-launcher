// Storage and Plugins layouts (issue #102): the built-in arrangements parse
// clean, and a plugins layout that leaves out the required store block gets
// it back in its built-in region, with a warning.
package themes

import (
	"strings"
	"testing"
)

func TestParseStorageBuiltinLayout(t *testing.T) {
	src, ok := builtinLayoutXML["storage"]
	if !ok {
		t.Fatal("no built-in XML for view \"storage\"")
	}
	layout, warnings := parseLayout("storage", []byte(src))
	if layout == nil || len(warnings) != 0 {
		t.Fatalf("built-in storage: layout = %+v, warnings = %v, want clean", layout, warnings)
	}
	if len(layout.Regions) != 2 || layout.Regions[0].Name != "top" || layout.Regions[1].Name != "main" {
		t.Fatalf("built-in storage regions = %+v, want top and main", layout.Regions)
	}
}

func TestParsePluginsBuiltinLayout(t *testing.T) {
	src, ok := builtinLayoutXML["plugins"]
	if !ok {
		t.Fatal("no built-in XML for view \"plugins\"")
	}
	layout, warnings := parseLayout("plugins", []byte(src))
	if layout == nil || len(warnings) != 0 {
		t.Fatalf("built-in plugins: layout = %+v, warnings = %v, want clean", layout, warnings)
	}
	if len(layout.Regions) != 2 || layout.Regions[0].Name != "top" || layout.Regions[1].Name != "main" {
		t.Fatalf("built-in plugins regions = %+v, want top and main", layout.Regions)
	}
}

// The store block is required: leaving it out warns and restores it in the
// main region, so the login row and the approve/enable actions stay reachable.
func TestParsePluginsLayoutRestoresStore(t *testing.T) {
	src := `
<layout view="plugins" catalog="1.0">
  <region name="main">
    <block name="header" />
    <block name="data" />
    <block name="builtin" />
  </region>
</layout>`
	layout, warnings := parseLayout("plugins", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, `leaves out required block "store"`) {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for store", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the store block restored")
	}
	found := false
	for _, region := range layout.Regions {
		for _, n := range region.Nodes {
			if n.Block == "store" {
				found = true
				if region.Name != "main" {
					t.Fatalf("store restored in region %q, want main", region.Name)
				}
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the store block restored", layout)
	}
}

// The cool theme's storage and plugins arrangements (its layouts/*.xml):
// the drives share a row with the game list, and the data and built-in
// blocks share a row below the store cards.
func TestCoolStoragePluginsLayouts(t *testing.T) {
	cool, ok := Find("cool")
	if !ok {
		t.Fatal("no cool theme")
	}
	storage, ok := cool.Layouts["storage"]
	if !ok {
		t.Fatalf("cool ships layouts %v, want a storage layout", cool.Layouts)
	}
	rows := 0
	for _, region := range storage.Regions {
		for _, n := range region.Nodes {
			if n.Row != nil {
				rows++
			}
		}
	}
	if rows == 0 {
		t.Fatalf("cool storage = %+v, want a row grouping games and drives", storage)
	}
	plugins, ok := cool.Layouts["plugins"]
	if !ok {
		t.Fatalf("cool ships layouts %v, want a plugins layout", cool.Layouts)
	}
	rows = 0
	for _, region := range plugins.Regions {
		for _, n := range region.Nodes {
			if n.Row != nil {
				rows++
			}
		}
	}
	if rows == 0 {
		t.Fatalf("cool plugins = %+v, want a row grouping data and built-in", plugins)
	}
}
