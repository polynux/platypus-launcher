package main

// The fake `versions` capability: a few channels (one behind a password), a
// handful of builds, a typed build id with a link, install honouring `target`,
// and `installed` on job.done. State lives in HYDRA_PLUGIN_DATA/state.json like
// the rest of echo's. See docs/plugins/protocol.md, "Versions".
//
// Channels: latest (build b300), beta (b310, password "hydra-beta"), legacy
// (b100). Builds: b310, b300, b290, b200, b100, plus every build Hydra's ledger
// says was installed here ("archive"), plus any build typed as `manual`
// (b followed by digits). Knob ECHO_VERSIONS_OFF takes a comma list of
// channels, builds, manual, links and leaves those parts out of versions.list.

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// testPassword unlocks the beta channel. Fixed so tests and the UI can use it.
const testPassword = "hydra-beta"

const latestChannel, latestBuild = "latest", "b300"

type channelDef struct {
	id, name, description, updated, build string
	password                              bool
}

type buildDef struct {
	id, name, published, channel string
}

var channelDefs = []channelDef{
	{"latest", "Latest", "The current release.", "2026-09-01", "b300", false},
	{"beta", "Beta", "Release candidates. Needs the beta password.", "2026-09-30", "b310", true},
	{"legacy", "Legacy", "The 1.x line, kept for old saves.", "2024-03-15", "b100", false},
}

var buildDefs = []buildDef{
	{"b310", "3.1.0 beta", "2026-09-30", "beta"},
	{"b300", "3.0.0", "2026-09-01", "latest"},
	{"b290", "2.9.1", "2026-07-14", "latest"},
	{"b200", "2.0.0", "2025-11-02", ""},
	{"b100", "1.0.0", "2024-03-15", "legacy"},
}

var manualBuild = regexp.MustCompile(`^b[0-9]{1,6}$`)

// versionRec is what echo remembers about an installed game's version.
type versionRec struct {
	Build   string `json:"build"`
	Channel string `json:"channel,omitempty"`
	Name    string `json:"name"`
}

type targetParam struct {
	Channel string            `json:"channel"`
	Build   string            `json:"build"`
	Manual  map[string]string `json:"manual"`
}

type pinParam struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type ledgerParam struct {
	Build       string `json:"build"`
	Channel     string `json:"channel"`
	VersionName string `json:"versionName"`
	Date        string `json:"date"`
}

func findChannel(id string) (channelDef, bool) {
	for _, c := range channelDefs {
		if c.id == id {
			return c, true
		}
	}
	return channelDef{}, false
}

func findBuild(id string) (buildDef, bool) {
	for _, b := range buildDefs {
		if b.id == id {
			return b, true
		}
	}
	return buildDef{}, false
}

// versionsOff says whether the ECHO_VERSIONS_OFF knob leaves a part out.
func versionsOff(part string) bool {
	knobMu.Lock()
	defer knobMu.Unlock()
	return slices.Contains(kn.VersionsOff, part)
}

func unlockKey(game, channel string) string { return game + "/" + channel }

// versionsFeatures is what initialize reports for the capability unless
// ECHO_FEATURES says otherwise. Callers hold knobMu.
func versionsFeatures() map[string]bool {
	off := func(part string) bool { return slices.Contains(kn.VersionsOff, part) }
	return map[string]bool{
		"channels": !off("channels"),
		"builds":   !off("builds"),
		"password": true,
		"manual":   !off("manual"),
	}
}

// resolveTarget turns a target into the version to install. No target means
// the latest. A password channel stays locked until versions.unlock accepted
// its password. Callers hold no locks.
func resolveTarget(game string, t *targetParam) (versionRec, error) {
	bad := func(format string, a ...any) (versionRec, error) {
		return versionRec{}, &rpcError{Code: -32000, Message: fmt.Sprintf(format, a...)}
	}
	if t == nil || (t.Channel == "" && t.Build == "" && len(t.Manual) == 0) {
		b, _ := findBuild(latestBuild)
		return versionRec{Build: b.id, Channel: latestChannel, Name: b.name}, nil
	}
	channel := t.Channel
	if channel != "" {
		c, ok := findChannel(channel)
		if !ok {
			return bad("Echo has no channel %q", channel)
		}
		if c.password {
			stMu.Lock()
			unlocked := st.Unlocked[unlockKey(game, channel)]
			stMu.Unlock()
			if !unlocked {
				return bad("The %s channel is locked: enter its password first.", c.name)
			}
		}
		if t.Build == "" && len(t.Manual) == 0 {
			b, _ := findBuild(c.build)
			return versionRec{Build: b.id, Channel: c.id, Name: b.name}, nil
		}
	}
	if t.Build != "" {
		if b, ok := findBuild(t.Build); ok {
			if c, ok := findChannel(b.channel); ok && c.password {
				stMu.Lock()
				unlocked := st.Unlocked[unlockKey(game, c.id)]
				stMu.Unlock()
				if !unlocked {
					return bad("Build %s is in the locked %s channel: enter its password first.", b.id, c.name)
				}
			}
			return versionRec{Build: b.id, Channel: channel, Name: b.name}, nil
		}
		// A build Hydra's ledger remembers: echo can refetch any build id.
		if manualBuild.MatchString(t.Build) {
			return versionRec{Build: t.Build, Channel: channel, Name: "build " + t.Build}, nil
		}
		return bad("Echo has no build %q", t.Build)
	}
	// The UI sends what the user typed as manual.input; "build" is the
	// structured form other callers may use.
	id := strings.TrimSpace(t.Manual["input"])
	if id == "" {
		id = t.Manual["build"]
	}
	if id != "" {
		if !manualBuild.MatchString(id) {
			return bad("Echo build ids are a b and digits, like b200 (got %q).", id)
		}
		name := "build " + id
		if b, ok := findBuild(id); ok {
			name = b.name
		}
		return versionRec{Build: id, Name: name}, nil
	}
	return bad("Type a build id, like b200.")
}

// installedAnswer is job.done's `installed`.
func installedAnswer(rec versionRec) map[string]any {
	a := map[string]any{
		"build": rec.Build, "versionName": rec.Name,
		"archiveRef": map[string]any{"echo": true, "build": rec.Build},
	}
	if rec.Channel != "" {
		a["channel"] = rec.Channel
	}
	return a
}

func versionsList(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	res := map[string]any{
		"channels": []any{}, "builds": []any{}, "manual": nil, "links": []any{},
		"updateAvailable": false, "pinned": p.Pinned,
	}
	if g.external != "" || g.client {
		return res, nil // one build only
	}
	stMu.Lock()
	rec, installed := st.Versions[p.ID]
	unlocked := map[string]bool{}
	for _, c := range channelDefs {
		unlocked[c.id] = !c.password || st.Unlocked[unlockKey(p.ID, c.id)]
	}
	stMu.Unlock()

	if !versionsOff("channels") {
		var channels []map[string]any
		for _, c := range channelDefs {
			e := map[string]any{
				"id": c.id, "name": c.name, "description": c.description, "updatedAt": c.updated,
				"buildId": c.build, "requires": "none", "unlocked": unlocked[c.id],
				"current": installed && rec.Channel == c.id,
			}
			if c.password {
				e["requires"] = "password"
			}
			channels = append(channels, e)
		}
		res["channels"] = channels
	}
	if !versionsOff("builds") {
		var builds []map[string]any
		known := map[string]bool{}
		for _, b := range buildDefs {
			known[b.id] = true
			e := map[string]any{
				"id": b.id, "name": b.name, "publishedAt": b.published, "source": "store",
				"available": unlocked[b.channel] || b.channel == "", "current": installed && rec.Build == b.id,
			}
			if b.channel != "" {
				e["channel"] = b.channel
			}
			builds = append(builds, e)
		}
		for _, l := range p.Ledger {
			if l.Build == "" || known[l.Build] {
				continue
			}
			known[l.Build] = true
			name := l.VersionName
			if name == "" {
				name = "build " + l.Build
			}
			published, _, _ := strings.Cut(l.Date, "T")
			builds = append(builds, map[string]any{
				"id": l.Build, "name": name, "publishedAt": published, "source": "archive",
				"available": true, "current": installed && rec.Build == l.Build,
			})
		}
		res["builds"] = builds
	}
	if !versionsOff("manual") {
		res["manual"] = map[string]any{
			"label": "Build id",
			"hint":  "b and digits, like b200. The depots page lists the ids.",
		}
	}
	if !versionsOff("links") {
		res["links"] = []map[string]string{{"label": "Echo depots (example.com)", "url": "https://example.com/echo/depots/" + p.ID}}
	}
	if installed {
		want := latestBuild
		if p.Pinned != nil {
			switch p.Pinned.Kind {
			case "build":
				want = p.Pinned.ID
			case "channel":
				if c, ok := findChannel(p.Pinned.ID); ok {
					want = c.build
				}
			}
		}
		res["updateAvailable"] = rec.Build != want
	}
	return res, nil
}

func versionsUnlock(p params) (any, error) {
	if p.ID == "" || p.Channel == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id and channel are required"}
	}
	if _, ok := games[p.ID]; !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	c, ok := findChannel(p.Channel)
	if !ok {
		return nil, fmt.Errorf("Echo has no channel %q", p.Channel)
	}
	if !c.password {
		return nil, fmt.Errorf("The %s channel has no password.", c.name)
	}
	if p.Password != testPassword {
		return map[string]any{"unlocked": false}, nil
	}
	stMu.Lock()
	defer stMu.Unlock()
	st.Unlocked[unlockKey(p.ID, p.Channel)] = true
	save()
	return map[string]any{"unlocked": true}, nil
}
