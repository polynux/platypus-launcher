package stats

import (
	"slices"
	"strings"
	"testing"
)

func f(v float64) *float64 { return &v }

func TestNormalizeAchievementsBlanksHiddenLockedOnly(t *testing.T) {
	out := NormalizeAchievements(AchievementsReply{Total: 4, Unlocked: 1, List: []Achievement{
		{ID: "open", Name: "Open", Description: "d", Icon: "https://cdn.example/a.png", GlobalPercent: f(12.5)},
		{ID: "secret", Name: "Real name", Description: "Real text", Icon: "https://cdn.example/s.png", Hidden: true},
		{ID: "won", Name: "Won", Hidden: true, UnlockedAt: 1700000000},
		{ID: "", Name: "No id"},
		{ID: "odd", Name: "", GlobalPercent: f(150), Icon: "javascript:alert(1)"},
	}})
	if out.Total != 4 || out.Unlocked != 1 || len(out.List) != 4 {
		t.Fatalf("out = %+v", out)
	}
	open := out.List[0]
	if open.Name != "Open" || open.Icon == "" || open.GlobalPercent == nil || *open.GlobalPercent != 12.5 || open.Unlocked {
		t.Fatalf("plain entry changed: %+v", open)
	}
	secret := out.List[1]
	if secret.Name != HiddenName || secret.Description != "" || secret.Icon != "" || !secret.Hidden {
		t.Fatalf("hidden locked entry leaked: %+v", secret)
	}
	if won := out.List[2]; won.Name != "Won" || !won.Unlocked || won.UnlockedAt != 1700000000 {
		t.Fatalf("hidden unlocked entry should show: %+v", won)
	}
	odd := out.List[3]
	if odd.Name != "odd" || odd.GlobalPercent != nil || odd.Icon != "" {
		t.Fatalf("unsafe fields kept: %+v", odd)
	}
}

func TestNormalizeAchievementsBoundsAndTotals(t *testing.T) {
	var list []Achievement
	for i := 0; i < MaxAchievements+5; i++ {
		list = append(list, Achievement{ID: "a", Name: "n"})
	}
	out := NormalizeAchievements(AchievementsReply{Total: 3, List: list})
	if len(out.List) != MaxAchievements {
		t.Fatalf("list = %d entries, want the bound %d", len(out.List), MaxAchievements)
	}
	if out.Total != MaxAchievements {
		t.Fatalf("total %d is below the list it came with", out.Total)
	}
	if empty := NormalizeAchievements(AchievementsReply{}); empty.List == nil {
		t.Fatal("an empty answer must give an empty list, not nil")
	}
}

func TestPlaytimeByGameKeepsTheLastAndNeverNegative(t *testing.T) {
	got := PlaytimeByGame(PlaytimeReply{Games: []PlaytimeGame{
		{ID: "a", Minutes: 10}, {ID: "", Minutes: 99}, {ID: "b", Minutes: -4}, {ID: "a", Minutes: 25},
	}})
	if len(got) != 2 || got["a"] != 25 || got["b"] != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestMergePlaytimeIsTheLargerSide(t *testing.T) {
	for _, c := range []struct{ local, store, want int }{{30, 150, 150}, {200, 150, 200}, {0, 0, 0}, {-5, 0, 0}} {
		if got := MergePlaytime(c.local, c.store); got != c.want {
			t.Errorf("MergePlaytime(%d, %d) = %d, want %d", c.local, c.store, got, c.want)
		}
	}
}

func TestCleanCode(t *testing.T) {
	if got, err := CleanCode("ABCD\tEFGH"); err == nil || got != "" {
		t.Fatalf("a tab inside the code should be refused: %q, %v", got, err)
	}
	if got, err := CleanCode("  ABCD-EFGH  "); err != nil || got != "ABCD-EFGH" {
		t.Fatalf("CleanCode = %q, %v", got, err)
	}
	if _, err := CleanCode(" \n "); err != ErrCodeEmpty {
		t.Fatalf("blank: %v", err)
	}
	if _, err := CleanCode(strings.Repeat("é", MaxCodeLen+1)); err != ErrCodeTooLong {
		t.Fatalf("long (runes, not bytes): %v", err)
	}
	if _, err := CleanCode("ab\x00cd"); err != ErrCodeControl {
		t.Fatalf("control character: %v", err)
	}
}

func TestMaskCodeNeverShowsTheMiddle(t *testing.T) {
	got := MaskCode("ABCD-1234-WXYZ")
	if got != "AB…YZ (14 characters)" {
		t.Fatalf("MaskCode = %q", got)
	}
	if strings.Contains(got, "1234") {
		t.Fatalf("middle of the code leaked: %q", got)
	}
	if short := MaskCode("abc"); short != "(3 characters)" {
		t.Fatalf("short code = %q", short)
	}
}

func TestTitlesOfTrimsDropsEmptyAndBounds(t *testing.T) {
	got := TitlesOf([]string{" A ", "", "  ", "B"})
	if !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("titles = %v", got)
	}
	if empty := TitlesOf(nil); empty == nil {
		t.Fatal("no titles must be an empty list, not nil")
	}
	many := make([]string, MaxTitles+3)
	for i := range many {
		many[i] = "t"
	}
	if len(TitlesOf(many)) != MaxTitles {
		t.Fatal("titles are not bounded")
	}
}
