package app

import (
	"fmt"
	"slices"
	"strings"

	"platypus/internal/apperr"
	"platypus/internal/themes"
	"platypus/internal/types"
)

// ListThemes returns the themes the Settings picker offers (issues #93, #105):
// the built-ins first, then the data plugins' themes with their source plugin
// id. Each carries its status and the warnings found in its theme.toml.
func (a *App) ListThemes() []types.ThemeInfo {
	out := themes.Infos()
	for _, t := range a.pluginThemes() {
		info := t.Info
		if info.Warnings == nil {
			info.Warnings = []string{}
		}
		if info.Supports == nil {
			info.Supports = []string{}
		}
		out = append(out, info)
	}
	return out
}

// pluginThemes returns the data plugins' themes, sorted by theme name. A
// theme whose id is already taken (a built-in, or an earlier plugin) is listed
// as invalid, so two entries never share a pickable id.
func (a *App) pluginThemes() []themes.Theme {
	if a.plugins == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, t := range themes.All() {
		seen[t.Info.ID] = true
	}
	var out []themes.Theme
	for _, p := range a.plugins.Plugins() {
		if p.Theme == nil {
			continue
		}
		th := *p.Theme
		if seen[th.Info.ID] {
			th.Info.Status = themes.StatusInvalid
			th.Info.Warnings = append([]string{fmt.Sprintf("theme id %q is already used", th.Info.ID)}, th.Info.Warnings...)
		} else {
			seen[th.Info.ID] = true
		}
		if th.Info.Warnings == nil {
			th.Info.Warnings = []string{}
		}
		if th.Info.Supports == nil {
			th.Info.Supports = []string{}
		}
		out = append(out, th)
	}
	slices.SortFunc(out, func(x, y themes.Theme) int {
		return strings.Compare(x.Info.Name, y.Info.Name)
	})
	return out
}

// findTheme returns the theme with the given id: a built-in first, then the
// first plugin theme with that id. Built-ins win a collision, and the picker
// lists the loser as invalid so it can't be picked.
func (a *App) findTheme(id string) (themes.Theme, bool) {
	if t, ok := themes.Find(id); ok {
		return t, true
	}
	for _, t := range a.pluginThemes() {
		if t.Info.ID == id && t.Info.Status == themes.StatusOK {
			// pluginThemes marks collisions invalid, so the first ok match
			// is the usable one.
			return t, true
		}
	}
	// Still return invalid plugin themes for their warnings: SetTheme refuses
	// them, and the picker shows why.
	for _, t := range a.pluginThemes() {
		if t.Info.ID == id {
			return t, true
		}
	}
	return themes.Theme{}, false
}

// GetTheme returns the resolved tokens of a theme (issues #93, #105). An empty
// id means the theme the user picked, which is what the UI applies at boot.
// Relative font and wash image paths of a plugin theme become art URLs here.
func (a *App) GetTheme(id string) types.ThemeTokens {
	if id == "" {
		id = a.settings.Theme()
	}
	if t, ok := themes.Find(id); ok {
		return themes.ResolveTheme(t, true)
	}
	t, ok := a.findTheme(id)
	if !ok {
		return themes.Resolve("")
	}
	out := themes.ResolveTheme(t, true)
	return a.withThemeURLs(out)
}

// withThemeURLs turns a plugin theme's relative asset paths into art URLs.
// A file that vanished since discovery drops out: its font leaves the map and
// its wash layer keeps the gradient without the picture.
func (a *App) withThemeURLs(out types.ThemeTokens) types.ThemeTokens {
	if a.plugins == nil {
		out.Fonts = nil
		for i := range out.Wash {
			out.Wash[i].Image = ""
		}
		return out
	}
	// The theme id is the plugin theme's id; its source plugin owns the files.
	// findTheme already picked the usable one, so look its source up again.
	t, ok := a.findTheme(out.ID)
	if !ok || t.Info.Source == "" {
		return out
	}
	if len(out.Fonts) > 0 {
		fonts := make(map[string]string, len(out.Fonts))
		for family, rel := range out.Fonts {
			if u := a.plugins.ThemeURL(t.Info.Source, rel); u != "" {
				fonts[family] = u
			}
		}
		out.Fonts = fonts
	}
	for i, l := range out.Wash {
		if l.Image == "" {
			continue
		}
		out.Wash[i].Image = a.plugins.ThemeURL(t.Info.Source, l.Image)
	}
	return out
}

// GetThemeCatalog returns the versioned variant catalog used by the UI.
func (a *App) GetThemeCatalog() []types.ThemeCatalogEntry { return themes.Catalog }

// GetThemeLayout returns the validated layout for a view in the picked theme
// (issues #96, #105), or nil when the theme ships none: the UI then uses its
// built-in arrangement.
func (a *App) GetThemeLayout(view string) *types.ThemeLayout {
	id := a.settings.Theme()
	if t, ok := themes.Find(id); ok {
		return themes.LayoutForTheme(t, view)
	}
	t, ok := a.findTheme(id)
	if !ok {
		return nil
	}
	return themes.LayoutForTheme(t, view)
}

// SetTheme saves the picked theme ("" for the built-in default). Only a theme
// listed as "ok" can be picked.
func (a *App) SetTheme(id string) (types.LauncherSettings, error) {
	if id != "" {
		t, ok := a.findTheme(id)
		if !ok {
			return a.launcherSettings(), apperr.New("theme.unknown",
				fmt.Sprintf("no theme called %q", id), map[string]string{"theme": id})
		}
		if t.Info.Status != themes.StatusOK {
			return a.launcherSettings(), apperr.New("theme.unusable",
				fmt.Sprintf("theme %q can't be used: %s", id, t.Info.Status), map[string]string{"theme": id})
		}
	}
	if err := a.settings.SetTheme(id); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}
