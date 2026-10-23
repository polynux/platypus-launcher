// Command platypus-steam is the Steam client plugin: the local Steam
// install's library, Store metadata, launches through the Steam client, art
// from the client's library cache, playtime from localconfig.vdf and Steam
// Cloud from cached store details.
//
// It reuses internal/steam (no copy-paste): the same scan, the same art
// names, the same client command and the same Store API client the built-in
// source used, so ids (steam_<appid> via the manifest gameIdPrefix) and the
// Source (steam) are byte-identical: zero data migration.
//
// Install it with plugins/install.sh steam. It needs no login.
//
// Protocol: library.list, game.details (optional, part of the library
// capability), game.launchSpec (delegate store-client) and playtime.list,
// plus initialize and locale.changed. A missing Steam install lists no games
// with no error; a readable-but-broken root errors (a source warning).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"platypus/internal/steam"
	"platypus/internal/types"
)

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Message }

type params struct {
	ID         string   `json:"id"`
	Preview    bool     `json:"preview"`
	Executable string   `json:"executable"`
	Protocols  []string `json:"protocols"`
	Locale     string   `json:"locale"`
}

var (
	outMu = sync.Mutex{}
	out   = json.NewEncoder(os.Stdout)

	metaMu sync.Mutex
	meta   = steam.NewStoreClient()
)

func send(m message) {
	m.JSONRPC = "2.0"
	outMu.Lock()
	defer outMu.Unlock()
	_ = out.Encode(m)
}

// supportedProtocols is what this plugin speaks: 1.0 to 1.15.
var supportedProtocols = []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6", "1.7", "1.8", "1.9", "1.10", "1.11", "1.12", "1.13", "1.14", "1.15"}

func negotiate(offered []string) (string, bool) {
	best, found := "", false
	for _, o := range offered {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		speaks := false
		for _, s := range supportedProtocols {
			if s == o {
				speaks = true
				break
			}
		}
		if !speaks {
			continue
		}
		if !found || versionLess(best, o) {
			best, found = o, true
		}
	}
	return best, found
}

func versionLess(a, b string) bool {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a, "%d.%d", &a1, &a2)
	fmt.Sscanf(b, "%d.%d", &b1, &b2)
	return a1 < b1 || (a1 == b1 && a2 < b2)
}

func setLocale(loc string) {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		loc = "en"
	}
	metaMu.Lock()
	defer metaMu.Unlock()
	if meta == nil {
		meta = steam.NewStoreClient()
	}
	meta.Locale = loc
}

func storeClient() *steam.StoreClient {
	metaMu.Lock()
	defer metaMu.Unlock()
	if meta == nil {
		meta = steam.NewStoreClient()
	}
	// Test hook: point the Store API at a fake (httptest) server.
	if base := strings.TrimSpace(os.Getenv("PLATYPUS_STEAM_STORE_BASE")); base != "" {
		meta.Base = base
	}
	return meta
}

func initializeResult(p params) (map[string]any, error) {
	setLocale(p.Locale)
	picked, ok := negotiate(p.Protocols)
	if !ok {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("no protocol version in common with Platypus's %v", p.Protocols)}
	}
	return map[string]any{
		"name":         "Steam",
		"version":      "1.0.0",
		"api":          1,
		"protocol":     picked,
		"capabilities": []string{"library", "launch", "playtime", "local-art"},
	}, nil
}

// library lists the local Steam install: one entry per installed manifest,
// the same set ScanSteamLibrary finds. Art prefers the client's library
// cache (file://, served by the core's local-art server) with the same CDN
// fallback the built-in source used. Every installed game launches through
// the client, so offline is "ok".
func library() (any, error) {
	root := steam.Root()
	if root == "" {
		return map[string]any{"games": []any{}}, nil
	}
	// A readable-but-broken root errors (a source warning); a missing
	// install lists nothing with no error (above).
	if _, err := os.Stat(filepath.Join(root, "steamapps")); err != nil && !os.IsNotExist(err) {
		return nil, &rpcError{Code: -32000, Message: fmt.Sprintf("steam library %s: %v", root, err)}
	}
	games, err := steam.ScanSteamLibrary()
	if err != nil {
		return nil, &rpcError{Code: -32000, Message: err.Error()}
	}
	c := storeClient()
	list := make([]map[string]any, 0, len(games))
	for _, g := range games {
		if g.StoreAppID == "" {
			continue
		}
		e := map[string]any{
			"id":          g.StoreAppID,
			"title":       g.Title,
			"installed":   true,
			"installable": false,
			"offline":     "ok",
			"clientLaunch": true,
		}
		if g.InstallDir != "" {
			e["installPath"] = g.InstallDir
		}
		if g.LastPlayed > 0 {
			e["lastPlayed"] = g.LastPlayed
		}
		if has, ok := c.CachedHasSteamCloud(g.StoreAppID); ok && has {
			e["cloudSync"] = true
		}
		if u := artFileURL(root, g.CoverURL); u != "" {
			e["coverUrl"] = u
		}
		if u := artFileURL(root, g.HeroURL); u != "" {
			e["heroUrl"] = u
		}
		if u := artFileURL(root, g.BannerURL); u != "" {
			e["bannerUrl"] = u
		}
		list = append(list, e)
	}
	if list == nil {
		list = []map[string]any{}
	}
	return map[string]any{"games": list}, nil
}

// artFileURL turns a scanned art URL back into a local-art file:// URL when
// it names the client's library cache, else keeps the CDN fallback.
func artFileURL(root, scanned string) string {
	if scanned == "" {
		return ""
	}
	if rel, ok := strings.CutPrefix(scanned, steam.ArtURLPrefix); ok {
		abs := filepath.Join(root, "appcache/librarycache", filepath.FromSlash(rel))
		return "file://" + abs
	}
	return scanned
}

// details merges Store API metadata with the local library cache, as the
// built-in source did. An id that is neither installed nor known to the
// store errors (never data or an empty page); an installed game the store
// no longer lists still answers (possibly empty), as before.
func details(p params) (any, error) {
	if strings.TrimSpace(p.ID) == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	root := steam.Root()
	c := storeClient()
	d, err := c.Details(root, p.ID)
	if err != nil {
		return nil, &rpcError{Code: -32000, Message: err.Error()}
	}
	if d == nil {
		d = &types.GameDetails{}
	}
	if detailsEmpty(d) && !installed(root, p.ID) {
		// Neither installed nor described by the store: an unknown id
		// errors (never data or an empty page). An installed game the
		// store delisted still answers, possibly empty, as before.
		return nil, &rpcError{Code: -32000, Message: fmt.Sprintf("unknown game %q", p.ID)}
	}
	out := map[string]any{}
	if d.Summary != "" {
		out["summary"] = d.Summary
	}
	if len(d.Screenshots) > 0 {
		out["screenshots"] = d.Screenshots
	}
	if len(d.Genres) > 0 {
		out["genres"] = d.Genres
	}
	if len(d.Developers) > 0 {
		out["developers"] = d.Developers
	}
	if len(d.Publishers) > 0 {
		out["publishers"] = d.Publishers
	}
	if d.ReleaseDate != "" {
		out["releaseDate"] = d.ReleaseDate
	}
	if d.HeroURL != "" {
		out["heroUrl"] = d.HeroURL
	}
	if d.Achievements != nil && d.Achievements.Total > 0 {
		list := make([]map[string]any, 0, len(d.Achievements.List))
		for _, a := range d.Achievements.List {
			e := map[string]any{"id": a.ID, "name": a.Name}
			if a.Description != "" {
				e["description"] = a.Description
			}
			if a.Icon != "" {
				e["icon"] = a.Icon
			}
			if a.UnlockedAt > 0 {
				e["unlockedAt"] = a.UnlockedAt
			}
			if a.GlobalPercent > 0 {
				e["globalPercent"] = a.GlobalPercent
			}
			list = append(list, e)
		}
		out["achievements"] = map[string]any{"total": d.Achievements.Total, "list": list}
	}
	if d.DescriptionHTML != "" {
		out["descriptionHtml"] = d.DescriptionHTML
	}
	if d.Website != "" {
		out["website"] = d.Website
	}
	if d.StoreURL != "" {
		out["storeUrl"] = d.StoreURL
	}
	if len(d.Languages) > 0 {
		out["languages"] = d.Languages
	}
	if d.Controller != "" {
		out["controller"] = d.Controller
	}
	if d.Requirements != nil && (d.Requirements.Minimum != "" || d.Requirements.Recommended != "") {
		out["requirements"] = map[string]any{"minimum": d.Requirements.Minimum, "recommended": d.Requirements.Recommended}
	}
	if d.AgeRating != "" {
		out["ageRating"] = d.AgeRating
	}
	if len(d.Tags) > 0 {
		out["tags"] = d.Tags
	}
	if len(d.Videos) > 0 {
		videos := make([]map[string]any, 0, len(d.Videos))
		for _, v := range d.Videos {
			if v.URL == "" {
				continue
			}
			e := map[string]any{"url": v.URL}
			if v.Thumbnail != "" {
				e["thumbnail"] = v.Thumbnail
			}
			if v.Title != "" {
				e["title"] = v.Title
			}
			if v.MIME != "" {
				e["mime"] = v.MIME
			}
			videos = append(videos, e)
		}
		if len(videos) > 0 {
			out["videos"] = videos
		}
	}
	return out, nil
}

// installed reports whether appID has a manifest in any library folder.
func installed(root, appID string) bool {
	if root == "" || appID == "" || !isAppID(appID) {
		return false
	}
	for _, lib := range steam.LibraryFolders(root) {
		if _, err := os.Stat(filepath.Join(lib, "steamapps", "appmanifest_"+appID+".acf")); err == nil {
			return true
		}
	}
	return false
}

// isAppID is the shape of a Steam app id: digits only, so no manifest path
// can escape the library folder.
func isAppID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// detailsEmpty reports whether the merged details carry nothing to show.
// StoreURL is derived from the id alone, so it doesn't count.
func detailsEmpty(d *types.GameDetails) bool {
	return d.Summary == "" && len(d.Screenshots) == 0 && len(d.Genres) == 0 &&
		len(d.Developers) == 0 && len(d.Publishers) == 0 && d.ReleaseDate == "" &&
		d.HeroURL == "" && d.Achievements == nil && d.DescriptionHTML == "" &&
		d.Website == "" && len(d.Languages) == 0 &&
		d.Controller == "" && d.Requirements == nil && d.AgeRating == "" &&
		len(d.Tags) == 0 && len(d.Videos) == 0
}

// launchSpec hands the game to the Steam client (delegate store-client with
// the same reaper match the core used), or answers an executable override
// with a direct spec for that file.
func launchSpec(p params) (any, error) {
	if strings.TrimSpace(p.ID) == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	if p.Executable != "" {
		if !filepath.IsAbs(p.Executable) {
			return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("executable %q must be an absolute path", p.Executable)}
		}
		if !installed(steam.Root(), p.ID) {
			return nil, &rpcError{Code: -32000, Message: fmt.Sprintf("unknown game %q", p.ID)}
		}
		return map[string]any{"exe": p.Executable}, nil
	}
	if !installed(steam.Root(), p.ID) {
		return nil, &rpcError{Code: -32000, Message: fmt.Sprintf("unknown game %q", p.ID)}
	}
	// The same command the built-in source ran (no launch arguments: the
	// core sends none with the spec; the xdg-open fallback can't forward
	// them, as before).
	cmd := steam.Command(types.Game{Store: types.StoreSteam, StoreAppID: p.ID})
	if len(cmd) == 0 {
		return nil, &rpcError{Code: -32000, Message: "no way to start the Steam client"}
	}
	return map[string]any{
		"delegate": "store-client",
		"client":   "Steam",
		"command":  cmd,
		"find":     map[string]any{"cmdline": []string{"SteamLaunch", "AppId=" + p.ID + "\x00"}},
	}, nil
}

func playtime() (any, error) {
	root := steam.Root()
	list := []map[string]any{}
	if root != "" {
		stats := steam.ReadPlayStats(steam.ActiveUserDir(root))
		for id, st := range stats {
			if st.PlaytimeMinute <= 0 && st.LastPlayed <= 0 {
				continue
			}
			e := map[string]any{"id": id, "minutes": max(st.PlaytimeMinute, 0)}
			if st.LastPlayed > 0 {
				e["lastPlayed"] = st.LastPlayed
			}
			list = append(list, e)
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	return map[string]any{"games": list}, nil
}

func dispatch(method string, raw json.RawMessage) (any, error) {
	var p params
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	switch method {
	case "initialize":
		return initializeResult(p)
	case "library.list":
		return library()
	case "game.details":
		return details(p)
	case "game.launchSpec":
		return launchSpec(p)
	case "playtime.list":
		return playtime()
	}
	return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
}

func handleNotification(method string, raw json.RawMessage) {
	if method != "locale.changed" {
		return
	}
	var p struct {
		Locale string `json:"locale"`
	}
	_ = json.Unmarshal(raw, &p)
	setLocale(p.Locale)
}

func handle(m message) {
	result, err := dispatch(m.Method, m.Params)
	resp := message{ID: m.ID, Result: result}
	var re *rpcError
	switch {
	case errors.As(err, &re):
		resp.Error, resp.Result = re, nil
	case err != nil:
		resp.Error, resp.Result = &rpcError{Code: -32000, Message: err.Error()}, nil
	case result == nil:
		resp.Result = struct{}{}
	}
	send(resp)
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64<<10), 32<<20)
	for in.Scan() {
		var m message
		if err := json.Unmarshal(in.Bytes(), &m); err != nil {
			send(message{Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		if m.ID == nil {
			handleNotification(m.Method, m.Params)
			continue
		}
		go handle(m)
	}
}
