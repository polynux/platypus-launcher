package app

import "platypus/internal/types"

// Collections (issue #45): the user's game lists and smart filters, kept in
// settings. Every change answers the whole list, in the user's order, so the
// sidebar and the library filter update without a rescan. Smart filters are
// evaluated by the UI on the loaded library; the core only stores them.

// ListCollections returns every collection, built-ins included.
func (a *App) ListCollections() ([]types.Collection, error) {
	return a.settings.Collections(), nil
}

// CreateCollection adds a collection: manual when smart is the zero filter.
func (a *App) CreateCollection(name string, smart types.SmartFilter) ([]types.Collection, error) {
	return a.settings.CreateCollection(name, smart)
}

// UpdateCollection renames a user's collection and changes its smart filter.
func (a *App) UpdateCollection(id, name string, smart types.SmartFilter) ([]types.Collection, error) {
	return a.settings.UpdateCollection(id, name, smart)
}

// PinCollection shows or hides a collection in the sidebar.
func (a *App) PinCollection(id string, pinned bool) ([]types.Collection, error) {
	return a.settings.PinCollection(id, pinned)
}

// ReorderCollections sets the order: ids must name every collection once.
func (a *App) ReorderCollections(ids []string) ([]types.Collection, error) {
	return a.settings.ReorderCollections(ids)
}

// DeleteCollection removes a user's collection; its games stay in the library.
func (a *App) DeleteCollection(id string) ([]types.Collection, error) {
	return a.settings.DeleteCollection(id)
}

// AddCollectionGame puts a game in a manual collection.
func (a *App) AddCollectionGame(id, gameID string) ([]types.Collection, error) {
	return a.settings.AddCollectionGame(id, gameID)
}

// RemoveCollectionGame takes a game out of a manual collection.
func (a *App) RemoveCollectionGame(id, gameID string) ([]types.Collection, error) {
	return a.settings.RemoveCollectionGame(id, gameID)
}

// RememberCollectionTitles records the last known title of games held in
// manual collections (issue #77): the UI sends the titles that changed.
func (a *App) RememberCollectionTitles(titles map[string]string) ([]types.Collection, error) {
	return a.settings.RememberCollectionTitles(titles)
}
