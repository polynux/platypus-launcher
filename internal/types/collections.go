package types

// Collections (issue #45): the user's lists of games, stored in settings and
// keyed by game id. A manual collection holds game ids; a smart one holds a
// SmartFilter over facts the library already has and no ids of its own. The
// UI evaluates the filter on the loaded library, so the core never answers a
// per-game question for it.

// Built-in collections: seeded by settings, pinned by default. Their name and
// filter can't change; they can be pinned, unpinned and reordered.
const (
	CollectionFavourites = "favourites" // manual
	CollectionInstalled  = "installed"  // smart: installed
	CollectionRecent     = "recent"     // smart: played in the last 30 days
)

const (
	// MaxCollectionName bounds a collection's name (bytes, trimmed).
	MaxCollectionName = 80
	// MaxCollections bounds how many collections settings keep.
	MaxCollections = 200
	// MaxCollectionGameID bounds a game id held in a manual collection.
	MaxCollectionGameID = 256
)

// Collection is a named list of games. Games holds a manual collection's
// game ids in the order they were added; it keeps ids of games that are no
// longer in the library, which the UI lists as missing. Titles keeps each
// game's last known title (by game id), so a missing game still has a name;
// settings written before titles existed have none, and the UI falls back to
// the id. Smart is set on a smart collection (and only then): its games are
// the library games the filter matches.
type Collection struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Builtin bool              `json:"builtin,omitempty"`
	Pinned  bool              `json:"pinned,omitempty"`
	Games   []string          `json:"games,omitempty"`
	Titles  map[string]string `json:"titles,omitempty"`
	Smart   *SmartFilter      `json:"smart,omitempty"`
}

// SmartFilter is a saved query over library facts. Every set field must
// match; an unset field matches anything. Values are the ones the library
// already shows: a store id ("custom" for sideloaded games), a ProtonDB tier
// ("native" and "none" too), a Steam Deck verdict, a game status ("none" for
// no status).
type SmartFilter struct {
	Store     string `json:"store,omitempty"`
	Installed *bool  `json:"installed,omitempty"`
	// Played true: the game has a last-played time or playtime; false: never played.
	Played *bool `json:"played,omitempty"`
	// PlayedWithinDays keeps the games played in the last N days (0: any time).
	PlayedWithinDays   int    `json:"played_within_days,omitempty"`
	MinPlaytimeMinutes int    `json:"min_playtime_minutes,omitempty"`
	Tier               string `json:"tier,omitempty"`
	Deck               string `json:"deck,omitempty"`
	YearFrom           int    `json:"year_from,omitempty"`
	YearTo             int    `json:"year_to,omitempty"`
	Status             string `json:"status,omitempty"`
	// Collection keeps the members of a manual collection (by its id).
	Collection string `json:"collection,omitempty"`
}

// CollectionTiers are the tier values a smart filter takes, besides "".
var CollectionTiers = []string{"platinum", "gold", "silver", "bronze", "borked", "native", "none"}

// CollectionDecks are the Steam Deck verdicts a smart filter takes, besides "".
var CollectionDecks = []string{"verified", "playable", "unsupported"}
