package app

import (
	"fmt"

	"platypus/internal/apperr"
	"platypus/internal/themes"
	"platypus/internal/types"
)

// ListThemes returns the themes the Settings picker offers (issue #93), each
// with its status and the warnings found in its theme.toml.
func (a *App) ListThemes() []types.ThemeInfo {
	return themes.Infos()
}

// GetTheme returns the resolved tokens of a theme (issue #93). An empty id
// means the theme the user picked, which is what the UI applies at boot.
func (a *App) GetTheme(id string) types.ThemeTokens {
	if id == "" {
		id = a.settings.Theme()
	}
	return themes.Resolve(id)
}

// SetTheme saves the picked theme ("" for the built-in default). Only a theme
// listed as "ok" can be picked.
func (a *App) SetTheme(id string) (types.LauncherSettings, error) {
	if id != "" {
		t, ok := themes.Find(id)
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
