// Package stats holds the pure parts of the playtime, achievements and redeem
// capabilities (protocol 1.7, docs/plans/launcher-parity.md §2.4): what a
// plugin's answer may put in front of the UI, what a typed code may send, and
// what a log line may show of it. The App does the calls and the caching.
package stats

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"hydra-launcher/internal/types"
)

// Bounds on what a plugin may send and what the UI may send back.
const (
	MaxAchievements = 2000
	MaxTitles       = 50
	MaxCodeLen      = 128
)

// HiddenName is what the UI shows for a hidden achievement the user hasn't
// unlocked yet.
const HiddenName = "Hidden achievement"

// AchievementsReply is the achievements.list answer. Unknown fields are ignored.
type AchievementsReply struct {
	Total    int           `json:"total"`
	Unlocked int           `json:"unlocked"`
	List     []Achievement `json:"list"`
}

// Achievement is one entry of an AchievementsReply. UnlockedAt is unix
// seconds; zero or missing means locked.
type Achievement struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Hidden        bool     `json:"hidden"`
	UnlockedAt    int64    `json:"unlockedAt"`
	GlobalPercent *float64 `json:"globalPercent"`
}

// PlaytimeReply is the playtime.list answer: the whole library in one call.
type PlaytimeReply struct {
	Games []PlaytimeGame `json:"games"`
}

// PlaytimeGame is one game's store playtime, in minutes.
type PlaytimeGame struct {
	ID      string `json:"id"`
	Minutes int    `json:"minutes"`
}

// NormalizeAchievements makes a plugin's answer safe and complete for the UI:
// entries without an id are dropped, the list is bounded, a locked hidden
// achievement loses its name, description and icon, icons must be http(s),
// and a global percentage outside 0-100 counts as missing.
func NormalizeAchievements(in AchievementsReply) types.AchievementProgress {
	out := types.AchievementProgress{Total: in.Total, Unlocked: in.Unlocked, List: []types.ProgressAchievement{}}
	for _, a := range in.List {
		if a.ID == "" || len(out.List) >= MaxAchievements {
			continue
		}
		unlocked := a.UnlockedAt > 0
		pa := types.ProgressAchievement{
			ID: a.ID, Name: a.Name, Description: a.Description, Icon: httpURL(a.Icon),
			Hidden: a.Hidden, Unlocked: unlocked, UnlockedAt: max(a.UnlockedAt, 0),
		}
		if pa.Name == "" {
			pa.Name = a.ID
		}
		if a.Hidden && !unlocked {
			pa.Name, pa.Description, pa.Icon = HiddenName, "", ""
		}
		if g := a.GlobalPercent; g != nil && *g >= 0 && *g <= 100 {
			pct := *g
			pa.GlobalPercent = &pct
		}
		out.List = append(out.List, pa)
	}
	if out.Total < len(out.List) {
		out.Total = len(out.List)
	}
	return out
}

// PlaytimeByGame turns a playtime.list answer into minutes per store game id.
// Negative minutes count as zero, and a later duplicate id wins.
func PlaytimeByGame(in PlaytimeReply) map[string]int {
	out := make(map[string]int, len(in.Games))
	for _, g := range in.Games {
		if g.ID == "" {
			continue
		}
		out[g.ID] = max(g.Minutes, 0)
	}
	return out
}

// MergePlaytime is the playtime Hydra shows: the larger of the local record
// and the store's. It is display-only; neither side is written back.
func MergePlaytime(local, store int) int {
	return max(local, store, 0)
}

// ErrCodeEmpty and friends are the messages the redeem dialog shows.
var (
	ErrCodeEmpty   = errors.New("type a code to redeem")
	ErrCodeTooLong = fmt.Errorf("that code is longer than %d characters", MaxCodeLen)
	ErrCodeControl = errors.New("a code has no line breaks or control characters")
)

// CleanCode trims a typed redeem code and rejects what can't be one: nothing
// at all, too long, or control characters (a pasted line break, a tab).
func CleanCode(raw string) (string, error) {
	code := strings.TrimSpace(raw)
	switch {
	case code == "":
		return "", ErrCodeEmpty
	case len([]rune(code)) > MaxCodeLen:
		return "", ErrCodeTooLong
	}
	for _, r := range code {
		if unicode.IsControl(r) {
			return "", ErrCodeControl
		}
	}
	return code, nil
}

// MaskCode is the form of a code that may appear in a log: its first and last
// two characters and its length. A short code shows only its length.
func MaskCode(code string) string {
	r := []rune(code)
	if len(r) < 8 {
		return fmt.Sprintf("(%d characters)", len(r))
	}
	return fmt.Sprintf("%s…%s (%d characters)", string(r[:2]), string(r[len(r)-2:]), len(r))
}

// TitlesOf keeps the non-empty granted titles of a redeem answer, bounded.
func TitlesOf(in []string) []string {
	out := []string{}
	for _, t := range in {
		if t = strings.TrimSpace(t); t != "" && len(out) < MaxTitles {
			out = append(out, t)
		}
	}
	return out
}

// httpURL keeps http(s) URLs with a host, "" for anything else.
func httpURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}
