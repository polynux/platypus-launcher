// Settings layouts (issue #117): the built-in arrangement parses clean in
// the Atlas category order, and a layout that leaves a section out gets it
// back in the main region, with a warning.
package themes

import (
	"strings"
	"testing"
)

func TestParseSettingsBuiltinLayout(t *testing.T) {
	src, ok := builtinLayoutXML["settings"]
	if !ok {
		t.Fatal("no built-in XML for view \"settings\"")
	}
	layout, warnings := parseLayout("settings", []byte(src))
	if layout == nil || len(warnings) != 0 {
		t.Fatalf("built-in settings: layout = %+v, warnings = %v, want clean", layout, warnings)
	}
	if len(layout.Regions) != 1 || layout.Regions[0].Name != "main" {
		t.Fatalf("built-in settings regions = %+v, want one main region", layout.Regions)
	}
	var got []string
	for _, node := range layout.Regions[0].Nodes {
		if node.Block == "" {
			t.Fatalf("built-in settings has a non-block node: %+v", node)
		}
		got = append(got, node.Block)
	}
	want := []string{"language", "tray", "theme", "appearance", "folders", "prefixes", "cleanup", "backups", "cloud", "ratings", "artwork", "gameinfo", "chat", "notices"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("built-in settings blocks = %v, want %v", got, want)
	}
}

// Every settings section is required: leaving one out warns and restores it
// in the main region, so every section stays reachable.
func TestParseSettingsLayoutRestoresSection(t *testing.T) {
	src := `
<layout view="settings" catalog="1.2">
  <region name="main">
    <block name="language" />
  </region>
</layout>`
	layout, warnings := parseLayout("settings", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, "notices") && strings.Contains(w, "required") {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for notices", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the notices block restored")
	}
	found := false
	for _, region := range layout.Regions {
		if region.Name != "main" {
			continue
		}
		for _, node := range region.Nodes {
			if node.Block == "notices" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the notices block restored in main", layout)
	}
}
