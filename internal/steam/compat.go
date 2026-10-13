package steam

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// LibraryFolders lists the Steam library folders for a Steam root: the root
// itself plus the extra paths from steamapps/libraryfolders.vdf. Missing or
// unreadable VDF means just the root.
func LibraryFolders(root string) []string {
	if root == "" {
		return nil
	}
	out := []string{root}
	data, err := os.ReadFile(filepath.Join(root, "steamapps/libraryfolders.vdf"))
	if err != nil {
		return out
	}
	re := regexp.MustCompile(`"path"\s+"([^"]+)"`)
	seen := map[string]bool{filepath.Clean(root): true}
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		if len(m) < 2 {
			continue
		}
		p := filepath.Clean(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// CompatDataDir finds the compatdata folder for a Steam app id by searching
// every library folder: <library>/steamapps/compatdata/<appid>. It returns ""
// when none exists (native games, uninstalled games).
func CompatDataDir(root, appID string) string {
	if root == "" || appID == "" {
		return ""
	}
	for _, lib := range LibraryFolders(root) {
		dir := filepath.Join(lib, "steamapps", "compatdata", appID)
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	return ""
}

// CompatPrefix is the Wine prefix Proton uses for a Steam app id:
// <compatdata>/<appid>/pfx with the steamuser account. ok is false when the
// compatdata folder doesn't exist.
func CompatPrefix(root, appID string) (prefix string, ok bool) {
	dir := CompatDataDir(root, appID)
	if dir == "" {
		return "", false
	}
	return filepath.Join(dir, "pfx"), true
}

// SingleUserID resolves <storeUserId> for Steam games: the single
// userdata/<id> folder under the Steam root when there is exactly one,
// otherwise "". Extra non-numeric entries are ignored.
func SingleUserID(root string) string {
	if root == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(root, "userdata"))
	if err != nil {
		return ""
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		ids = append(ids, e.Name())
	}
	if len(ids) != 1 {
		return ""
	}
	sort.Strings(ids)
	return ids[0]
}
