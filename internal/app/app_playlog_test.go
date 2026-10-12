package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/playlog"
	"platypus/internal/session"
	"platypus/internal/types"
)

func playlogApp(t *testing.T) *App {
	t.Helper()
	l, err := playlog.Open(filepath.Join(t.TempDir(), "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &App{host: stderrHost{}, playlog: l}
}

// logSession records a closed session of d in the app's log, ending now.
func logSession(t *testing.T, a *App, game string, d time.Duration) {
	t.Helper()
	now := time.Now()
	a.playlog.StartAt(game, now.Add(-d))
	if _, ok := a.playlog.FinishAt(game, now); !ok {
		t.Fatalf("no session closed for %s", game)
	}
}

func TestLocalPlaytimeIsMaxWithTheStoreReport(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	a.stats.setPlaytime("echo", map[string]int{"tiny": 150})
	logSession(t, a, "p.echo.tiny", 125*time.Second) // 2 min local, 150 store

	games := []types.Game{
		{ID: "p.echo.tiny", Source: "echo", StoreAppID: "tiny"},
	}
	a.applyPlaytime(games)
	g := games[0]
	if g.PlaytimeMin != 150 {
		t.Fatalf("shown = %d, want max(2, 150)", g.PlaytimeMin)
	}
	if !sameInt(g.PlaytimeLocalMin, 2) || !sameInt(g.PlaytimeStoreMin, 150) {
		t.Fatalf("parts = local %v, store %v; want 2, 150", g.PlaytimeLocalMin, g.PlaytimeStoreMin)
	}
	if g.LastPlayed != a.playlog.LastPlayed("p.echo.tiny") {
		t.Fatalf("last played = %d, want the log's", g.LastPlayed)
	}
}

func TestLocalPlaytimeWinsWhenItIsLarger(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	a.stats.setPlaytime("echo", map[string]int{"tiny": 150})
	logSession(t, a, "p.echo.tiny", 200*time.Minute)

	games := []types.Game{
		{ID: "p.echo.tiny", Source: "echo", StoreAppID: "tiny"},
	}
	a.applyPlaytime(games)
	g := games[0]
	if g.PlaytimeMin != 200 {
		t.Fatalf("shown = %d, want max(200, 150)", g.PlaytimeMin)
	}
	if !sameInt(g.PlaytimeLocalMin, 200) || !sameInt(g.PlaytimeStoreMin, 150) {
		t.Fatalf("parts = local %v, store %v; want 200, 150", g.PlaytimeLocalMin, g.PlaytimeStoreMin)
	}
}

func TestLocalPlaytimeReachesGamesWithoutAStoreReport(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	logSession(t, a, "p.echo.client", 70*time.Second) // 1 min, no store report
	logSession(t, a, "custom-1", 70*time.Second)

	games := []types.Game{
		{ID: "p.echo.client", Source: "echo", StoreAppID: "client", PlaytimeMin: 5},
		{ID: "custom-1", Source: types.SourceCustom, Store: types.StoreCustom},
	}
	a.applyPlaytime(games)
	// The plugin game's carried value (5) is local too: the larger shows.
	if g := games[0]; g.PlaytimeMin != 5 || !sameInt(g.PlaytimeLocalMin, 5) || g.PlaytimeStoreMin != nil {
		t.Fatalf("client = %+v, want shown 5, local 5, no store", g)
	}
	if g := games[1]; g.PlaytimeMin != 1 || !sameInt(g.PlaytimeLocalMin, 1) || g.PlaytimeStoreMin != nil {
		t.Fatalf("sideloaded = %+v, want shown 1, local 1, no store", g)
	}
	if games[0].LastPlayed == 0 || games[1].LastPlayed == 0 {
		t.Fatal("last played wasn't set from the log")
	}
}

func TestLocalPlaytimeMergesWithSteamsOwnRecord(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	// Steam's record rides with the listing: 30 min, played at 1000.
	logSession(t, a, "steam_10", 70*time.Second) // 1 min local

	games := []types.Game{
		{ID: "steam_10", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "10",
			PlaytimeMin: 30, LastPlayed: 1000},
	}
	a.applyPlaytime(games)
	g := games[0]
	if g.PlaytimeMin != 30 {
		t.Fatalf("shown = %d, want max(1, 30)", g.PlaytimeMin)
	}
	if !sameInt(g.PlaytimeLocalMin, 1) || !sameInt(g.PlaytimeStoreMin, 30) {
		t.Fatalf("parts = local %v, store %v; want 1, 30", g.PlaytimeLocalMin, g.PlaytimeStoreMin)
	}
	if g.LastPlayed <= 1000 {
		t.Fatalf("last played = %d, want the log's (after 1000)", g.LastPlayed)
	}

	// A Steam game played longer in Platypus shows the local total.
	logSession(t, a, "steam_11", 65*time.Minute)
	older := []types.Game{
		{ID: "steam_11", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "11",
			PlaytimeMin: 30, LastPlayed: 1000},
	}
	a.applyPlaytime(older)
	if older[0].PlaytimeMin != 65 {
		t.Fatalf("shown = %d, want max(65, 30)", older[0].PlaytimeMin)
	}
}

func TestLastPlayedIsMaxOfLocalAndStore(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	a.playlog.StartAt("g", time.Unix(5000, 0))
	a.playlog.FinishAt("g", time.Unix(5005, 0)) // short: no minutes, but played at 5005

	games := []types.Game{
		{ID: "g", Source: types.SourceSteam, Store: types.StoreSteam, LastPlayed: 9000},
		{ID: "h", Source: types.SourceCustom, Store: types.StoreCustom, LastPlayed: 1000},
	}
	// h never ran locally: the log knows nothing, the store value stays.
	a.playlog.StartAt("h2", time.Unix(100, 0))
	a.applyPlaytime(games)
	if games[0].LastPlayed != 9000 {
		t.Fatalf("last played = %d, want max(5005, 9000)", games[0].LastPlayed)
	}
	if games[1].LastPlayed != 1000 {
		t.Fatalf("last played = %d, want the store's 1000", games[1].LastPlayed)
	}
	// A short local run newer than the store wins.
	a.playlog.StartAt("h", time.Unix(2000, 0))
	a.playlog.FinishAt("h", time.Unix(2003, 0))
	a.applyPlaytime(games)
	if games[1].LastPlayed != 2003 {
		t.Fatalf("last played = %d, want the local 2003", games[1].LastPlayed)
	}
	if games[1].PlaytimeMin != 0 || games[1].PlaytimeLocalMin != nil {
		t.Fatalf("a 3 s run counted playtime: %+v", games[1])
	}
}

func TestSessionEventsOpenAndCloseLocalSessions(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	a.playSession(types.SessionEvent{GameID: "g", State: "starting"})
	if _, ok := a.playlog.Finish("g"); ok {
		t.Fatal("starting opened a session")
	}
	a.playSession(types.SessionEvent{GameID: "g", State: "running"})
	a.playSession(types.SessionEvent{GameID: "g", State: "stopping"})
	if _, ok := a.playlog.Finish("g"); !ok {
		t.Fatal("running opened no session")
	}
	// The run above lasted ~0 s: last played is set, minutes are not.
	if a.playlog.LastPlayed("g") == 0 {
		t.Fatal("stopped set no last played")
	}
	if got := a.playlog.Minutes("g"); got != 0 {
		t.Fatalf("minutes = %d, want 0", got)
	}
	a.playSession(types.SessionEvent{GameID: "g", State: "stopped"})
	a.playSession(types.SessionEvent{GameID: "g", State: "failed"})
	// A failure without a running state (the client never started the game)
	// records nothing.
	a.playSession(types.SessionEvent{GameID: "never-ran", State: "failed"})
	if a.playlog.LastPlayed("never-ran") != 0 {
		t.Fatal("a failure without a session recorded play")
	}
}

func TestHeartbeatKeepsTheSessionOpen(t *testing.T) {
	t.Parallel()
	a := playlogApp(t)
	a.playSession(types.SessionEvent{GameID: "g", State: "running"})
	a.playHeartbeat() // no tracker: nothing happens, nothing breaks
	if _, ok := a.playlog.Finish("g"); !ok {
		t.Fatal("the session vanished")
	}
}

func TestHeartbeatRefreshesRunningTrackerSessions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l, err := playlog.Open(filepath.Join(dir, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{host: stderrHost{}, playlog: l}
	a.sessions = session.NewTracker(func(e types.SessionEvent) { a.playSession(e) })
	if _, err := a.sessions.StartDirect("hg", exec.Command("sleep", "30"), "sleep 30", nil); err != nil {
		t.Fatal(err)
	}
	// The heartbeat only moves when it lands on a later Unix second than the
	// start: wait for the second boundary instead of a fixed 1100ms sleep,
	// which wastes a second and still assumes the scheduler is prompt.
	startSec := time.Now().Unix()
	boundary := time.Now().Add(5 * time.Second)
	for time.Now().Unix() <= startSec && time.Now().Before(boundary) {
		time.Sleep(50 * time.Millisecond)
	}
	a.playHeartbeat()
	raw, err := os.ReadFile(filepath.Join(dir, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Games map[string]struct {
			Open *struct {
				Start     int64 `json:"start"`
				Heartbeat int64 `json:"heartbeat"`
			} `json:"open"`
		} `json:"games"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	open := f.Games["hg"].Open
	if open == nil {
		t.Fatal("no open session on disk")
	}
	if open.Heartbeat <= open.Start {
		t.Fatalf("heartbeat %d didn't move past start %d", open.Heartbeat, open.Start)
	}
	if err := a.sessions.Stop("hg", true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for a.sessions.Running("hg") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
}
