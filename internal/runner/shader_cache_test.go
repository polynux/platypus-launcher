package runner

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// A steam-standalone game with a downloaded shadercache/<appid>/ tree next
// to its install gets the driver cache family at launch (issue #211).
func shaderGame(home, appID string) types.Game {
	install := filepath.Join(home, "games", "SomeGame_"+appID)
	if err := os.MkdirAll(install, 0o755); err != nil {
		panic(err)
	}
	cache := filepath.Join(home, "games", "shadercache", appID)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		panic(err)
	}
	g := types.Game{
		ID:         "p.steam-standalone." + appID,
		Source:     "steam-standalone",
		Store:      types.StoreSteam,
		StoreAppID: appID,
		Title:      "Some Game",
		InstallDir: install,
		Executable: filepath.Join(install, "game"),
		Settings:   types.GameSettings{},
	}
	return g
}

func TestShaderCacheEnvProton(t *testing.T) {
	home := t.TempDir()
	game := shaderGame(home, "730")
	_, env, err := BuildCommandFor(game, types.ProtonVersion{}, Target{Exe: game.Executable}, false, "", home)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	want := filepath.Join(home, "games", "shadercache", "730")
	if env["MESA_SHADER_CACHE_DIR"] != want {
		t.Fatalf("MESA_SHADER_CACHE_DIR = %q, want %q", env["MESA_SHADER_CACHE_DIR"], want)
	}
	if env["__GL_SHADER_DISK_CACHE_PATH"] != filepath.Join(want, "nvidiav1") {
		t.Fatalf("__GL_SHADER_DISK_CACHE_PATH = %q", env["__GL_SHADER_DISK_CACHE_PATH"])
	}
	if env["__GL_SHADER_DISK_CACHE_SKIP_CLEANUP"] != "1" {
		t.Fatalf("__GL_SHADER_DISK_CACHE_SKIP_CLEANUP = %q", env["__GL_SHADER_DISK_CACHE_SKIP_CLEANUP"])
	}
	if env["STEAM_COMPAT_SHADER_PATH"] != want {
		t.Fatalf("STEAM_COMPAT_SHADER_PATH = %q, want %q", env["STEAM_COMPAT_SHADER_PATH"], want)
	}
}

// Native launches get the disk caches but no compat shader path.
func TestShaderCacheEnvNative(t *testing.T) {
	home := t.TempDir()
	game := shaderGame(home, "730")
	_, env, err := BuildCommandFor(game, types.ProtonVersion{}, Target{Exe: game.Executable, Native: true}, false, "", home)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if env["MESA_SHADER_CACHE_DIR"] == "" || env["__GL_SHADER_DISK_CACHE_PATH"] == "" {
		t.Fatalf("native launch misses disk caches: %v", env)
	}
	if _, ok := env["STEAM_COMPAT_SHADER_PATH"]; ok {
		t.Fatalf("native launch sets STEAM_COMPAT_SHADER_PATH: %q", env["STEAM_COMPAT_SHADER_PATH"])
	}
}

// Without a downloaded tree (or for other sources, client launches, or a
// missing store id) the launch env is untouched: no behavior change and no
// stray cache dirs created by the launch.
func TestShaderCacheEnvAbsent(t *testing.T) {
	home := t.TempDir()
	install := filepath.Join(home, "games", "Bare_400")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	base := types.Game{
		Source:     "steam-standalone",
		Store:      types.StoreSteam,
		StoreAppID: "400",
		Title:      "Bare",
		InstallDir: install,
		Executable: filepath.Join(install, "game"),
	}
	cases := map[string]types.Game{
		"no tree": base,
	}
	withTree := shaderGame(home, "730")
	client := withTree
	client.ClientLaunch = true
	cases["client launch"] = client
	legacy := withTree
	legacy.Source = "steam"
	cases["legacy steam source"] = legacy
	noID := withTree
	noID.StoreAppID = ""
	cases["no store id"] = noID
	for name, game := range cases {
		_, env, err := BuildCommandFor(game, types.ProtonVersion{}, Target{Exe: game.Executable}, false, "", home)
		if err != nil {
			t.Fatalf("%s: BuildCommand: %v", name, err)
		}
		for _, k := range []string{"MESA_SHADER_CACHE_DIR", "__GL_SHADER_DISK_CACHE_PATH",
			"__GL_SHADER_DISK_CACHE_SKIP_CLEANUP", "STEAM_COMPAT_SHADER_PATH"} {
			if _, ok := env[k]; ok {
				t.Fatalf("%s: sets %s = %q", name, k, env[k])
			}
		}
	}
}

// Explicit values win: the user's env over everything, the launch spec's
// over the shader defaults.
func TestShaderCacheEnvOverride(t *testing.T) {
	home := t.TempDir()
	game := shaderGame(home, "730")
	game.Settings.EnvVars = map[string]string{"MESA_SHADER_CACHE_DIR": "/custom/mesa"}
	_, env, err := BuildCommandFor(game, types.ProtonVersion{},
		Target{Exe: game.Executable, Env: map[string]string{"__GL_SHADER_DISK_CACHE_PATH": "/custom/nv"}}, false, "", home)
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if env["MESA_SHADER_CACHE_DIR"] != "/custom/mesa" {
		t.Fatalf("user env lost: %q", env["MESA_SHADER_CACHE_DIR"])
	}
	if env["__GL_SHADER_DISK_CACHE_PATH"] != "/custom/nv" {
		t.Fatalf("spec env lost: %q", env["__GL_SHADER_DISK_CACHE_PATH"])
	}
	if env["STEAM_COMPAT_SHADER_PATH"] == "" {
		t.Fatal("compat default missing beside overrides")
	}
}
