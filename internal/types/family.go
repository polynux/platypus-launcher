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
// is Hydra's game id; SecondsLeft counts down from the plugin's figure.
type FamilyWarning struct {
	Source      string `json:"source"`
	GameID      string `json:"game_id"`
	Message     string `json:"message"`
	SecondsLeft int    `json:"seconds_left"`
}
