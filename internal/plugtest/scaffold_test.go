package plugtest_test

// plugins/new.sh is the template every new store plugin starts from, so it
// has to pass the same conformance suite as the bundled plugins, and keep
// installs across a restart.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/plugtest"
	"platypus/internal/testenv"
	"platypus/internal/types"
)

// scaffold runs new.sh from a temp copy (it writes next to itself) and
// returns the Suite for the plugin it generated.
func scaffold(t *testing.T) plugtest.Suite {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	src, err := filepath.Abs("../../plugins/new.sh")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "new.sh")
	if err := os.WriteFile(script, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sh", script, "scaffold-test", "scaffoldstore").CombinedOutput(); err != nil {
		t.Fatalf("new.sh: %v\n%s", err, out)
	}
	return plugtest.Suite{
		ID:      "scaffold-test",
		CopyDir: filepath.Join(dir, "scaffold-test"),
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{"fields": map[string]string{"token": "abc"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "stub-game")
		},
	}
}

func TestConformanceScaffold(t *testing.T) {
	testenv.RequireSlow(t)
	scaffold(t).Conformance(t)
}

// An install must survive the plugin process dying: the scaffold keeps its
// state under $PLATYPUS_PLUGIN_DATA / on disk, not in memory.
func TestScaffoldInstallSurvivesRestart(t *testing.T) {
	s := scaffold(t)
	sess := s.Start(t)
	s.Login(t, sess)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	games, err := sess.Store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	game := s.PickGame(t, games)
	sess.RunJob(jobs.KindInstall, game)

	sess.Kill()
	s.Login(t, sess) // the stub keeps its session in memory
	games, err = sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library after restart: %v", err)
	}
	for _, g := range games {
		if g.StoreAppID == game.StoreAppID && !g.IsInstalled {
			t.Fatalf("install forgotten after a restart: %+v", g)
		}
	}
}
