package runner

import (
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/types"
)

// ScanProtonVersions scans Steam, compatibilitytools.d, and Lutris for Proton/Wine runners.
func ScanProtonVersions() []types.ProtonVersion {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return ScanProtonVersionsFor(home)
}

// ScanProtonVersionsFor is ScanProtonVersions with an explicit home (issue #41).
func ScanProtonVersionsFor(home string) []types.ProtonVersion {
	var versions []types.ProtonVersion
	seen := make(map[string]bool)

	// 1. Steam Common Proton installations
	steamCommonPaths := []string{
		filepath.Join(home, ".local/share/Steam/steamapps/common"),
		filepath.Join(home, ".steam/steam/steamapps/common"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common"),
	}

	for _, basePath := range steamCommonPaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, "Proton") {
				protonBin := filepath.Join(basePath, name, "proton")
				if _, err := os.Stat(protonBin); err == nil {
					id := "steam:" + name
					if !seen[id] {
						seen[id] = true
						versions = append(versions, types.ProtonVersion{
							ID:       id,
							Name:     name,
							Path:     protonBin,
							IsCustom: false,
						})
					}
				}
			}
		}
	}

	// 2. Compatibilitytools.d (GE-Proton, Proton-TKG, etc.)
	compatDirs := []string{
		filepath.Join(home, ".local/share/Steam/compatibilitytools.d"),
		filepath.Join(home, ".steam/root/compatibilitytools.d"),
		filepath.Join(home, ".steam/compatibilitytools.d"),
	}

	for _, cDir := range compatDirs {
		entries, err := os.ReadDir(cDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			protonBin := filepath.Join(cDir, name, "proton")
			if _, err := os.Stat(protonBin); err == nil {
				id := "compat:" + name
				if !seen[id] {
					seen[id] = true
					versions = append(versions, types.ProtonVersion{
						ID:       id,
						Name:     name + " (Custom/GE)",
						Path:     protonBin,
						IsCustom: true,
					})
				}
			}
		}
	}

	// 3. Lutris Wine/GE Runners
	lutrisWineDir := filepath.Join(home, ".local/share/lutris/runners/wine")
	if entries, err := os.ReadDir(lutrisWineDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			wineBin := filepath.Join(lutrisWineDir, name, "bin/wine")
			if _, err := os.Stat(wineBin); err == nil {
				id := "lutris:" + name
				if !seen[id] {
					seen[id] = true
					versions = append(versions, types.ProtonVersion{
						ID:       id,
						Name:     name + " (Lutris Runner)",
						Path:     wineBin,
						IsCustom: true,
					})
				}
			}
		}
	}

	// 4. System Wine if available
	if _, err := os.Stat("/usr/bin/wine"); err == nil {
		versions = append(versions, types.ProtonVersion{
			ID:       "system:wine",
			Name:     "System Wine (/usr/bin/wine)",
			Path:     "/usr/bin/wine",
			IsSystem: true,
		})
	}

	// 5. umu-launcher when installed (issue #48): offered as
	// "Proton via umu" for Windows games. It never wins the auto pick
	// (PickAuto skips it); auto prefers it explicitly in SelectUmu.
	if umuPath, ok := FindUmuFor(home); ok {
		versions = append(versions, UmuEntry(umuPath))
	}

	// Mark the runner "auto" resolves to so the UI can show it.
	if pick, ok := PickAuto(versions); ok {
		for i := range versions {
			versions[i].IsDefault = versions[i].ID == pick.ID
		}
	}
	return versions
}
