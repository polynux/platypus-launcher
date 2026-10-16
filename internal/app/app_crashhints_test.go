package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/runner"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/testenv"
	"platypus/internal/types"
)

// A game whose last run was fine has no crash hints.
func TestGameCrashHintsIsNilWithoutAFailedLaunch(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	if got := a.GameCrashHints(types.Game{ID: "fine", Title: "Fine"}); got != nil {
		t.Fatalf("GameCrashHints = %+v, want nil", got)
	}
}

// A launch that exits non-zero leaves its log, and the hints read it back.
func TestGameCrashHintsReadsTheFailedLaunch(t *testing.T) {
	events := make(chan types.SessionEvent, 16)
	tr := session.NewTracker(func(e types.SessionEvent) { events <- e })
	tr.SetLogDir(t.TempDir())
	a := &App{sessions: tr}

	cmd := exec.Command("sh", "-c", "echo 'err:module:import_dll Library bar.dll (which is needed by L\"x.exe\") not found'; exit 1")
	if _, err := tr.StartDirect("crashy", cmd, "sh", nil); err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	deadline := time.After(10 * time.Second)
wait:
	for {
		select {
		case e := <-events:
			if e.State == "failed" {
				break wait
			}
		case <-deadline:
			t.Fatal("timed out waiting for the failed launch")
		}
	}
	got := a.GameCrashHints(types.Game{ID: "crashy", Title: "Crashy"})
	if got == nil || len(got.Hints) != 1 {
		t.Fatalf("GameCrashHints = %+v, want one hint", got)
	}
	if got.Hints[0].Rule != "missingDll" || got.Hints[0].Param != "bar.dll" {
		t.Fatalf("hint = %+v, want the missing bar.dll", got.Hints[0])
	}
	if got.Log == "" {
		t.Fatal("hints name no log")
	}
}

// Only verbs in the pinned winetricks list are ever run.
func TestRunWinetricksVerbRefusesAnUnknownVerb(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	err := a.RunWinetricksVerb(types.Game{ID: "g", Title: "G"}, "not-a-verb")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "crash.badVerb" {
		t.Fatalf("RunWinetricksVerb err = %v, want crash.badVerb", err)
	}
}

// protonScript makes a Proton folder with its launch script and wine binaries
// in layout (files/bin for newer builds, dist/bin for older ones). It returns
// the script path, which is what a runner's Path is.
func protonScript(t *testing.T, layout string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, layout)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"wine", "wineserver"} {
		if err := os.WriteFile(filepath.Join(bin, f), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(dir, "proton")
	if err := os.WriteFile(script, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

// A Proton game keeps its Wine prefix in <prefix>/pfx, so winetricks must get
// that folder and the build's own wine (issue #46 review).
func TestWinetricksEnvFollowsTheRunner(t *testing.T) {
	game := types.Game{ID: "g", Title: "G"}
	withPfx := func() string {
		prefix := t.TempDir()
		if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0o755); err != nil {
			t.Fatal(err)
		}
		return prefix
	}
	plainWine := filepath.Join(t.TempDir(), "wine")
	if err := os.WriteFile(plainWine, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("system wine uses PATH", func(t *testing.T) {
		prefix := t.TempDir()
		env, err := winetricksEnv(game, prefix, types.ProtonVersion{})
		if err != nil {
			t.Fatal(err)
		}
		if want := map[string]string{"WINEPREFIX": prefix}; !reflect.DeepEqual(env, want) {
			t.Fatalf("env = %v, want %v", env, want)
		}
	})

	t.Run("plain wine runner runs its own binary", func(t *testing.T) {
		prefix := t.TempDir()
		env, err := winetricksEnv(game, prefix, types.ProtonVersion{Path: plainWine})
		if err != nil {
			t.Fatal(err)
		}
		if want := map[string]string{"WINEPREFIX": prefix, "WINE": plainWine}; !reflect.DeepEqual(env, want) {
			t.Fatalf("env = %v, want %v", env, want)
		}
	})

	for _, layout := range []string{"files/bin", "dist/bin"} {
		t.Run("proton build in "+layout, func(t *testing.T) {
			script := protonScript(t, layout)
			prefix := withPfx()
			env, err := winetricksEnv(game, prefix, types.ProtonVersion{Path: script})
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(script)
			want := map[string]string{
				"WINEPREFIX": filepath.Join(prefix, "pfx"),
				"WINE":       filepath.Join(dir, layout, "wine"),
				"WINESERVER": filepath.Join(dir, layout, "wineserver"),
			}
			if !reflect.DeepEqual(env, want) {
				t.Fatalf("env = %v, want %v", env, want)
			}
		})
	}

	t.Run("proton game without its pfx folder is refused", func(t *testing.T) {
		// The compatdata root exists (the old check passed); Proton has not made pfx yet.
		_, err := winetricksEnv(game, t.TempDir(), types.ProtonVersion{Path: protonScript(t, "files/bin")})
		wantCode(t, err, "crash.noPrefix")
	})

	t.Run("proton build without wine binaries is refused", func(t *testing.T) {
		script := protonScript(t, "files/bin")
		if err := os.Remove(filepath.Join(filepath.Dir(script), "files/bin", "wineserver")); err != nil {
			t.Fatal(err)
		}
		_, err := winetricksEnv(game, withPfx(), types.ProtonVersion{Path: script})
		wantCode(t, err, "crash.noProtonWine")
	})

	t.Run("umu is refused, not guessed at", func(t *testing.T) {
		_, err := winetricksEnv(game, withPfx(), types.ProtonVersion{ID: runner.UmuRunnerID, Path: "/usr/bin/umu-run"})
		wantCode(t, err, "crash.umuUnsupported")
	})

	t.Run("system wine without a prefix is refused", func(t *testing.T) {
		_, err := winetricksEnv(game, filepath.Join(t.TempDir(), "missing"), types.ProtonVersion{})
		wantCode(t, err, "crash.noPrefix")
	})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("err = %v, want code %s", err, code)
	}
}

// RunWinetricksVerb runs the fake winetricks in the Proton prefix with the
// build's own wine (issue #66): the call log must show WINEPREFIX=<prefix>/pfx
// and WINE=<proton>/files/bin/wine for a Proton game.
func TestRunWinetricksVerbRunsInTheProtonPrefix(t *testing.T) {
	tb := testenv.SetupTestBin(t)
	home := t.TempDir()
	// Install the testbin Proton build where the runner scan finds it.
	compat := filepath.Join(home, ".local/share/Steam/compatibilitytools.d", "FakeProton")
	if err := os.MkdirAll(filepath.Dir(compat), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-r", tb.ProtonDir(), compat).CombinedOutput(); err != nil {
		t.Fatalf("cp proton build: %v: %s", err, out)
	}
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, sessions: session.NewTracker(func(types.SessionEvent) {}),
		dataDir: t.TempDir(), optHome: home}
	game := types.Game{ID: "g", Title: "G", Settings: types.GameSettings{RunnerID: "compat:FakeProton"}}
	prefix := runner.PrefixPath(game, a.homeDir(), a.prefixesFolder())
	if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.RunWinetricksVerb(game, "d3dx9"); err != nil {
		t.Fatalf("RunWinetricksVerb: %v", err)
	}
	wantPrefix := filepath.Join(prefix, "pfx")
	wantWine := filepath.Join(compat, "files", "bin", "wine")
	deadline := time.Now().Add(10 * time.Second)
	for {
		log := tb.Read(t)
		if strings.Contains(log, "winetricks") && strings.Contains(log, "WINEPREFIX="+wantPrefix) &&
			strings.Contains(log, "WINE="+wantWine) && strings.Contains(log, "d3dx9") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("winetricks log = %q, want WINEPREFIX=%s WINE=%s and d3dx9", log, wantPrefix, wantWine)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
