// Workshop/UGC downloads (protocol 1.22): one published file's content is a
// depot manifest, named by its hcontent_file and downloaded from the game's
// workshop depot (depots.workshopdepot, or the app id when PICS names none) —
// the same content path DepotDownloader's -pubfile takes (ContentDownloader.
// DownloadPubfileAsync -> DownloadAppAsync with (appId, hcontent)). Files land
// in <install_dir>/<published_file_id>, which the plugin points at the
// store's own workshop content folder or the Platypus-managed one.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"platypus/internal/depot"
)

type workshopItem struct {
	ID       string `json:"id"`       // published file id, decimal
	HContent uint64 `json:"hcontent"` // hcontent_file: the content manifest id
	Size     uint64 `json:"size"`     // the store's file_size, for progress only
}

func runWorkshop(ctx context.Context, req request) error {
	if req.InstallDir == "" {
		return fmt.Errorf("install_dir is missing")
	}
	if len(req.Items) == 0 {
		emit(map[string]any{"type": "result", "items": []any{}})
		return nil
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return err
	}
	defer cm.Close()
	ss := &session{cm: cm, dataDir: dataDir(), req: req}

	toks, err := cm.PICSAccessTokens([]uint32{req.AppID}, nil, 0)
	if err != nil {
		return callErr("product info tokens", err)
	}
	kv, _, err := cm.ProductInfo(req.AppID, toks[req.AppID], 0)
	if err != nil {
		return callErr("product info", err)
	}
	// DepotDownloader's workshop path: the app's workshop depot when PICS
	// names one, else the app id itself.
	depotID := req.AppID
	if depots := kv.Get("depots"); depots != nil {
		var named uint32
		if _, err := fmt.Sscanf(depots.String("workshopdepot"), "%d", &named); err == nil && named != 0 {
			depotID = named
		}
	}
	servers, err := cm.ServersForSteamPipe(cellID(), 32, 0)
	if err != nil {
		return callErr("server list", err)
	}
	ss.hosts = depotHosts(servers, req.AppID)
	if len(ss.hosts) == 0 {
		return fmt.Errorf("Steam returned no download hosts for this app")
	}
	// UGC manifests may encrypt their filenames; that needs the depot key.
	// A key failure is only fatal when a manifest actually needs it.
	key, keyErr := cm.DepotKey(depotID, req.AppID, 0)
	hosts := ss.hostsForDepot(ctx, DepotRef{DepotID: depotID, KeyAppID: req.AppID})
	client := depot.NewClient()

	// Fetch every manifest first: the job's total comes from them, and a
	// missing or refused item fails before anything is written.
	type itemPlan struct {
		item workshopItem
		man  *depot.Manifest
	}
	var plans []itemPlan
	var total uint64
	for _, item := range req.Items {
		if item.HContent == 0 {
			continue // a legacy file_url item is the plugin's HTTP path
		}
		code, err := cm.ManifestRequestCode(req.AppID, depotID, item.HContent, "public", "", 30*time.Second)
		if err != nil {
			return callErr(fmt.Sprintf("workshop item %s request code", item.ID), err)
		}
		raw, err := fetchManifest(ctx, client, hosts, depotID, item.HContent, code)
		if err != nil {
			return fmt.Errorf("workshop item %s: %w", item.ID, err)
		}
		man, err := depot.ParseManifest(raw)
		if err != nil {
			return fmt.Errorf("workshop item %s: %w", item.ID, err)
		}
		if man.Meta.FilenamesEncrypted {
			if keyErr != nil {
				return fmt.Errorf("workshop item %s: depot %d key: %w", item.ID, depotID, keyErr)
			}
			if err := man.DecryptFilenames(key); err != nil {
				return fmt.Errorf("workshop item %s: %w", item.ID, err)
			}
		}
		plans = append(plans, itemPlan{item: item, man: man})
		total += man.TotalSize()
	}
	if err := os.MkdirAll(req.InstallDir, 0o755); err != nil {
		return err
	}
	onProg, _, _ := progress(total)
	var base uint64
	out := []map[string]any{}
	for _, p := range plans {
		dir := filepath.Join(req.InstallDir, p.item.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := pruneWorkshopDir(dir, p.man); err != nil {
			return fmt.Errorf("workshop item %s: %w", p.item.ID, err)
		}
		w, err := depot.Preallocate(p.man, dir)
		if err != nil {
			return fmt.Errorf("workshop item %s: %w", p.item.ID, err)
		}
		sched := depot.NewScheduler(client, depot.SchedulerConfig{InitialConns: seededInitialConns(len(hosts), 4, len(p.man.UniqueChunks()))})
		dlErr := sched.DownloadStream(ctx, p.man, depotID, hosts, key, nil, func(id string, data []byte) error {
			return w.Write(id, data)
		}, withBase(onProg, base))
		cErr := w.Close()
		if dlErr != nil {
			return fmt.Errorf("workshop item %s: %w", p.item.ID, dlErr)
		}
		if cErr != nil {
			return fmt.Errorf("workshop item %s: %w", p.item.ID, cErr)
		}
		base += p.man.TotalSize()
		bad, err := depot.Verify(p.man, dir)
		if err != nil {
			return fmt.Errorf("workshop item %s: %w", p.item.ID, err)
		}
		if len(bad) > 0 {
			return fmt.Errorf("workshop item %s: verify failed: %v", p.item.ID, bad)
		}
		out = append(out, map[string]any{"id": p.item.ID, "hcontent": p.item.HContent, "bytes": p.man.TotalSize()})
	}
	emit(map[string]any{"type": "result", "items": out, "depot": depotID})
	return nil
}

// pruneWorkshopDir deletes what the item's new manifest no longer lists: a
// Workshop update can remove files, and a stale one left beside the new
// content would keep loading. Directories the manifest still uses stay.
func pruneWorkshopDir(dir string, man *depot.Manifest) error {
	want := map[string]bool{}
	for _, f := range man.Files {
		want[filepath.FromSlash(f.Name)] = true
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if !want[rel] {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // only empty ones go
	}
	return nil
}
