package settings

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"platypus/internal/types"
)

// Collections (issue #45) live in settings, keyed by game id, so a rescan
// never touches them. A game that leaves the library keeps its place in a
// manual collection: the UI lists it as missing.

// builtinCollections are the collections every settings file starts with:
// pinned, in this order. Their names come from the UI's locale; these are
// the stored fallbacks.
func builtinCollections() []types.Collection {
	yes := true
	return []types.Collection{
		{ID: types.CollectionFavourites, Name: "Favourites", Builtin: true, Pinned: true},
		{ID: types.CollectionInstalled, Name: "Installed", Builtin: true, Pinned: true, Smart: &types.SmartFilter{Installed: &yes}},
		{ID: types.CollectionRecent, Name: "Recently played", Builtin: true, Pinned: true, Smart: &types.SmartFilter{Played: &yes, PlayedWithinDays: 30}},
	}
}

// ensureBuiltins puts back any built-in collection that is missing, ahead of
// the user's own. The caller holds s.mu.
func (s *Store) ensureBuiltins() {
	var missing []types.Collection
	for _, b := range builtinCollections() {
		if collectionIndex(s.data.Collections, b.ID) < 0 {
			missing = append(missing, b)
		}
	}
	if len(missing) > 0 {
		s.data.Collections = append(missing, s.data.Collections...)
	}
}

// Collections returns every collection in the user's order, built-ins included.
func (s *Store) Collections() []types.Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBuiltins()
	return copyCollections(s.data.Collections)
}

// CreateCollection adds a collection, manual when smart is the zero filter,
// and returns the whole list. A new collection starts pinned, so it shows in
// the sidebar at once.
func (s *Store) CreateCollection(name string, smart types.SmartFilter) ([]types.Collection, error) {
	name, err := cleanCollectionName(name)
	if err != nil {
		return nil, err
	}
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		if len(list) >= types.MaxCollections {
			return nil, fmt.Errorf("a library keeps at most %d collections", types.MaxCollections)
		}
		c := types.Collection{ID: fmt.Sprintf("collection-%d", time.Now().UnixNano()), Name: name, Pinned: true}
		if smart != (types.SmartFilter{}) {
			if err := validateSmart(list, smart); err != nil {
				return nil, err
			}
			c.Smart = &smart
		}
		return append(list, c), nil
	})
}

// UpdateCollection renames a user's collection and changes its smart filter.
// A manual collection stays manual (a zero filter) and a smart one stays smart.
// Built-in collections can't change.
func (s *Store) UpdateCollection(id, name string, smart types.SmartFilter) ([]types.Collection, error) {
	name, err := cleanCollectionName(name)
	if err != nil {
		return nil, err
	}
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		i := collectionIndex(list, id)
		if i < 0 {
			return nil, unknownCollection(id)
		}
		c := &list[i]
		if c.Builtin {
			return nil, errors.New("a built-in collection can't be changed")
		}
		if (c.Smart != nil) != (smart != (types.SmartFilter{})) {
			return nil, errors.New("a collection can't change between manual and smart")
		}
		if c.Smart != nil {
			if err := validateSmart(list, smart); err != nil {
				return nil, err
			}
			c.Smart = &smart
		}
		c.Name = name
		return list, nil
	})
}

// PinCollection shows a collection in the sidebar, or hides it there (it stays
// in the library's filter list).
func (s *Store) PinCollection(id string, pinned bool) ([]types.Collection, error) {
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		i := collectionIndex(list, id)
		if i < 0 {
			return nil, unknownCollection(id)
		}
		list[i].Pinned = pinned
		return list, nil
	})
}

// ReorderCollections sets the order to ids, which must name every collection
// exactly once.
func (s *Store) ReorderCollections(ids []string) ([]types.Collection, error) {
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		if len(ids) != len(list) {
			return nil, errors.New("the order must list every collection once")
		}
		out := make([]types.Collection, 0, len(list))
		for _, id := range ids {
			i := collectionIndex(list, id)
			if i < 0 || slices.ContainsFunc(out, func(c types.Collection) bool { return c.ID == id }) {
				return nil, errors.New("the order must list every collection once")
			}
			out = append(out, list[i])
		}
		return out, nil
	})
}

// DeleteCollection removes a user's collection. Its games stay in the
// library. A smart collection that filters on it blocks the delete.
func (s *Store) DeleteCollection(id string) ([]types.Collection, error) {
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		i := collectionIndex(list, id)
		if i < 0 {
			return nil, unknownCollection(id)
		}
		if list[i].Builtin {
			return nil, errors.New("a built-in collection can't be deleted")
		}
		for _, c := range list {
			if c.Smart != nil && c.Smart.Collection == id {
				return nil, fmt.Errorf("the smart collection %q filters on %q", c.Name, list[i].Name)
			}
		}
		return slices.Delete(list, i, i+1), nil
	})
}

// AddCollectionGame puts a game in a manual collection (at the end; a game
// already there keeps its place).
func (s *Store) AddCollectionGame(id, gameID string) ([]types.Collection, error) {
	if err := validCollectionGameID(gameID); err != nil {
		return nil, err
	}
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		i, err := manualCollection(list, id)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(list[i].Games, gameID) {
			list[i].Games = append(list[i].Games, gameID)
		}
		return list, nil
	})
}

// RemoveCollectionGame takes a game out of a manual collection. A game that
// isn't there is not an error.
func (s *Store) RemoveCollectionGame(id, gameID string) ([]types.Collection, error) {
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		i, err := manualCollection(list, id)
		if err != nil {
			return nil, err
		}
		list[i].Games = slices.DeleteFunc(list[i].Games, func(g string) bool { return g == gameID })
		delete(list[i].Titles, gameID)
		return list, nil
	})
}

// RememberCollectionTitles records the last known title of each game in
// titles, in every manual collection that holds it. The UI sends the titles of
// the library games whose remembered title is out of date; a game without a
// title is skipped.
func (s *Store) RememberCollectionTitles(titles map[string]string) ([]types.Collection, error) {
	return s.changeCollections(func(list []types.Collection) ([]types.Collection, error) {
		for i := range list {
			if list[i].Smart != nil {
				continue
			}
			for _, gameID := range list[i].Games {
				title := titles[gameID]
				if title == "" {
					continue
				}
				if list[i].Titles == nil {
					list[i].Titles = map[string]string{}
				}
				list[i].Titles[gameID] = title
			}
		}
		return list, nil
	})
}

// changeCollections applies change to a copy of the list and keeps the result
// only if settings save. It returns the list after the change.
func (s *Store) changeCollections(change func([]types.Collection) ([]types.Collection, error)) ([]types.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBuiltins()
	prev := s.data.Collections
	next, err := change(copyCollections(prev))
	if err != nil {
		return nil, err
	}
	s.data.Collections = next
	if err := s.save(); err != nil {
		s.data.Collections = prev
		return nil, err
	}
	return copyCollections(next), nil
}

func collectionIndex(list []types.Collection, id string) int {
	return slices.IndexFunc(list, func(c types.Collection) bool { return c.ID == id })
}

func manualCollection(list []types.Collection, id string) (int, error) {
	i := collectionIndex(list, id)
	if i < 0 {
		return 0, unknownCollection(id)
	}
	if list[i].Smart != nil {
		return 0, errors.New("a smart collection takes its games from its filter")
	}
	return i, nil
}

func unknownCollection(id string) error {
	return fmt.Errorf("unknown collection %q", id)
}

func cleanCollectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a collection needs a name")
	}
	if utf8.RuneCountInString(name) > types.MaxCollectionName {
		return "", fmt.Errorf("a collection name is at most %d characters", types.MaxCollectionName)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", errors.New("a collection name has a control character")
		}
	}
	return name, nil
}

func validCollectionGameID(gameID string) error {
	if gameID == "" || len(gameID) > types.MaxCollectionGameID {
		return fmt.Errorf("a game id is 1 to %d characters", types.MaxCollectionGameID)
	}
	return nil
}

// validateSmart checks a filter's values against the ones the UI knows, and
// that a collection it filters on is a manual one (so no filter depends on
// another smart filter).
func validateSmart(list []types.Collection, f types.SmartFilter) error {
	if len(f.Store) > 64 {
		return errors.New("a store id is at most 64 characters")
	}
	if f.BadgeKind != "" && !slices.Contains([]string{"rating", "verdict", "protection", "note", "platform", "drm", "anticheat"}, f.BadgeKind) {
		return fmt.Errorf("unknown badge kind %q", f.BadgeKind)
	}
	if f.BadgeValue != "" && (f.BadgeKind == "" || len(f.BadgeValue) > 128) {
		return errors.New("a badge value needs a kind and is at most 128 characters")
	}
	if f.BadgeKind == "rating" && f.BadgeValue != "" && !slices.Contains(types.CollectionTiers, f.BadgeValue) {
		return fmt.Errorf("unknown rating badge %q", f.BadgeValue)
	}
	if f.BadgeKind == "verdict" && f.BadgeValue != "" && !slices.Contains(types.CollectionDecks, f.BadgeValue) {
		return fmt.Errorf("unknown verdict badge %q", f.BadgeValue)
	}
	if f.Status != "" && f.Status != "none" && !types.ValidGameStatus(f.Status) {
		return fmt.Errorf("unknown status %q", f.Status)
	}
	if f.PlayedWithinDays < 0 || f.MinPlaytimeMinutes < 0 || f.YearFrom < 0 || f.YearTo < 0 {
		return errors.New("a smart filter's numbers can't be negative")
	}
	if f.YearFrom > 0 && f.YearTo > 0 && f.YearFrom > f.YearTo {
		return errors.New("a smart filter's years run from the earlier to the later")
	}
	if f.Collection != "" {
		i := collectionIndex(list, f.Collection)
		if i < 0 {
			return unknownCollection(f.Collection)
		}
		if list[i].Smart != nil {
			return errors.New("a smart filter can only test a manual collection")
		}
	}
	return nil
}

// copyCollections deep-copies the lists and filters, so callers never share
// a slice with the store.
func copyCollections(in []types.Collection) []types.Collection {
	out := make([]types.Collection, len(in))
	for i, c := range in {
		c.Games = slices.Clone(c.Games)
		c.Titles = maps.Clone(c.Titles)
		if c.Smart != nil {
			f := *c.Smart
			c.Smart = &f
		}
		out[i] = c
	}
	return out
}
