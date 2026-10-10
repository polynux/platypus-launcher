package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync/atomic"

	"platypus/internal/customize"
	"platypus/internal/types"
)

// Customization: the user's own title and art for a game (docs/plans: game
// page). It is core-only. Images are copied into <data>/custom-art and the
// library carries the result: Game.Title and the art URLs hold the user's
// values, with StoreTitle keeping the store's. Nothing here reaches a plugin,
// and plugin data is never modified. A search for art on the web may come later;
// it would only need to hand SetCustomArt a downloaded file.

// customStore is the customization store, set once at startup.
type customStore struct {
	p atomic.Pointer[customize.Store]
}

func (c *customStore) get() *customize.Store { return c.p.Load() }

// openCustomization opens the store under the data folder.
func (a *App) openCustomization(dataDir string) error {
	s, err := customize.Open(filepath.Join(dataDir, "custom-art"))
	a.custom.p.Store(s) // Open returns a usable store even when the index was corrupt
	return err
}

// customArt serves the user's images to the UI under customize.URLPrefix.
func (a *App) customArt(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s := a.custom.get(); s != nil && s.Serve(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// applyCustomization puts the user's title and images on the library's games.
func (a *App) applyCustomization(games []types.Game) {
	s := a.custom.get()
	if s == nil {
		return
	}
	snap := s.Snapshot()
	for i := range games {
		g := &games[i]
		e := snap.Entry(g.ID)
		if e.Title != "" {
			g.StoreTitle, g.Title = g.Title, e.Title
			g.Customized = append(g.Customized, "title")
		}
		if e.SortTitle != "" {
			g.SortTitle = e.SortTitle
			g.Customized = append(g.Customized, "sort_title")
		}
		for _, kind := range customize.Kinds {
			u := snap.URL(g.ID, kind)
			if u == "" {
				continue
			}
			switch kind {
			case customize.Cover:
				g.CoverURL = u
			case customize.Banner:
				g.BannerURL, g.HeroURL = u, u
			case customize.Logo:
				g.LogoURL = u
			case customize.Icon:
				g.IconURL = u
			case customize.Background:
				g.BackgroundURL = u
			}
			g.Customized = append(g.Customized, string(kind))
		}
	}
}

func (a *App) customization(gameID string) (types.Customization, error) {
	s := a.custom.get()
	if s == nil {
		return types.Customization{}, fmt.Errorf("customization is not available")
	}
	e := s.Get(gameID)
	c := types.Customization{Title: e.Title, SortTitle: e.SortTitle, Art: map[string]string{}}
	for kind := range e.Art {
		c.Art[string(kind)] = s.URL(gameID, kind)
	}
	return c, nil
}

// GetCustomization is what the user changed about a game's look.
func (a *App) GetCustomization(gameID string) (types.Customization, error) {
	return a.customization(gameID)
}

// SetCustomTitle sets the title and the sort title shown for a game. Empty
// ones go back to the store's title and to sorting by title.
func (a *App) SetCustomTitle(gameID, title, sortTitle string) (types.Customization, error) {
	s := a.custom.get()
	if s == nil {
		return types.Customization{}, fmt.Errorf("customization is not available")
	}
	if err := s.SetTitle(gameID, title, sortTitle); err != nil {
		return types.Customization{}, err
	}
	return a.customization(gameID)
}

// SetCustomArt copies the image at path in as the game's cover, banner, logo,
// icon or background. The file is checked (PNG, JPEG, GIF or WebP, a size cap)
// and never modified.
func (a *App) SetCustomArt(gameID, kind, path string) (types.Customization, error) {
	s := a.custom.get()
	if s == nil {
		return types.Customization{}, fmt.Errorf("customization is not available")
	}
	if err := s.SetArt(gameID, customize.Kind(kind), path); err != nil {
		return types.Customization{}, err
	}
	return a.customization(gameID)
}

// ResetCustomArt brings the store's art back for one kind.
func (a *App) ResetCustomArt(gameID, kind string) (types.Customization, error) {
	s := a.custom.get()
	if s == nil {
		return types.Customization{}, fmt.Errorf("customization is not available")
	}
	if err := s.ResetArt(gameID, customize.Kind(kind)); err != nil {
		return types.Customization{}, err
	}
	return a.customization(gameID)
}

// ResetCustomization drops every change the user made to a game's look.
func (a *App) ResetCustomization(gameID string) (types.Customization, error) {
	s := a.custom.get()
	if s == nil {
		return types.Customization{}, fmt.Errorf("customization is not available")
	}
	if err := s.Reset(gameID); err != nil {
		return types.Customization{}, err
	}
	return a.customization(gameID)
}

// PickImage opens a file picker for an image. Returns "" when the user cancels.
func (a *App) PickImage(title string) (string, error) {
	return a.host.OpenFile(FileDialog{
		Title: title,
		Filters: []FileFilter{
			{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp"}},
		},
	})
}
