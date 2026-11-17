//go:build !windows

package paths

import "path/filepath"

// xdgUserDir returns $envKey when it holds an absolute path, else
// home/fallback. An XDG variable counts only when absolute, per the spec.
func xdgUserDir(envKey, home, fallback string) string {
	if v := absEnv(envKey); v != "" {
		return v
	}
	return filepath.Join(home, fallback)
}

// BaseDataDirFor returns the user's data base: $XDG_DATA_HOME, else
// ~/.local/share.
func BaseDataDirFor(home string) string {
	return xdgUserDir("XDG_DATA_HOME", home, filepath.Join(".local", "share"))
}

// BaseConfigDirFor returns the user's config base: $XDG_CONFIG_HOME, else
// ~/.config.
func BaseConfigDirFor(home string) string {
	return xdgUserDir("XDG_CONFIG_HOME", home, ".config")
}

func defaultDataDir(home string) string {
	return filepath.Join(BaseDataDirFor(home), AppName)
}

func defaultConfigDir(home string) string {
	return filepath.Join(BaseConfigDirFor(home), AppName)
}

func defaultCacheDir(home string) string {
	return filepath.Join(xdgUserDir("XDG_CACHE_HOME", home, ".cache"), AppName)
}

// SaveDirsFor returns the OS-native folders games keep saves in, keyed by the
// root labels savebackup expands (the Ludusavi placeholders). Linux uses the
// XDG bases; the store plugins add their own roots on top (issue #82).
func SaveDirsFor(home string) map[string]string {
	return map[string]string{
		"home":      home,
		"xdgData":   BaseDataDirFor(home),
		"xdgConfig": BaseConfigDirFor(home),
	}
}

// legacyDataDir is the pre-XDG data folder when XDG points elsewhere and only
// the legacy folder exists. Data is never moved automatically (issue #41).
func legacyDataDir(data, home string) (string, bool) {
	if absEnv("PLATYPUS_DATA_DIR") != "" || absEnv("XDG_DATA_HOME") == "" {
		return "", false
	}
	legacy := filepath.Join(home, ".local", "share", AppName)
	if data != legacy && !existsDir(data) && existsDir(legacy) {
		return legacy, true
	}
	return "", false
}
