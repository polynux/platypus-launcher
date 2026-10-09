package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

// Wails only logs through its own contexts, so the tests drop the warnings. It
// is set once: plugins of an earlier test may still be reading it.
func init() { logWarning = func(context.Context, string, ...any) {} }

// Family and plugin settings (protocol 1.8) around the echo test plugin. The
// echo manifest offers both; the protocol list decides whether they are on.

const familyCaps = `["library","install","launch","auth","discard","family","settings"]`
const familyProtocols = `["1.0","1.1","1.8"]`

// familyEnv is an App with echo approved and logged in, and the family
// warnings it emits recorded.
type familyEnv struct {
	a    *App
	m    *plugins.Manager
	st   *settings.Store
	mu   sync.Mutex
	seen []types.FamilyWarning
}

func familyApp(t *testing.T, protocols, caps string) *familyEnv {
	t.Helper()
	e := familyAppNotStarted(t, protocols, caps)
	if err := e.a.plugins.Call(context.Background(), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	return e
}

// familyAppNotStarted is familyApp before echo has logged in or started: its
// manifest says what it offers, but nothing has initialized yet.
func familyAppNotStarted(t *testing.T, protocols, caps string) *familyEnv {
	t.Helper()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	e := &familyEnv{st: st}
	a := &App{settings: st, ctx: context.Background()}
	a.familyEmit = func(w types.FamilyWarning) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.seen = append(e.seen, w)
	}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		OnNotify: a.pluginNotify, OnReady: a.pluginReady,
	})
	t.Cleanup(a.plugins.Shutdown)
	a.stores = stores.NewRegistry(a.pluginStores)
	installEchoApp(t, pluginsDir)
	manifest := `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
		"protocol":{"versions":` + protocols + `},"store":"echo","exec":"bin/hydra-echo-app",
		"capabilities":` + caps + `,"requires":[{"bin":"sleep","hint":"part of coreutils"}]}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	e.a, e.m = a, a.plugins
	return e
}

func (e *familyEnv) calls(t *testing.T) map[string]int {
	t.Helper()
	var calls map[string]int
	if err := e.m.Call(context.Background(), "echo", "calls", nil, &calls); err != nil {
		t.Fatal(err)
	}
	return calls
}

// config sets echo's knobs (a `config` request) and returns what it answers.
func (e *familyEnv) config(t *testing.T, knobs map[string]any) map[string]any {
	t.Helper()
	var res map[string]any
	if err := e.m.Call(context.Background(), "echo", "config", knobs, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// store is echo as the store adapter sees it: its library and launches.
func (e *familyEnv) store() *stores.Plugin {
	p, _ := e.a.plugins.Plugin("echo")
	return stores.NewPlugin(e.m, p, func() []string { return nil })
}

// libraryGame finds an echo game in the plugin's library, as the library shows it.
func (e *familyEnv) libraryGame(t *testing.T, storeID string) types.Game {
	t.Helper()
	games, err := e.store().Library(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range games {
		if g.StoreAppID == storeID {
			return g
		}
	}
	t.Fatalf("echo has no game %q in its library", storeID)
	return types.Game{}
}

func (e *familyEnv) warnings() []types.FamilyWarning {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.seen)
}

func TestFamilyMembersAreListedAndNormalized(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	members, err := e.a.GetFamilyMembers("echo")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %+v, want the two family members echo has", members)
	}
	ana, ben := members[0], members[1]
	if ana.ID != "ana" || ana.Name != "Ana" || ana.Role != "adult" || ana.AvatarURL == "" || ana.State != "online" {
		t.Errorf("ana = %+v", ana)
	}
	if ben.ID != "ben" || ben.Role != "child" || ben.AvatarURL != "" {
		t.Errorf("ben = %+v", ben)
	}
}

func TestBorrowedGamesCarryTheirLenderWhileFamilyIsOn(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	ana := e.libraryGame(t, "ana")
	if ana.SharedBy == nil || ana.SharedBy.ID != "ana" || ana.SharedBy.Name != "Ana" {
		t.Fatalf("shared by = %+v, want Ana", ana.SharedBy)
	}
	if !slices.Contains(ana.Features, "family") || !ana.IsInstalled {
		t.Fatalf("a borrowed game is installed and offers family: %+v", ana)
	}
	if own := e.libraryGame(t, "tiny"); own.SharedBy != nil {
		t.Fatalf("the user's own game is marked shared: %+v", own.SharedBy)
	}
}

func TestFamilyOffMeansNothingSentAndNoBorrowedMark(t *testing.T) {
	t.Setenv("ECHO_OFF", "family")
	e := familyApp(t, familyProtocols, familyCaps)
	if _, err := e.a.GetFamilyMembers("echo"); err == nil {
		t.Fatal("members of a plugin with family off: want an error")
	}
	if ana := e.libraryGame(t, "ana"); ana.SharedBy != nil {
		t.Fatalf("family off, yet the game is marked shared: %+v", ana.SharedBy)
	}
	if n := e.calls(t)["family.members"]; n != 0 {
		t.Fatalf("family.members reached echo %d times with family off", n)
	}
}

func TestFamilyBelowProtocol18SendsNothing(t *testing.T) {
	e := familyApp(t, `["1.0","1.1","1.7"]`, familyCaps)
	if _, err := e.a.GetFamilyMembers("echo"); err == nil {
		t.Fatal("members over 1.7: want an error, the capability is off for that protocol")
	}
	if ana := e.libraryGame(t, "ana"); ana.SharedBy != nil {
		t.Fatalf("over 1.7 the game is marked shared: %+v", ana.SharedBy)
	}
	if n := e.calls(t)["family.members"]; n != 0 {
		t.Fatalf("family.members reached echo %d times over 1.7", n)
	}
}

func TestLaunchingABorrowedGameInUseFailsOnceWithTheOwnersName(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"familyInUse": []string{"ana"}})
	ana := e.libraryGame(t, "ana")
	_, err := e.store().Launch(context.Background(), ana, false)
	if err == nil {
		t.Fatal("a game in use launched")
	}
	if !plugins.IsUnavailable(err) || !strings.Contains(err.Error(), "Family member Ana is playing this game") {
		t.Fatalf("err = %v, want -32002 with the owner's name", err)
	}
	if n := e.calls(t)["game.launchSpec"]; n != 1 {
		t.Fatalf("game.launchSpec reached echo %d times, want one: a -32002 is not retried", n)
	}
	if _, err := e.store().Launch(context.Background(), e.libraryGame(t, "ben"), false); err != nil {
		t.Fatalf("a game not in use launches: %v", err)
	}
}

func TestFamilyWarningIsRaisedForTheUIOnlyWhileFamilyIsOn(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"familyWarn": 30})
	if _, err := e.store().Launch(context.Background(), e.libraryGame(t, "ben"), false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(e.warnings()) == 1 })
	w := e.warnings()[0]
	if w.Source != "echo" || w.GameID != stores.GameID("echo", "ben") || w.SecondsLeft != 30 || !strings.Contains(w.Message, "Ben") {
		t.Fatalf("warning = %+v", w)
	}
}

func TestFamilyWarningFromAPluginWithFamilyOffIsIgnored(t *testing.T) {
	t.Setenv("ECHO_OFF", "family")
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"familyWarn": 30})
	if _, err := e.store().Launch(context.Background(), e.libraryGame(t, "ben"), false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(e.warnings()); n != 0 {
		t.Fatalf("%d warnings from a plugin without family", n)
	}
}

func TestPluginSettingsShowOnlyThePluginScopeAndDropUnknownKinds(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	view, err := e.a.GetPluginSettings("echo")
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, g := range view.Groups {
		for _, f := range g.Fields {
			fields = append(fields, f.ID)
		}
	}
	if got := strings.Join(fields, ","); got != "eos-overlay,region,note,volume" {
		t.Fatalf("plugin fields = %s: the game field and the unknown kind must not appear", got)
	}
	if len(view.Values) != 0 {
		t.Fatalf("nothing saved yet, values = %v", view.Values)
	}
}

func TestSavedPluginSettingsReachThePluginAndSurviveARestart(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	_, err := e.a.SavePluginSettings("echo", map[string]any{"eos-overlay": true, "region": "us", "volume": 7.0, "game-overlay": false})
	if err != nil {
		t.Fatal(err)
	}
	snap := e.config(t, nil)["settings"].(map[string]any)
	plug := snap["plugin"].(map[string]any)
	if plug["eos-overlay"] != true || plug["region"] != "us" || plug["volume"] != 7.0 {
		t.Fatalf("echo got %v", plug)
	}
	if _, ok := plug["game-overlay"]; ok {
		t.Fatal("a per-game field was saved as plugin-wide")
	}
	if got := e.a.settings.PluginValues("echo"); got["region"] != "us" || len(got) != 3 {
		t.Fatalf("saved values = %v", got)
	}

	// echo keeps its values in memory: a restart loses them until core sends them again.
	if err := e.m.Call(context.Background(), "echo", "crash", nil, nil); err == nil {
		t.Fatal("crash answered")
	}
	waitFor(t, func() bool {
		plug := e.config(t, nil)["settings"].(map[string]any)["plugin"].(map[string]any)
		return plug["region"] == "us"
	})
}

func TestPluginSettingsRefuseWrongValuesAndKeepNothing(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 11.0}); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("out of range: err = %v", err)
	}
	if got := e.a.settings.PluginValues("echo"); len(got) != 0 {
		t.Fatalf("refused values were saved: %v", got)
	}
}

func TestGameSettingsOnlyForAGameWhoseFeaturesIncludeThem(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	tiny := e.libraryGame(t, "tiny")
	view, err := e.a.GetGameSettingsFields(tiny)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Groups) != 1 || view.Groups[0].Fields[0].ID != "game-overlay" {
		t.Fatalf("game view = %+v", view.Groups)
	}
	if _, err := e.a.SaveGameSettingsFields(tiny, map[string]any{"game-overlay": false, "eos-overlay": true}); err != nil {
		t.Fatal(err)
	}
	snap := e.config(t, nil)["settings"].(map[string]any)["games"].(map[string]any)
	if got := snap["tiny"].(map[string]any); got["game-overlay"] != false || len(got) != 1 {
		t.Fatalf("echo got for tiny: %v", got)
	}
	saved, ok := e.a.settings.PluginGameSettings(tiny.ID)
	if !ok || saved.StoreID != "tiny" || saved.Values["game-overlay"] != false {
		t.Fatalf("saved = %+v %v", saved, ok)
	}
}

func TestGameSettingsOffWhenTheGameDoesNotOfferThem(t *testing.T) {
	t.Setenv("ECHO_GAME_FEATURES", `{"tiny":["install"]}`)
	e := familyApp(t, familyProtocols, familyCaps)
	tiny := e.libraryGame(t, "tiny")
	if _, err := e.a.GetGameSettingsFields(tiny); err == nil {
		t.Fatal("a game whose features leave out settings shows its settings")
	}
	if _, err := e.a.SaveGameSettingsFields(tiny, map[string]any{"game-overlay": true}); err == nil {
		t.Fatal("a game whose features leave out settings saved them")
	}
	if n := e.calls(t)["settings.apply"]; n != 0 {
		t.Fatalf("settings.apply reached echo %d times for a game without settings", n)
	}
}

func TestSettingsBelowProtocol18AreNeverSent(t *testing.T) {
	e := familyApp(t, `["1.0","1.1","1.7"]`, familyCaps)
	if _, err := e.a.GetPluginSettings("echo"); err == nil {
		t.Fatal("settings over 1.7: want an error")
	}
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 3.0}); err == nil {
		t.Fatal("saving settings over 1.7: want an error")
	}
	if n := e.calls(t)["settings.apply"]; n != 0 {
		t.Fatalf("settings.apply reached echo %d times over 1.7", n)
	}
}

// waitFor polls cond for up to two seconds: notifications and the plugin's
// start hook run on their own goroutines.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestASaveKeepsTheKeysItDoesNotSend(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"eos-overlay": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 3.0}); err != nil {
		t.Fatal(err)
	}
	got := e.a.settings.PluginValues("echo")
	if got["eos-overlay"] != true || got["volume"] != 3.0 || len(got) != 2 {
		t.Fatalf("saved = %v: a save sends only what changed and keeps the rest", got)
	}
	if plug := e.config(t, nil)["settings"].(map[string]any)["plugin"].(map[string]any); plug["eos-overlay"] != true || plug["volume"] != 3.0 {
		t.Fatalf("echo got %v, want the whole saved set", plug)
	}
}

func TestAPluginThatRefusesSettingsKeepsThemSavedAndSaysSo(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"settingsRefuse": true})
	res, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 3.0})
	if err != nil {
		t.Fatalf("a refused apply is not an error, the values are saved: %v", err)
	}
	if res.Applied || !strings.Contains(res.Message, "saved, but Echo didn't take them") {
		t.Fatalf("result = %+v, want not applied with the saved-but-not-applied message", res)
	}
	if got := e.a.settings.PluginValues("echo"); got["volume"] != 3.0 {
		t.Fatalf("saved = %v, want the refused values kept", got)
	}
	e.config(t, map[string]any{"settingsRefuse": false})
	res, err = e.a.SavePluginSettings("echo", map[string]any{"volume": 4.0})
	if err != nil || !res.Applied || res.Message != "" {
		t.Fatalf("after the plugin takes settings again: res = %+v, err = %v", res, err)
	}
}

func TestSavesAndStartsOfAPluginAreOrdered(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"settingsDelayMs": 30})
	var wg sync.WaitGroup
	for v := 1; v <= 8; v++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": v}); err != nil {
				t.Error(err)
			}
		}(float64(v))
	}
	wg.Wait()
	saved := e.a.settings.PluginValues("echo")["volume"]
	plug := e.config(t, nil)["settings"].(map[string]any)["plugin"].(map[string]any)
	if plug["volume"] != saved {
		t.Fatalf("echo holds volume %v, saved %v: the last applied value must be the last saved one", plug["volume"], saved)
	}
}

func TestPluginStartReadsTheValuesInsideItsTurn(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 3.0}); err != nil {
		t.Fatal(err)
	}
	// A save holds the plugin's settings turn; the start waits for it and must
	// send what the save stored, not the values it saw before.
	lock := e.a.settingsLocks.of("echo")
	lock.Lock()
	done := make(chan struct{})
	go func() {
		e.a.pluginReady("echo")
		close(done)
	}()
	if err := e.a.settings.SetPluginValues("echo", map[string]any{"volume": 9.0}); err != nil {
		t.Fatal(err)
	}
	lock.Unlock()
	<-done
	plug := e.config(t, nil)["settings"].(map[string]any)["plugin"].(map[string]any)
	if plug["volume"] != 9.0 {
		t.Fatalf("echo got volume %v, want 9 (the value stored inside the turn)", plug["volume"])
	}
}

func TestPluginStartSendsOnlyValuesTheSchemaStillTakes(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if err := e.a.settings.SetPluginValues("echo", map[string]any{"region": "mars", "volume": 3.0, "gone": true}); err != nil {
		t.Fatal(err)
	}
	e.a.pluginReady("echo")
	plug := e.config(t, nil)["settings"].(map[string]any)["plugin"].(map[string]any)
	if len(plug) != 1 || plug["volume"] != 3.0 {
		t.Fatalf("echo got %v, want only volume: region is no choice of the schema, gone is no field", plug)
	}
}

func TestSettingsOfGamesTheLibraryLostAreForgotten(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	tiny := e.libraryGame(t, "tiny")
	if err := e.a.settings.SetPluginGameSettings(tiny.ID, settings.PluginGameValues{Source: "echo", StoreID: "tiny", Values: map[string]any{"game-overlay": false}}); err != nil {
		t.Fatal(err)
	}
	gone := stores.GameID("echo", "gone")
	if err := e.a.settings.SetPluginGameSettings(gone, settings.PluginGameValues{Source: "echo", StoreID: "gone", Values: map[string]any{"game-overlay": false}}); err != nil {
		t.Fatal(err)
	}
	all := []stores.Store{e.store()}
	// A listing that failed says nothing about its games: their values stay.
	e.a.forgetRemovedGameSettings(all, [][]types.Game{nil}, []error{context.DeadlineExceeded})
	if _, ok := e.a.settings.PluginGameSettings(gone); !ok {
		t.Fatal("a failed library listing forgot a game's settings")
	}
	e.a.forgetRemovedGameSettings(all, [][]types.Game{{tiny}}, []error{nil})
	if _, ok := e.a.settings.PluginGameSettings(gone); ok {
		t.Fatal("a game the library no longer lists kept its settings")
	}
	if _, ok := e.a.settings.PluginGameSettings(tiny.ID); !ok {
		t.Fatal("a game the library still lists lost its settings")
	}
}

func TestSettingsOfAPluginThatIsGoneAreForgotten(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if _, err := e.a.SavePluginSettings("echo", map[string]any{"volume": 3.0}); err != nil {
		t.Fatal(err)
	}
	tiny := e.libraryGame(t, "tiny")
	if _, err := e.a.SaveGameSettingsFields(tiny, map[string]any{"game-overlay": false}); err != nil {
		t.Fatal(err)
	}
	e.a.forgetSettingsOfRemoved(nil) // a scan with no plugins: echo is gone
	if got := e.a.settings.PluginValues("echo"); len(got) != 0 {
		t.Fatalf("plugin values of a removed plugin kept: %v", got)
	}
	if _, ok := e.a.settings.PluginGameSettings(tiny.ID); ok {
		t.Fatal("per-game values of a removed plugin kept")
	}
}

func TestFamilyWarningBeforeTheRuntimeInitializedIsIgnored(t *testing.T) {
	e := familyAppNotStarted(t, familyProtocols, familyCaps)
	if rt := e.m.Runtime("echo"); rt.Initialized {
		t.Fatalf("echo is initialized before anything started it: %+v", rt)
	}
	// The manifest says echo offers family, but its own initialize has not
	// said so yet: a notification from it then is not taken.
	e.a.familyWarning("echo", json.RawMessage(`{"id":"ben","message":"Ben is taking this game back","secondsLeft":30}`))
	if n := len(e.warnings()); n != 0 {
		t.Fatalf("%d warnings from a plugin that has not initialized", n)
	}
}

// The "shared" game is lent by Ana and Ben, whose copies differ: Ana's has
// the soundtrack, Ben's the soundtrack and the track pack.
func TestSharedGameListsEveryCopyWithItsDLC(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	shared := e.libraryGame(t, "shared")
	if shared.SharedBy == nil || shared.SharedBy.ID != "ana" {
		t.Fatalf("shared by = %+v, want Ana (the default copy)", shared.SharedBy)
	}
	if len(shared.Copies) != 2 {
		t.Fatalf("copies = %+v, want Ana's and Ben's", shared.Copies)
	}
	ana, ben := shared.Copies[0], shared.Copies[1]
	if ana.OwnerID != "ana" || ana.OwnerName != "Ana" || !ana.Available || ana.Note == "" || len(ana.DLC) != 1 {
		t.Errorf("ana's copy = %+v", ana)
	}
	if ben.OwnerID != "ben" || ben.OwnerName != "Ben" || !ben.Available || len(ben.DLC) != 2 || ben.DLC[1].Name != "Track pack" {
		t.Errorf("ben's copy = %+v", ben)
	}
	if shared.PreferredCopy != "" {
		t.Fatalf("nothing chosen yet, preferred = %q", shared.PreferredCopy)
	}
}

func TestSettingAPreferredCopyIsSentStoredAndReadBackAfterARestart(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if err := e.a.SetPreferredCopy(e.libraryGame(t, "shared"), "ben"); err != nil {
		t.Fatal(err)
	}
	shared := e.libraryGame(t, "shared")
	if shared.SharedBy == nil || shared.SharedBy.ID != "ben" || shared.PreferredCopy != "ben" {
		t.Fatalf("after the pick: shared by %+v, preferred %q: the copy played is Ben's", shared.SharedBy, shared.PreferredCopy)
	}

	// The choice lives in HYDRA_PLUGIN_DATA: echo reads it back after a crash.
	if err := e.m.Call(context.Background(), "echo", "crash", nil, nil); err == nil {
		t.Fatal("crash answered")
	}
	waitFor(t, func() bool { return e.libraryGame(t, "shared").PreferredCopy == "ben" })
}

func TestSetPreferredCopyIsRefusedWithoutSendingWhenItDoesNotApply(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		caps     string
		env      string
		game     string
		owner    string
		config   map[string]any
	}{
		{name: "family off", protocol: familyProtocols, env: "family", game: "shared", owner: "ben"},
		{name: "below protocol 1.8", protocol: `["1.0","1.1","1.7"]`, game: "shared", owner: "ben"},
		{name: "the user's own game", game: "tiny", owner: "ben"},
		{name: "a copy nobody lends", game: "shared", owner: "carl"},
		{name: "an unavailable copy", game: "shared", owner: "ben", config: map[string]any{"copyUnavailable": []string{"ben"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.env != "" {
				t.Setenv("ECHO_OFF", c.env)
			}
			protocols := c.protocol
			if protocols == "" {
				protocols = familyProtocols
			}
			e := familyApp(t, protocols, familyCaps)
			if c.config != nil {
				e.config(t, c.config)
			}
			if err := e.a.SetPreferredCopy(e.libraryGame(t, c.game), c.owner); err == nil {
				t.Fatal("refused, but it was accepted")
			}
			if n := e.calls(t)["family.setPreferredCopy"]; n != 0 {
				t.Fatalf("family.setPreferredCopy reached echo %d times", n)
			}
		})
	}
}

func TestOwnCopyHasNoPickerAndNoPreferredCopy(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	tiny := e.libraryGame(t, "tiny")
	if tiny.SharedBy != nil || len(tiny.Copies) != 0 || tiny.PreferredCopy != "" {
		t.Fatalf("the user's own game carries copies: %+v", tiny)
	}
	if err := e.a.SetPreferredCopy(tiny, "ana"); err == nil {
		t.Fatal("a pick on the user's own game was accepted")
	}
}

func TestAnInUseCopyRefusesLaunchOnlyWhileItIsPicked(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	e.config(t, map[string]any{"copyInUse": []string{"ben"}})
	if err := e.a.SetPreferredCopy(e.libraryGame(t, "shared"), "ben"); err != nil {
		t.Fatal(err)
	}
	_, err := e.store().Launch(context.Background(), e.libraryGame(t, "shared"), false)
	if !plugins.IsUnavailable(err) || !strings.Contains(err.Error(), "Family member Ben is playing this game") {
		t.Fatalf("launch from Ben's copy in use: err = %v", err)
	}
	if err := e.a.SetPreferredCopy(e.libraryGame(t, "shared"), "ana"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store().Launch(context.Background(), e.libraryGame(t, "shared"), false); err != nil {
		t.Fatalf("launch from Ana's copy, not in use: %v", err)
	}
}
