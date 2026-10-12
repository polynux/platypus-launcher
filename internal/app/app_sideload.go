package app

import (
	"os"
	"path/filepath"
)

// Sideloaded games: picking the executable (issue #32). The working
// directory and Wine prefix live in the game's GameSettings (issues
// #33, #34); CustomGame stays {id, title, executable, added_at}.

// PickGameExecutable opens the native file picker for the sideloaded
// game's executable. Returns "" when the user cancels. Linux binaries
// have no extension, so no filter is forced: every file is shown, with
// Windows programs as a second choice.
func (a *App) PickGameExecutable(start string) (string, error) {
	def := start
	if st, err := os.Stat(start); err == nil && !st.IsDir() {
		def = filepath.Dir(start)
	} else if st, err := os.Stat(def); err != nil || !st.IsDir() {
		def, _ = os.UserHomeDir()
	}
	return a.host.OpenFile(FileDialog{
		Title:   "Select the game's executable",
		Default: def,
		Filters: []FileFilter{
			{Name: "All files", Extensions: []string{"*"}},
			{Name: "Windows programs", Extensions: []string{"exe"}},
		},
	})
}

