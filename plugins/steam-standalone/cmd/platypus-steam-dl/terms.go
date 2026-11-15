// Terms from PICS app info (protocol 1.11, docs/plugins/protocol.md,
// "Terms"). Steam lists a game's EULAs under common/eulas, an indexed object
// whose entries carry id, name, url and version (confirmed live on Trackmania
// 2225070). The version is what Platypus keys acceptance on. Steam records the
// acceptance itself, so the plugin has no game.acceptTerms.
package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"platypus/internal/steam"
)

// eulaTerm is one term as game.details carries it.
type eulaTerm struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	URL     string `json:"url,omitempty"`
	Version string `json:"version"`
}

// parseEULAs reads common/eulas from app info. Entries without an id or a
// version are dropped (acceptance is keyed on both), and the indexed order of
// the store's object is kept.
func parseEULAs(kv *steam.KV) []eulaTerm {
	eulas := kv.Get("common", "eulas")
	if eulas == nil {
		return nil
	}
	keys := make([]string, 0, len(eulas.Children))
	for k := range eulas.Children {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, aerr := strconv.Atoi(keys[i])
		b, berr := strconv.Atoi(keys[j])
		if aerr == nil && berr == nil {
			return a < b
		}
		return keys[i] < keys[j]
	})
	out := make([]eulaTerm, 0, len(keys))
	for _, k := range keys {
		e := eulas.Children[k]
		id, version := e.String("id"), e.String("version")
		if id == "" || version == "" {
			continue
		}
		out = append(out, eulaTerm{ID: id, Title: e.String("name"), URL: e.String("url"), Version: version})
	}
	return out
}

// runTerms answers the plugin's `terms` subcommand: the app's EULAs from PICS
// app info, fetched over the authenticated CM session like the sizes are.
func runTerms(ctx context.Context, req request) error {
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
	emit(map[string]any{"type": "result", "terms": parseEULAs(kv)})
	return nil
}
