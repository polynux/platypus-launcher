package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Install options, review fixes: what a job sends is checked when it starts
// (or is sized), not only when it was queued, and only while the store offers
// it. echo refuses unowned DLC and unknown choices, and it keeps the choices of
// its last install, so the plugin itself shows what a job really sent.

// waitFailed waits for a job to fail and returns it.
func (e *versionsEnv) waitFailed(t *testing.T, id string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && j.State == jobs.StateFailed {
				return j
			}
			if j.ID == id && j.State == jobs.StateDone {
				t.Fatalf("job %s finished, want it to fail", id)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not fail in time", id)
	return types.Job{}
}

// echoInstalled is what echo has recorded as installed for the game: the
// add-ons (dlc) and packs of its last install.
func (e *versionsEnv) echoInstalled(t *testing.T, id string) map[string]bool {
	t.Helper()
	var res struct {
		Groups []types.OptionGroup `json:"groups"`
	}
	if err := e.m.Call(context.Background(), "echo", "game.options", map[string]any{"id": id}, &res); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, g := range res.Groups {
		for _, v := range g.Values {
			if v.Installed {
				out[g.ID+"/"+v.ID] = true
			}
		}
	}
	return out
}

func (e *versionsEnv) queueRaw(t *testing.T, kind string, storeAppID string, options types.InstallOptions) types.Job {
	t.Helper()
	dir := filepath.Join(e.root, "Games", storeAppID)
	j, err := e.a.jobs.AddWithOptions(kind, "echo", e.game.ID, storeAppID, e.game.Title, dir, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestStartDropsWhatTheStoreNoLongerOffers(t *testing.T) {
	e := optionsEnv(t)
	// Queued while the user owned the DLC (or a value existed); both are gone
	// by the time the job starts. A group the store never had is stale too.
	j := e.queueRaw(t, jobs.KindInstall, "tiny", types.InstallOptions{
		"language": "fr", "dlc": []any{"deluxe", "season"}, "packs": []any{"nope", "hd"}, "gone": true,
	})
	e.waitJob(t, j.ID, jobs.StateDone)
	got := e.echoInstalled(t, "tiny")
	if !got["dlc/season"] || !got["packs/hd"] {
		t.Fatalf("installed = %v, want the owned add-on and pack", got)
	}
	if got["dlc/deluxe"] {
		t.Fatal("an unowned DLC was installed")
	}
}

func TestStartFailsRetryablyWhenTheStoreCannotBeAsked(t *testing.T) {
	e := optionsEnv(t)
	j := e.queueRaw(t, jobs.KindInstall, "gone", types.InstallOptions{"language": "fr"})
	failed := e.waitFailed(t, j.ID)
	if !strings.Contains(failed.Error, "couldn't check add-ons with the store; retry") {
		t.Fatalf("error = %q, want the retryable add-ons message", failed.Error)
	}
}

func TestVersionSwitchKeepsTheSavedChoices(t *testing.T) {
	e := optionsEnv(t)
	first, err := e.a.InstallGameWithOptions(e.game, types.InstallOptions{"language": "de"})
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, first.ID, jobs.StateDone)
	e.game.IsInstalled = true

	j := e.switchTo(t, types.VersionTarget{Build: "b200"})
	if j.Target == nil || j.Target.Build != "b200" {
		t.Fatalf("switch target = %+v", j.Target)
	}
	if j.Options["language"] != "de" {
		t.Fatalf("switch options = %v, want the saved language", j.Options)
	}
}

func TestPlainUpdateQueuesSavedChoicesWhenTheStoreIsDown(t *testing.T) {
	e := optionsEnv(t)
	if err := e.a.settings.SetOptions(e.game.ID, types.InstallOptions{"language": "fr"}); err != nil {
		t.Fatal(err)
	}
	e.game.IsInstalled = true
	gone := e.game
	gone.StoreAppID = "gone" // the store can't say what the game offers now

	j, err := e.a.UpdateGame(gone)
	if err != nil {
		t.Fatalf("plain update refused because the store is unreachable: %v", err)
	}
	if j.Options["language"] != "fr" {
		t.Fatalf("queued options = %v, want the saved ones as they were saved", j.Options)
	}
	e.waitFailed(t, j.ID) // the job checks them again when it starts
}

func TestOptionsAreNotSentWhereTheStoreDoesNotOfferThem(t *testing.T) {
	// Protocol 1.3: the install-options capability is off at runtime, so a job
	// queued with choices (from before a downgrade, say) sends none. Its size
	// shows it: installSize would count the add-ons if they were sent.
	e := versionsApp(t, allVersions, optionsCaps)
	ctx := context.Background()
	plain := types.Job{Kind: jobs.KindInstall, Source: "echo", StoreGameID: "tiny", InstallDir: e.root}
	base, err := jobBackend{e.a}.Size(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	withAddOns := plain
	withAddOns.Options = types.InstallOptions{"dlc": []any{"season", "soundtrack"}, "packs": []any{"hd"}}
	sized, err := jobBackend{e.a}.Size(ctx, withAddOns)
	if err != nil {
		t.Fatal(err)
	}
	if sized.DownloadBytes != base.DownloadBytes {
		t.Fatalf("size with choices %d, plain %d: choices were sent to a plugin that doesn't offer them", sized.DownloadBytes, base.DownloadBytes)
	}
}

func TestSizeUsesTheChosenAddOns(t *testing.T) {
	e := optionsEnv(t)
	ctx := context.Background()
	plain := types.Job{Kind: jobs.KindInstall, Source: "echo", StoreGameID: "tiny", InstallDir: e.root}
	base, err := jobBackend{e.a}.Size(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	with := plain
	with.Options = types.InstallOptions{"dlc": []any{"season", "soundtrack"}, "packs": []any{"hd"}}
	sized, err := jobBackend{e.a}.Size(ctx, with)
	if err != nil {
		t.Fatal(err)
	}
	if extra := sized.DownloadBytes - base.DownloadBytes; extra != (1+2+3)<<20 {
		t.Fatalf("the job's size adds %d bytes, want the 6 MiB of its add-ons", extra)
	}
}

func TestInstallSizeIsTheBaseSize(t *testing.T) {
	e := optionsEnv(t)
	var base struct {
		DiskBytes int64 `json:"diskBytes"`
	}
	if err := e.m.Call(context.Background(), "echo", "game.installSize", map[string]any{"id": "tiny"}, &base); err != nil {
		t.Fatal(err)
	}
	got, err := e.a.GetInstallSize(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if got != base.DiskBytes || got == 0 {
		t.Fatalf("install size = %d, want the store's base size %d", got, base.DiskBytes)
	}
}
