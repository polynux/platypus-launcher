package themes

import "platypus/internal/types"

// Catalog v1.0 is the frontend contract for the Settings testbed.
const CatalogVersion = "1.0"

var Catalog = []types.ThemeCatalogEntry{
	{Entry: "settings", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "presentation", Values: []string{"page", "dialog", "sheet"}, Default: "page", Since: "1.0"},
		{Name: "navigation", Values: []string{"scroll", "tabs"}, Default: "scroll", Since: "1.0"},
	}},
	{Entry: "section", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "chrome", Values: []string{"hairline", "framed", "plain"}, Default: "hairline", Since: "1.0"},
	}},
	// Dashboard layout entries (issue #96): its regions, then its blocks in
	// the built-in order (inventory §2.1). None of them is required: advice
	// and decoration stay hideable, and the dashboard acts on nothing.
	{Entry: "dashboard.main", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.side", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.hero", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.welcome", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.empty", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.continue", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.waiting", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.recent", Since: "1.0", Kind: "block", Region: "side"},
	{Entry: "dashboard.activity", Since: "1.0", Kind: "block", Region: "main", Attrs: []string{"limit"}},
	{Entry: "view-header", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "actions", Values: []string{"right", "below"}, Default: "right", Since: "1.0"},
		{Name: "subtitle", Values: []string{"shown", "hidden"}, Default: "shown", Since: "1.0"},
		{Name: "size", Values: []string{"regular", "compact"}, Default: "regular", Since: "1.0"},
	}},
	{Entry: "empty-state", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "chrome", Values: []string{"dashed", "plain"}, Default: "dashed", Since: "1.0"},
		{Name: "icon", Values: []string{"on", "off"}, Default: "off", Since: "1.0"},
	}},
	{Entry: "list-row", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "media", Values: []string{"on", "off"}, Default: "on", Since: "1.0"},
		{Name: "density", Values: []string{"comfortable", "compact"}, Default: "comfortable", Since: "1.0"},
	}},
	// Library game representations (issue #98): the tile the continue grid
	// and the cards share, and the row the table and the recent lists share.
	// Warm keeps today's dashboard tile (3:4, compat and overlay on) and
	// today's full table.
	{Entry: "game-tile", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "aspect", Values: []string{"2:3", "3:4", "16:9"}, Default: "3:4", Since: "1.0"},
		{Name: "compat", Values: []string{"on", "off"}, Default: "on", Since: "1.0"},
		{Name: "overlay", Values: []string{"on", "off"}, Default: "on", Since: "1.0"},
		{Name: "density", Values: []string{"comfortable", "compact"}, Default: "comfortable", Since: "1.0"},
	}},
	{Entry: "game-row", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "columns", Values: []string{"full", "minimal"}, Default: "full", Since: "1.0"},
		{Name: "mode", Values: []string{"full", "compact"}, Default: "full", Since: "1.0"},
	}},
	// Library layout entries (issue #98): its regions, then its blocks in the
	// built-in order (inventory §2.2). Only the plugin warnings are required:
	// they tell the truth about the stores, so a layout can't hide them.
	{Entry: "library.main", Since: "1.0", Kind: "region"},
	{Entry: "library.side", Since: "1.0", Kind: "region"},
	{Entry: "library.title", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.mode", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.search", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.chips", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.filters", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.warnings", Since: "1.0", Kind: "block", Required: true, Region: "main"},
	{Entry: "library.empty", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.last-played", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.all-titles", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.activity", Since: "1.0", Kind: "block", Region: "main", Attrs: []string{"limit"}},
	{Entry: "library.recent", Since: "1.0", Kind: "block", Region: "side"},
	{Entry: "library.table", Since: "1.0", Kind: "block", Region: "main"},
}
