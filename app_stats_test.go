package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"platypus/internal/stats"
	"platypus/internal/types"
)

// statsManifest declares the 1.7 stats capabilities over protocols that include 1.7.
const statsManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","achievements","playtime","redeem"]}`

// statsApp is an approved, initialized echo offering the stats capabilities.
// changes counts the playtime notifications the UI would get.
func statsApp(t *testing.T, manifest string) (a *App, changes *int) {
	t.Helper()
	a, m := echoApp(t, manifest)
	if err := m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	a.ctx = context.Background()
	n := 0
	a.stats.changed = func() { n++ }
	return a, &n
}

func TestPlaytimeMergesTheStoreValueForDisplayOnly(t *testing.T) {
	a, changes := statsApp(t, statsManifest)
	a.refreshPlaytime("echo")
	if *changes != 1 {
		t.Fatalf("first report notified %d times, want 1", *changes)
	}
	a.refreshPlaytime("echo")
	if *changes != 1 {
		t.Fatalf("an unchanged report notified the UI again: %d", *changes)
	}
	if !a.stats.isFresh("echo") {
		t.Fatal("playtime wasn't marked as read for this run")
	}

	games := []types.Game{
		{Source: "echo", StoreAppID: "tiny", PlaytimeMin: 30},   // store 150 is more
		{Source: "echo", StoreAppID: "huge", PlaytimeMin: 2000}, // local is more
		{Source: "echo", StoreAppID: "client", PlaytimeMin: 5},  // no report: untouched
		{Source: "steam", StoreAppID: "tiny", PlaytimeMin: 7},   // another source: untouched
	}
	a.stats.applyPlaytime(games)
	if g := games[0]; g.PlaytimeMin != 150 || !sameInt(g.PlaytimeLocalMin, 30) || !sameInt(g.PlaytimeStoreMin, 150) {
		t.Fatalf("tiny merged = %+v", g)
	}
	if g := games[1]; g.PlaytimeMin != 2000 || !sameInt(g.PlaytimeLocalMin, 2000) || !sameInt(g.PlaytimeStoreMin, 1200) {
		t.Fatalf("huge merged = %+v", g)
	}
	if g := games[2]; g.PlaytimeMin != 5 || g.PlaytimeStoreMin != nil || g.PlaytimeLocalMin != nil {
		t.Fatalf("client changed without a report: %+v", g)
	}
	if g := games[3]; g.PlaytimeMin != 7 || g.PlaytimeStoreMin != nil {
		t.Fatalf("steam game took echo's playtime: %+v", g)
	}
}

// sameInt reports whether a pointer field holds want.
func sameInt(got *int, want int) bool {
	return got != nil && *got == want
}

func TestAchievementsAreFetchedOnceAndHiddenUntilUnlocked(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	g := echoGame(t)

	got, err := a.GetGameAchievements(g)
	if err != nil || got == nil {
		t.Fatalf("GetGameAchievements = %v, %v", got, err)
	}
	if got.Total != 5 || got.Unlocked != 2 || len(got.List) != 5 {
		t.Fatalf("progress = %+v", got)
	}
	byID := map[string]types.ProgressAchievement{}
	for _, x := range got.List {
		byID[x.ID] = x
	}
	var secret types.ProgressAchievement
	for _, x := range got.List {
		if x.Hidden && !x.Unlocked {
			secret = x
		}
	}
	if secret.Name != stats.HiddenName || secret.Description != "" || secret.Icon != "" || secret.ID == "secret" {
		t.Fatalf("a locked hidden achievement leaked its text or store id: %+v", secret)
	}
	if x := byID["ninety"]; x.Name != "Ninety seconds" || !x.Unlocked || x.UnlockedAt == 0 || x.GlobalPercent != nil {
		t.Fatalf("an unlocked hidden achievement must show, with no global share: %+v", x)
	}
	if x := byID["installed"]; x.GlobalPercent == nil || *x.GlobalPercent != 100 {
		t.Fatalf("installed global share = %v", x.GlobalPercent)
	}

	again, err := a.GetGameAchievements(g)
	if err != nil || again == nil || again.Total != 5 {
		t.Fatalf("second read = %+v, %v", again, err)
	}
	if n := echoCalls(t, a)["achievements.list"]; n != 1 {
		t.Fatalf("achievements.list reached the plugin %d times, want 1 (cached)", n)
	}
}

func TestAchievementsAreAbsentWhenTheGameOrPluginDoesNotOfferThem(t *testing.T) {
	// Protocol 1.5 is the last this plugin agrees to: achievements (1.7) stay off.
	a, _ := statsApp(t, strings.Replace(statsManifest, `,"1.6","1.7"`, "", 1))
	if got, err := a.GetGameAchievements(echoGame(t)); got != nil || err != nil {
		t.Fatalf("over 1.5: %v, %v; want nil and no error (the page uses the catalogue)", got, err)
	}
	if n := echoCalls(t, a)["achievements.list"]; n != 0 {
		t.Fatalf("achievements.list was sent over 1.5: %d", n)
	}

	b, _ := statsApp(t, statsManifest)
	g := echoGame(t)
	g.Features = []string{"library", "launch"}
	if got, _ := b.GetGameAchievements(g); got != nil {
		t.Fatalf("a game whose features leave out achievements got progress: %+v", got)
	}
	if n := echoCalls(t, b)["achievements.list"]; n != 0 {
		t.Fatalf("achievements.list reached the plugin for a game without the feature: %d", n)
	}
}

func TestRedeemGrantsTheTitleForTheEchoCode(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	res, err := a.RedeemCode("echo", "  ECHO-OK \n")
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if res.Message == "" || !slices.Equal(res.Titles, []string{"Echo Bonus Pack"}) {
		t.Fatalf("result = %+v", res)
	}
	if _, err := a.RedeemCode("echo", "ECHO-NOPE"); err == nil || !strings.Contains(err.Error(), "doesn't know that code") {
		t.Fatalf("a wrong code: %v, want the plugin's readable message", err)
	}
	if _, err := a.RedeemCode("echo", "   "); !errors.Is(err, stats.ErrCodeEmpty) {
		t.Fatalf("an empty code: %v", err)
	}
	if _, err := a.RedeemCode("echo", strings.Repeat("A", stats.MaxCodeLen+1)); !errors.Is(err, stats.ErrCodeTooLong) {
		t.Fatalf("a long code: %v", err)
	}
	if _, err := a.RedeemCode("echo", "ECHO\tOK"); !errors.Is(err, stats.ErrCodeControl) {
		t.Fatalf("a code with a tab: %v", err)
	}
}

func TestRedeemRefusesASecondSubmissionWhileOneRuns(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	a.stats.redeemMu.Lock()
	defer a.stats.redeemMu.Unlock()
	if _, err := a.RedeemCode("echo", "ECHO-OK"); err == nil || !strings.Contains(err.Error(), "already being redeemed") {
		t.Fatalf("a second submission: %v", err)
	}
}

func TestRedeemIsNotSentToAPluginWithoutTheCapability(t *testing.T) {
	a, _ := statsApp(t, strings.Replace(statsManifest, `,"1.6","1.7"`, "", 1))
	if _, err := a.RedeemCode("echo", "ECHO-OK"); err == nil || !strings.Contains(err.Error(), "doesn't redeem codes") {
		t.Fatalf("over 1.5: %v", err)
	}
	if n := echoCalls(t, a)["redeem.submit"]; n != 0 {
		t.Fatalf("redeem.submit was sent over 1.5: %d", n)
	}
}

func TestSessionEndRefreshesOnlyTheEndedGamesPlugin(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	a.stats.delay = 10 * time.Millisecond
	a.stats.noteSource("p.echo.tiny", "echo")

	// Three ends in a burst make one read; a game of an unknown store makes none.
	a.schedulePlaytimeRefresh("p.echo.tiny")
	a.schedulePlaytimeRefresh("p.echo.tiny")
	a.schedulePlaytimeRefresh("p.echo.tiny")
	a.schedulePlaytimeRefresh("p.other.game")
	time.Sleep(300 * time.Millisecond)

	if n := echoCalls(t, a)["playtime.list"]; n != 1 {
		t.Fatalf("playtime.list reached echo %d times after a burst of session ends, want 1", n)
	}
	a.stats.mu.Lock()
	defer a.stats.mu.Unlock()
	if _, ok := a.stats.playtime["echo"]; !ok {
		t.Fatal("the ended game's store playtime was not read")
	}
}

func TestPlaytimeReadIsNotStartedTwiceAtOnce(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	a.startPlaytimeOnce("echo")
	a.startPlaytimeOnce("echo")
	deadline := time.Now().Add(5 * time.Second)
	for !a.stats.isFresh("echo") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if n := echoCalls(t, a)["playtime.list"]; n != 1 {
		t.Fatalf("playtime.list reached echo %d times for two listings at once, want 1", n)
	}
}

func TestPlaytimeClaimIsExclusiveAndAFailureBacksOff(t *testing.T) {
	var c statsCache
	if !c.claim("echo") {
		t.Fatal("first claim refused")
	}
	if c.claim("echo") {
		t.Fatal("a second claim got through while the first was in flight")
	}
	c.release("echo", true)
	if !c.backingOff("echo") {
		t.Fatal("a failed read is retried at once")
	}
	c.retryAt["echo"] = time.Now().Add(-time.Second)
	if c.backingOff("echo") || !c.claim("echo") {
		t.Fatal("the read is not retried once the backoff has passed")
	}
}

func TestFailedPlaytimeReadIsNotRetriedOnEveryListing(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	a.stats.release("echo", true)
	a.startPlaytimeOnce("echo")
	if !a.stats.claim("echo") {
		t.Fatal("a listing started a read during the backoff after a failure")
	}
	a.stats.release("echo", false)
}

func TestPlaytimeTooltipFieldsKeepAStoreZeroAndOmitAnUnknownLocal(t *testing.T) {
	a, _ := statsApp(t, statsManifest)
	a.stats.setPlaytime("echo", map[string]int{"zero": 0, "tiny": 30})
	games := []types.Game{
		{Source: "echo", StoreAppID: "zero", PlaytimeMin: 0},
		{Source: "echo", StoreAppID: "tiny", PlaytimeMin: 30},
	}
	a.stats.applyPlaytime(games)
	zero, err := json.Marshal(games[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zero), `"playtime_store_minutes":0`) {
		t.Fatalf("a store report of 0 is missing from the JSON: %s", zero)
	}
	if strings.Contains(string(zero), "playtime_local_minutes") {
		t.Fatalf("an unknown local part was sent: %s", zero)
	}
	if games[0].PlaytimeLocalMin != nil || !sameInt(games[0].PlaytimeStoreMin, 0) {
		t.Fatalf("zero game fields = local %v, store %v", games[0].PlaytimeLocalMin, games[0].PlaytimeStoreMin)
	}
	if !sameInt(games[1].PlaytimeLocalMin, 30) {
		t.Fatalf("a known local value was dropped: %+v", games[1])
	}
}
