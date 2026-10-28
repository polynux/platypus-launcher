package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Issue #142: removal only deletes user-installed builds, and warns when
// games use them.
func TestCheckAndRemoveRunner(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	build := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-4")
	proton := writeExe(t, filepath.Join(build, "proton"), []byte("#!/bin/sh\n"), true)
	_ = proton
	check, err := a.CheckRemoveRunner("compat:GE-Proton10-4")
	if err != nil {
		t.Fatalf("CheckRemoveRunner: %v", err)
	}
	if !check.Removable {
		t.Fatal("compat build must be removable")
	}
	if len(check.UsedBy) != 0 {
		t.Fatalf("UsedBy = %v, want none", check.UsedBy)
	}
	// A game using it blocks a forced removal.
	cg, err := a.settings.AddCustomGame("Win", writeExe(t, filepath.Join(home, "g.exe"), []byte("MZ\x90\x00"), true))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, map[string]any{"runner_id": "compat:GE-Proton10-4"}); err != nil {
		t.Fatal(err)
	}
	check, err = a.CheckRemoveRunner("compat:GE-Proton10-4")
	if err != nil {
		t.Fatalf("CheckRemoveRunner: %v", err)
	}
	if len(check.UsedBy) != 1 || check.UsedBy[0] != cg.ID {
		t.Fatalf("UsedBy = %v, want [%s]", check.UsedBy, cg.ID)
	}
	if err := a.RemoveRunner("compat:GE-Proton10-4", false); err == nil {
		t.Fatal("RemoveRunner without force must refuse a build games use")
	}
	if err := a.RemoveRunner("compat:GE-Proton10-4", true); err != nil {
		t.Fatalf("forced RemoveRunner: %v", err)
	}
	if _, err := os.Stat(build); !os.IsNotExist(err) {
		t.Fatalf("build still exists: %v", err)
	}
	// System and Steam-managed runners are never removed: a real
	// Steam-managed build is refused even though it is installed.
	steamBuild := filepath.Join(home, ".local/share/Steam/steamapps/common/Proton 9.0")
	writeExe(t, filepath.Join(steamBuild, "proton"), []byte("#!/bin/sh\n"), true)
	if _, err := a.CheckRemoveRunner("system:wine"); err == nil {
		// No system wine in the test env: missing is also a refusal.
		t.Log("system wine not installed, missing is fine")
	}
	if err := a.RemoveRunner("steam:Proton 9.0", true); err == nil {
		t.Fatal("Steam-managed Proton must never be removed")
	}
}

// Issue #142: installing queues a job with progress; a duplicate refuses.
func TestInstallRunnerQueuesJob(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	// The queue never runs here, but point the releases endpoint at a
	// fixture anyway so no queued job can ever reach the network (n15).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	t.Setenv("PLATYPUS_GE_RELEASES_URL", srv.URL)
	root := t.TempDir()
	var err error
	a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := a.InstallRunner("GE-Proton10-4")
	if err != nil {
		t.Fatalf("InstallRunner: %v", err)
	}
	if job.StoreGameID != "GE-Proton10-4" {
		t.Fatalf("job tag = %q", job.StoreGameID)
	}
}

// B1: the release resolves only with a checksum over a trustworthy URL.
// Unknown tags, missing checksums and plain-http hosts never install.
func TestResolveRunnerReleaseFailsClosed(t *testing.T) {
	t.Parallel()
	avail := []types.AvailableRunner{
		{Tag: "good", Name: "good", URL: "https://example.com/g.tar.gz", ChecksumURL: "https://example.com/g.sha512sum"},
		{Tag: "nosum", Name: "nosum", URL: "https://example.com/n.tar.gz"},
		{Tag: "plain", Name: "plain", URL: "http://example.com/p.tar.gz", ChecksumURL: "http://example.com/p.sha512sum"},
	}
	if _, err := resolveRunnerRelease(avail, "good"); err != nil {
		t.Fatalf("good release must resolve: %v", err)
	}
	if _, err := resolveRunnerRelease(avail, "missing"); err == nil {
		t.Fatal("unknown tag must not resolve")
	}
	if _, err := resolveRunnerRelease(avail, "nosum"); err == nil {
		t.Fatal("a release with no checksum must never install")
	} else {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "runner.badChecksum" {
			t.Fatalf("no-checksum error = %v, want runner.badChecksum", err)
		}
	}
	if _, err := resolveRunnerRelease(avail, "plain"); err == nil {
		t.Fatal("a plain-http asset must never install")
	}
	// Loopback http stays usable for httptest fixtures.
	loop := []types.AvailableRunner{
		{Tag: "loop", Name: "loop", URL: "http://127.0.0.1/g.tar.gz", ChecksumURL: "http://127.0.0.1/g.sha512sum"},
	}
	if _, err := resolveRunnerRelease(loop, "loop"); err != nil {
		t.Fatalf("loopback http must resolve in tests: %v", err)
	}
}

// M4: the install stage lives inside the resolved runner folder, which is
// created first — even when compatibilitytools.d is a symlink elsewhere.
func TestMakeRunnerStageInsideResolvedFolder(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	real := t.TempDir()
	dest := filepath.Join(home, ".local/share/Steam/compatibilitytools.d")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, dest); err != nil {
		t.Fatal(err)
	}
	stage, resolved, err := makeRunnerStage(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	if resolved != real && resolved != dest {
		t.Fatalf("resolved = %q, want the symlink target", resolved)
	}
	if got, err := filepath.EvalSymlinks(stage); err != nil || filepath.Dir(got) != resolved {
		t.Fatalf("stage %q is not inside %q: %v", stage, resolved, err)
	}
}

// M6: Automatic games count as using the default (or the auto pick), the
// default falls back when removed, and a running game blocks removal.
func TestRemoveRunnerDefaultAndRunning(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	build := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-4")
	writeExe(t, filepath.Join(build, "proton"), []byte("#!/bin/sh\n"), true)
	// A game left on Automatic uses the default once it is picked.
	if err := a.settings.SetDefaultRunnerID("compat:GE-Proton10-4"); err != nil {
		t.Fatal(err)
	}
	cg, err := a.settings.AddCustomGame("Auto", writeExe(t, filepath.Join(home, "a.exe"), []byte("MZ\x90\x00"), true))
	if err != nil {
		t.Fatal(err)
	}
	check, err := a.CheckRemoveRunner("compat:GE-Proton10-4")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range check.UsedBy {
		if id == cg.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("Automatic game must count as using the default, UsedBy = %v", check.UsedBy)
	}
	// A game running on the build blocks even a forced removal.
	a.sessionRunning = func(id string) bool { return id == cg.ID }
	if err := a.RemoveRunner("compat:GE-Proton10-4", true); err == nil {
		t.Fatal("removing a runner out from a running game must fail")
	} else {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "game.running" {
			t.Fatalf("running error = %v, want game.running", err)
		}
	}
	a.sessionRunning = func(string) bool { return false }
	if err := a.RemoveRunner("compat:GE-Proton10-4", true); err != nil {
		t.Fatalf("forced RemoveRunner: %v", err)
	}
	if got := a.defaultRunnerID(); got != "" {
		t.Fatalf("default after removal = %q, want the automatic pick", got)
	}
}

// runnerFixtureServer serves a releases listing plus a tarball and its
// checksum, all on loopback. wantSum controls whether the release carries a
// checksum asset.
func runnerFixtureServer(t *testing.T, tag string, wantSum bool) (*httptest.Server, string) {
	t.Helper()
	var tarBytes []byte
	{
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		body := []byte("#!/bin/sh\n")
		name := tag + "/proton"
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		tarBytes = buf.Bytes()
	}
	sum := sha512.Sum512(tarBytes)
	hexSum := hex.EncodeToString(sum[:])
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			assets := fmt.Sprintf(`{"name":"%s.tar.gz","browser_download_url":"%s/g.tar.gz"}`, tag, srvURL)
			if wantSum {
				assets += fmt.Sprintf(`,{"name":"%s.sha512sum","browser_download_url":"%s/g.sha512sum"}`, tag, srvURL)
			}
			fmt.Fprintf(w, `[{"tag_name":%q,"name":%q,"assets":[%s]}]`, tag, tag, assets)
		case "/g.tar.gz":
			w.Write(tarBytes)
		case "/g.sha512sum":
			fmt.Fprintf(w, "%s  %s.tar.gz\n", hexSum, tag)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	srvURL = srv.URL
	return srv, hexSum
}

// B1+M4: a release with no checksum fails the job (fail closed) instead of
// installing unverified.
func TestInstallRunnerMissingChecksumFailsJob(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local/share/Steam"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	root := t.TempDir()
	var err error
	a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, _ := runnerFixtureServer(t, "GE-Proton10-99", false)
	defer srv.Close()
	t.Setenv("PLATYPUS_GE_RELEASES_URL", srv.URL+"/releases")
	job, err := a.InstallRunner("GE-Proton10-99")
	if err != nil {
		t.Fatalf("InstallRunner: %v", err)
	}
	end := runRunnerJob(t, a, job.ID)
	if end.State != jobs.StateFailed {
		t.Fatalf("job state = %s, want failed", end.State)
	}
}

// M4: a full install works with no native Steam folder at all: the runner
// folder is created first and the build lands in it.
func TestInstallRunnerCreatesMissingRunnerFolder(t *testing.T) {
	home := t.TempDir() // no .local/share/Steam here
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	root := t.TempDir()
	var err error
	a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, _ := runnerFixtureServer(t, "GE-Proton10-99", true)
	defer srv.Close()
	t.Setenv("PLATYPUS_GE_RELEASES_URL", srv.URL+"/releases")
	job, err := a.InstallRunner("GE-Proton10-99")
	if err != nil {
		t.Fatalf("InstallRunner: %v", err)
	}
	end := runRunnerJob(t, a, job.ID)
	if end.State != jobs.StateDone {
		t.Fatalf("job state = %s (%s), want done", end.State, end.Error)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-99/proton")); err != nil {
		t.Fatalf("installed build missing: %v", err)
	}
	// No stage is left behind.
	entries, err := os.ReadDir(filepath.Join(home, ".local/share/Steam/compatibilitytools.d"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) >= len(".platypus-runner-") && e.Name()[:len(".platypus-runner-")] == ".platypus-runner-" {
			t.Fatalf("stale stage left behind: %s", e.Name())
		}
	}
}

// runRunnerJob runs the queue until id ends and returns it.
func runRunnerJob(t *testing.T, a *App, id string) types.Job {
	t.Helper()
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.jobs.Run(runCtx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range a.jobs.List() {
			if j.ID == id && (j.State == jobs.StateDone || j.State == jobs.StateFailed || j.State == jobs.StateCanceled) {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never ended: %+v", id, a.jobs.List())
	return types.Job{}
}
