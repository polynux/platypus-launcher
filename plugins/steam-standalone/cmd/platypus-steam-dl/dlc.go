// DLC catalog for game.options (protocol 1.5): the game's DLC app ids
// (PICS extended.listofdlc) with each DLC's depots (depots.<id>.dlcappid),
// names (PICS common.name per DLC app), per-DLC install sizes (manifest
// sizes on the download platform) and which DLC the account owns (its CM
// licences mapped through PICS package info). The depot->DLC map covers
// both platform selections; by_os names the depots each platform would
// download, so the plugin can tell DLC with nothing on its platform apart.
// One CM session: no manifests, keys or downloads.
package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"platypus/internal/steamcm"
)

func runDLC(ctx context.Context, req request) error {
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
		return callErr("product info", err)
	}
	// A password branch resolves through its private-beta section (merged
	// into kv); without its password the catalog degrades to the open
	// sections, still listing every DLC.
	_, _ = branchesForRequest(cm, kv, req.AppID, toks[req.AppID], req)
	branch := req.Branch
	if branch == "" {
		branch = "public"
	}
	// Depot->DLC per platform (an install filters the set its platform
	// downloads) merged into the union map game.options shows.
	depots := map[uint32]uint32{}
	byOS := map[string][]uint32{}
	var firstOS string
	for _, osname := range []string{"linux", "windows"} {
		refs, err := selectDepots(kv, req.AppID, branch, osname)
		if err != nil {
			continue
		}
		if firstOS == "" {
			firstOS = osname
		}
		for _, r := range refs {
			depots[r.DepotID] = r.DLCAppID
			byOS[osname] = append(byOS[osname], r.DepotID)
		}
	}
	for osname := range byOS {
		sort.Slice(byOS[osname], func(i, j int) bool { return byOS[osname][i] < byOS[osname][j] })
	}
	byDLC := map[uint32][]uint32{}
	for _, id := range listDLC(kv) {
		byDLC[id] = []uint32{}
	}
	var base []uint32
	for depot, dlc := range depots {
		if dlc == 0 {
			base = append(base, depot)
			continue
		}
		byDLC[dlc] = append(byDLC[dlc], depot)
	}
	ids := make([]uint32, 0, len(byDLC))
	for id := range byDLC {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	// Sizes count the manifests of the download platform (the linux set,
	// else the windows one — the same order an install uses).
	sizes := map[uint32]uint64{}
	if firstOS != "" {
		if refs, err := selectDepots(kv, req.AppID, branch, firstOS); err == nil {
			for _, r := range refs {
				if r.DLCAppID == 0 {
					continue
				}
				sizes[r.DLCAppID] += manifestSize(kv.Get("depots").String(fmt.Sprint(r.DepotID), "manifests", branch, "size"))
			}
		}
	}
	// Names come from each DLC app's own PICS info (one session, no Store
	// calls); ownership from the session's licences mapped through PICS
	// package info. Either may come back empty; the plugin falls back.
	names := dlcNames(cm, ids)
	owned := dlcOwned(cm, ids)
	dlc := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		deps := byDLC[id]
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		e := map[string]any{"appid": id, "depots": deps}
		if name := names[id]; name != "" {
			e["name"] = name
		}
		if size := sizes[id]; size != 0 {
			e["sizeBytes"] = size
		}
		dlc = append(dlc, e)
	}
	sort.Slice(base, func(i, j int) bool { return base[i] < base[j] })
	if base == nil {
		base = []uint32{}
	}
	res := map[string]any{"type": "result", "dlc": dlc, "depots": depots, "base": base, "by_os": byOS}
	if owned != nil {
		res["owned"] = owned
	}
	emit(res)
	return nil
}

// dlcNames reads each DLC app's display name from PICS (common.name). An
// app with no info keeps the plugin's "Add-on N" fallback.
func dlcNames(cm *steamcm.Session, ids []uint32) map[uint32]string {
	out := map[uint32]string{}
	if len(ids) == 0 {
		return out
	}
	toks, err := cm.PICSAccessTokens(ids, nil, 0)
	if err != nil {
		return out
	}
	for _, id := range ids {
		kv, _, err := cm.ProductInfo(id, toks[id], 0)
		if err != nil {
			continue
		}
		if name := strings.TrimSpace(kv.String("common", "name")); name != "" {
			out[id] = name
		}
	}
	return out
}

// dlcOwned reports which of the catalog ids the account's CM licences
// cover, via PICS package info (package id -> app ids). Nil when the
// session carries no licences, so the caller can fall back.
func dlcOwned(cm *steamcm.Session, ids []uint32) []uint32 {
	if len(ids) == 0 {
		return []uint32{}
	}
	lics := cm.Licenses()
	if len(lics) == 0 {
		return nil
	}
	pkgs := make([]uint32, 0, len(lics))
	for _, l := range lics {
		pkgs = append(pkgs, l.PackageID)
	}
	apps, err := cm.PackageApps(pkgs, 0)
	if err != nil {
		return nil
	}
	licensed := map[uint32]bool{}
	for _, got := range apps {
		for _, id := range got {
			licensed[id] = true
		}
	}
	var owned []uint32
	for _, id := range ids {
		if licensed[id] {
			owned = append(owned, id)
		}
	}
	if owned == nil {
		owned = []uint32{}
	}
	return owned
}

// manifestSize parses a PICS depot manifest size (manifests.<branch>.size,
// the disk bytes the install counts).
func manifestSize(s string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
