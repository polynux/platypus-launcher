package steam

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"hydra-launcher/internal/types"
)

// ScanSteamLibrary scans local Steam library folders and installed ACF manifests.
func ScanSteamLibrary() ([]types.Game, error) {
	var games []types.Game
	home, err := os.UserHomeDir()
	if err != nil {
		return games, err
	}

	steamPaths := []string{
		filepath.Join(home, ".local/share/Steam"),
		filepath.Join(home, ".steam/steam"),
	}

	var steamRoot string
	for _, p := range steamPaths {
		if _, err := os.Stat(p); err == nil {
			steamRoot = p
			break
		}
	}
	if steamRoot == "" {
		return games, nil
	}

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

	// Official Steam Grid CDN URLs
	coverURL := "https://steamcdn-a.akamaihd.net/steam/apps/" + appID + "/library_600x900_2x.jpg"
	bannerURL := "https://steamcdn-a.akamaihd.net/steam/apps/" + appID + "/header.jpg"

	return types.Game{
		ID:          "steam_" + appID,
		Store:       types.StoreSteam,
		StoreAppID:  appID,
		Title:       name,
		CoverURL:    coverURL,
		BannerURL:   bannerURL,
		InstallDir:  fullInstallDir,
		IsInstalled: true,
		Settings: types.GameSettings{
			RunnerID:    "auto",
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
