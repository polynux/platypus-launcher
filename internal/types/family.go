package types

// SharedBy names the family member who lends a game (the `family` capability,
// protocol 1.8, docs/plugins/protocol.md "Family"). A game with it is borrowed.
type SharedBy struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FamilyMember is one member of the store's family group, as the Families
// section shows it. Role is "adult", "child" or "" when the plugin said
// neither. State is the plugin's own words ("online", "playing X"...).
type FamilyMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	AvatarURL string `json:"avatar_url,omitempty"`
	State     string `json:"state,omitempty"`
}

// FamilyWarning is a family.warning notification: the plugin tells the user
// that a game they are playing is about to be taken back by its owner. Game
// is Platypus's game id; SecondsLeft counts down from the plugin's figure.
type FamilyWarning struct {
	Source      string `json:"source"`
	GameID      string `json:"game_id"`
	Message     string `json:"message"`
	SecondsLeft int    `json:"seconds_left"`
}

// GameCopy is one family member's copy of a borrowed game (the `family`
// capability, protocol 1.8). Copies can differ: DLC, a note, or whether the
// copy can be borrowed now (Available) and whether it is in use (InUse).
type GameCopy struct {
	OwnerID   string    `json:"owner_id"`
	OwnerName string    `json:"owner_name"`
	DLC       []GameDLC `json:"dlc,omitempty"`
	Note      string    `json:"note,omitempty"`
	Available bool      `json:"available"`
	InUse     bool      `json:"in_use,omitempty"`
	// Exclude says why part of the copy isn't lent (1.11), shown as text.
	Exclude string `json:"exclude,omitempty"`
}

// GameDLC is an add-on a copy has. Kind is "dlc", "soundtrack" or "other"
// (1.11); a copy's soundtracks are counted apart in a CopyComparison.
type GameDLC struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// CopyComparison sets each other copy of a borrowed game against a baseline
// copy: the one the user prefers, or else the copy with the most add-ons (ties
// go to the lower owner id). Diffs has one entry per other copy, in copy order.
type CopyComparison struct {
	Baseline string     `json:"baseline"`
	Diffs    []CopyDiff `json:"diffs"`
}

// CopyDiff counts how one copy's add-ons differ from the baseline's. The
// "only" counts are add-ons this copy has and the baseline lacks ("here") and
// the reverse ("baseline"). Soundtracks are counted apart from the rest.
type CopyDiff struct {
	OwnerID                 string `json:"owner_id"`
	OnlyHere                int    `json:"only_here"`
	OnlyBaseline            int    `json:"only_baseline"`
	Shared                  int    `json:"shared"`
	SoundtracksOnlyHere     int    `json:"soundtracks_only_here"`
	SoundtracksOnlyBaseline int    `json:"soundtracks_only_baseline"`
	SoundtracksShared       int    `json:"soundtracks_shared"`
}
