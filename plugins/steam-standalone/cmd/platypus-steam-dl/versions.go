// Branches, beta passwords and version targets from PICS app info
// (protocol 1.3, docs/plugins/protocol.md, "Versions"). Branch metadata
// comes from depots/branches/<name> (buildid, timeupdated, description,
// pwdrequired), confirmed live, plus per-depot manifests/<branch>/gid.
// The plugin's versions.list and versions.unlock are served by the
// `versions` and `unlock` subcommands; installs resolve their target here
// so one CM session covers the PICS read and the download.
package main

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"platypus/internal/steam"
	"platypus/internal/steamcm"
)

// notFoundError marks an unknown app: the plugin turns it into a plain
// error for the user, not a "try again later".
type notFoundError struct{ msg string }

func (e notFoundError) Error() string { return e.msg }

// notFoundOr maps "no product info" to not-found, everything else to a
// call error.
func notFoundOr(op string, appID uint32, err error) error {
	var ce *steamcm.CMError
	if errors.As(err, &ce) && strings.Contains(ce.Msg, "no product info") {
		return notFoundError{msg: fmt.Sprintf("Steam has no product info for app %d", appID)}
	}
	return callErr(op, err)
}

// refused reports a server refusal (EResult set, e.g. a wrong beta
// password) as opposed to a dropped connection (EResult 0): only the
// former is an answer, the latter must error.
func refused(err error) bool {
	var ce *steamcm.CMError
	return errors.As(err, &ce) && ce.EResult != 0
}

// branchInfo is one PICS branch with the manifest gid per depot that
// carries it. Manifests only lists depots with a gid for this branch.
type branchInfo struct {
	Name        string            `json:"name"`
	BuildID     string            `json:"build_id"`
	TimeUpdated string            `json:"time_updated,omitempty"`
	Description string            `json:"description,omitempty"`
	PwdRequired bool              `json:"pwd_required"`
	Manifests   map[uint32]uint64 `json:"manifests"`
}

// versionTarget mirrors the plugin protocol's install target: a channel, a
// build, typed values (manual.input, one manifest per depot per line), and
// for a build from the install history the reference Platypus kept.
type versionTarget struct {
	Channel    string            `json:"channel,omitempty"`
	Build      string            `json:"build,omitempty"`
	Manual     map[string]string `json:"manual,omitempty"`
	ArchiveRef json.RawMessage   `json:"archiveRef,omitempty"`
}

// resolvedTarget is what fetchDepots downloads: the branch for the request
// codes, the manifest gid overrides, and the build id to record.
type resolvedTarget struct {
	branch      string
	password    string
	manifests   map[uint32]uint64
	build       string
	versionName string
}

// parseBranches reads the PICS branch list. "public" comes first, the rest
// sort by name. Apps without a branches section (old fixtures, minimal
// entries) get a synthesized public branch from the depots' public
// manifests, so installs without a target keep working; an app with
// neither is an error, never an empty list that could install the wrong
// build.
func parseBranches(kv *steam.KV) ([]branchInfo, error) {
	depots := kv.Get("depots")
	if depots == nil {
		return nil, fmt.Errorf("app has no depots section")
	}
	// Per-depot manifests per branch, keyed depot -> branch -> gid.
	byDepot := map[uint32]map[string]uint64{}
	for id, d := range depots.Children {
		var depotID uint32
		fmt.Sscanf(id, "%d", &depotID)
		if depotID == 0 {
			continue
		}
		manifests := d.Get("manifests")
		if manifests == nil {
			continue
		}
		for branch, m := range manifests.Children {
			var gid uint64
			fmt.Sscanf(m.String("gid"), "%d", &gid)
			if gid == 0 {
				continue
			}
			if byDepot[depotID] == nil {
				byDepot[depotID] = map[string]uint64{}
			}
			byDepot[depotID][branch] = gid
		}
	}
	branches := map[string]*branchInfo{}
	brSection := depots.Get("branches")
	if brSection != nil {
		for name, b := range brSection.Children {
			bi := &branchInfo{
				Name:        name,
				BuildID:     b.String("buildid"),
				Description: b.String("description"),
				PwdRequired: b.String("pwdrequired") == "1",
				Manifests:   map[uint32]uint64{},
			}
			if ts := b.String("timeupdated"); ts != "" {
				if unix, err := strconv.ParseInt(ts, 10, 64); err == nil {
					bi.TimeUpdated = time.Unix(unix, 0).UTC().Format("2006-01-02")
				}
			}
			branches[name] = bi
		}
	}
	for depotID, perBranch := range byDepot {
		for branch, gid := range perBranch {
			bi, ok := branches[branch]
			if !ok {
				bi = &branchInfo{Name: branch, Manifests: map[uint32]uint64{}}
				branches[branch] = bi
			}
			bi.Manifests[depotID] = gid
		}
	}
	// Drop branches with no manifests anywhere: not installable.
	out := make([]branchInfo, 0, len(branches))
	for _, bi := range branches {
		if len(bi.Manifests) == 0 {
			continue
		}
		out = append(out, *bi)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("app has no downloadable branches")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == "public" {
			return true
		}
		if out[j].Name == "public" {
			return false
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// findBranch returns the branch with this name, or nil.
func findBranch(branches []branchInfo, name string) *branchInfo {
	for i := range branches {
		if branches[i].Name == name {
			return &branches[i]
		}
	}
	return nil
}

// branchForBuild returns the first branch (public first) at this build id.
func branchForBuild(branches []branchInfo, build string) *branchInfo {
	for i := range branches {
		if branches[i].BuildID == build {
			return &branches[i]
		}
	}
	return nil
}

// branchForManifests returns the first branch (public first) whose manifests
// cover every override gid, so a pinned set reinstalls under its own branch.
// Nil when no single branch carries them all.
func branchForManifests(branches []branchInfo, overrides map[uint32]uint64) *branchInfo {
	if len(overrides) == 0 {
		return nil
	}
	for i := range branches {
		match := true
		for depot, gid := range overrides {
			if branches[i].Manifests[depot] != gid {
				match = false
				break
			}
		}
		if match {
			return &branches[i]
		}
	}
	return nil
}

// parseManualManifests reads manual.input: one manifest per depot per line,
// "depot: gid" (":" or "=" separates). Anything else is an error naming the
// line, so a typo never installs the default instead.
func parseManualManifests(input string) (map[uint32]uint64, error) {
	out := map[uint32]uint64{}
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sep := strings.IndexAny(line, ":=")
		if sep < 0 {
			return nil, fmt.Errorf("bad manifest line %q: want \"depot: gid\"", line)
		}
		var depot uint32
		var gid uint64
		if _, err := fmt.Sscanf(strings.TrimSpace(line[:sep]), "%d", &depot); err != nil || depot == 0 {
			return nil, fmt.Errorf("bad manifest line %q: want \"depot: gid\"", line)
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(line[sep+1:]), "%d", &gid); err != nil || gid == 0 {
			return nil, fmt.Errorf("bad manifest line %q: want \"depot: gid\"", line)
		}
		out[depot] = gid
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("type one manifest id per depot, like \"11: 1234567890\"")
	}
	return out, nil
}

// parseArchiveManifests reads the install history's reference:
// {"depots": {"<id>": "<gid>"}}.
func parseArchiveManifests(raw json.RawMessage) (map[uint32]uint64, error) {
	var ref struct {
		Depots map[string]string `json:"depots"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil, fmt.Errorf("bad archive reference: %v", err)
	}
	out := map[uint32]uint64{}
	for idStr, gidStr := range ref.Depots {
		var depot uint32
		var gid uint64
		if _, err := fmt.Sscanf(idStr, "%d", &depot); err != nil || depot == 0 {
			return nil, fmt.Errorf("bad archive reference: depot %q", idStr)
		}
		if _, err := fmt.Sscanf(gidStr, "%d", &gid); err != nil || gid == 0 {
			return nil, fmt.Errorf("bad archive reference: manifest %q", gidStr)
		}
		out[depot] = gid
	}
	return out, nil
}

// manualBuildID names a manifest-pinned set stably for the install ledger.
func manualBuildID(manifests map[uint32]uint64) string {
	depots := make([]uint32, 0, len(manifests))
	for d := range manifests {
		depots = append(depots, d)
	}
	sort.Slice(depots, func(i, j int) bool { return depots[i] < depots[j] })
	var sb strings.Builder
	for _, d := range depots {
		fmt.Fprintf(&sb, "%d:%d,", d, manifests[d])
	}
	sum := sha1.Sum([]byte(sb.String()))
	return fmt.Sprintf("manual-%.12x", sum)
}

// resolveTarget turns the install target into a branch, manifest overrides
// and the build id to record. No target means the default branch. A locked
// channel without its password is refused, and an explicit build that is no
// longer the branch's tip is refused too: both must error, never silently
// install another build.
func resolveTarget(branches []branchInfo, req request) (resolvedTarget, error) {
	passwordFor := func(branch string) string {
		if req.BranchPassword != "" {
			return req.BranchPassword
		}
		return req.BranchPasswords[branch]
	}
	locked := func(bi *branchInfo, pw string) error {
		if bi.PwdRequired && pw == "" {
			return fmt.Errorf("the %s branch is locked: enter its password first", bi.Name)
		}
		return nil
	}
	t := req.Target
	if t == nil {
		branch := req.Branch
		if branch == "" {
			branch = "public"
		}
		bi := findBranch(branches, branch)
		if bi == nil {
			return resolvedTarget{}, fmt.Errorf("Steam has no branch %q for app %d", branch, req.AppID)
		}
		pw := passwordFor(branch)
		if err := locked(bi, pw); err != nil {
			return resolvedTarget{}, err
		}
		return resolvedTarget{branch: branch, password: pw, build: bi.BuildID, versionName: bi.Name}, nil
	}
	overrides := map[uint32]uint64{}
	if input := strings.TrimSpace(t.Manual["input"]); input != "" {
		m, err := parseManualManifests(input)
		if err != nil {
			return resolvedTarget{}, err
		}
		for d, g := range m {
			overrides[d] = g
		}
	}
	if len(t.ArchiveRef) > 0 {
		m, err := parseArchiveManifests(t.ArchiveRef)
		if err != nil {
			return resolvedTarget{}, err
		}
		for d, g := range m {
			overrides[d] = g
		}
	}
	branch := ""
	if t.Channel != "" {
		branch = t.Channel
	} else if len(overrides) > 0 {
		if bi := branchForManifests(branches, overrides); bi != nil {
			branch = bi.Name
		} else {
			branch = "public"
		}
	} else if t.Build != "" {
		bi := branchForBuild(branches, t.Build)
		if bi == nil {
			return resolvedTarget{}, fmt.Errorf("Steam has no build %q for app %d", t.Build, req.AppID)
		}
		branch = bi.Name
	} else {
		branch = "public"
	}
	bi := findBranch(branches, branch)
	if bi == nil {
		return resolvedTarget{}, fmt.Errorf("Steam has no branch %q for app %d", branch, req.AppID)
	}
	pw := passwordFor(branch)
	if err := locked(bi, pw); err != nil {
		return resolvedTarget{}, err
	}
	// An explicit build pins exactly that build: with manifest overrides
	// the ledger's build id travels through, otherwise the branch must
	// still be at it.
	build := bi.BuildID
	versionName := bi.Name
	if len(overrides) > 0 {
		if t.Build != "" {
			build = t.Build
		} else {
			build = manualBuildID(overrides)
		}
		versionName = "pinned manifests"
	} else if t.Build != "" && t.Build != bi.BuildID {
		return resolvedTarget{}, fmt.Errorf("the %s branch is now at build %s, not %s: pick the version again", bi.Name, bi.BuildID, t.Build)
	} else if t.Build != "" {
		build = t.Build
	}
	return resolvedTarget{branch: branch, password: pw, manifests: overrides, build: build, versionName: versionName}, nil
}

// versionResult adds what the plugin records for the install ledger to an
// install/update result: the branch, the build id and the manifest gid per
func versionResult(rt resolvedTarget, depots []fetchedDepot, res map[string]any) map[string]any {
	manifests := map[uint32]uint64{}
	for _, d := range depots {
		manifests[d.ref.DepotID] = d.ref.GID
	}
	res["branch"] = rt.branch
	res["build"] = rt.build
	res["version_name"] = rt.versionName
	res["manifests"] = manifests
	return res
}

// runVersions answers the plugin's versions list: PICS branches with their
// per-depot manifests. Unknown apps and CM failures are errors; an empty
// branch list would install the wrong build, so it errors too.
func runVersions(ctx context.Context, req request) error {
	if req.AppID == 0 {
		return fmt.Errorf("app_id is missing")
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	toks, err := cm.PICSAccessTokens([]uint32{req.AppID}, nil, 0)
	if err != nil {
		return callErr("product info tokens", err)
	}
	kv, _, err := cm.ProductInfo(req.AppID, toks[req.AppID], 0)
	if err != nil {
		return notFoundOr("product info", req.AppID, err)
	}
	branches, err := parseBranches(kv)
	if err != nil {
		return fmt.Errorf("app %d: %v", req.AppID, err)
	}
	emit(map[string]any{"type": "result", "branches": branches})
	return nil
}

// runUnlock checks one branch password. A wrong password is
// {"unlocked": false}, not an error: only transport failures error.
func runUnlock(ctx context.Context, req request) error {
	if req.AppID == 0 {
		return fmt.Errorf("app_id is missing")
	}
	if req.Branch == "" {
		return fmt.Errorf("branch is missing")
	}
	if req.BranchPassword == "" {
		return fmt.Errorf("password is missing")
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	toks, err := cm.PICSAccessTokens([]uint32{req.AppID}, nil, 0)
	if err != nil {
		return callErr("product info tokens", err)
	}
	kv, _, err := cm.ProductInfo(req.AppID, toks[req.AppID], 0)
	if err != nil {
		return notFoundOr("product info", req.AppID, err)
	}
	branches, err := parseBranches(kv)
	if err != nil {
		return fmt.Errorf("app %d: %v", req.AppID, err)
	}
	bi := findBranch(branches, req.Branch)
	if bi == nil {
		return fmt.Errorf("Steam has no branch %q for app %d", req.Branch, req.AppID)
	}
	if !bi.PwdRequired {
		return fmt.Errorf("the %s branch has no password", bi.Name)
	}
	if err := cm.CheckBetaPassword(req.AppID, req.BranchPassword, 0); err != nil {
		if refused(err) {
			emit(map[string]any{"type": "result", "unlocked": false})
			return nil
		}
		return err
	}
	emit(map[string]any{"type": "result", "unlocked": true})
	return nil
}
