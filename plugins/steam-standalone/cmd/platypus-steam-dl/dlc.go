// DLC catalog for game.options (protocol 1.5): the game's DLC app ids
// (PICS extended.listofdlc) with each DLC's depots (depots.<id>.dlcappid)
// across both platform selections, plus the depot->DLC map installs filter
// by. One PICS read: no manifests, keys or downloads.
package main

import (
	"context"
	"fmt"
	"sort"
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
	// Depot->DLC across both platform selections: an install filters the
	// set it downloads, and game.options shows the union.
	branch := req.Branch
	if branch == "" {
		branch = "public"
	}
	depots := map[uint32]uint32{}
	for _, osname := range []string{"linux", "windows"} {
		refs, err := selectDepots(kv, req.AppID, branch, osname)
		if err != nil {
			continue
		}
		for _, r := range refs {
			depots[r.DepotID] = r.DLCAppID
		}
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
	dlc := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		deps := byDLC[id]
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		dlc = append(dlc, map[string]any{"appid": id, "depots": deps})
	}
	sort.Slice(base, func(i, j int) bool { return base[i] < base[j] })
	if base == nil {
		base = []uint32{}
	}
	emit(map[string]any{"type": "result", "dlc": dlc, "depots": depots, "base": base})
	return nil
}
