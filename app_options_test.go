package main

import (
	"context"
	"strings"
	"testing"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/types"
)

// Install options (protocol 1.5) around the echo test plugin. The echo
// manifest offers install-options only over 1.5; the protocol list here is
// what Hydra and the manifest both speak.

const optionsProtocols = `["1.0","1.1","1.3","1.5"]`
const optionsCaps = `["library","install","auth","discard","versions","install-options"]`

// optionsEnv is a versions environment that also offers install options.
func optionsEnv(t *testing.T) *versionsEnv {
	t.Helper()
	return versionsApp(t, optionsProtocols, optionsCaps)
}

func groupIDs(groups []types.OptionGroup) []string {
	var ids []string
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	return ids
}

func TestEchoOffersOptionsButCoreIgnoresTheUnknownKind(t *testing.T) {
	e := optionsEnv(t)
	got, err := e.a.GetGameOptions(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if ids := strings.Join(groupIDs(got.Groups), ","); ids != "language,dlc,packs,subtitles" {
		t.Fatalf("groups = %s: the carousel group must be ignored, the others kept in order", ids)
	}
	if got.Chosen["language"] != "en" || got.Chosen["subtitles"] != false {
		t.Fatalf("defaults = %v", got.Chosen)
	}
	for _, g := range got.Groups {
		if g.ID == "dlc" && (!g.DLC || len(g.Values) != 3) {
			t.Fatalf("dlc group = %+v", g)
		}
	}
}

func TestOptionsAreRememberedAndResentOnUpdate(t *testing.T) {
	e := optionsEnv(t)
	choices := types.InstallOptions{"language": "fr", "dlc": []any{"season"}, "packs": []any{"hd"}, "subtitles": true}
	j, err := e.a.InstallGameWithOptions(e.game, choices)
	if err != nil {
		t.Fatal(err)
	}
	if j.Options["language"] != "fr" {
		t.Fatalf("queued options = %v", j.Options)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if n := e.calls(t)["game.install"]; n != 1 {
		t.Fatalf("game.install reached echo %d times", n)
	}

	saved, ok := e.a.settings.Options(e.game.ID)
	if !ok || saved["language"] != "fr" {
		t.Fatalf("choices not remembered: %v %v", saved, ok)
	}

	// The installed add-on is reported back by the store.
	e.game.IsInstalled = true
	got, err := e.a.GetGameOptions(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if got.Chosen["language"] != "fr" {
		t.Fatalf("chosen = %v, want the remembered language", got.Chosen)
	}
	for _, g := range got.Groups {
		if g.ID != "dlc" {
			continue
		}
		for _, v := range g.Values {
			if v.ID == "season" && !v.Installed {
				t.Fatalf("season pass not reported installed: %+v", v)
			}
		}
	}

	// A plain update sends the remembered choices again.
	u, err := e.a.UpdateGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if u.Options["language"] != "fr" || u.Options["subtitles"] != true {
		t.Fatalf("plain update options = %v, want the remembered ones", u.Options)
	}
	e.waitJob(t, u.ID, jobs.StateDone)

	// Changing the options of an installed game is an update with the new ones.
	c, err := e.a.UpdateGameWithOptions(e.game, types.InstallOptions{"language": "de"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != jobs.KindUpdate || c.Options["language"] != "de" {
		t.Fatalf("option change job = %+v", c)
	}
	e.waitJob(t, c.ID, jobs.StateDone)
	if saved, _ := e.a.settings.Options(e.game.ID); saved["language"] != "de" {
		t.Fatalf("the new choices were not remembered: %v", saved)
	}
}

func TestUnownedDLCIsRefusedWhateverTheDialogSays(t *testing.T) {
	e := optionsEnv(t)
	_, err := e.a.InstallGameWithOptions(e.game, types.InstallOptions{"dlc": []any{"deluxe"}})
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("unowned dlc: err = %v, want a refusal", err)
	}
	if n := e.calls(t)["game.install"]; n != 0 {
		t.Fatalf("game.install reached echo %d times for refused DLC", n)
	}
	if len(e.a.jobs.List()) != 0 {
		t.Fatalf("a job was queued for refused DLC: %+v", e.a.jobs.List())
	}
	if _, ok := e.a.settings.Options(e.game.ID); ok {
		t.Fatal("refused choices were remembered")
	}
}

func TestUnknownChoicesAreRefused(t *testing.T) {
	e := optionsEnv(t)
	for _, bad := range []types.InstallOptions{
		{"language": "klingon"},
		{"packs": []any{"nope"}},
		{"subtitles": "yes"},
	} {
		if _, err := e.a.InstallGameWithOptions(e.game, bad); err == nil {
			t.Errorf("choices %v were accepted", bad)
		}
	}
	if n := e.calls(t)["game.install"]; n != 0 {
		t.Fatalf("game.install reached echo %d times", n)
	}
}

func TestSizeWithOptionsCountsTheChosenAddOns(t *testing.T) {
	e := optionsEnv(t)
	ctx := context.Background()
	var plain, with struct {
		DownloadBytes int64 `json:"downloadBytes"`
	}
	if err := e.m.Call(ctx, "echo", "game.installSize", map[string]any{"id": "tiny"}, &plain); err != nil {
		t.Fatal(err)
	}
	options := map[string]any{"dlc": []any{"season", "soundtrack"}, "packs": []any{"hd"}}
	if err := e.m.Call(ctx, "echo", "game.installSize", map[string]any{"id": "tiny", "options": options}, &with); err != nil {
		t.Fatal(err)
	}
	if extra := with.DownloadBytes - plain.DownloadBytes; extra != (1+2+3)<<20 {
		t.Fatalf("options add %d bytes, want 6 MiB", extra)
	}
	// The queue sizes a job with its options, so the free-space check counts them.
	chosen, err := e.a.InstallGameWithOptions(e.game, types.InstallOptions{"dlc": []any{"season", "soundtrack"}, "packs": []any{"hd"}})
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, chosen.ID, jobs.StateDone)
	if n := e.calls(t)["game.installSize"]; n == 0 {
		t.Fatal("the job never asked for its size")
	}
}

func TestNoOptionCallsWithoutProtocol15OrWithTheCapabilityOff(t *testing.T) {
	for name, env := range map[string]*versionsEnv{
		"over 1.3":          versionsApp(t, allVersions, `["library","install","auth","discard","versions","install-options"]`),
		"capability not on": versionsApp(t, optionsProtocols, echoCaps),
	} {
		if _, err := env.a.GetGameOptions(env.game); err == nil {
			t.Errorf("%s: GetGameOptions succeeded", name)
		}
		if _, err := env.a.InstallGameWithOptions(env.game, types.InstallOptions{"language": "fr"}); err == nil {
			t.Errorf("%s: choices were accepted", name)
		}
		j, err := env.a.InstallGame(env.game)
		if err != nil {
			t.Fatalf("%s: plain install: %v", name, err)
		}
		if j.Options != nil {
			t.Errorf("%s: a plain install carries options %v", name, j.Options)
		}
		env.waitJob(t, j.ID, jobs.StateDone)
		calls := env.calls(t)
		if calls["game.options"] != 0 || calls["game.install"] != 1 {
			t.Errorf("%s: calls = %v, want no game.options and one install", name, calls)
		}
		if len(env.a.jobs.List()) == 0 {
			t.Fatalf("%s: no job was queued", name)
		}
	}
}

func TestOptionsCombineWithAVersionTarget(t *testing.T) {
	e := optionsEnv(t)
	if err := e.a.PinGameVersion(e.game, types.VersionPin{Kind: types.PinBuild, ID: "b200"}); err != nil {
		t.Fatal(err)
	}
	j, err := e.a.InstallGameWithOptions(e.game, types.InstallOptions{"language": "de"})
	if err != nil {
		t.Fatal(err)
	}
	if j.Target == nil || j.Target.Build != "b200" || j.Options["language"] != "de" {
		t.Fatalf("job = target %+v options %v, want the pin and the language", j.Target, j.Options)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if h := e.a.ledger.Entries(e.game.ID); len(h) != 1 || h[0].Build != "b200" {
		t.Fatalf("ledger = %+v, want the pinned build installed", h)
	}
}
