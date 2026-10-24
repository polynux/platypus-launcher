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
	// The built-in arrangement (issue #108) also uses the top region (the
	// hero or welcome banner, full width) and the bottom region (the
	// activity feed, full width); middle columns are main and side.
	{Entry: "dashboard.top", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.main", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.side", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.bottom", Since: "1.0", Kind: "region"},
	{Entry: "dashboard.hero", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "dashboard.welcome", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "dashboard.empty", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "dashboard.continue", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "dashboard.waiting", Since: "1.0", Kind: "block", Region: "side"},
	{Entry: "dashboard.recent", Since: "1.0", Kind: "block", Region: "side"},
	{Entry: "dashboard.activity", Since: "1.0", Kind: "block", Region: "bottom", Attrs: []string{"limit"}},
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
	{Entry: "dialog", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "presentation", Values: []string{"dialog", "sheet"}, Default: "dialog", Since: "1.0"},
		// auto is the use site's built-in size: every dialog keeps its own
		// width unless the theme names one.
		{Name: "size", Values: []string{"auto", "sm", "md", "lg", "xl"}, Default: "auto", Since: "1.0"},
	}},
	{Entry: "game-settings", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "presentation", Values: []string{"dialog", "sheet", "page"}, Default: "dialog", Since: "1.0", Inherits: "dialog.presentation"},
		{Name: "tabs", Values: []string{"vertical", "horizontal"}, Default: "vertical", Since: "1.0"},
	}},
	// Shell navigation entries (issue #104, ADR 0002 answer 4): the app shell
	// is a view whose regions depend on shell.navigation (sidebar today,
	// topbar as the alternate). Each variant has its own region; the
	// sidebar's parts (inventory §1.2) are blocks either region can hold.
	// None is required: the app keeps the Settings and Plugins entries
	// reachable in both variants by construction.
	{Entry: "shell", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "navigation", Values: []string{"sidebar", "topbar"}, Default: "sidebar", Since: "1.0"},
	}},
	{Entry: "shell.sidebar", Since: "1.0", Kind: "region"},
	{Entry: "shell.topbar", Since: "1.0", Kind: "region"},
	{Entry: "shell.nav", Since: "1.0", Kind: "block", Region: "sidebar"},
	{Entry: "shell.tools", Since: "1.0", Kind: "block", Region: "sidebar"},
	{Entry: "shell.collections", Since: "1.0", Kind: "block", Region: "sidebar"},
	{Entry: "shell.sources", Since: "1.0", Kind: "block", Region: "sidebar"},
	{Entry: "shell.store", Since: "1.0", Kind: "block", Region: "sidebar"},
	{Entry: "shell.footer", Since: "1.0", Kind: "block", Region: "sidebar"},
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
	// The built-in arrangement (issue #108) groups the header in top and the
	// catalog in bottom, both full width; the header block composes the
	// title, mode, search, chips and filters exactly as the built-in header
	// shows them, while themes like cool place the parts separately.
	{Entry: "library.top", Since: "1.0", Kind: "region"},
	{Entry: "library.main", Since: "1.0", Kind: "region"},
	{Entry: "library.side", Since: "1.0", Kind: "region"},
	{Entry: "library.bottom", Since: "1.0", Kind: "region"},
	{Entry: "library.header", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "library.title", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.mode", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.search", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.chips", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.filters", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "library.warnings", Since: "1.0", Kind: "block", Required: true, Region: "top"},
	{Entry: "library.empty", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "library.last-played", Since: "1.0", Kind: "block", Region: "bottom"},
	{Entry: "library.grid", Since: "1.0", Kind: "block", Region: "bottom"},
	{Entry: "library.all-titles", Since: "1.0", Kind: "block", Region: "bottom"},
	{Entry: "library.activity", Since: "1.0", Kind: "block", Region: "bottom", Attrs: []string{"limit"}},
	{Entry: "library.recent", Since: "1.0", Kind: "block", Region: "bottom"},
	{Entry: "library.table", Since: "1.0", Kind: "block", Region: "bottom"},
	// Hero frame entries (issue #101): one frame for the dashboard and game
	// page heroes. The dashboard reads the global hero axes; the game page
	// reads game-hero, which follows the global choice unless the theme
	// names a value for the game itself (like game-settings follows
	// dialog). Warm keeps today's looks through the defaults: the
	// dashboard's tall glass banner, the game page's regular solid one.
	// Cool picks the compact values.
	{Entry: "hero", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "height", Values: []string{"tall", "regular", "compact"}, Default: "tall", Since: "1.0"},
		{Name: "gradient", Values: []string{"warm", "neutral", "none"}, Default: "warm", Since: "1.0"},
		{Name: "chips", Values: []string{"glass", "solid"}, Default: "glass", Since: "1.0"},
	}},
	{Entry: "game-hero", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "height", Values: []string{"tall", "regular", "compact"}, Default: "regular", Since: "1.0", Inherits: "hero.height"},
		{Name: "gradient", Values: []string{"warm", "neutral", "none"}, Default: "warm", Since: "1.0", Inherits: "hero.gradient"},
		{Name: "chips", Values: []string{"glass", "solid"}, Default: "solid", Since: "1.0", Inherits: "hero.chips"},
	}},
	// Hero content blocks (issue #101): today's dashboard hero content and
	// today's game page hero content. The dashboard keeps placing hero (the
	// frame with its content); attention stays available for layouts that
	// show the content on its own. The game page uses the blocks in fixed
	// positions until it becomes a layout view (#103). Actions are required:
	// they hold Install, Play and Update.
	{Entry: "dashboard.attention", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "game.heading", Since: "1.0", Kind: "block", Region: "hero"},
	{Entry: "game.actions", Since: "1.0", Kind: "block", Required: true, Region: "hero"},
	// Storage layout entries (issue #102): its regions, then its blocks in
	// the built-in order (inventory §2.7). None of them is required: the
	// drives and the game list act on nothing by themselves (remove/move
	// flows stay inside the games block). The built-in arrangement (issue
	// #108) keeps the header full width and stacks drives over games; themes
	// like cool move the drives to a side column.
	{Entry: "storage.top", Since: "1.0", Kind: "region"},
	{Entry: "storage.main", Since: "1.0", Kind: "region"},
	{Entry: "storage.side", Since: "1.0", Kind: "region"},
	{Entry: "storage.header", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "storage.drives", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "storage.games", Since: "1.0", Kind: "block", Region: "main"},
	// Plugins layout entries (issue #102): its regions, then its blocks in
	// the built-in order (inventory §2.8). Only the store cards are required:
	// each card renders its own login row and approve/enable actions, so a
	// layout can't hide them on their own. Data cards carry no actions and
	// the built-in list stays hideable.
	{Entry: "plugins.top", Since: "1.0", Kind: "region"},
	{Entry: "plugins.main", Since: "1.0", Kind: "region"},
	{Entry: "plugins.side", Since: "1.0", Kind: "region"},
	{Entry: "plugins.header", Since: "1.0", Kind: "block", Region: "top"},
	{Entry: "plugins.store", Since: "1.0", Kind: "block", Required: true, Region: "main"},
	{Entry: "plugins.data", Since: "1.0", Kind: "block", Region: "main"},
	{Entry: "plugins.builtin", Since: "1.0", Kind: "block", Region: "main"},
}
