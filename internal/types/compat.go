package types

// Compat is what the launcher shows about a game's compatibility and
// protection: community ratings, native Linux build, DRM and anti-cheat. Every
// badge names its source and when it was checked; a fact nobody reported is
// left out, never guessed.
type Compat struct {
	// Badges are compatibility facts supplied by provider plugins.
	Badges []CompatBadge `json:"badges"`
	// Notes and Links accompany provider facts on the game page.
	Notes []string     `json:"notes,omitempty"`
	Links []CompatLink `json:"links,omitempty"`
	// Report summarizes the provider's community reports, when available.
	Report *CompatReport `json:"report,omitempty"`
	// SteamAppID is the ID the ratings use, "" when the game has none.
	SteamAppID string `json:"steam_app_id,omitempty"`
	// SteamAppIDSource says where it came from: "user", "plugin" or "steam".
	SteamAppIDSource string `json:"steam_app_id_source,omitempty"`
	// Native marks a Linux build: shown instead of a community rating.
	Native bool `json:"native"`
	// Deck is Steam's Deck verdict, nil when there is none to show.
	Deck *DeckRating `json:"deck,omitempty"`
	// RatingsOnline says whether Platypus may fetch ratings from the network
	// (the "Fetch ratings online" setting).
	RatingsOnline bool `json:"ratings_online"`
	// DRM are the store's protection facts (plugin game.details, protocol 1.10).
	DRM []CompatBadge `json:"drm"`
	// DRMNotes is the store's free text about protection, "" when none.
	DRMNotes string `json:"drm_notes,omitempty"`
	// AntiCheat are anti-cheat systems: from AreWeAntiCheatYet for the game's
	// Steam AppID or Epic id, and from the installed files.
	AntiCheat []CompatBadge `json:"anticheat"`
	// Wiki credits PCGamingWiki for the facts above that came from it: nil when
	// none did. The UI shows the licence line and the page with it.
	Wiki *WikiCredit `json:"wiki,omitempty"`
}

// WikiCredit is the PCGamingWiki page a fact was read from, with when it was
// fetched (unix seconds) and the licence the content is under.
type WikiCredit struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	FetchedAt  int64  `json:"fetched_at"`
	LicenceURL string `json:"licence_url"`
}

// DeckRating is Steam's Deck verdict for an app. FetchedAt is when Steam
// answered, so the UI can say "as of"; Stale means a newer answer is being
// fetched. Notes are the short caveats worth showing, if any.
type DeckRating struct {
	Rating    string   `json:"rating"` // "verified" | "playable" | "unsupported"
	Notes     []string `json:"notes,omitempty"`
	FetchedAt int64    `json:"fetched_at"` // unix seconds
	Stale     bool     `json:"stale,omitempty"`
	URL       string   `json:"url"`
}

// CompatBadge is one compatibility or protection fact. Kind identifies the
// badge type, Source names where it came from, and CheckedAt is when (unix seconds).
type CompatBadge struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	Tone      string `json:"tone,omitempty"`
	Link      string `json:"link,omitempty"`
	Source    string `json:"source"`
	CheckedAt int64  `json:"checked_at"`
}

// CompatReport is a provider's report count and rating summary for one game.
type CompatReport struct {
	Count int    `json:"count"`
	State string `json:"state"`
	Tier  string `json:"tier,omitempty"`
}

type CompatLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
