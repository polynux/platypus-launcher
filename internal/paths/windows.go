package paths

import "path/filepath"

// Windows defaults, kept pure so Linux tests cover them (issue #82); the
// build-tagged defaults_windows.go supplies the platform values.

// windowsBase is native when the platform reports it, else home/rel.
func windowsBase(native, home, rel string) string {
	if native != "" {
		return native
	}
	return filepath.Join(home, rel)
}

// windowsAppDir is the app folder under the native base, else under
// home/rel.
func windowsAppDir(native, home, rel string) string {
	return filepath.Join(windowsBase(native, home, rel), AppName)
}

// windowsCacheDir nests under the data folder: data and cache both use the
// local app data base, so the cache must not resolve to data itself.
func windowsCacheDir(localAppData, home string) string {
	return filepath.Join(windowsAppDir(localAppData, home, filepath.Join("AppData", "Local")), "cache")
}

// windowsSaveDirs is the Windows known-folder set savebackup expands, with
// the profile defaults for the folders a host may not name. Folders that
// depend on the system (Public, ProgramData, Windows) are omitted when the
// environment does not provide them.
func windowsSaveDirs(home, appData, localAppData, public, programData, systemRoot string) map[string]string {
	appData = windowsBase(appData, home, filepath.Join("AppData", "Roaming"))
	localAppData = windowsBase(localAppData, home, filepath.Join("AppData", "Local"))
	dirs := map[string]string{
		"home":            home,
		"documents":       filepath.Join(home, "Documents"),
		"appData":         appData,
		"localAppData":    localAppData,
		"localAppDataLow": filepath.Join(filepath.Dir(localAppData), "LocalLow"),
	}
	if public != "" {
		dirs["public"] = public
	}
	if programData != "" {
		dirs["programData"] = programData
	}
	if systemRoot != "" {
		dirs["windir"] = systemRoot
	}
	return dirs
}
