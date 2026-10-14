package types

// Per-game status (issue #51): where the user stands with a game. "" means
// none (the game was never given one); the UI shows no badge for it.
const (
	GameStatusBacklog   = "backlog"
	GameStatusPlaying   = "playing"
	GameStatusFinished  = "finished"
	GameStatusAbandoned = "abandoned"
)

// ValidGameStatus reports whether s is one of the game statuses.
func ValidGameStatus(s string) bool {
	return s == GameStatusBacklog || s == GameStatusPlaying || s == GameStatusFinished || s == GameStatusAbandoned
}
