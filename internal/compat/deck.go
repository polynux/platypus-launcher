package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DeckReportURL is the per-app Steam Deck compatibility endpoint. It is
// undocumented, so it can change without notice; Platypus fetches one app at
// a time, at runtime, and only what the UI shows.
const DeckReportURL = "https://store.steampowered.com/saleaction/ajaxgetdeckappcompatibilityreport"

// DeckPage is the store page a user can open for an app.
func DeckPage(appID string) string { return "https://store.steampowered.com/app/" + appID }

// DeckReport is what Steam says about one app on Deck: its verdict and the
// short English caveats worth showing. Rating is "verified", "playable" or
// "unsupported"; unknown apps have no report (ErrNotFound), never an empty
// rating.
type DeckReport struct {
	Rating string
	Notes  []string
}

// DeckSource is a Deck verdict provider keyed by Steam AppID.
type DeckSource interface {
	Report(ctx context.Context, appID string) (DeckReport, error)
}

// SteamDeck reads per-app Deck verdicts through a Fetcher.
type SteamDeck struct {
	Fetch *Fetcher
	// Base is DeckReportURL unless a test points it at a fake server.
	Base string
}

// Report fetches the Deck verdict of appID. An app Steam has no verdict for
// (no results, or resolved_category 0) is ErrNotFound.
func (d *SteamDeck) Report(ctx context.Context, appID string) (DeckReport, error) {
	if !validAppID(appID) {
		return DeckReport{}, fmt.Errorf("%q is not a Steam AppID", appID)
	}
	base := d.Base
	if base == "" {
		base = DeckReportURL
	}
	body, err := d.Fetch.Get(ctx, base+"?nAppID="+appID+"&l=english")
	if err != nil {
		return DeckReport{}, err
	}
	return parseDeckReport(body)
}

// deckResponse is the report envelope. Results is an object for an app Steam
// knows and an empty array (or missing) for one it has no verdict on.
type deckResponse struct {
	Results json.RawMessage `json:"results"`
}

type deckResults struct {
	Category int        `json:"resolved_category"`
	Items    []deckItem `json:"resolved_items"`
}

type deckItem struct {
	DisplayType int    `json:"display_type"`
	Token       string `json:"loc_token"`
}

// deckCategory maps resolved_category to a rating, "" when Steam has no
// verdict (0) or says something Platypus doesn't know.
func deckCategory(c int) string {
	switch c {
	case 3:
		return "verified"
	case 2:
		return "playable"
	case 1:
		return "unsupported"
	}
	return ""
}

// parseDeckReport reads the verdict out of a report body. It reads only the
// fields the UI shows and tolerates anything extra.
func parseDeckReport(body []byte) (DeckReport, error) {
	var env deckResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return DeckReport{}, fmt.Errorf("steam answered something unreadable: %w", err)
	}
	if len(env.Results) == 0 {
		return DeckReport{}, ErrNotFound
	}
	var res deckResults
	if err := json.Unmarshal(env.Results, &res); err != nil {
		// An empty array ("results":[]): Steam has no verdict for the app.
		if strings.HasPrefix(strings.TrimSpace(string(env.Results)), "[") {
			return DeckReport{}, ErrNotFound
		}
		return DeckReport{}, fmt.Errorf("steam answered something unreadable: %w", err)
	}
	rating := deckCategory(res.Category)
	if rating == "" {
		return DeckReport{}, ErrNotFound
	}
	return DeckReport{Rating: rating, Notes: deckNotes(res.Items)}, nil
}

// maxDeckNotes caps the caveats kept from a report: the tooltip stays short.
const maxDeckNotes = 3

// deckNoteWords maps the common result tokens to short English sentences.
// Anything else is omitted: an unknown token is never shown as-is.
var deckNoteWords = map[string]string{
	"#SteamDeckVerified_TestResult_ControllerGlyphsDoNotMatchDeckDevice":         "In-game button prompts don't match the Deck's buttons.",
	"#SteamDeckVerified_TestResult_InterfaceTextIsNotLegible":                    "Some text is too small to read on the Deck.",
	"#SteamDeckVerified_TestResult_DefaultControllerConfigNotFullyFunctional":    "The default controller setup doesn't cover everything.",
	"#SteamDeckVerified_TestResult_TextInputDoesNotAutomaticallyInvokesKeyboard": "Typing doesn't bring up the on-screen keyboard on its own.",
	"#SteamDeckVerified_TestResult_NativeResolutionNotSupported":                 "Doesn't run at the Deck's native resolution.",
	"#SteamDeckVerified_TestResult_UnsupportedAntiCheat_Other":                   "Its anti-cheat doesn't run on the Deck.",
}

// deckNotes keeps the caveats of a report: the warning and failure items
// (display types 2 and 3) with a known sentence, most important first.
func deckNotes(items []deckItem) []string {
	out := []string{}
	for _, it := range items {
		if it.DisplayType != 2 && it.DisplayType != 3 {
			continue // a pass: nothing to warn about
		}
		s, ok := deckNoteWords[it.Token]
		if !ok {
			if strings.Contains(it.Token, "UnsupportedAntiCheat") {
				s = deckNoteWords["#SteamDeckVerified_TestResult_UnsupportedAntiCheat_Other"]
			} else {
				continue
			}
		}
		out = append(out, s)
		if len(out) >= maxDeckNotes {
			break
		}
	}
	return out
}
