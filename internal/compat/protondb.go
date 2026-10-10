package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProtonDBSummaryURL is the per-app summary endpoint. It is undocumented, so
// it can change without notice; Platypus fetches one app at a time, at runtime,
// and never bundles ProtonDB's dump (ODbL share-alike).
const ProtonDBSummaryURL = "https://www.protondb.com/api/v1/reports/summaries/"

// ProtonDBPage is the page a user can open for an app.
func ProtonDBPage(appID string) string { return "https://www.protondb.com/app/" + appID }

// Summary is what ProtonDB says about one app.
type Summary struct {
	Tier       string  `json:"tier"`
	Confidence string  `json:"confidence"`
	Total      int     `json:"total"`
	Score      float64 `json:"score"`
}

// RatingSource is a community rating provider keyed by Steam AppID. The
// ProtonDB summary is the one in use; a self-hosted mirror would implement
// the same method.
type RatingSource interface {
	Summary(ctx context.Context, appID string) (Summary, error)
}

// ProtonDB reads per-app summaries through a Fetcher.
type ProtonDB struct {
	Fetch *Fetcher
	// Base is ProtonDBSummaryURL unless a test points it at a fake server.
	Base string
}

// Summary fetches the summary of appID. A 404 is ErrNotFound: ProtonDB has
// no reports for the app.
func (p *ProtonDB) Summary(ctx context.Context, appID string) (Summary, error) {
	if !validAppID(appID) {
		return Summary{}, fmt.Errorf("%q is not a Steam AppID", appID)
	}
	base := p.Base
	if base == "" {
		base = ProtonDBSummaryURL
	}
	body, err := p.Fetch.Get(ctx, base+appID+".json")
	if err != nil {
		return Summary{}, err
	}
	var s Summary
	if err := json.Unmarshal(body, &s); err != nil {
		return Summary{}, fmt.Errorf("protondb answered something unreadable: %w", err)
	}
	return s, nil
}

// reliableConfidence are the ProtonDB confidence levels a tier is shown for.
var reliableConfidence = map[string]bool{"strong": true, "good": true, "moderate": true}

// minReports is how many reports a tier needs before Platypus shows it.
const minReports = 5

// knownTiers are the ProtonDB medals; "pending", "native" and others are not
// shown as tiers.
var knownTiers = map[string]bool{"platinum": true, "gold": true, "silver": true, "bronze": true, "borked": true}

// ShownTier returns the tier to show for a summary, "" when the answer is
// too thin or not a medal.
func ShownTier(s Summary) string {
	if !knownTiers[s.Tier] || !reliableConfidence[s.Confidence] || s.Total < minReports {
		return ""
	}
	return s.Tier
}

// validAppID is a Steam AppID: digits only, and not empty.
func validAppID(id string) bool {
	return id != "" && strings.Trim(id, "0123456789") == ""
}
