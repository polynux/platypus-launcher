package types

// AchievementProgress is the user's own progress on a game, from a plugin's
// achievements capability (protocol 1.7). It replaces the catalogue-only
// GameDetails.Achievements when present.
type AchievementProgress struct {
	Total    int                   `json:"total"`
	Unlocked int                   `json:"unlocked"`
	List     []ProgressAchievement `json:"list"`
}

// ProgressAchievement is one achievement with the user's state. A hidden,
// locked one arrives with its name, description and icon blanked: the store
// keeps them secret until the user unlocks it.
type ProgressAchievement struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Hidden        bool     `json:"hidden"`
	Unlocked      bool     `json:"unlocked"`
	UnlockedAt    int64    `json:"unlocked_at"`              // unix seconds, 0 when locked or the store doesn't say
	GlobalPercent *float64 `json:"global_percent,omitempty"` // share of all players; nil when the store doesn't say
}

// RedeemResult is what a store says about a redeemed code (redeem capability,
// protocol 1.7). Titles are the games or items it granted, when it names them.
type RedeemResult struct {
	Message string   `json:"message"`
	Titles  []string `json:"titles"`
}
