package app

import "platypus/internal/types"

// Game status and notes (issue #51): the user's own progress and notes per
// game, stored by Platypus and never synced with a store. The library carries
// them on Game.Status and Game.UserNotes (see applyGameStatus).

// GetGameStatus returns a game's own status and notes ("", "" when unset).
func (a *App) GetGameStatus(gameID string) (types.GameStatusInfo, error) {
	return types.GameStatusInfo{Status: a.settings.GameStatus(gameID), UserNotes: a.settings.GameNotes(gameID)}, nil
}

// SetGameStatus saves a game's status ("", back to none, clears it) and
// returns its status and notes, so the UI updates without a rescan.
func (a *App) SetGameStatus(gameID, status string) (types.GameStatusInfo, error) {
	if err := a.settings.SetGameStatus(gameID, status); err != nil {
		return types.GameStatusInfo{}, err
	}
	return a.GetGameStatus(gameID)
}

// SetGameNotes saves a game's notes (blank clears them) and returns its
// status and notes, so the UI updates without a rescan.
func (a *App) SetGameNotes(gameID, notes string) (types.GameStatusInfo, error) {
	if err := a.settings.SetGameNotes(gameID, notes); err != nil {
		return types.GameStatusInfo{}, err
	}
	return a.GetGameStatus(gameID)
}

// applyGameStatus puts the status and notes on the library's games: the
// user's own values (settings rows) always win; a store's hint — the store's
// suggestion, carried on Game.StoreStatus/StoreNotes and never sent to the
// UI — fills a field the user never touched. A "none" status row and an
// empty notes row are the user's choice of nothing: they stay "" and the
// hint stays off. One snapshot read, never a call per game.
func (a *App) applyGameStatus(games []types.Game) {
	statuses, notes := a.settings.GameStatusNotes()
	for i := range games {
		if st, ok := statuses[games[i].ID]; ok {
			if st == "none" {
				games[i].Status = "" // the user chose no status over the hint
			} else {
				games[i].Status = st
			}
		} else {
			games[i].Status = games[i].StoreStatus
		}
		if n, ok := notes[games[i].ID]; ok {
			games[i].UserNotes = n
		} else {
			games[i].UserNotes = games[i].StoreNotes
		}
	}
}
