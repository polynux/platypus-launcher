// Atlas layout views (issue #116): the built-in arrangements parse clean,
// and a layout that leaves out a required block gets it back in its
// built-in region, with a warning.
package themes

import (
	"strings"
	"testing"
)

func TestParseLaneViewsBuiltinLayouts(t *testing.T) {
	want := map[string][][]string{
		"activity":  {{"header"}, {"filter", "days"}},
		"fixes":     {{"header"}, {"cards"}},
		"runners":   {{"header"}, {"default", "installed", "available"}},
		"downloads": {{"header"}, {"queue", "finished"}},
	}
	for view, regions := range want {
		src, ok := builtinLayoutXML[view]
		if !ok {
			t.Fatalf("no built-in XML for view %q", view)
		}
		layout, warnings := parseLayout(view, []byte(src))
		if layout == nil || len(warnings) != 0 {
			t.Fatalf("built-in %s: layout = %+v, warnings = %v, want clean", view, layout, warnings)
		}
		if len(layout.Regions) != 2 || layout.Regions[0].Name != "top" || layout.Regions[1].Name != "main" {
			t.Fatalf("built-in %s regions = %+v, want top and main", view, layout.Regions)
		}
		for i, blocks := range regions {
			var got []string
			for _, n := range layout.Regions[i].Nodes {
				got = append(got, n.Block)
			}
			if strings.Join(got, ",") != strings.Join(blocks, ",") {
				t.Fatalf("built-in %s region %q blocks = %v, want %v", view, layout.Regions[i].Name, got, blocks)
			}
		}
	}
}

// The queue block is required: leaving it out warns and restores it in the
// main region, so pause/resume/cancel stay reachable.
func TestParseDownloadsLayoutRestoresQueue(t *testing.T) {
	src := `
<layout view="downloads" catalog="1.2">
  <region name="top">
    <block name="header" />
  </region>
  <region name="main">
    <block name="finished" />
  </region>
</layout>`
	layout, warnings := parseLayout("downloads", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, `leaves out required block "queue"`) {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for queue", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the queue block restored")
	}
	found := false
	for _, region := range layout.Regions {
		for _, n := range region.Nodes {
			if n.Block == "queue" {
				found = true
				if region.Name != "main" {
					t.Fatalf("queue restored in region %q, want main", region.Name)
				}
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the queue block restored", layout)
	}
}

// The cards block is required: leaving it out warns and restores it in the
// main region, so the fix actions stay reachable.
func TestParseFixesLayoutRestoresCards(t *testing.T) {
	src := `
<layout view="fixes" catalog="1.2">
  <region name="top">
    <block name="header" />
  </region>
  <region name="main">
  </region>
</layout>`
	layout, warnings := parseLayout("fixes", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, `leaves out required block "cards"`) {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for cards", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the cards block restored")
	}
	found := false
	for _, region := range layout.Regions {
		for _, n := range region.Nodes {
			if n.Block == "cards" {
				found = true
				if region.Name != "main" {
					t.Fatalf("cards restored in region %q, want main", region.Name)
				}
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the cards block restored", layout)
	}
}

// The installed table is required: leaving it out warns and restores it in
// the main region, so the remove action and the per-game usage truth stay
// reachable.
func TestParseRunnersLayoutRestoresInstalled(t *testing.T) {
	src := `
<layout view="runners" catalog="1.2">
  <region name="top">
    <block name="header" />
  </region>
  <region name="main">
    <block name="default" />
    <block name="available" />
  </region>
</layout>`
	layout, warnings := parseLayout("runners", []byte(src))
	restored := false
	for _, w := range warnings {
		if strings.Contains(w, `leaves out required block "installed"`) {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("warnings = %v, want the required-block warning for installed", warnings)
	}
	if layout == nil {
		t.Fatal("layout is nil, want the installed block restored")
	}
	found := false
	for _, region := range layout.Regions {
		for _, n := range region.Nodes {
			if n.Block == "installed" {
				found = true
				if region.Name != "main" {
					t.Fatalf("installed restored in region %q, want main", region.Name)
				}
			}
		}
	}
	if !found {
		t.Fatalf("layout = %+v, want the installed block restored", layout)
	}
}
