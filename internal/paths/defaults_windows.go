//go:build windows

package paths

import (
	"os"
	"path/filepath"
)

// Windows uses the user's native folders (issue #82): config is %APPDATA%
// (os.UserConfigDir), which roams with the user's profile and only holds
// settings; data and cache are under %LOCALAPPDATA% (os.UserCacheDir: Go
// exposes the local app data folder only there), machine-local like the Linux
// data folder because backups, the library and prefixes must not roam. Cache
// nests under data so the two never resolve alike. XDG does not apply.

func userConfigBase() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	return ""
}

func userCacheBase() string {
	if d, err := os.UserCacheDir(); err == nil {
		return d
	}
	return ""
}

// BaseDataDirFor returns the user's local app data base: %LOCALAPPDATA%.
func BaseDataDirFor(home string) string {
	return windowsBase(userCacheBase(), home, filepath.Join("AppData", "Local"))
}

// BaseConfigDirFor returns the user's roaming app data base: %APPDATA%.
func BaseConfigDirFor(home string) string {
	return windowsBase(userConfigBase(), home, filepath.Join("AppData", "Roaming"))
}

func defaultDataDir(home string) string {
	return windowsAppDir(userCacheBase(), home, filepath.Join("AppData", "Local"))
}

func defaultConfigDir(home string) string {
	return windowsAppDir(userConfigBase(), home, filepath.Join("AppData", "Roaming"))
}

func defaultCacheDir(home string) string {
	return windowsCacheDir(userCacheBase(), home)
}

// SaveDirsFor returns the OS-native folders games keep saves in, keyed by the
// root labels savebackup expands (the Ludusavi <win…> placeholders); the
// store plugins add their own roots on top (issue #82).
func SaveDirsFor(home string) map[string]string {
	return windowsSaveDirs(home,
		os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"),
		os.Getenv("PUBLIC"), os.Getenv("ProgramData"), os.Getenv("SystemRoot"))
}

// legacyDataDir: there is no pre-XDG Windows folder; the native one always
// wins.
func legacyDataDir(data, home string) (string, bool) {
	return "", false
}
