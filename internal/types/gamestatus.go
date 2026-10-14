package types

// Per-game status (issue #51): where the user stands with a game. "" means
// none (the game was never given one); the UI shows no badge for it.
const (
	GameStatusBacklog   = "backlog"
	GameStatusPlaying   = "playing"
	GameStatusFinished  = "finished"
	GameStatusAbandoned = "abandoned"
)

// MaxGameNotes caps one game's user notes in bytes: they live in
// settings.json, which is read whole on every load, and a store's hint is
// capped the same way (protocol 1.16).
const MaxGameNotes = 8 << 10

// ValidGameStatus reports whether s is one of the game statuses.
func ValidGameStatus(s string) bool {
	return s == GameStatusBacklog || s == GameStatusPlaying || s == GameStatusFinished || s == GameStatusAbandoned
}

// GameStatusInfo is a game's own status and notes (issue #51): what the
// game page shows and saves. Status is "" for none; UserNotes is plain text.
type GameStatusInfo struct {
	Status    string `json:"status"`
	UserNotes string `json:"user_notes"`
}
