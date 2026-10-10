package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// PCGamingWiki (PCGW) is a per-game source of protection facts, looked up by
// Steam AppID. The wiki is CC BY-NC-SA 3.0: Hydra shows the credit, the page
// and the licence wherever a PCGW fact is shown (see Compat.Wiki), and keeps
// only the facts it reads, not the page. Hydra is non-commercial; any change
// to that (a paid tier, ads) ends the permission to use this data.

const (
	// PCGWAppIDURL redirects a Steam AppID to its page.
	PCGWAppIDURL = "https://www.pcgamingwiki.com/api/appid.php?appid="
	// PCGWAPIURL is the MediaWiki API. Anonymous use only: no bot password is
	// shipped, and cargoquery (which needs one) is not used.
	PCGWAPIURL = "https://www.pcgamingwiki.com/w/api.php"
	// PCGWLicenceURL is the licence of the wiki's content.
	PCGWLicenceURL = "https://creativecommons.org/licenses/by-nc-sa/3.0/"
	// PCGWContact is where PCGW can reach Hydra's maintainers, as its API
	// policy asks. TODO(compat-pcgw): replace before release.
	PCGWContact = "https://example.invalid/hydra-launcher"
	// PCGWUserAgent names Hydra and its contact, as PCGW's API policy asks.
	PCGWUserAgent = "Hydra-Launcher/1.0 (Linux; compatibility metadata; " + PCGWContact + ")"
	// PCGWAttribution is the credit shown under every PCGW fact.
	PCGWAttribution = "Source: PCGamingWiki contributors, CC BY-NC-SA 3.0"
)

// PCGWPage is the wiki page of a title, as a link.
func PCGWPage(title string) string {
	return "https://www.pcgamingwiki.com/wiki/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
}

// WikiRecord is what Hydra keeps of a PCGW page: its title and the facts
// ParseWikitext read from it.
type WikiRecord struct {
	Title     string       `json:"title"`
	DRM       []string     `json:"drm,omitempty"`
	Removed   []RemovedDRM `json:"removed,omitempty"`
	Online    string       `json:"online,omitempty"`
	AntiCheat []string     `json:"anticheat,omitempty"`
}

// WikiSource looks a game up by Steam AppID. ErrNotFound means the wiki has no
// page for it (or the page is about another game).
type WikiSource interface {
	Lookup(ctx context.Context, appID string) (WikiRecord, error)
}

// PCGW reads PCGW through a Fetcher built with NewNoRedirectClient and
// NewPCGWFetcher. The two URLs are replaced in tests.
type PCGW struct {
	Fetch    *Fetcher
	AppIDURL string
	API      string
}

// NewPCGWFetcher is the fetcher for PCGW: one request at a time, about one a
// second, and a 429 waits a minute before the one retry.
func NewPCGWFetcher() *Fetcher {
	f := NewFetcher(PCGWUserAgent)
	f.Client = NewNoRedirectClient()
	f.Concurrency = 1
	f.Retries = 1
	f.Backoff = time.Minute
	return f
}

// Lookup resolves the page of appID (the appid redirect gives its title), then
// reads and parses its wikitext. A page that doesn't name appID as its Steam
// AppID is not that game's page, so it is ErrNotFound.
func (p *PCGW) Lookup(ctx context.Context, appID string) (WikiRecord, error) {
	if !validAppID(appID) {
		return WikiRecord{}, fmt.Errorf("%q is not a Steam AppID", appID)
	}
	loc, err := p.Fetch.Location(ctx, p.appIDURL()+appID)
	if err != nil {
		return WikiRecord{}, err
	}
	title := pageTitle(loc)
	if title == "" {
		return WikiRecord{}, ErrNotFound
	}
	body, err := p.Fetch.Get(ctx, p.api()+"?action=parse&format=json&redirects=1&prop=wikitext&page="+url.QueryEscape(title))
	if err != nil {
		return WikiRecord{}, err
	}
	var resp struct {
		Parse *struct {
			Title    string `json:"title"`
			Wikitext struct {
				Text string `json:"*"`
			} `json:"wikitext"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return WikiRecord{}, fmt.Errorf("PCGamingWiki answered something unreadable: %w", err)
	}
	if resp.Parse == nil {
		return WikiRecord{}, ErrNotFound
	}
	facts := ParseWikitext(resp.Parse.Wikitext.Text)
	if !namesAppID(facts, appID) {
		return WikiRecord{}, ErrNotFound
	}
	return WikiRecord{
		Title:     resp.Parse.Title,
		DRM:       facts.DRM,
		Removed:   facts.Removed,
		Online:    facts.Online,
		AntiCheat: facts.AntiCheat,
	}, nil
}

func (p *PCGW) appIDURL() string {
	if p.AppIDURL != "" {
		return p.AppIDURL
	}
	return PCGWAppIDURL
}

func (p *PCGW) api() string {
	if p.API != "" {
		return p.API
	}
	return PCGWAPIURL
}

// pageTitle is the title of a wiki page from its URL ("/wiki/Portal_2" gives
// "Portal 2"), "" when the URL is not a page.
func pageTitle(loc string) string {
	u, err := url.Parse(loc)
	if err != nil || !strings.HasPrefix(u.Path, "/wiki/") {
		return ""
	}
	title := strings.ReplaceAll(strings.TrimPrefix(u.Path, "/wiki/"), "_", " ")
	if title == "" || strings.ContainsAny(title, "{}[]|<>") {
		return ""
	}
	return title
}

// namesAppID says whether the page is about appID: its main Steam AppID, or
// one of the versions it lists.
func namesAppID(f WikiFacts, appID string) bool {
	if f.SteamAppID == appID {
		return true
	}
	for _, id := range f.SteamSide {
		if id == appID {
			return true
		}
	}
	return false
}
