// Command platypus-linux-compat provides cached and live compatibility facts
// from ProtonDB, Steam Deck, AreWeAntiCheatYet, PCGamingWiki, and local scans.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"platypus/internal/compat"
	"platypus/internal/types"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type game struct {
	ID         string   `json:"id"`
	Store      string   `json:"store"`
	StoreAppID string   `json:"storeAppId"`
	SteamAppID string   `json:"steamAppId"`
	Platforms  []string `json:"platforms,omitempty"`
	InstallDir string   `json:"installDir,omitempty"`
	Online     bool     `json:"online"`
	// ProtectionOnline allows the PCGamingWiki and AreWeAntiCheatYet sources.
	// Without it they answer from their caches (issue #150).
	ProtectionOnline bool `json:"protectionOnline"`
	StoreDRM         bool `json:"storeDRM,omitempty"`
}

type lookupParams struct {
	Games []game `json:"games"`
}
type badge struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Tone      string `json:"tone,omitempty"`
	Link      string `json:"link,omitempty"`
	Source    string `json:"source"`
	Detail    string `json:"detail,omitempty"`
	CheckedAt int64  `json:"checkedAt"`
}
type fact struct {
	ID         string      `json:"id"`
	Badges     []badge     `json:"badges"`
	Report     *report     `json:"report,omitempty"`
	Links      []any       `json:"links"`
	Notes      []string    `json:"notes,omitempty"`
	Wiki       *wikiCredit `json:"wiki,omitempty"`
	TTLSeconds int         `json:"ttlSeconds"`
}
type wikiCredit struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	FetchedAt  int64  `json:"fetchedAt"`
	LicenceURL string `json:"licenceUrl"`
}
type report struct {
	Count int    `json:"count"`
	State string `json:"state"`
	Tier  string `json:"tier,omitempty"`
}

var proton compat.RatingSource = &compat.ProtonDB{Fetch: compat.NewFetcher("Platypus/1.0 (Linux compatibility facts)")}
var svc *compat.Service

// configureService opens all plugin-owned source caches and configures fetchers.
func configureService() {
	data := os.Getenv("PLATYPUS_PLUGIN_DATA")
	cache, _ := compat.OpenCache(filepath.Join(data, "source-cache.json"))
	fetch := compat.NewFetcher("Platypus/1.0 (Linux compatibility facts)")
	awacyFetch := compat.NewFetcher("Platypus/1.0 (Linux compatibility facts)")
	awacyFetch.Interval, awacyFetch.Concurrency = 0, 1
	svc = &compat.Service{Cache: cache, Ratings: proton,
		Deck:      &compat.SteamDeck{Fetch: fetch},
		AntiCheat: &compat.AntiCheatStore{Path: filepath.Join(data, "awacy.json"), URL: compat.AWACYURL, Fetch: awacyFetch},
		Wiki:      &compat.PCGW{Fetch: compat.NewPCGWFetcher()},
	}
}

// wireBadge converts a core compatibility badge into the protocol representation.
func wireBadge(b types.CompatBadge) badge {
	return badge{Kind: b.Kind, Label: b.Label, Tone: b.Tone, Link: b.Link, Source: b.Source, CheckedAt: b.CheckedAt, Detail: b.Detail}
}

// newAntiCheat removes anti-cheat names already supplied by AWACY or another source.
func newAntiCheat(have []badge, add []types.CompatBadge) []types.CompatBadge {
	seen := map[string]bool{}
	for _, b := range have {
		if b.Kind == "anticheat" {
			seen[strings.ToLower(b.Label)] = true
		}
	}
	out := make([]types.CompatBadge, 0, len(add))
	for _, b := range add {
		key := strings.ToLower(b.Label)
		if !seen[key] {
			seen[key] = true
			out = append(out, b)
		}
	}
	return out
}

func lookup(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var p lookupParams
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Games) == 0 {
		return nil, &rpcError{Code: -32602, Message: "games is required"}
	}
	facts := make([]fact, 0, len(p.Games))
	sourceDown := false
	stopped := false
	for _, g := range p.Games {
		if g.ID == "" {
			return nil, &rpcError{Code: -32602, Message: "game id is required"}
		}
		if ctx.Err() != nil {
			stopped = true
			break
		}
		id := g.SteamAppID
		if id == "" && g.Store == "steam" {
			id = g.StoreAppID
		}
		f := fact{ID: g.ID, Badges: []badge{}, Links: []any{}, TTLSeconds: int((7 * 24 * time.Hour).Seconds())}
		if id != "" {
			var summary compat.Summary
			var err error
			if g.Online {
				if svc != nil && svc.Ratings != nil {
					var rating compat.Rating
					var found bool
					rating, found = svc.ProtonDB(ctx, id)
					if found {
						summary = rating.Summary
						if !rating.Found {
							err = compat.ErrNotFound
						}
					}
				} else {
					summary, err = proton.Summary(ctx, id)
				}
			} else if svc != nil {
				cached, found := svc.ProtonDBCacheOnly(id)
				if found {
					summary = cached.Summary
					if !cached.Found {
						err = compat.ErrNotFound
					}
				}
			}
			if errors.Is(err, compat.ErrPaused) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (err != nil && ctx.Err() != nil) {
				stopped = true
				break
			}
			if err != nil && !errors.Is(err, compat.ErrNotFound) {
				sourceDown = true
				if svc == nil {
					continue
				}
			}
			if err == nil {
				tier := compat.ShownTier(summary)
				state := "rated"
				if tier == "" {
					state = "few"
				}
				f.Report = &report{Count: summary.Total, State: state, Tier: tier}
				if tier != "" {
					label := strings.ToUpper(tier[:1]) + tier[1:]
					f.Badges = append(f.Badges, badge{Kind: "rating", Label: label, Tone: tier, Link: compat.ProtonDBPage(id), Source: "ProtonDB", CheckedAt: time.Now().Unix()})
				}
			}
			if errors.Is(err, compat.ErrNotFound) {
				f.Report = &report{State: "none"}
			}
		}
		if svc != nil {
			if svc.Deck != nil {
				var d compat.DeckResult
				var ok bool
				if g.Online {
					d, ok = svc.DeckRating(ctx, id)
				} else {
					d, ok = svc.DeckRatingCacheOnly(id)
				}
				if ok && d.Found {
					b := badge{Kind: "verdict", Label: d.Report.Rating, Tone: d.Report.Rating, Link: compat.DeckPage(id), Source: "Steam Deck", CheckedAt: d.FetchedAt.Unix()}
					f.Badges = append(f.Badges, b)
					for _, n := range d.Report.Notes {
						f.Badges = append(f.Badges, badge{Kind: "note", Label: n, Source: "Steam Deck", CheckedAt: d.FetchedAt.Unix()})
					}
				}
			}
			var cat *compat.AntiCheatData
			if svc.AntiCheat != nil {
				if g.ProtectionOnline {
					cat = svc.AntiCheatCatalogue(ctx)
				} else {
					cat = svc.AntiCheat.Load()
				}
			}
			var entry compat.AntiCheatGame
			var found bool
			if cat != nil {
				if id != "" {
					entry, found = cat.BySteam(id)
				}
				if !found && g.Store == "epic" {
					entry, found = cat.ByEpic(g.StoreAppID)
				}
			}
			if found {
				for _, b := range compat.AntiCheatBadges(entry, cat.FetchedAt) {
					f.Badges = append(f.Badges, wireBadge(b))
				}
			}
			if id != "" && svc.Wiki != nil {
				var w compat.WikiAnswer
				var ok bool
				if g.ProtectionOnline {
					w, ok = svc.WikiPage(ctx, id)
				} else {
					w, ok = svc.WikiPageCacheOnly(id)
				}
				if ok && w.Found {
					drm := compat.WikiDRMBadges(w.Record, w.FetchedAt, g.StoreDRM)
					ac := compat.WikiAntiCheatBadges(w.Record, w.FetchedAt)
					for _, b := range drm {
						f.Badges = append(f.Badges, wireBadge(b))
					}
					for _, b := range newAntiCheat(f.Badges, ac) {
						f.Badges = append(f.Badges, wireBadge(b))
					}
					if len(drm)+len(ac) > 0 {
						f.Wiki = &wikiCredit{Title: w.Record.Title, URL: compat.PCGWPage(w.Record.Title), FetchedAt: w.FetchedAt.Unix(), LicenceURL: compat.PCGWLicenceURL}
					}
				}
			}
		}
		if g.InstallDir != "" && svc != nil {
			scan := svc.Scan
			if scan == nil {
				scan = compat.ScanInstall
			}
			hits, truncated := scan(g.InstallDir)
			local := compat.LocalEntry{FetchedAt: time.Now(), Hits: hits, Truncated: truncated}
			if svc.Cache != nil {
				_ = svc.Cache.SetLocal(g.InstallDir, local)
			}
			for _, b := range compat.LocalBadges(local.Hits, local.FetchedAt) {
				f.Badges = append(f.Badges, wireBadge(b))
			}
		}
		facts = append(facts, f)
	}
	if len(facts) == 0 && sourceDown && !stopped {
		return nil, &rpcError{Code: -32002, Message: "ProtonDB is temporarily unavailable"}
	}
	return map[string]any{"facts": facts}, nil
}

func handle(ctx context.Context, req message) message {
	resp := message{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		var p struct {
			Protocols []string `json:"protocols"`
		}
		_ = json.Unmarshal(req.Params, &p)
		chosen := "1.0"
		for _, v := range p.Protocols {
			if v == "1.17" {
				chosen = v
			}
		}
		resp.Result = map[string]any{"protocol": chosen, "capabilities": []string{"compat-facts"}}
	case "compat.lookup":
		resp.Result, resp.Error = lookup(ctx, req.Params)
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

func main() {
	configureService()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	for scanner.Scan() {
		var req message
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if req.ID == nil {
			continue
		}
		resp := handle(context.Background(), req)
		if err := json.NewEncoder(writer).Encode(resp); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		_ = writer.Flush()
	}
}
