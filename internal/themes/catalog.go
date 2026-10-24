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
	{Entry: "dialog", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "presentation", Values: []string{"dialog", "sheet"}, Default: "dialog", Since: "1.0"},
		{Name: "size", Values: []string{"sm", "md", "lg", "xl"}, Default: "md", Since: "1.0"},
	}},
	{Entry: "game-settings", Since: "1.0", Axes: []types.ThemeCatalogAxis{
		{Name: "presentation", Values: []string{"dialog", "sheet", "page"}, Default: "dialog", Since: "1.0"},
		{Name: "tabs", Values: []string{"vertical", "horizontal"}, Default: "vertical", Since: "1.0"},
	}},
}
