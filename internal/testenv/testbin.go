// Fake programs on PATH (issue #66): testbin/ holds shell fakes for steam,
// winetricks, protontricks and xdg-open, plus a fake Proton build
// (proton/proton with files/bin/wine and files/bin/wineserver) with a real
// layout. Tests put testbin first on PATH so plugin `requires` checks and
// LookPath calls succeed on machines without Steam, and read back what the
// fakes were called with.
package testenv

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testBinFiles are the fakes SetupTestBin expects in testbin/.
var testBinFiles = []string{
	"steam",
	"winetricks",
	"protontricks",
	"xdg-open",
	filepath.Join("proton", "proton"),
	filepath.Join("proton", "files", "bin", "wine"),
	filepath.Join("proton", "files", "bin", "wineserver"),
}

// TestBin is a testbin setup: the fakes dir (first on PATH) and the call log
// every fake appends to (one line per call: argv plus WINEPREFIX, WINE,
// WINESERVER and STEAM_COMPAT_DATA_PATH).
type TestBin struct {
	Dir string // repo testbin/ dir
	Log string // call log path ($PLATYPUS_TESTBIN_LOG)
}

// SetupTestBin puts testbin first on PATH and points the call log at a temp
// file. Behaviour knobs are env, read by every fake: PLATYPUS_TESTBIN_EXIT
// (exit code), PLATYPUS_TESTBIN_SLEEP (seconds to run) and
// PLATYPUS_TESTBIN_STDERR_FILE (canned stderr file).
func SetupTestBin(t *testing.T) *TestBin {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("testenv: cannot locate testbin")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "testbin")
	for _, f := range testBinFiles {
		p := filepath.Join(dir, f)
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			t.Fatalf("testenv: testbin fake %s missing: %v", f, err)
		}
		// The execute bit can be lost through checkouts; the fakes must run.
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatalf("testenv: chmod %s: %v", f, err)
		}
	}
	log := filepath.Join(t.TempDir(), "calls.log")
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLATYPUS_TESTBIN_LOG", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &TestBin{Dir: dir, Log: log}
}

// Read returns the call log so far.
func (b *TestBin) Read(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(b.Log)
	if err != nil {
		t.Fatalf("testenv: read call log: %v", err)
	}
	return string(raw)
}

// Lines returns the call log as non-empty lines.
func (b *TestBin) Lines(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(b.Read(t), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// ProtonDir is the fake Proton build (its `proton` script plus files/bin/).
func (b *TestBin) ProtonDir() string { return filepath.Join(b.Dir, "proton") }

// Proton is the fake Proton launcher script (a runner's Path).
func (b *TestBin) Proton() string { return filepath.Join(b.Dir, "proton", "proton") }
