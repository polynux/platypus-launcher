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
}
