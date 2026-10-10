// Package datadir names Platypus's data folder, ~/.local/share/platypus.
//
// It is a leaf package so the settings, runner and session packages agree on
// one answer. The only thing that moves it is Use, which the app calls when
// bringing the previous data folder over failed and the old one is kept in
// place (see internal/migrate).
package datadir

import (
	"path/filepath"
	"sync"
)

// Name is the folder's name under ~/.local/share.
const Name = "platypus"

var (
	mu       sync.RWMutex
	override string
)

// Default is the data folder of the user whose home is home.
func Default(home string) string {
	return filepath.Join(home, ".local/share", Name)
}

// Path is the data folder in use: Default unless Use was called.
func Path(home string) string {
	mu.RLock()
	defer mu.RUnlock()
	if override != "" {
		return override
	}
	return Default(home)
}

// Use makes dir the data folder for the rest of the run ("" undoes it).
func Use(dir string) {
	mu.Lock()
	override = dir
	mu.Unlock()
}
