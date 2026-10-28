package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"platypus/internal/types"
)

// umu-launcher (Open-Wine-Components/umu-launcher) runs Windows games with
// Proton inside the Steam Linux Runtime container, as Steam does, and applies
// protonfixes per game from the umu-database. See docs/umu.md for the survey.
//
// Platypus keeps its own prefix location and wrapper stack and only swaps the
// Proton/Wine invocation for `umu-run <exe> <args>` with the umu env
// (WINEPREFIX, GAMEID, STORE, PROTONPATH). Steam client launches are unchanged.

const (
	// UmuRunnerID is the runner setting (and scanned entry id) that runs a
	// Windows game through umu-launcher.
	UmuRunnerID = "umu"
	// UmuName is the scanned entry's name; the UI shows its own
	// "(recommended)" label for it (gameSettings:runner.umuOption).
	UmuName = "Proton via umu"
	// UmuLatest is the PROTONPATH when no Proton build is installed: umu
	// downloads the latest UMU-Proton on first launch (see docs/umu.md).
	UmuLatest = "UMU-Latest"
)

// FindUmu locates the umu-run binary: on PATH first, then the known install
// locations. The Lutris runtime copy is only read, never written.
func FindUmu() (string, bool) {
	home, _ := os.UserHomeDir()
	return FindUmuFor(home)
}

// FindUmuFor is FindUmu with an explicit home (tests pass a temp dir so the
// Lutris runtime check never sees the user's copy; fakes go on a temp PATH).
func FindUmuFor(home string) (string, bool) {
	if p, err := exec.LookPath("umu-run"); err == nil {
		return p, true
	}
	var cands []string
	if home != "" {
		cands = append(cands,
			filepath.Join(home, ".local", "bin", "umu-run"),
			// What Lutris bundles (lutris/util/wine/proton.py get_umu_path
			// also checks /app/share, /usr/local/share, /usr/share, /opt).
			filepath.Join(home, ".local", "share", "lutris", "runtime", "umu", "umu-run"),
			filepath.Join(home, ".local", "share", "lutris", "runtime", "umu", "umu_run.py"),
		)
	}
	cands = append(cands,
		"/app/share/umu/umu-run",
		"/usr/local/share/umu/umu-run",
		"/usr/share/umu/umu-run",
		"/opt/umu/umu-run",
	)
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, true
		}
	}
	return "", false
}

// IsUmuPath reports whether a runner path is the umu launcher.
func IsUmuPath(path string) bool {
	return strings.HasSuffix(path, "/umu-run") || strings.HasSuffix(path, "/umu_run.py") || path == "umu-run" || path == "umu_run.py"
}

// IsUmu reports whether a runner runs games through umu-launcher.
func IsUmu(r types.ProtonVersion) bool {
	return r.ID == UmuRunnerID || IsUmuPath(r.Path)
}

// UmuEntry is the runner list entry for an installed umu-run.
func UmuEntry(umuPath string) types.ProtonVersion {
	return types.ProtonVersion{ID: UmuRunnerID, Name: UmuName, Path: umuPath}
}

// UmuProton picks the Proton build umu runs: the auto pick (same rule as a
// direct launch, issue #146's default included). It returns false when no
// Proton build is installed or the auto pick is not a Proton build (plain
// Wine): the caller then uses UmuLatest.
func UmuProton(runners []types.ProtonVersion, defaultID string) (types.ProtonVersion, bool) {
	var rest []types.ProtonVersion
	for _, r := range runners {
		if !IsUmu(r) {
			rest = append(rest, r)
		}
	}
	p, ok := PickAuto(rest, defaultID)
	if !ok || !strings.HasSuffix(p.Path, "proton") {
		return types.ProtonVersion{}, false
	}
	return p, true
}

// UmuProtonPath is the PROTONPATH for umu: the Proton build's folder (the
// directory holding its `proton` file), or UMU-Latest when no Proton build
// applies. UMU-Latest downloads on first launch (see docs/umu.md).
func UmuProtonPath(proton types.ProtonVersion) string {
	if strings.HasSuffix(proton.Path, "proton") && proton.Path != "" {
		return filepath.Dir(proton.Path)
	}
	return UmuLatest
}

// StoreForUmu maps a game's store to umu's STORE (umu-database ids). Steam
// and sideloaded games carry no STORE: their GAMEID already names the fix.
func StoreForUmu(game types.Game) string {
	switch strings.ToLower(strings.TrimSpace(string(game.Store))) {
	case "gog":
		return "gog"
	case "epic", "egs":
		return "egs"
	case "ubisoft", "ubi":
		return "ubisoft"
	case "amazon":
		return "amazon"
	case "humble", "humblebundle":
		return "humble"
	case "itchio", "itch":
		return "itchio"
	case "battlenet", "battle.net":
		return "battlenet"
	}
	return ""
}

// GameIDForUmu is the GAMEID umu applies protonfixes with: umu-<Steam AppID>
// for Steam games and sideloaded games with a compat AppID, else umu-0.
// GOG/Epic games keep umu-0 with their STORE: the umu-database lookup is
// skipped (no network at launch), so no fix is applied rather than a wrong
// one. See docs/umu.md.
func GameIDForUmu(game types.Game, compatAppID string) string {
	if id := strings.TrimSpace(game.StoreAppID); id != "" && id != string(types.StoreCustom) &&
		(game.Store == types.StoreSteam || game.Source == types.SourceSteam) {
		return "umu-" + id
	}
	if id := strings.TrimSpace(compatAppID); id != "" {
		return "umu-" + id
	}
	return "umu-0"
}

// EffectiveUmuPrefix is the WINEPREFIX umu runs with for a game's prefix.
// umu expects the pfx dir itself, while direct Proton launches keep the Wine
// prefix in <compat>/pfx: a prefix that already has that layout (made by a
// direct Proton launch) keeps working by pointing at it, otherwise the prefix
// itself is the Wine prefix. No migration: existing prefixes keep working.
func EffectiveUmuPrefix(prefixPath string) string {
	if prefixPath == "" {
		return ""
	}
	if st, err := os.Stat(filepath.Join(prefixPath, "pfx")); err == nil && st.IsDir() {
		return filepath.Join(prefixPath, "pfx")
	}
	return prefixPath
}

// SelectUmu decides whether a runner setting runs through umu and which
// Proton build umu runs: explicit "umu", or "auto" with umu installed (then
// umu wraps the auto Proton pick, or the default one). An explicit
// Proton/Wine choice always runs directly, and so does an automatic game whose
// default runner is a Wine build umu can't run (issue #146). ok is false when
// umu is not installed.
func SelectUmu(runnerID string, runners []types.ProtonVersion, defaultID string) (umu, proton types.ProtonVersion, ok bool) {
	var found *types.ProtonVersion
	for i := range runners {
		if IsUmu(runners[i]) {
			found = &runners[i]
			break
		}
	}
	if found == nil {
		return types.ProtonVersion{}, types.ProtonVersion{}, false
	}
	if runnerID != UmuRunnerID && !IsAuto(runnerID) {
		return types.ProtonVersion{}, types.ProtonVersion{}, false
	}
	if IsAuto(runnerID) {
		// A default umu can't run — a Wine build — runs directly (issue #146):
		// the picked runner must be the one that runs, not UMU-Latest.
		for _, r := range runners {
			if (r.ID == defaultID || r.Name == defaultID) && !IsUmu(r) && !strings.HasSuffix(r.Path, "proton") {
				return types.ProtonVersion{}, types.ProtonVersion{}, false
			}
		}
	}
	p, _ := UmuProton(runners, defaultID)
	return *found, p, true
}

// BuildUmuCommandFor builds the launch command for a Windows game run through
// umu-launcher. The wrapper stack is unchanged (gamemoderun → gamescope →
// mangohud → umu-run <exe> <args>); only the runner invocation and its env
// differ from BuildCommandFor, which supplies the wrappers and creates the
// prefix. proton is the Proton build umu runs (zero means UMU-Latest) and
// compatAppID the game's Steam compat AppID ("", when none).
func BuildUmuCommandFor(game types.Game, proton types.ProtonVersion, umuPath, compatAppID string, target Target, preview bool, prefixesFolder, home string) ([]string, map[string]string, error) {
	args, env, err := BuildCommandFor(game, UmuEntry(umuPath), target, preview, prefixesFolder, home)
	if err != nil {
		return nil, nil, err
	}
	prefix := PrefixPath(game, home, prefixesFolder)
	env["WINEPREFIX"] = EffectiveUmuPrefix(prefix)
	env["STEAM_COMPAT_DATA_PATH"] = prefix
	env["GAMEID"] = GameIDForUmu(game, compatAppID)
	if s := StoreForUmu(game); s != "" {
		env["STORE"] = s
	} else {
		delete(env, "STORE")
	}
	env["PROTONPATH"] = UmuProtonPath(proton)
	return args, env, nil
}
