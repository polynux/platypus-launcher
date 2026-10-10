package main

// Fake cloud saves (capability cloud-saves, protocol 1.2): echo keeps one
// record per game in $PLATYPUS_PLUGIN_DATA/cloud.json and moves files between
// "device" and "cloud" by changing counts. Nothing touches the disk but the
// record and a backup folder.
//
// Knobs, read at start and changeable with `config` like the capability ones:
//
//	ECHO_CLOUD           state a game without a record starts in: synced (default), pending,
//	                     conflict (one file), conflict2 (two files), unsupported, offline, error.
//	                     unsupported, offline and error stay whatever pull, push or resolve do.
//	ECHO_CLOUD_CLIENT    1: the store's client is running and can start the game; nolaunch: it
//	                     runs and can't; unset: not running
//	ECHO_CLOUD_DELAY_MS  pause between the four steps of pull, push and resolve (default 0), so a
//	                     cancel has something to stop
//	ECHO_CLOUD_CHURN     the first n cloud.status answers carry a different stamp each time, like
//	                     a game that is still writing
//
// `config` takes {cloud, cloudClient, cloudDelayMs, cloudChurn}; setting `cloud` forgets every record.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type cloudFile struct {
	Size    int64  `json:"size"`
	Mtime   int64  `json:"mtime"`
	Machine string `json:"machine,omitempty"`
}

type cloudConflict struct {
	Path   string    `json:"path"`
	Local  cloudFile `json:"local"`
	Remote cloudFile `json:"remote"`
}

type cloudCounts struct {
	Same     int `json:"same"`
	Download int `json:"download"`
	Upload   int `json:"upload"`
	Conflict int `json:"conflict"`
	Unmapped int `json:"unmapped"`
}

type cloudRecord struct {
	Sticky    string          `json:"sticky,omitempty"` // unsupported, offline or error
	Same      int             `json:"same"`
	Download  int             `json:"download"`
	Upload    int             `json:"upload"`
	Conflicts []cloudConflict `json:"conflicts,omitempty"`
	Changes   int             `json:"changes"` // pulls, pushes and resolves so far: the stamp's base
	Backups   int             `json:"backups"`
}

type cloudFiles struct {
	Games map[string]*cloudRecord `json:"games"`
}

type cloudKnobs struct {
	State   string `json:"cloud"`
	Client  string `json:"cloudClient"`
	DelayMs int    `json:"cloudDelayMs"`
	Churn   int    `json:"cloudChurn"`
}

var (
	cloudMu   sync.Mutex
	cloudKn   cloudKnobs
	cloudData = cloudFiles{Games: map[string]*cloudRecord{}}
	cloudDone = map[string]json.RawMessage{} // jobId → the status it ended with
	cloudNow  = func() time.Time { return time.Now() }
)

func loadCloud() {
	cloudKn = cloudKnobs{State: os.Getenv("ECHO_CLOUD"), Client: os.Getenv("ECHO_CLOUD_CLIENT")}
	cloudKn.DelayMs, _ = strconv.Atoi(os.Getenv("ECHO_CLOUD_DELAY_MS"))
	cloudKn.Churn, _ = strconv.Atoi(os.Getenv("ECHO_CLOUD_CHURN"))
	if p := cloudPath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &cloudData)
		}
	}
	if cloudData.Games == nil {
		cloudData.Games = map[string]*cloudRecord{}
	}
}

func cloudPath() string {
	if dir := os.Getenv("PLATYPUS_PLUGIN_DATA"); dir != "" {
		return filepath.Join(dir, "cloud.json")
	}
	return ""
}

// saveCloud writes the records. Callers hold cloudMu.
func saveCloud() {
	if p := cloudPath(); p != "" {
		data, _ := json.MarshalIndent(cloudData, "", "  ")
		_ = os.WriteFile(p, data, 0o600)
	}
}

func configureCloud(raw json.RawMessage) {
	var present map[string]json.RawMessage
	_ = json.Unmarshal(raw, &present)
	cloudMu.Lock()
	defer cloudMu.Unlock()
	if v, ok := present["cloud"]; ok {
		_ = json.Unmarshal(v, &cloudKn.State)
		cloudData.Games = map[string]*cloudRecord{}
		cloudDone = map[string]json.RawMessage{}
		saveCloud()
	}
	if v, ok := present["cloudClient"]; ok {
		_ = json.Unmarshal(v, &cloudKn.Client)
	}
	if v, ok := present["cloudDelayMs"]; ok {
		_ = json.Unmarshal(v, &cloudKn.DelayMs)
	}
	if v, ok := present["cloudChurn"]; ok {
		_ = json.Unmarshal(v, &cloudKn.Churn)
	}
}

func seedCloud(state string) *cloudRecord {
	now := cloudNow().Unix()
	file := func(p string, i int64) cloudConflict {
		return cloudConflict{Path: p,
			Local:  cloudFile{Size: 3700 + i, Mtime: now - 3600},
			Remote: cloudFile{Size: 3718 + i, Mtime: now - 86400*30, Machine: "Deck"}}
	}
	switch state {
	case "pending":
		return &cloudRecord{Same: 2, Download: 1, Upload: 1}
	case "conflict":
		return &cloudRecord{Same: 2, Conflicts: []cloudConflict{file("Profile/save.dat", 0)}}
	case "conflict2":
		return &cloudRecord{Same: 1, Conflicts: []cloudConflict{file("Profile/save.dat", 0), file("Settings/options.cfg", 9)}}
	case "unsupported", "offline", "error":
		return &cloudRecord{Sticky: state}
	}
	return &cloudRecord{Same: 3}
}

// cloudRec returns the game's record, seeding it from the knob. Callers hold cloudMu.
func cloudRec(id string) *cloudRecord {
	r, ok := cloudData.Games[id]
	if !ok {
		r = seedCloud(cloudKn.State)
		cloudData.Games[id] = r
	}
	return r
}

var stickyMessages = map[string]string{
	"unsupported": "Echo's saves go through a storage API that Platypus can't sync from outside the game (test knob).",
	"offline":     "Echo's cloud can't be reached right now (test knob).",
	"error":       "Echo's cloud answered with an error (test knob).",
}

// cloudStatus builds the answer for a record. Callers hold cloudMu.
func cloudStatus(id string, r *cloudRecord, churn bool) map[string]any {
	state := "synced"
	switch {
	case r.Sticky != "":
		state = r.Sticky
	case len(r.Conflicts) > 0:
		state = "conflict"
	case r.Download > 0 || r.Upload > 0:
		state = "pending"
	}
	counts := cloudCounts{Same: r.Same, Download: r.Download, Upload: r.Upload, Conflict: len(r.Conflicts)}
	if r.Sticky != "" {
		counts = cloudCounts{}
	}
	stamp := fmt.Sprintf("%s-%d", id, r.Changes)
	if churn && cloudKn.Churn > 0 {
		stamp += fmt.Sprintf("-churn%d", cloudKn.Churn)
		cloudKn.Churn--
	}
	st := map[string]any{"state": state, "counts": counts, "checkedAt": cloudNow().Unix(), "stamp": stamp}
	if msg := stickyMessages[r.Sticky]; msg != "" {
		st["message"] = msg
	} else if state == "conflict" {
		st["message"] = fmt.Sprintf("%d file(s) changed on this device and in the cloud.", len(r.Conflicts))
	}
	if len(r.Conflicts) > 0 && r.Sticky == "" {
		st["conflicts"] = r.Conflicts
	}
	if r.Backups > 0 {
		st["backupDir"] = filepath.Join(os.Getenv("PLATYPUS_PLUGIN_DATA"), "cloud-backups", id)
	}
	switch cloudKn.Client {
	case "":
	default:
		st["client"] = map[string]any{"running": true, "name": "Echo client", "canLaunch": cloudKn.Client != "nolaunch"}
	}
	return st
}

// cloudParams checks what every cloud.* request must carry.
func cloudParams(method string, p params) *rpcError {
	if p.ID == "" {
		return &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	if _, ok := games[p.ID]; !ok {
		return &rpcError{Code: -32000, Message: fmt.Sprintf("unknown game %q", p.ID)}
	}
	var ctx struct {
		OS         string `json:"os"`
		InstallDir string `json:"installDir"`
		PrefixDir  string `json:"prefixDir"`
	}
	if len(p.Ctx) == 0 || json.Unmarshal(p.Ctx, &ctx) != nil || (ctx.OS != "linux" && ctx.OS != "windows") ||
		!filepath.IsAbs(ctx.InstallDir) || (ctx.OS == "windows" && !filepath.IsAbs(ctx.PrefixDir)) {
		return &rpcError{Code: -32602, Message: "invalid params: ctx needs os (linux or windows), an absolute installDir and, for windows, an absolute prefixDir"}
	}
	if method != "cloud.status" && p.JobID == "" {
		return &rpcError{Code: -32602, Message: "invalid params: jobId is required"}
	}
	return nil
}

func cloudCall(method string, p params) (any, error) {
	if err := cloudParams(method, p); err != nil {
		return nil, err
	}
	if method == "cloud.status" {
		cloudMu.Lock()
		defer cloudMu.Unlock()
		return cloudStatus(p.ID, cloudRec(p.ID), true), nil
	}

	cloudMu.Lock()
	if done, ok := cloudDone[p.JobID]; ok {
		cloudMu.Unlock()
		return done, nil // a repeated jobId returns the first run's result
	}
	delay := time.Duration(cloudKn.DelayMs) * time.Millisecond
	cloudMu.Unlock()

	stop := make(chan struct{})
	jobsMu.Lock()
	if _, busy := cancels[p.JobID]; busy {
		jobsMu.Unlock()
		return nil, &rpcError{Code: -32000, Message: "that job is already running"}
	}
	cancels[p.JobID] = stop
	jobsMu.Unlock()
	defer func() {
		jobsMu.Lock()
		delete(cancels, p.JobID)
		jobsMu.Unlock()
	}()

	phase := map[string]string{"cloud.pull": "download", "cloud.push": "upload", "cloud.resolve": "check"}[method]
	const steps = 4
	for i := 0; i <= steps; i++ {
		notify("cloud.progress", map[string]any{"id": p.ID, "jobId": p.JobID, "phase": phase, "done": i, "total": steps})
		if i == steps {
			break
		}
		select {
		case <-stop:
			return nil, &rpcError{Code: -32000, Message: "canceled"}
		case <-time.After(delay):
		}
	}

	cloudMu.Lock()
	defer cloudMu.Unlock()
	r := cloudRec(p.ID)
	if method == "cloud.resolve" {
		if err := checkChoices(r, p.Choices); err != nil {
			return nil, err
		}
	}
	if cloudKn.Client == "" && r.Sticky == "" {
		switch method {
		case "cloud.pull":
			if r.Download > 0 {
				r.Same, r.Download = r.Same+r.Download, 0
				r.Backups++
				r.Changes++
			}
		case "cloud.push":
			// A file changed on both sides is never uploaded over the cloud copy.
			if r.Upload > 0 && len(r.Conflicts) == 0 {
				r.Same, r.Upload = r.Same+r.Upload, 0
				r.Changes++
			}
		case "cloud.resolve":
			resolveCloud(r, p.Choices)
		}
	}
	saveCloud()
	st, _ := json.Marshal(cloudStatus(p.ID, r, false))
	cloudDone[p.JobID] = st
	return json.RawMessage(st), nil
}

// checkChoices refuses a choice for a file that isn't a conflict. Callers hold cloudMu.
func checkChoices(r *cloudRecord, choices map[string]string) *rpcError {
	for path, c := range choices {
		if c != "local" && c != "remote" {
			return &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: choice for %q must be local or remote", path)}
		}
		found := false
		for _, cf := range r.Conflicts {
			found = found || cf.Path == path
		}
		if !found {
			return &rpcError{Code: -32602, Message: fmt.Sprintf("invalid params: %q is not a conflict", path)}
		}
	}
	return nil
}

// resolveCloud applies the user's choices to the record's conflicts. Callers hold cloudMu.
func resolveCloud(r *cloudRecord, choices map[string]string) {
	left := r.Conflicts[:0:0]
	for _, cf := range r.Conflicts {
		switch choices[cf.Path] {
		case "local":
			r.Upload++
		case "remote":
			r.Same++
			r.Backups++
		default:
			left = append(left, cf)
		}
	}
	r.Conflicts = left
	r.Changes++
}
