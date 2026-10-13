package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/migrate"
	"platypus/internal/settings"
)

// Nothing migrated: fresh installs, second starts and both-folders cases show
// no notice.
func TestMigrationNoticeNilWhenNothingHappened(t *testing.T) {
	t.Parallel()
	for name, rep := range map[string]migrate.Report{
		"fresh install": {},
		"second start":  {Root: "/home/u/.local/share/platypus"},
		"both exist":    {BothExist: true},
	} {
		st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		a := &App{settings: st, migration: rep}
		if n := a.MigrationNotice(); n != nil {
			t.Errorf("%s: notice = %+v, want nil", name, n)
		}
	}
}

// A move maps to a notice: what happened, the set-aside plugins by name, and
// the report's failures in plain words.
func TestMigrationNoticeMapsReport(t *testing.T) {
	t.Parallel()
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, migration: migrate.Report{
		Moved:    true,
		SetAside: []string{"echo", "custom"},
		Err:      errors.New("rewriting stored paths: jobs.json: permission denied"),
	}}
	n := a.MigrationNotice()
	if n == nil {
		t.Fatal("notice is nil, want one")
	}
	if !n.Moved || n.Copied {
		t.Errorf("moved/copied = %v/%v, want true/false", n.Moved, n.Copied)
	}
	if len(n.SetAside) != 2 || n.SetAside[0] != "echo" || n.SetAside[1] != "custom" {
		t.Errorf("set aside = %v, want [echo custom]", n.SetAside)
	}
	if len(n.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one", n.Warnings)
	}
	if got := n.Warnings[0]; !strings.Contains(got, "jobs.json") || !strings.Contains(got, "next start") {
		t.Errorf("warning = %q, want the cause in plain words", got)
	}

	// A copy says copied, and warnings never come back nil (the UI lists them).
	a.migration = migrate.Report{Copied: true}
	n = a.MigrationNotice()
	if n == nil || !n.Copied || n.Warnings == nil || n.SetAside == nil {
		t.Fatalf("copy notice = %+v, want copied with empty lists", n)
	}
}

// Dismissing persists in settings.json: the notice never returns after
// dismissal, and a reopened store still says dismissed.
func TestDismissMigrationNoticePersists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, migration: migrate.Report{Moved: true, SetAside: []string{"echo"}}}
	if a.MigrationNotice() == nil {
		t.Fatal("no notice before dismissal")
	}
	if err := a.DismissMigrationNotice(); err != nil {
		t.Fatal(err)
	}
	if n := a.MigrationNotice(); n != nil {
		t.Fatalf("notice after dismissal = %+v, want nil", n)
	}
	again, err := settings.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !again.MigrationNoticeDismissed() {
		t.Error("dismissal did not persist in settings.json")
	}
	b := &App{settings: again, migration: migrate.Report{Moved: true}}
	if n := b.MigrationNotice(); n != nil {
		t.Errorf("notice after relaunch = %+v, want nil", n)
	}
}
