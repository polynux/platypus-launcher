// Package paths resolves Platypus's own folders from the environment (issue #41).
//
// Precedence, per folder:
//
//	data:    $PLATYPUS_DATA_DIR, else <data base>/platypus
//	config:  $PLATYPUS_CONFIG_DIR, else <config base>/platypus
//	cache:   $PLATYPUS_CACHE_DIR, else <cache base>/platypus
//	plugins: $PLATYPUS_PLUGINS, else <data>/plugins
//
// The bases are the platform's own (issue #82). On Linux they are the XDG
// folders: $XDG_DATA_HOME, $XDG_CONFIG_HOME, $XDG_CACHE_HOME, with
// ~/.local/share, ~/.config and ~/.cache as the defaults. On Windows config
// is %APPDATA% and data/cache are under %LOCALAPPDATA%; the choice is in
// defaults_windows.go.
//
// An XDG variable counts only when set to an absolute path; relative or
// empty means unset, per the XDG spec. PLATYPUS_* variables count only when
// absolute too, so a relative value can never escape the data folder.
//
// With no variables set every Linux path is byte-for-byte what it is today.
// When $XDG_DATA_HOME points elsewhere but only the legacy folder exists,
// Resolve keeps the legacy folder and logs one info line; user data is never
// moved automatically. settings.json stays in the data dir in this pass; the
// config resolver exists for later. Only truly re-fetchable data (game-info
// cache) belongs in the cache dir.
//
// Steam install roots are not resolved here: the Steam plugins report them
// (game.saveContext; internal/steam for the plugins themselves) and nothing
// in the core asks for them (issue #82).
package paths

import (
	"log"
	"os"
	"path/filepath"

	"platypus/internal/datadir"
)

// AppName is the folder name under the platform bases.
const AppName = "platypus"

// absEnv returns the value of key when it is set to an absolute path, else "".
// Relative or empty means unset, per the XDG spec.
func absEnv(key string) string {
	if v := os.Getenv(key); v != "" && filepath.IsAbs(v) {
		return v
	}
	return ""
}

// DataDirFor is Platypus's data folder for the user whose home is home,
// before the legacy-exists fallback (see Resolve).
func DataDirFor(home string) string {
	if v := absEnv("PLATYPUS_DATA_DIR"); v != "" {
		return v
	}
	return defaultDataDir(home)
}

// ConfigDirFor resolves the config folder (settings.json stays in the data
// dir in this pass; exposed for later).
func ConfigDirFor(home string) string {
	if v := absEnv("PLATYPUS_CONFIG_DIR"); v != "" {
		return v
	}
	return defaultConfigDir(home)
}

// CacheDirFor resolves the cache folder (only truly re-fetchable data).
func CacheDirFor(home string) string {
	if v := absEnv("PLATYPUS_CACHE_DIR"); v != "" {
		return v
	}
	return defaultCacheDir(home)
}

// PluginsDirFor resolves the plugins folder: $PLATYPUS_PLUGINS, else
// <dataDir>/plugins, the same rule plugins/install.sh uses.
func PluginsDirFor(dataDir string) string {
	if v := absEnv("PLATYPUS_PLUGINS"); v != "" {
		return v
	}
	return filepath.Join(dataDir, "plugins")
}

// Config carries the folders one run uses. Resolve fills it once, in
// cmd/platypus-core main, from the environment.
type Config struct {
	Home       string // defaults to os.UserHomeDir()
	DataDir    string
	ConfigDir  string
	CacheDir   string
	PluginsDir string
}

// Resolve reads the folders from the environment. With no variables set
// every path is what it is today. If $XDG_DATA_HOME points elsewhere and
// <XDG_DATA_HOME>/platypus does not exist but ~/.local/share/platypus does,
// the legacy folder wins and one info line is logged; data is never moved.
// A datadir.Use override (the migration case) always wins.
func Resolve() Config {
	home, _ := os.UserHomeDir()
	return ResolveFor(home)
}

// ResolveFor is Resolve for an explicit home (tests pass temp dirs).
func ResolveFor(home string) Config {
	data := DataDirFor(home)
	if legacy, ok := legacyDataDir(data, home); ok {
		log.Printf("paths: using legacy data folder %s (XDG_DATA_HOME points at %s)", legacy, data)
		data = legacy
	}
	if ov, ok := datadir.Override(); ok {
		data = ov
	}
	return Config{
		Home:       home,
		DataDir:    data,
		ConfigDir:  ConfigDirFor(home),
		CacheDir:   CacheDirFor(home),
		PluginsDir: PluginsDirFor(data),
	}
}

func existsDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
