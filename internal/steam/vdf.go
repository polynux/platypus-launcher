package steam

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"platypus/internal/types"
)

// ScanSteamLibrary scans local Steam library folders and installed ACF manifests.
func ScanSteamLibrary() ([]types.Game, error) {
	var games []types.Game
	steamRoot := Root()
	if steamRoot == "" {
		return games, nil
	}
	stats := ReadPlayStats(ActiveUserDir(steamRoot))

	// Find library folders
	libraryFolders := []string{steamRoot}
	vdfPath := filepath.Join(steamRoot, "steamapps/libraryfolders.vdf")
	if data, err := os.ReadFile(vdfPath); err == nil {
		re := regexp.MustCompile(`"path"\s+"([^"]+)"`)
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			if len(m) > 1 {
				libraryFolders = append(libraryFolders, m[1])
			}
		}
	}

	seenAppIDs := make(map[string]bool)

	for _, lib := range libraryFolders {
		steamapps := filepath.Join(lib, "steamapps")
		files, err := os.ReadDir(steamapps)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasPrefix(f.Name(), "appmanifest_") || !strings.HasSuffix(f.Name(), ".acf") {
				continue
			}

			acfPath := filepath.Join(steamapps, f.Name())
			game, err := parseAcfFile(acfPath, steamapps)
			if err != nil || game.StoreAppID == "" {
				continue
			}

			// Filter out Steam runtimes, common redistributables, Proton itself
			if isSystemApp(game.StoreAppID, game.Title) {
				continue
			}

			if !seenAppIDs[game.StoreAppID] {
				seenAppIDs[game.StoreAppID] = true
				applyArt(&game, steamRoot)
				if st, ok := stats[game.StoreAppID]; ok {
					game.LastPlayed = st.LastPlayed
					game.PlaytimeMin = st.PlaytimeMinute
				}
				games = append(games, game)
			}
		}
	}

	return games, nil
}

func parseAcfFile(path string, steamapps string) (types.Game, error) {
	file, err := os.Open(path)
	if err != nil {
		return types.Game{}, err
	}
	defer file.Close()

	var appID, name, installDir string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, `"appid"`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				appID = parts[3]
			}
		} else if strings.HasPrefix(line, `"name"`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				name = parts[3]
			}
		} else if strings.HasPrefix(line, `"installdir"`) {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				installDir = parts[3]
			}
		}
	}

	fullInstallDir := filepath.Join(steamapps, "common", installDir)

	return types.Game{
		ID:          "steam_" + appID,
		Source:      types.SourceSteam,
		Store:       types.StoreSteam,
		StoreAppID:  appID,
		Title:       name,
		InstallDir:  fullInstallDir,
		IsInstalled: true,
		Settings: types.GameSettings{
			RunnerID:    "",
			UseGameMode: true,
			UseMangoHud: false,
		},
	}, nil
}

func isSystemApp(appID, title string) bool {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "proton") ||
		strings.Contains(lower, "steam linux runtime") ||
		strings.Contains(lower, "steamworks shared") ||
		strings.Contains(lower, "steam controller") ||
		strings.Contains(lower, "steamvr") {
		return true
	}
	// Common utility Steam AppIDs
	switch appID {
	case "228980", "1070560", "1391110", "1493710", "894760", "1887720", "105600":
		return true
	}
	return false
}

// ArtURLPrefix is the asset-server route that serves Steam's local library
// cache (see main.go). Paths after it are relative to appcache/librarycache.
const ArtURLPrefix = "/platypus-art/steam/"

const cdn = "https://steamcdn-a.akamaihd.net/steam/apps/"

// applyArt prefers the art the Steam client already downloaded: newer titles
// only have hashed CDN URLs, so the fixed CDN paths 404 for them.
func applyArt(g *types.Game, root string) {
	pick := func(fallback string, names ...string) string {
		if rel := ArtPath(root, g.StoreAppID, names...); rel != "" {
			return ArtURLPrefix + filepath.ToSlash(rel)
		}
		return fallback
	}
	g.CoverURL = pick(cdn+g.StoreAppID+"/library_600x900_2x.jpg", "library_600x900_2x.jpg", "library_600x900.jpg", "library_capsule.jpg")
	g.BannerURL = pick(cdn+g.StoreAppID+"/header.jpg", "header.jpg", "library_header.jpg")
	g.HeroURL = pick(cdn+g.StoreAppID+"/library_hero.jpg", "library_hero.jpg")
}
