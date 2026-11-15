// Atlas layout views (issue #116-#118): the exact required block names per
// view, straight from the catalog. Flipping a block back to hideable must
// fail here, not just in the generated docs.
package themes

import (
	"sort"
	"strings"
	"testing"
)

func TestRequiredBlocksPerView(t *testing.T) {
	got := map[string][]string{}
	for _, e := range Catalog {
		if e.Kind != "block" {
			continue
		}
		view := e.Entry
		if i := strings.LastIndex(e.Entry, "."); i >= 0 {
			view = e.Entry[:i]
		} else {
			continue
		}
		switch view {
		case "activity", "fixes", "runners", "downloads", "settings", "library-header":
		default:
			continue
		}
		if e.Required {
			got[view] = append(got[view], e.Entry)
		}
	}
	for _, names := range got {
		sort.Strings(names)
	}
	want := map[string][]string{
		"activity": {},
		"fixes":    {"fixes.cards"},
		"runners":  {"runners.available", "runners.default", "runners.installed"},
		"downloads": {
			"downloads.finished",
			"downloads.header",
			"downloads.queue",
		},
		"settings": {
			"settings.appearance", "settings.artwork", "settings.backups",
			"settings.chat", "settings.cleanup", "settings.cloud",
			"settings.folders", "settings.gameinfo", "settings.language",
			"settings.notices", "settings.prefixes", "settings.ratings",
			"settings.theme", "settings.tray",
		},
		"library-header": {"library-header.warnings"},
	}
	for view, names := range want {
		if len(got[view]) != len(names) {
			t.Errorf("view %s: required = %v, want %v", view, got[view], names)
			continue
		}
		for i := range names {
			if got[view][i] != names[i] {
				t.Errorf("view %s: required = %v, want %v", view, got[view], names)
				break
			}
		}
	}
}
