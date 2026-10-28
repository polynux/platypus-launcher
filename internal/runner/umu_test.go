package runner

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"platypus/internal/types"
)

// fakeBinDir creates executable fakes for the given binary names and points
// PATH at them (issue #48: fake umu-run binaries in a temp PATH only).
func fakeBinDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nexec echo fake\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func umuGame() types.Game {
	return types.Game{ID: "umu-game", Title: "Umu Game", Source: types.SourceCustom, Store: types.StoreCustom, Executable: "/games/g.exe"}
}

func umuProton(home string) types.ProtonVersion {
	dir := filepath.Join(home, ".local/share/Steam/compatibilitytools.d", "GE-Proton10-34")
	return types.ProtonVersion{ID: "compat:GE-Proton10-34", Name: "GE-Proton10-34 (Custom/GE)", Path: filepath.Join(dir, "proton"), IsCustom: true}
}

// A sideloaded game with a compat AppID gets umu-<appid> and no STORE.
func TestUmuSideloadWithCompatAppID(t *testing.T) {
	home := t.TempDir()
	bin := fakeBinDir(t, "umu-run")
	umu, ok := FindUmuFor(home)
	if !ok || umu != filepath.Join(bin, "umu-run") {
		t.Fatalf("FindUmuFor = %q, %v", umu, ok)
	}
	args, env, err := BuildUmuCommandFor(umuGame(), umuProton(home), umu, "620", Target{Exe: "/games/g.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if env["GAMEID"] != "umu-620" {
		t.Fatalf("GAMEID = %q, want umu-620", env["GAMEID"])
	}
	if _, ok := env["STORE"]; ok {
		t.Fatalf("STORE = %q, want unset for a compat-AppID game", env["STORE"])
	}
	prefix := PrefixPath(umuGame(), home, "")
	if env["WINEPREFIX"] != prefix || env["STEAM_COMPAT_DATA_PATH"] != prefix {
		t.Fatalf("prefix env = %q / %q, want %q", env["WINEPREFIX"], env["STEAM_COMPAT_DATA_PATH"], prefix)
	}
	if want := filepath.Dir(umuProton(home).Path); env["PROTONPATH"] != want {
		t.Fatalf("PROTONPATH = %q, want %q", env["PROTONPATH"], want)
	}
	if last := args[len(args)-1]; last != "/games/g.exe" {
		t.Fatalf("command tail = %q, want the exe", last)
	}
	if !slices.Contains(args, umu) {
		t.Fatalf("command %q has no umu-run", args)
	}
}

// Without a compat AppID a sideloaded game gets umu-0 (no fix rather than a
// wrong one; the umu-database lookup is skipped, see docs/umu.md).
func TestUmuSideloadWithoutCompatAppID(t *testing.T) {
	home := t.TempDir()
	fakeBinDir(t, "umu-run")
	umu, _ := FindUmuFor(home)
	_, env, err := BuildUmuCommandFor(umuGame(), umuProton(home), umu, "", Target{Exe: "/games/g.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if env["GAMEID"] != "umu-0" {
		t.Fatalf("GAMEID = %q, want umu-0", env["GAMEID"])
	}
}

// A plugin Steam game gets umu-<appid> with no STORE.
func TestUmuPluginSteamGame(t *testing.T) {
	home := t.TempDir()
	fakeBinDir(t, "umu-run")
	umu, _ := FindUmuFor(home)
	game := types.Game{ID: "s1", Title: "Steam Plugin Game", Source: "steam-plugin", Store: types.StoreSteam, StoreAppID: "765", Executable: "/g/game.exe"}
	_, env, err := BuildUmuCommandFor(game, umuProton(home), umu, "", Target{Exe: "/g/game.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if env["GAMEID"] != "umu-765" {
		t.Fatalf("GAMEID = %q, want umu-765", env["GAMEID"])
	}
	if _, ok := env["STORE"]; ok {
		t.Fatalf("STORE = %q, want unset for a Steam game", env["STORE"])
	}
}

// A GOG game gets umu-0 with STORE=gog (mapping skipped offline).
func TestUmuGogGame(t *testing.T) {
	home := t.TempDir()
	fakeBinDir(t, "umu-run")
	umu, _ := FindUmuFor(home)
	game := types.Game{ID: "g1", Title: "GOG Game", Source: "gog-plugin", Store: types.StoreGOG, StoreAppID: "1234", Executable: "/g/game.exe"}
	_, env, err := BuildUmuCommandFor(game, umuProton(home), umu, "", Target{Exe: "/g/game.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if env["GAMEID"] != "umu-0" {
		t.Fatalf("GAMEID = %q, want umu-0", env["GAMEID"])
	}
	if env["STORE"] != "gog" {
		t.Fatalf("STORE = %q, want gog", env["STORE"])
	}
}

// A prefix made by a direct Proton launch (<compat>/pfx) keeps working under
// umu: WINEPREFIX points at the pfx dir, while STEAM_COMPAT_DATA_PATH stays
// today's compat folder. New prefixes use the prefix itself.
func TestUmuProtonPfxLayout(t *testing.T) {
	home := t.TempDir()
	fakeBinDir(t, "umu-run")
	umu, _ := FindUmuFor(home)
	game := umuGame()
	prefix := PrefixPath(game, home, "")
	if err := os.MkdirAll(filepath.Join(prefix, "pfx", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, env, err := BuildUmuCommandFor(game, umuProton(home), umu, "", Target{Exe: "/games/g.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if env["WINEPREFIX"] != filepath.Join(prefix, "pfx") {
		t.Fatalf("WINEPREFIX = %q, want the legacy pfx dir", env["WINEPREFIX"])
	}
	if env["STEAM_COMPAT_DATA_PATH"] != prefix {
		t.Fatalf("STEAM_COMPAT_DATA_PATH = %q, want %q", env["STEAM_COMPAT_DATA_PATH"], prefix)
	}

	fresh := types.Game{ID: "fresh", Title: "Fresh", Source: types.SourceCustom, Store: types.StoreCustom, Executable: "/games/f.exe"}
	_, freshEnv, err := BuildUmuCommandFor(fresh, umuProton(home), umu, "", Target{Exe: "/games/f.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	if want := PrefixPath(fresh, home, ""); freshEnv["WINEPREFIX"] != want {
		t.Fatalf("fresh WINEPREFIX = %q, want %q", freshEnv["WINEPREFIX"], want)
	}
}

// The wrapper stack is unchanged around umu-run.
func TestUmuWrappersOrder(t *testing.T) {
	home := t.TempDir()
	bin := fakeBinDir(t, "umu-run", "gamemoderun", "gamescope", "mangohud")
	umu := filepath.Join(bin, "umu-run")
	game := umuGame()
	game.Settings.UseGameMode = true
	game.Settings.UseGamescope = true
	game.Settings.UseMangoHud = true
	args, _, err := BuildUmuCommandFor(game, umuProton(home), umu, "", Target{Exe: "/games/g.exe", Args: []string{"-a"}}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	want := []string{"gamemoderun", "gamescope", "--", "mangohud", umu, "/games/g.exe", "-a"}
	if !slices.Equal(args, want) {
		t.Fatalf("command = %q, want %q", args, want)
	}
}

// With no Proton build installed, PROTONPATH is UMU-Latest (downloaded on
// first launch); a Wine-only pick never becomes a PROTONPATH.
func TestUmuProtonPathLatest(t *testing.T) {
	if got := UmuProtonPath(types.ProtonVersion{}); got != UmuLatest {
		t.Fatalf("empty proton -> %q, want UMU-Latest", got)
	}
	wine := types.ProtonVersion{ID: "system:wine", Name: "System Wine", Path: "/usr/bin/wine", IsSystem: true}
	if got := UmuProtonPath(wine); got != UmuLatest {
		t.Fatalf("wine runner -> %q, want UMU-Latest", got)
	}
	runners := []types.ProtonVersion{wine, UmuEntry("/usr/bin/umu-run")}
	if p, ok := UmuProton(runners, ""); ok {
		t.Fatalf("UmuProton with wine only = %q, want none", p.ID)
	}
}

// Without umu installed, selection says no and the direct path is untouched
// (no GAMEID/PROTONPATH); auto still resolves to the best Proton.
func TestUmuFallbackWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir()) // no umu-run anywhere on PATH
	if _, ok := FindUmuFor(home); ok {
		t.Fatal("FindUmuFor found umu with an empty PATH and home")
	}
	runners := []types.ProtonVersion{umuProton(home), {ID: "system:wine", Name: "System Wine", Path: "/usr/bin/wine", IsSystem: true}}
	if _, _, ok := SelectUmu("auto", runners, ""); ok {
		t.Fatal("SelectUmu(auto) without umu must not select umu")
	}
	if _, _, ok := SelectUmu("umu", runners, ""); ok {
		t.Fatal("SelectUmu(umu) without umu must not select umu")
	}
	args, env, err := BuildCommandFor(umuGame(), umuProton(home), Target{Exe: "/games/g.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildCommandFor: %v", err)
	}
	if _, ok := env["GAMEID"]; ok {
		t.Fatalf("direct launch sets GAMEID = %q", env["GAMEID"])
	}
	if _, ok := env["PROTONPATH"]; ok {
		t.Fatalf("direct launch sets PROTONPATH = %q", env["PROTONPATH"])
	}
	if !slices.Contains(args, umuProton(home).Path) {
		t.Fatalf("direct command %q has no proton", args)
	}
}

// Selection rules: explicit umu and auto-with-umu run via umu with the auto
// Proton build; explicit Proton/Wine runs directly; PickAuto never picks umu.
func TestUmuSelection(t *testing.T) {
	proton := types.ProtonVersion{ID: "steam:Proton 11", Name: "Proton 11", Path: "/x/Proton 11/proton"}
	wine := types.ProtonVersion{ID: "system:wine", Name: "System Wine", Path: "/usr/bin/wine", IsSystem: true}
	umu := UmuEntry("/usr/bin/umu-run")
	runners := []types.ProtonVersion{proton, wine, umu}

	u, p, ok := SelectUmu("umu", runners, "")
	if !ok || u.Path != umu.Path {
		t.Fatalf("explicit umu = %q, %v", u.Path, ok)
	}
	if p.ID != proton.ID {
		t.Fatalf("umu proton = %q, want the auto pick %q", p.ID, proton.ID)
	}
	if _, _, ok := SelectUmu("auto", runners, ""); !ok {
		t.Fatal("auto with umu present must select umu")
	}
	if _, _, ok := SelectUmu("", runners, ""); !ok {
		t.Fatal("empty runner with umu present must select umu")
	}
	if _, _, ok := SelectUmu(proton.ID, runners, ""); ok {
		t.Fatal("explicit Proton must run directly, not via umu")
	}
	if _, _, ok := SelectUmu(wine.ID, runners, ""); ok {
		t.Fatal("explicit Wine must run directly, not via umu")
	}
	if got, _ := PickAuto(runners, ""); got.ID != proton.ID {
		t.Fatalf("PickAuto = %q, want %q (never umu)", got.ID, proton.ID)
	}
	if got, ok := PickAuto([]types.ProtonVersion{umu}, ""); ok {
		t.Fatalf("PickAuto with only umu = %q, want none", got.ID)
	}
}

// Store mapping for umu's STORE.
func TestUmuStoreMapping(t *testing.T) {
	for store, want := range map[types.StoreType]string{
		types.StoreEpic: "egs", types.StoreGOG: "gog", types.StoreSteam: "",
		types.StoreUbisoft: "ubisoft", types.StoreCustom: "",
	} {
		if got := StoreForUmu(types.Game{Store: store}); got != want {
			t.Fatalf("store %q -> %q, want %q", store, got, want)
		}
	}
}

// The Lutris-bundled runtime copy is found without PATH (read-only check).
func TestUmuFindLutrisRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	runtime := filepath.Join(home, ".local", "share", "lutris", "runtime", "umu")
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(runtime, "umu-run")
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := FindUmuFor(home); !ok || got != want {
		t.Fatalf("FindUmuFor = %q, %v, want %q", got, ok, want)
	}
}

func TestUmuCommandHasNoProtonRunVerb(t *testing.T) {
	home := t.TempDir()
	fakeBinDir(t, "umu-run")
	umu, _ := FindUmuFor(home)
	args, _, err := BuildUmuCommandFor(umuGame(), umuProton(home), umu, "", Target{Exe: "/games/g.exe"}, true, "", home)
	if err != nil {
		t.Fatalf("BuildUmuCommandFor: %v", err)
	}
	for i, a := range args {
		if a == umu && i+1 < len(args) && strings.TrimSpace(args[i+1]) == "run" {
			t.Fatalf("umu command %q must not carry Proton's run verb", args)
		}
	}
}
