package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/types"
)

// backupsEnv is a versionsEnv with a backup service rooted in the test dir.
func backupsEnv(t *testing.T) *versionsEnv {
	t.Helper()
	e := versionsApp(t, allVersions, echoCaps)
	e.a.sessions = session.NewTracker(func(types.SessionEvent) {})
	e.a.sessions.SetLogDir(filepath.Join(e.root, "logs"))
	e.a.savebk = savebackup.New(savebackup.Options{
		Settings: e.a.settings,
		DataDir:  filepath.Join(e.root, "data"),
		CacheDir: filepath.Join(e.root, "cache"),
		Home:     e.root,
	})
	return e
}

func TestGateNoPathsWithAndWithoutAllow(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	game := types.Game{ID: "tiny-1", Source: "echo", StoreAppID: "tiny", Title: "Echo Tiny"}
	g, err := e.a.backupBeforeDelete(context.Background(), game, "uninstall", false)
	if err == nil || apperr.CodeOf(err) != "savebackup.noPaths" {
		t.Fatalf("err = %v, want savebackup.noPaths", err)
	}
	if !g.NoPaths {
		t.Errorf("gate = %+v, want NoPaths", g)
	}
	g, err = e.a.backupBeforeDelete(context.Background(), game, "uninstall", true)
	if err != nil {
		t.Fatalf("allowNoSaves err = %v", err)
	}
	if !g.NoPaths || g.BackupID != "" {
		t.Errorf("gate = %+v, want bare NoPaths", g)
	}
}

func TestGateOffSkipsWithoutError(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	if err := e.a.settings.SetBackupsMode("off"); err != nil {
		t.Fatal(err)
	}
	game := types.Game{ID: "tiny-1", Source: "echo", StoreAppID: "tiny", Title: "Echo Tiny"}
	g, err := e.a.backupBeforeDelete(context.Background(), game, "uninstall", false)
	if err != nil || g.NoPaths || g.BackupID != "" {
		t.Errorf("gate = %+v, err = %v, want empty", g, err)
	}
}

func TestGateFailureDeletesNothing(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	installed := e.installedTiny(t)
	saves := filepath.Join(installed.InstallDir, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte("progress"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.a.settings.SetGameSavePaths(installed.ID, []string{saves}); err != nil {
		t.Fatal(err)
	}
	// Break the backups folder: a file where the data dir should be.
	blocker := filepath.Join(e.root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.a.savebk = savebackup.New(savebackup.Options{
		Settings: e.a.settings, DataDir: blocker,
		CacheDir: filepath.Join(e.root, "cache"), Home: e.root,
	})
	if _, err := e.a.backupBeforeDelete(context.Background(), installed, "uninstall", false); err == nil {
		t.Fatal("gate succeeded with an unwritable backups folder")
	}
	if _, err := os.Stat(filepath.Join(saves, "slot.sav")); err != nil {
		t.Errorf("the failed gate lost the saves: %v", err)
	}
	if _, err := os.Stat(installed.InstallDir); err != nil {
		t.Errorf("the failed gate lost the install dir: %v", err)
	}
}

// The uninstall order on the echo fixture: the backup (holding the save
// bytes) exists before game.uninstall deletes the install dir.
func TestUninstallBacksUpBeforeRemove(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	installed := e.installedTiny(t)
	saves := filepath.Join(installed.InstallDir, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "slot.sav"), []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.a.settings.SetGameSavePaths(installed.ID, []string{saves}); err != nil {
		t.Fatal(err)
	}
	j, err := e.a.UninstallGame(installed)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	list := e.a.savebk.List(installed.ID)
	if len(list) != 1 {
		t.Fatalf("backups = %d, want 1 with trigger uninstall", len(list))
	}
	if list[0].Trigger != "uninstall" || list[0].Files != 1 {
		t.Errorf("backup = %+v, want 1 file with trigger uninstall", list[0])
	}
	if _, err := os.Stat(installed.InstallDir); !os.IsNotExist(err) {
		t.Errorf("install dir still there: %v", err)
	}
	meta, ok := savebackup.ReadBackupMeta(filepath.Join(e.root, "data", "backups"), installed.ID, list[0].ID)
	if !ok || len(meta.Files) != 1 || meta.Files[0].Size != int64(len("precious")) {
		t.Errorf("backup meta = %+v, want the save bytes", meta)
	}
}

func TestManualBackupAndRestoreJobs(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	installed := e.installedTiny(t)
	saves := filepath.Join(e.root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.a.settings.SetGameSavePaths(installed.ID, []string{saves}); err != nil {
		t.Fatal(err)
	}
	j, err := e.a.CreateSaveBackup(installed)
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != jobs.KindBackup {
		t.Fatalf("kind = %q, want backup", j.Kind)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	list, err := e.a.ListSaveBackups(installed.ID)
	if err != nil || len(list) != 1 || list[0].Trigger != "manual" {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	// Clobber and restore through the job: the before-restore backup runs first.
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("clobbered"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := e.a.RestoreSaveBackup(installed, list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != jobs.KindRestore {
		t.Fatalf("kind = %q, want restore", r.Kind)
	}
	e.waitJob(t, r.ID, jobs.StateDone)
	data, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	if string(data) != "v1" {
		t.Errorf("restored = %q, want v1", data)
	}
	if got := len(e.a.savebk.List(installed.ID)); got != 2 {
		t.Errorf("backups = %d, want 2 (manual + before-restore)", got)
	}
	if err := e.a.DeleteSaveBackup(installed.ID, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := len(e.a.savebk.List(installed.ID)); got != 1 {
		t.Errorf("backups = %d after delete, want 1", got)
	}
}

func TestAfterSessionBackup(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	installed := e.installedTiny(t)
	if err := e.a.settings.SetBackupAfterSession(true); err != nil {
		t.Fatal(err)
	}
	saves := filepath.Join(e.root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("session-progress"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.a.settings.SetGameSavePaths(installed.ID, []string{saves}); err != nil {
		t.Fatal(err)
	}
	e.a.backupAfterSession(installed)
	list := e.a.savebk.List(installed.ID)
	if len(list) != 1 || list[0].Trigger != "after-session" {
		t.Fatalf("backups = %+v, want 1 after-session", list)
	}
}

func TestSessionHooksRunCloudFirstBackupSecond(t *testing.T) {
	t.Parallel()
	var order []string
	tr := session.NewTracker(func(types.SessionEvent) {})
	done := make(chan struct{})
	_, err := tr.StartDirect("g", exec.Command("true"), "true", nil,
		session.WithOnExit(func(session.ExitInfo) { order = append(order, "cloud") }),
		session.WithOnExit(func(session.ExitInfo) {
			order = append(order, "backup")
			close(done)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("onExit hooks never ran")
	}
	if len(order) != 2 || order[0] != "cloud" || order[1] != "backup" {
		t.Errorf("order = %v, want [cloud backup]", order)
	}
}

func TestSavePathsReportOriginAndExistence(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t)
	installed := e.installedTiny(t)
	missing := filepath.Join(e.root, "not-there")
	existing := filepath.Join(e.root, "there")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.a.settings.SetGameSavePaths(installed.ID, []string{missing, existing}); err != nil {
		t.Fatal(err)
	}
	paths, err := e.a.GetSavePaths(installed)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths = %+v, want 2", paths)
	}
	for _, p := range paths {
		if p.Origin != "user" {
			t.Errorf("origin = %q, want user", p.Origin)
		}
		want := p.Path == existing
		if p.Exists != want {
			t.Errorf("exists = %v for %s", p.Exists, p.Path)
		}
	}
	if !strings.Contains(installed.ID, "echo") {
		t.Errorf("game id = %q", installed.ID)
	}
}
