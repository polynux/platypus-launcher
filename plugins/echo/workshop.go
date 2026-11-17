package main

// The fake `workshop` capability (protocol 1.22). Echo has two subscribed
// items for tiny, one already synced and one with an update waiting, so the
// game page's list shows every state; a workshop.sync job marks them all as
// written. No network, no real files: the knobs of the other capabilities
// (off, missing, gameFeatures, ...) apply here too. See
// docs/plugins/protocol.md, "Workshop".

import (
	"path/filepath"
	"sync"
	"time"
)

// workshopItemDef is one subscribed item echo lists.
type workshopItemDef struct {
	id, title, preview string
	size               int64
	updatedAt          int64
	installed          bool
	needsUpdate        bool
}

var workshopItems = map[string][]workshopItemDef{
	"tiny": {
		{id: "900000001", title: "Echo Extra Level", preview: "https://example.com/echo/extra.jpg", size: 3 << 20, updatedAt: 1791000000, installed: true},
		{id: "900000002", title: "Echo Skin Pack", preview: "https://example.com/echo/skins.jpg", size: 1 << 20, updatedAt: 1791100000, installed: true, needsUpdate: true},
	},
}

// workshopMu guards the synced state (a sync marks every item installed).
var (
	workshopMu     sync.Mutex
	workshopSynced = map[string]bool{} // by game id + item id
)

// workshopList answers workshop.list: the game's subscribed items with the
// plugin's own disk state. A game without items answers [], not an error.
func workshopList(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok {
		return nil, &rpcError{Code: -32000, Message: "unknown game " + p.ID}
	}
	if g.external != "" || g.client {
		return nil, &rpcError{Code: -32601, Message: "method not found: workshop.list"}
	}
	workshopMu.Lock()
	defer workshopMu.Unlock()
	list := []map[string]any{}
	for _, it := range workshopItems[p.ID] {
		installed, needs := it.installed, it.needsUpdate
		if workshopSynced[p.ID+"/"+it.id] {
			installed, needs = true, false
		}
		list = append(list, map[string]any{
			"id": it.id, "title": it.title, "previewUrl": it.preview,
			"sizeBytes": it.size, "updatedAt": it.updatedAt,
			"installed": installed, "needsUpdate": needs,
		})
	}
	return list, nil
}

// startWorkshop validates a workshop.sync request and runs its job in the
// background: a short progress line, then job.done.
func startWorkshop(p params) (any, error) {
	if p.ID == "" || p.JobID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id and jobId are required"}
	}
	if !filepath.IsAbs(p.InstallDir) {
		return nil, &rpcError{Code: -32602, Message: "invalid params: installDir must be absolute"}
	}
	g, ok := games[p.ID]
	if !ok {
		return nil, &rpcError{Code: -32000, Message: "unknown game " + p.ID}
	}
	if g.external != "" || g.client {
		return nil, &rpcError{Code: -32601, Message: "method not found: workshop.sync"}
	}
	stop := make(chan struct{})
	jobsMu.Lock()
	if _, running := cancels[p.JobID]; running {
		// A repeated jobId after a restart runs once.
		jobsMu.Unlock()
		return map[string]any{}, nil
	}
	cancels[p.JobID] = stop
	jobsMu.Unlock()
	go func() {
		defer func() {
			jobsMu.Lock()
			delete(cancels, p.JobID)
			jobsMu.Unlock()
		}()
		items := workshopItems[p.ID]
		var total int64
		for _, it := range items {
			total += it.size
		}
		var done int64
		for _, it := range items {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			done += it.size
			notify("job.progress", map[string]any{"jobId": p.JobID, "phase": "downloading", "bytes": done, "total": total})
		}
		workshopMu.Lock()
		for _, it := range items {
			workshopSynced[p.ID+"/"+it.id] = true
		}
		workshopMu.Unlock()
		notify("job.done", map[string]any{"jobId": p.JobID})
	}()
	return map[string]any{}, nil
}
