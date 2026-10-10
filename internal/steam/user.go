package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"platypus/internal/types"
)

// steamID64 offset: account ID (the userdata folder name) = SteamID64 - this.
const steamID64Base = 76561197960265728

// Root returns the Steam install directory, or "" if Steam isn't installed.
func Root() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, p := range []string{
		filepath.Join(home, ".local/share/Steam"),
		filepath.Join(home, ".steam/steam"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam"),
	} {
		if _, err := os.Stat(filepath.Join(p, "steamapps")); err == nil {
			return p
		}
	}
	return ""
}

// ActiveUserDir returns userdata/<accountid> for the most recently logged-in
// account. Falls back to the userdata folder with the newest localconfig.vdf.
func ActiveUserDir(root string) string {
	if root == "" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(root, "config/loginusers.vdf")); err == nil {
		if kv, err := ParseKV(string(data)); err == nil {
			if users := kv.Get("users"); users != nil {
				var best string
				var bestScore int64 = -1
				for sid, u := range users.Children {
					score, _ := strconv.ParseInt(u.String("Timestamp"), 10, 64)
					if u.String("MostRecent") == "1" {
						score = 1 << 62
					}
					if score > bestScore {
						best, bestScore = sid, score
					}
				}
				if id, err := strconv.ParseUint(best, 10, 64); err == nil && id > steamID64Base {
					dir := filepath.Join(root, "userdata", strconv.FormatUint(id-steamID64Base, 10))
					if _, err := os.Stat(dir); err == nil {
						return dir
					}
				}
			}
		}
	}
	entries, _ := filepath.Glob(filepath.Join(root, "userdata/*/config/localconfig.vdf"))
	var newest string
	var newestT time.Time
	for _, e := range entries {
		if st, err := os.Stat(e); err == nil && st.ModTime().After(newestT) {
			newest, newestT = filepath.Dir(filepath.Dir(e)), st.ModTime()
		}
	}
	return newest
}

// PlayStats is per-app playtime from the user's localconfig.vdf.
type PlayStats struct {
	LastPlayed     int64 // unix seconds
	PlaytimeMinute int
}

// ReadPlayStats parses Software/Valve/Steam/apps/<appid>/{LastPlayed,Playtime}.
func ReadPlayStats(userDir string) map[string]PlayStats {
	out := map[string]PlayStats{}
	data, err := os.ReadFile(filepath.Join(userDir, "config/localconfig.vdf"))
	if err != nil {
		return out
	}
	kv, err := ParseKV(string(data))
	if err != nil {
		return out
	}
	apps := kv.Get("UserLocalConfigStore", "Software", "Valve", "Steam", "apps")
	if apps == nil {
		return out
	}
	for id, app := range apps.Children {
		last, _ := strconv.ParseInt(app.String("LastPlayed"), 10, 64)
		mins, _ := strconv.Atoi(app.String("Playtime"))
		if last > 0 || mins > 0 {
			out[id] = PlayStats{LastPlayed: last, PlaytimeMinute: mins}
		}
	}
	return out
}

// ArtPath finds a cached library image for appID in appcache/librarycache.
// Steam stores files either flat (<appid>/library_600x900.jpg) or in hashed
// subfolders (<appid>/<sha>/library_capsule.jpg). Names are tried in order.
// Returns the path relative to the librarycache dir, or "".
func ArtPath(root, appID string, names ...string) string {
	base := filepath.Join(root, "appcache/librarycache", appID)
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(base, name)); err == nil {
			return filepath.Join(appID, name)
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(base, e.Name(), name)); err == nil {
					return filepath.Join(appID, e.Name(), name)
				}
			}
		}
	}
	return ""
}

// libraryCacheEntry is one ["key", {version, data}] pair from
// userdata/<id>/config/librarycache/<appid>.json.
type libraryCacheEntry struct {
	Data json.RawMessage `json:"data"`
}

type rawAchievement struct {
	ID          string  `json:"strID"`
	Name        string  `json:"strName"`
	Description string  `json:"strDescription"`
	Image       string  `json:"strImage"`
	Achieved    bool    `json:"bAchieved"`
	Unlocked    int64   `json:"rtUnlocked"`
	GlobalPct   float64 `json:"flAchieved"`
}

// LocalLibraryData reads the Steam client's per-user library cache for an app:
// achievement progress and the store description snippet. Steam only writes
// this file once the game has been opened in its library view.
func LocalLibraryData(userDir, appID string) (*types.Achievements, string) {
	data, err := os.ReadFile(filepath.Join(userDir, "config/librarycache", appID+".json"))
	if err != nil {
		return nil, ""
	}
	var pairs [][]json.RawMessage
	if json.Unmarshal(data, &pairs) != nil {
		return nil, ""
	}
	var ach *types.Achievements
	var snippet string
	for _, pair := range pairs {
		if len(pair) != 2 {
			continue
		}
		var key string
		var entry libraryCacheEntry
		if json.Unmarshal(pair[0], &key) != nil || json.Unmarshal(pair[1], &entry) != nil {
			continue
		}
		switch key {
		case "achievements":
			var a struct {
				Highlight    []rawAchievement `json:"vecHighlight"`
				Unachieved   []rawAchievement `json:"vecUnachieved"`
				AchievedHide []rawAchievement `json:"vecAchievedHidden"`
				Total        int              `json:"nTotal"`
				Achieved     int              `json:"nAchieved"`
			}
			if json.Unmarshal(entry.Data, &a) != nil || a.Total == 0 {
				continue
			}
			res := &types.Achievements{Total: a.Total, Unlocked: a.Achieved}
			for _, group := range [][]rawAchievement{a.Highlight, a.AchievedHide, a.Unachieved} {
				for _, r := range group {
					res.List = append(res.List, toAchievement(r))
				}
			}
			ach = res
		case "descriptions":
			var d struct {
				Snippet string `json:"strSnippet"`
			}
			if json.Unmarshal(entry.Data, &d) == nil {
				snippet = strings.TrimSpace(d.Snippet)
			}
		}
	}
	if ach != nil {
		// Unlocked first (newest first), then locked by global rarity (most common first).
		sort.SliceStable(ach.List, func(i, j int) bool {
			a, b := ach.List[i], ach.List[j]
			if a.Unlocked != b.Unlocked {
				return a.Unlocked
			}
			if a.Unlocked {
				return a.UnlockedAt > b.UnlockedAt
			}
			return a.GlobalPercent > b.GlobalPercent
		})
	}
	return ach, snippet
}

func toAchievement(r rawAchievement) types.Achievement {
	return types.Achievement{
		ID:            r.ID,
		Name:          strings.TrimSpace(r.Name),
		Description:   r.Description,
		Icon:          r.Image,
		Unlocked:      r.Achieved || r.Unlocked > 0,
		UnlockedAt:    r.Unlocked,
		GlobalPercent: r.GlobalPct,
	}
}
