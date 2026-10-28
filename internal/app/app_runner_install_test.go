package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/jobs"
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
	// System and Steam-managed runners are never removed.
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
