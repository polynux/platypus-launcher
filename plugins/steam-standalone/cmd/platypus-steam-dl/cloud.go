// Cloud saves, read + pull side (protocol 1.2, issue #6, plan phases P1+P2).
//
// One-shot `cloud` subcommand: the request carries op status|pull|push|
// resolve plus the CloudContext and (for pull/push/resolve) the jobId.
// Uploads are out of scope here: cloud.push and `local` resolve choices
// compute the status and say so, without calling any Steam upload API.
//
// Layout under PLATYPUS_PLUGIN_DATA, per game (D6: our own baseline, never
// the official client's remotecache.vdf):
//
//	cloud/<app>/baseline.json  path -> last SHA-1 identical on both sides,
//	                            plus `local` resolve choices
//	cloud/jobs/<jobId>.json    the finished result, for repeated jobIds
//	cloud-backups/<app>/<time>/ replaced local files, rel paths preserved
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steam"
	"platypus/internal/steamcm"
)

// CloudContext mirrors types.CloudContext (docs/plugins/protocol.md, "Cloud
// saves"): what Platypus knows about the launch. The plugin maps its save
// locations through it and never learns Platypus's prefix layout.
type cloudContext struct {
	OS         string `json:"os"`
	Home       string `json:"home"`
	XDGData    string `json:"xdgData"`
	XDGConfig  string `json:"xdgConfig"`
	InstallDir string `json:"installDir"`
	PrefixDir  string `json:"prefixDir"`
	WinUser    string `json:"winUser"`
}

func (c cloudContext) valid() error {
	if c.OS != "linux" && c.OS != "windows" {
		return fmt.Errorf("ctx needs os linux or windows")
	}
	if !filepath.IsAbs(c.InstallDir) {
		return fmt.Errorf("ctx needs an absolute installDir")
	}
	if c.OS == "windows" && !filepath.IsAbs(c.PrefixDir) {
		return fmt.Errorf("ctx needs an absolute prefixDir for windows")
	}
	return nil
}

// --- ufs rules --------------------------------------------------------------

// cloudSaveRule is one ufs/savefiles entry: which local files belong to the
// game. Platforms is nil when the entry carries no platforms block (all
// platforms); otherwise the lowercased OS names it lists.
type cloudSaveRule struct {
	Root      string
	Path      string
	Pattern   string
	Recursive bool
	Platforms map[string]bool
}

// cloudOverride is one ufs/rootoverrides entry: on a matching client OS the
// root is replaced before mapping, with addpath prefixed and pathtransforms
// applied to the relative part.
type cloudOverride struct {
	Root       string
	OS         string
	UseInstead string
	AddPath    string
	Find       []string
	Replace    []string
}

// cloudUFS is the parsed ufs section. HasRules is false for API-storage
// games (ufs carries only quota/maxnumfiles, e.g. Terraria): nothing to map.
type cloudUFS struct {
	HasUFS    bool
	HasRules  bool
	Rules     []cloudSaveRule
	Overrides []cloudOverride
}

// indexedChildren returns the children of an indexed object ({"0": {...},
// "1": {...}}) in numeric order, like parseEULAs does.
func indexedChildren(kv *steam.KV) []*steam.KV {
	if kv == nil {
		return nil
	}
	keys := make([]string, 0, len(kv.Children))
	for k := range kv.Children {
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
	out := make([]*steam.KV, 0, len(keys))
	for _, k := range keys {
		out = append(out, kv.Children[k])
	}
	return out
}

// parseCloudRules reads ufs from PICS app info (already fetched for the
// product info the other subcommands use). Real shapes are recorded in
// docs/research/steam-cloud-social.md §1.4 (SUPERHOT, Hades, Stardew Valley).
func parseCloudRules(kv *steam.KV) *cloudUFS {
	ufs := kv.Get("ufs")
	if ufs == nil {
		return &cloudUFS{}
	}
	out := &cloudUFS{HasUFS: true}
	for _, s := range indexedChildren(ufs.Get("savefiles")) {
		root, path, pattern := s.String("root"), s.String("path"), s.String("pattern")
		if root == "" || pattern == "" {
			continue // a rule without a root or pattern can't match anything
		}
		r := cloudSaveRule{Root: root, Path: path, Pattern: pattern,
			Recursive: s.String("recursive") == "1"}
		if plats := s.Get("platforms"); plats != nil && len(plats.Children) > 0 {
			r.Platforms = map[string]bool{}
			for k := range plats.Children {
				r.Platforms[strings.ToLower(k)] = true
			}
		}
		out.Rules = append(out.Rules, r)
	}
	for _, o := range indexedChildren(ufs.Get("rootoverrides")) {
		osName, use := o.String("os"), o.String("useinstead")
		if o.String("root") == "" || use == "" {
			continue
		}
		ov := cloudOverride{Root: o.String("root"), OS: osName,
			UseInstead: use, AddPath: o.String("addpath")}
		if cmp := o.String("oscompare"); cmp != "" && cmp != "=" {
			continue // only equality matches are understood
		}
		for _, t := range indexedChildren(o.Get("pathtransforms")) {
			find, replace := t.String("find"), t.String("replace")
			if find == "" {
				continue
			}
			ov.Find = append(ov.Find, find)
			ov.Replace = append(ov.Replace, replace)
		}
		out.Overrides = append(out.Overrides, ov)
	}
	out.HasRules = len(out.Rules) > 0
	return out
}

// --- root mapping -----------------------------------------------------------

// resolveCloudRoot maps a root token to a local directory. Windows profile
// roots probe the prefix for the live spelling (Documents vs My Documents);
// anything needing names the context doesn't carry (the SteamCloudDocuments
// user/game pair) is unmapped rather than guessed.
func resolveCloudRoot(name, osName string, ctx cloudContext) (string, bool) {
	home := ctx.Home
	xdgData := ctx.XDGData
	if xdgData == "" && home != "" {
		xdgData = filepath.Join(home, ".local", "share")
	}
	xdgConfig := ctx.XDGConfig
	if xdgConfig == "" && home != "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	switch name {
	case "GameInstall":
		if ctx.InstallDir != "" {
			return ctx.InstallDir, true
		}
		return "", false
	case "LinuxHome":
		if home != "" {
			return home, true
		}
		return "", false
	case "LinuxXdgDataHome":
		if xdgData != "" {
			return xdgData, true
		}
		return "", false
	case "LinuxXdgConfigHome":
		if xdgConfig != "" {
			return xdgConfig, true
		}
		return "", false
	}
	if osName == "windows" {
		user := ctx.WinUser
		if user == "" {
			user = "steamuser"
		}
		profile := filepath.Join(ctx.PrefixDir, "drive_c", "users", user)
		switch name {
		case "WinAppDataLocalLow":
			return filepath.Join(profile, "AppData", "LocalLow"), true
		case "WinAppDataLocal":
			return filepath.Join(profile, "AppData", "Local"), true
		case "WinAppDataRoaming":
			return filepath.Join(profile, "AppData", "Roaming"), true
		case "WinMyDocuments":
			return probeDir(profile, []string{"Documents", "My Documents"}), true
		case "WinSavedGames":
			return probeDir(profile, []string{"Saved Games"}), true
		}
		return "", false
	}
	return "", false
}

// probeDir returns the first existing candidate under base, else the default
// (first) joined to base. Save locations are created on demand, so a game
// that never ran still maps somewhere sensible.
func probeDir(base string, candidates []string) string {
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(base, c)); err == nil && st.IsDir() {
			return filepath.Join(base, c)
		}
	}
	return filepath.Join(base, candidates[0])
}

// splitCloudName splits "%Root%rest" into its root token and relative part.
// The same cloud name is used on every OS; the platform decides where it
// lives locally.
func splitCloudName(name string) (root, rel string, ok bool) {
	if !strings.HasPrefix(name, "%") {
		return "", "", false
	}
	end := strings.Index(name[1:], "%")
	if end < 0 {
		return "", "", false
	}
	return name[1 : 1+end], name[1+end+1:], true
}

// mapCloudName resolves one cloud file name to a local path through the ufs
// rules. Entries group by machine/prefix on the wire (issue #6 L2); the
// caller groups winners by local path, so one file synced from two machines
// compares once.
func mapCloudName(cloudName string, ufs *cloudUFS, ctx cloudContext, steamID uint64) (local, rel string, mapped bool) {
	root, cloudRel, ok := splitCloudName(cloudName)
	if !ok {
		return "", "", false
	}
	cloudRel = substituteSteamID(cloudRel, steamID)
	// The returned rel is the stable cloud-relative part (the status path
	// and baseline key); mapping works on a transformed copy.
	rel = cloudRel
	for _, ov := range ufs.Overrides {
		if !strings.EqualFold(ov.Root, root) || !strings.EqualFold(ov.OS, ctx.OS) {
			continue
		}
		root = ov.UseInstead
		if ov.AddPath != "" {
			rel = ov.AddPath + "/" + rel
		}
		for i := range ov.Find {
			rel = strings.ReplaceAll(rel, ov.Find[i], ov.Replace[i])
		}
		break // first matching override wins, like the client's rule order
	}
	base, ok := resolveCloudRoot(root, ctx.OS, ctx)
	if !ok {
		return "", "", false
	}
	parts := splitRel(rel)
	if len(parts) == 0 || len(parts) > 32 {
		return "", "", false // traversal or absurd depth: never placed
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `:\`) || filepath.IsAbs(p) {
			return "", "", false
		}
	}
	local = base
	if ctx.OS == "windows" {
		for _, p := range parts {
			local = filepath.Join(local, resolveCaseInsensitive(local, p))
		}
	} else {
		local = filepath.Join(append([]string{base}, parts...)...)
	}
	return local, cloudRel, true
}

// substituteSteamID expands {64BitSteamID} and {Steam3AccountID} (the low 32
// bits) in a rule path.
func substituteSteamID(rel string, steamID uint64) string {
	if steamID != 0 {
		rel = strings.ReplaceAll(rel, "{64BitSteamID}", strconv.FormatUint(steamID, 10))
		rel = strings.ReplaceAll(rel, "{Steam3AccountID}", strconv.FormatUint(steamID&0xffffffff, 10))
	}
	return rel
}

// splitRel splits a cloud-relative path (always "/"-separated) into parts.
func splitRel(rel string) []string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	return strings.Split(rel, "/")
}

// resolveCaseInsensitive resolves one path component case-insensitively
// (Wine prefixes are case-insensitive): an existing entry keeps its live
// spelling, a missing one keeps the cloud spelling for creation.
func resolveCaseInsensitive(dir, want string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return want
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), want) {
			return e.Name()
		}
	}
	return want
}

// --- local scan -------------------------------------------------------------

// ruleSearchDir maps one rule to the local directory its files live under
// (root + transformed path), for scanning the device side.
func ruleSearchDir(r cloudSaveRule, ufs *cloudUFS, ctx cloudContext, steamID uint64) (string, bool) {
	root, rel := r.Root, substituteSteamID(r.Path, steamID)
	for _, ov := range ufs.Overrides {
		if !strings.EqualFold(ov.Root, root) || !strings.EqualFold(ov.OS, ctx.OS) {
			continue
		}
		root = ov.UseInstead
		if ov.AddPath != "" {
			rel = ov.AddPath + "/" + rel
		}
		for i := range ov.Find {
			rel = strings.ReplaceAll(rel, ov.Find[i], ov.Replace[i])
		}
		break
	}
	base, ok := resolveCloudRoot(root, ctx.OS, ctx)
	if !ok {
		return "", false
	}
	if rel == "" {
		return base, true
	}
	parts := splitRel(rel)
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, `:\`) {
			return "", false
		}
	}
	if ctx.OS == "windows" {
		out := base
		for _, p := range parts {
			out = filepath.Join(out, resolveCaseInsensitive(out, p))
		}
		return out, true
	}
	return filepath.Join(append([]string{base}, parts...)...), true
}

// joinCloudRel joins a rule path with a found name: an empty rule path means
// the root itself, never a stray leading slash.
func joinCloudRel(base, name string) string {
	if base == "" {
		return name
	}
	return base + "/" + name
}

// scanLocal finds the device files matching the rules: rel (cloud-style path)
// by local path. Symlinked directories are not descended into (a Proton
func scanLocal(ufs *cloudUFS, ctx cloudContext, steamID uint64) map[string]string {
	out := map[string]string{} // local path -> cloud rel
	for _, r := range ufs.Rules {
		if r.Platforms != nil && !r.Platforms[strings.ToLower(ctx.OS)] {
			continue
		}
		dir, ok := ruleSearchDir(r, ufs, ctx, steamID)
		if !ok {
			continue
		}
		relBase := substituteSteamID(r.Path, steamID)
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		if r.Recursive {
			_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if p != dir && d.Type()&os.ModeSymlink != 0 {
						return filepath.SkipDir
					}
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return nil
				}
				if strings.HasPrefix(d.Name(), ".platypus-cloud-") {
					return nil // our own half-written temp files: never saves
				}
				rel, err := filepath.Rel(dir, p)
				if err != nil {
					return nil
				}
				if matchCloudPattern(r.Pattern, rel) {
					if _, dup := out[p]; !dup {
						out[p] = joinCloudRel(relBase, filepath.ToSlash(rel))
					}
				}
				return nil
			})
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
				continue
			}
			if strings.HasPrefix(e.Name(), ".platypus-cloud-") {
				continue // our own half-written temp files: never saves
			}
			if ok, _ := filepath.Match(r.Pattern, e.Name()); ok {
				p := filepath.Join(dir, e.Name())
				if _, dup := out[p]; !dup {
					out[p] = joinCloudRel(relBase, e.Name())
				}
			}
		}
	}
	return out
}

// matchCloudPattern matches a cloud glob ("/"-separated) against a
// dir-relative slash path: a pattern without "/" matches the base name at
// any depth, otherwise the whole relative path.
func matchCloudPattern(pattern, rel string) bool {
	rel = filepath.ToSlash(rel)
	if !strings.Contains(pattern, "/") {
		base := rel
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		ok, _ := filepath.Match(pattern, base)
		return ok
	}
	ok, _ := filepath.Match(pattern, rel)
	return ok
}

// --- changelist -------------------------------------------------------------

// cloudFile is one listed cloud file with its resolved local path.
type cloudFile struct {
	CloudName string // full wire name: %Root%rel
	Rel       string // cloud-relative part: the status path
	Local     string // resolved local path ("" when unmapped)
	SHA       []byte
	Size      uint32
	MTime     int64
	Machine   string
	Persist   uint64 // persist_state: 0 persisted, 1 forgotten, 2 deleted
	Unmapped  bool
}

// fetchChangelist calls Cloud.GetAppFileChangelist#1 (synced_change_number 0
// = the whole list) and decodes the reply. Field numbers are from
// steammessages_cloud.steamclient.proto (docs/research/steam-cloud-social.md
// §1.2); usage order (prefix + name as filename) follows Madeira.
func fetchChangelist(cm *steamcm.Session, appID uint32, timeout time.Duration) ([]cloudFile, error) {
	req := &depot.Enc{}
	req.Varint(1, uint64(appID))
	raw, err := cm.Call("Cloud.GetAppFileChangelist#1", req.Bytes(), timeout)
	if err != nil {
		return nil, err
	}
	m, err := depot.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding the cloud file list: %w", err)
	}
	prefixes := m.AllBytes(4)
	machines := m.AllBytes(5)
	var out []cloudFile
	for _, fb := range m.AllBytes(2) {
		fm, err := depot.Decode(fb)
		if err != nil {
			return nil, fmt.Errorf("decoding a cloud file entry: %w", err)
		}
		f := cloudFile{
			SHA:     fm.Bytes(2),
			Size:    uint32(fm.Int(4)),
			MTime:   int64(fm.Int(3)),
			Persist: fm.Int(5),
		}
		if f.Persist == 1 {
			continue // forgotten by the cloud: not ours to sync
		}
		name := string(fm.Bytes(1))
		if i := int(fm.Int(7)); i >= 0 && i < len(prefixes) {
			f.CloudName = string(prefixes[i]) + name
		} else {
			f.CloudName = name
		}
		if i := int(fm.Int(8)); i >= 0 && i < len(machines) {
			f.Machine = string(machines[i])
		}
		out = append(out, f)
	}
	return out, nil
}

// --- baseline ---------------------------------------------------------------

type cloudBaselineFile struct {
	SHA  string `json:"sha"`
	Size uint32 `json:"size"`
	// Local is the resolved local path this entry was recorded for. An
	// entry counts only when it still equals the current resolved path,
	// so a native<->Proton switch never reads the other side's record.
	Local string `json:"local,omitempty"`
}

type cloudChoice struct {
	Side     string `json:"side"` // "local": the user kept this device's copy
	LocalSHA string `json:"localSha"`
	CloudSHA string `json:"cloudSha"`
}

type cloudBaseline struct {
	Files   map[string]cloudBaselineFile `json:"files"`
	Choices map[string]cloudChoice       `json:"choices,omitempty"`
}

func cloudAppDir(appID uint32) string {
	return filepath.Join(dataDir(), "cloud", strconv.FormatUint(uint64(appID), 10))
}

func cloudJobsDir() string {
	return filepath.Join(dataDir(), "cloud", "jobs")
}

func loadBaseline(appID uint32) *cloudBaseline {
	b := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	raw, err := os.ReadFile(filepath.Join(cloudAppDir(appID), "baseline.json"))
	if err != nil {
		return b
	}
	_ = json.Unmarshal(raw, b)
	if b.Files == nil {
		b.Files = map[string]cloudBaselineFile{}
	}
	return b
}

// saveBaseline writes the baseline atomically; callers hold no lock (one-shot
// helper runs never overlap on one game: the plugin runs one cloud job at a
// time per game, and status writes nothing).
func saveBaseline(appID uint32, b *cloudBaseline) error {
	dir := cloudAppDir(appID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "baseline-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, "baseline.json"))
}

// safeJobName maps a jobId to a fixed cache file name. Ids outside the safe
// set hash to a fixed name (collisions across games would return the wrong
// result, so the hash covers the app id too).
func safeJobName(appID uint32, jobID string) string {
	safe := len(jobID) > 0 && len(jobID) <= 64
	for _, r := range jobID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			safe = false
			break
		}
	}
	if safe {
		return jobID
	}
	sum := sha1.Sum([]byte(strconv.FormatUint(uint64(appID), 10) + "\x00" + jobID))
	return "h-" + hex.EncodeToString(sum[:])
}

func jobResultPath(appID uint32, jobID string) string {
	return filepath.Join(cloudJobsDir(), safeJobName(appID, jobID)+".json")
}

func loadJobResult(appID uint32, jobID string) (json.RawMessage, bool) {
	raw, err := os.ReadFile(jobResultPath(appID, jobID))
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return json.RawMessage(raw), true
}

func saveJobResult(appID uint32, jobID string, res json.RawMessage) {
	if jobID == "" {
		return
	}
	_ = os.MkdirAll(cloudJobsDir(), 0o700)
	tmp, err := os.CreateTemp(cloudJobsDir(), "job-*.json")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(res); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return
	}
	_ = os.Rename(tmpName, jobResultPath(appID, jobID))
}

// --- compare ----------------------------------------------------------------

type cloudLocal struct {
	Size  int64
	MTime int64
	SHA   [20]byte
	Found bool
}

func statLocal(path string) cloudLocal {
	var out cloudLocal
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		return out
	}
	h := sha1.New()
	n, err := io.Copy(h, io.LimitReader(f, 1<<31))
	if err != nil {
		return out
	}
	out.Found = true
	out.Size = n
	out.MTime = st.ModTime().Unix()
	copy(out.SHA[:], h.Sum(nil))
	return out
}

type cloudPlanFile struct {
	Rel       string
	CloudName string
	Local     string
	Remote    cloudFile
	HasRemote bool
	LocalInfo cloudLocal
	Want      string // same|download|upload|conflict
}

// compareCloud groups the listing by resolved local path (issue #6 L2: one
// file synced from two machines compares once — the latest entry wins) and
// decides per file against the baseline (D6). A baseline entry counts only
// for the local path it was recorded for, so a native<->Proton switch
// compares fresh instead of misreading the other side. Local-only files
// matching the rules are upload candidates; cloud names nobody maps are
// unmapped.
func compareCloud(list []cloudFile, local map[string]string, ufs *cloudUFS, ctx cloudContext, steamID uint64, base *cloudBaseline) (files []cloudPlanFile, unmapped int) {
	byLocal := map[string]*cloudFile{}
	ordered := []string{}
	for i := range list {
		f := &list[i]
		if _, _, ok := splitCloudName(f.CloudName); !ok {
			unmapped++
			continue
		}
		localPath, rel, ok := mapCloudName(f.CloudName, ufs, ctx, steamID)
		if !ok {
			unmapped++
			continue
		}
		f.Local, f.Rel = localPath, rel
		if prev, dup := byLocal[localPath]; dup {
			if f.MTime > prev.MTime {
				*prev = *f
			}
			continue
		}
		cp := *f
		byLocal[localPath] = &cp
		ordered = append(ordered, localPath)
	}
	seen := map[string]bool{}
	for _, lp := range ordered {
		f := byLocal[lp]
		seen[lp] = true
		pf := cloudPlanFile{Rel: f.Rel, CloudName: f.CloudName, Local: lp, Remote: *f, HasRemote: true}
		li := statLocal(lp)
		pf.LocalInfo = li
		remoteSHA := hex.EncodeToString(f.SHA)
		b, known := base.Files[f.Rel]
		known = known && b.Local == lp
		switch {
		case f.Persist == 2 && !li.Found:
			pf.Want = "same" // deleted in the cloud, already gone here
		case f.Persist == 2:
			pf.Want = "conflict" // deleted in the cloud, still here: ask
		case li.Found && hex.EncodeToString(li.SHA[:]) == remoteSHA:
			pf.Want = "same"
		case !li.Found:
			pf.Want = "download" // nothing local to lose; cloud deletes are forbidden
		case !known:
			pf.Want = "conflict" // no common ancestor: ask
		case b.SHA == hex.EncodeToString(li.SHA[:]):
			pf.Want = "download"
		case b.SHA == remoteSHA:
			pf.Want = "upload"
		default:
			pf.Want = "conflict"
		}
		if ch, ok := base.Choices[f.Rel]; ok && pf.Want == "conflict" &&
			ch.Side == "local" && ch.LocalSHA == hex.EncodeToString(li.SHA[:]) && ch.CloudSHA == remoteSHA {
			pf.Want = "upload" // the user already kept this device's copy
		}
		files = append(files, pf)
	}
	// Local files the cloud never listed: upload candidates when nothing is
	// recorded, conflicts when the baseline proves the cloud deleted them.
	for lp, rel := range local {
		if seen[lp] {
			continue
		}
		pf := cloudPlanFile{Rel: rel, Local: lp, HasRemote: false}
		pf.LocalInfo = statLocal(lp)
		if !pf.LocalInfo.Found {
			continue
		}
		if b, known := base.Files[rel]; known && b.Local == lp && b.SHA != "" {
			pf.Want = "conflict" // was synced, now gone from the cloud: ask
		} else {
			pf.Want = "upload"
		}
		files = append(files, pf)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, unmapped
}

// noteSame records every file identical on both sides in the baseline, so a
// later change on one side compares as download/upload instead of conflict.
// Callers save through saveIfDirty; status included (it writes only the
// plugin's private baseline, never save files).
func (e *cloudEnv) noteSame(files []cloudPlanFile) {
	for _, f := range files {
		if f.Want != "same" || !f.HasRemote || f.Remote.Persist == 2 {
			continue
		}
		if b, ok := e.base.Files[f.Rel]; ok && b.Local == f.Local && b.SHA == hex.EncodeToString(f.Remote.SHA) {
			continue
		}
		e.base.Files[f.Rel] = cloudBaselineFile{SHA: hex.EncodeToString(f.Remote.SHA), Size: f.Remote.Size, Local: f.Local}
		e.dirty = true
	}
}

// saveIfDirty writes the baseline when this run learned something (same
// files recorded, downloads written, choices kept). One write per run.
func (e *cloudEnv) saveIfDirty() error {
	if !e.dirty {
		return nil
	}
	e.dirty = false
	return saveBaseline(e.appID, e.base)
}

// --- CloudStatus ------------------------------------------------------------

type cloudStatusFile struct {
	Size    int64  `json:"size"`
	MTime   int64  `json:"mtime"`
	Machine string `json:"machine,omitempty"`
}

type cloudStatusConflict struct {
	Path   string          `json:"path"`
	Local  cloudStatusFile `json:"local"`
	Remote cloudStatusFile `json:"remote"`
}

type cloudStatusCounts struct {
	Same     int `json:"same"`
	Download int `json:"download"`
	Upload   int `json:"upload"`
	Conflict int `json:"conflict"`
	Unmapped int `json:"unmapped"`
}

type cloudStatusClient struct {
	Running   bool   `json:"running"`
	Name      string `json:"name"`
	CanLaunch bool   `json:"canLaunch"`
}

// buildStatus answers the CloudStatus for a comparison. Only the protocol's
// keys are carried (plugtest rejects extras).
func buildStatus(appID uint32, files []cloudPlanFile, unmapped int, message string) map[string]any {
	counts := cloudStatusCounts{Unmapped: unmapped}
	var conflicts []cloudStatusConflict
	for _, f := range files {
		switch f.Want {
		case "same":
			counts.Same++
		case "download":
			counts.Download++
		case "upload":
			counts.Upload++
		case "conflict":
			counts.Conflict++
			remote := cloudStatusFile{Size: int64(f.Remote.Size), MTime: f.Remote.MTime, Machine: f.Remote.Machine}
			local := cloudStatusFile{}
			if f.LocalInfo.Found {
				local = cloudStatusFile{Size: f.LocalInfo.Size, MTime: f.LocalInfo.MTime}
			}
			conflicts = append(conflicts, cloudStatusConflict{Path: f.Rel, Local: local, Remote: remote})
		}
	}
	state := "synced"
	switch {
	case counts.Conflict > 0:
		state = "conflict"
	case counts.Download > 0 || counts.Upload > 0:
		state = "pending"
	}
	st := map[string]any{
		"state": state, "message": message,
		"counts":    counts,
		"conflicts": conflicts,
		"checkedAt": time.Now().Unix(),
	}
	if conflicts == nil {
		st["conflicts"] = []cloudStatusConflict{}
	}
	if dir := latestBackupDir(appID); dir != "" {
		st["backupDir"] = dir
	}
	if cl := detectSteamClient(); cl != nil {
		st["client"] = cl
	}
	return st
}

// detectSteamClient reports the official client when its pid file names a
// live steam process (plan D7): Platypus then doesn't sync that game itself.
func detectSteamClient() *cloudStatusClient {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}
	for _, dir := range []string{
		filepath.Join(home, ".steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam"),
	} {
		raw, err := os.ReadFile(filepath.Join(dir, "steam.pid"))
		if err != nil {
			continue
		}
		pid := strings.TrimSpace(string(raw))
		if pid == "" {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) != "steam" {
			continue
		}
		return &cloudStatusClient{Running: true, Name: "Steam", CanLaunch: true}
	}
	return nil
}

// --- pull -------------------------------------------------------------------

// downloadInfo is a ClientFileDownload#1 answer: where the bytes are and how
// to check them.
type downloadInfo struct {
	RawSize   uint32
	SHA       []byte
	Time      int64
	Deleted   bool
	Host      string
	Path      string
	HTTPS     bool
	Headers   map[string]string
	Encrypted bool
}

func clientFileDownload(cm *steamcm.Session, appID uint32, cloudName string, timeout time.Duration) (*downloadInfo, error) {
	req := &depot.Enc{}
	req.Varint(1, uint64(appID))
	req.Str(2, cloudName)
	raw, err := cm.Call("Cloud.ClientFileDownload#1", req.Bytes(), timeout)
	if err != nil {
		return nil, err
	}
	m, err := depot.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding the cloud download: %w", err)
	}
	out := &downloadInfo{
		RawSize: uint32(m.Int(3)),
		SHA:     m.Bytes(4), Time: int64(m.Int(5)),
		Deleted: m.Int(6) != 0,
		Host:    string(m.Bytes(7)), Path: string(m.Bytes(8)),
		HTTPS:     m.Int(9) != 0,
		Headers:   map[string]string{},
		Encrypted: m.Int(11) != 0,
	}
	for _, hb := range m.AllBytes(10) {
		hm, err := depot.Decode(hb)
		if err != nil {
			continue
		}
		out.Headers[string(hm.Bytes(1))] = string(hm.Bytes(2))
	}
	return out, nil
}

// maxCloudFile caps one download (Valve's own per-file soft limit is 100 MB
// per write call, 256 MB per file; live saves are KBs).
const maxCloudFile = 256 << 20

// fetchCloudBytes GETs the file body and returns the raw bytes: a body whose
// SHA-1 already matches the listing is used as is, otherwise it is tried as
// a single-entry zip (stored or deflate) and SHA-1 verified after unpacking.
// A mismatch fails before anything is written.
func fetchCloudBytes(ctx context.Context, dl *downloadInfo) ([]byte, error) {
	if dl.Encrypted {
		return nil, fmt.Errorf("encrypted cloud file: refusing")
	}
	scheme := "http"
	if dl.HTTPS {
		scheme = "https"
	}
	if dl.Host == "" || dl.Path == "" {
		return nil, fmt.Errorf("the cloud gave no download address")
	}
	url := scheme + "://" + dl.Host + dl.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range dl.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading the cloud file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("downloading the cloud file: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCloudFile+1))
	if err != nil {
		return nil, fmt.Errorf("reading the cloud file: %w", err)
	}
	if len(body) > maxCloudFile {
		return nil, fmt.Errorf("the cloud file is bigger than %d bytes", maxCloudFile)
	}
	if sum := sha1.Sum(body); bytes.Equal(sum[:], dl.SHA) {
		return body, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("the cloud file failed its SHA-1 check: nothing was written")
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("the cloud archive holds %d files, want 1", len(zr.File))
	}
	zf := zr.File[0]
	if zf.FileInfo().IsDir() || zf.UncompressedSize64 > maxCloudFile {
		return nil, fmt.Errorf("the cloud archive entry is not a plain file")
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("unpacking the cloud file: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(rc, maxCloudFile+1))
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("unpacking the cloud file: %w", err)
	}
	if len(raw) > maxCloudFile {
		return nil, fmt.Errorf("the cloud file is bigger than %d bytes", maxCloudFile)
	}
	if sum := sha1.Sum(raw); !bytes.Equal(sum[:], dl.SHA) {
		return nil, fmt.Errorf("the cloud file failed its SHA-1 check: nothing was written")
	}
	return raw, nil
}

// backupRootFor names the run's own backup folder (timestamp + job id), so
// one pull or resolve leaves one folder behind instead of one per file.
func backupRootFor(appID uint32, jobID string) string {
	return filepath.Join(dataDir(), "cloud-backups", strconv.FormatUint(uint64(appID), 10),
		time.Now().UTC().Format("20060102-150405")+"-"+safeJobName(appID, jobID))
}

// backupLocal copies the local file into the run's backup folder, preserving
// its rel path (so a platform switch stays told apart) and its mtime.
func (e *cloudEnv) backupLocal(local, rel string) error {
	if e.backupRoot == "" {
		e.backupRoot = backupRootFor(e.appID, e.jobID)
	}
	dst := filepath.Join(e.backupRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	if err := copyFile(dst, local); err != nil {
		return err
	}
	e.backedUp = true
	return nil
}

// writeCloudFile backs up a replaced local file, then writes the download
// atomically and stamps the cloud mtime.
func (e *cloudEnv) writeCloudFile(local, rel string, raw []byte, mtime int64) (backedUp bool, err error) {
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		if err := e.backupLocal(local, rel); err != nil {
			return false, err
		}
		backedUp = true
	}
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return backedUp, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(local), ".platypus-cloud-*")
	if err != nil {
		return backedUp, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return backedUp, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return backedUp, err
	}
	// Saves are the user's own data; a fresh file must not leak through a
	// shared umask (0644 &^ umask still group-reads on most systems).
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return backedUp, err
	}
	if err := os.Rename(tmpName, local); err != nil {
		os.Remove(tmpName)
		return backedUp, err
	}
	ts := time.Unix(mtime, 0)
	_ = os.Chtimes(local, ts, ts)
	return backedUp, nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	ts := st.ModTime()
	_ = os.Chtimes(dst, ts, ts)
	return nil
}

// latestBackupDir reports the app's backup dir once it holds a backup, so
// CloudStatus carries backupDir only when there is something to show.
func latestBackupDir(appID uint32) string {
	root := filepath.Join(dataDir(), "cloud-backups", strconv.FormatUint(uint64(appID), 10))
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return root
}

// pruneBackups keeps the last keep timestamped backups per game (the user's
// decision Q-I); the newest (currently written) dir is never pruned.
func pruneBackups(appID uint32, keep int) {
	root := filepath.Join(dataDir(), "cloud-backups", strconv.FormatUint(uint64(appID), 10))
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) <= keep {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names[:len(names)-keep] {
		os.RemoveAll(filepath.Join(root, n))
	}
}

// --- ops --------------------------------------------------------------------

type cloudEnv struct {
	cm      *steamcm.Session
	appID   uint32
	jobID   string
	steamID uint64
	ctx     cloudContext
	ufs     *cloudUFS
	base    *cloudBaseline
	timeout time.Duration
	// dirty marks an updated baseline the run must save (every exit path
	// saves once through saveIfDirty). backupRoot is the run's own backup
	// folder, created on first use.
	dirty      bool
	backupRoot string
	backedUp   bool
}

// prepareCloud logs on, reads appinfo ufs and validates the context. Status
// writes no save file; pull/resolve record the baseline they update.
func prepareCloud(ctx context.Context, req request, op string, rawCtx json.RawMessage) (*cloudEnv, error) {
	if req.AppID == 0 {
		return nil, fmt.Errorf("app_id is missing")
	}
	var cc cloudContext
	hasCtx := len(rawCtx) > 0 && string(rawCtx) != "null"
	if hasCtx {
		if err := json.Unmarshal(rawCtx, &cc); err != nil {
			return nil, fmt.Errorf("bad ctx: %w", err)
		}
	}
	if op != "status" && req.JobID == "" {
		return nil, fmt.Errorf("job_id is missing")
	}
	if err := cc.valid(); err != nil {
		return nil, fmt.Errorf("bad ctx: %w", err)
	}
	cm, err := dialCM(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	env := &cloudEnv{cm: cm, appID: req.AppID, jobID: req.JobID, ctx: cc, base: loadBaseline(req.AppID), timeout: 30 * time.Second}
	env.steamID = cm.SteamID()
	toks, err := cm.PICSAccessTokens([]uint32{req.AppID}, nil, 0)
	if err != nil {
		cm.Close()
		return nil, callErr("product info tokens", err)
	}
	kv, _, err := cm.ProductInfo(req.AppID, toks[req.AppID], 0)
	if err != nil {
		cm.Close()
		return nil, notFoundOr("product info", req.AppID, err)
	}
	env.ufs = parseCloudRules(kv)
	return env, nil
}

func (e *cloudEnv) close() {
	e.cm.Close()
}

// cloudCompare lists the cloud, scans the device and compares. The machine
// names ride the conflicts so the user sees which device holds the copy.
func (e *cloudEnv) cloudCompare(ctx context.Context) ([]cloudPlanFile, int, error) {
	list, err := fetchChangelist(e.cm, e.appID, e.timeout)
	if err != nil {
		return nil, 0, callErr("cloud file list", err)
	}
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}
	local := scanLocal(e.ufs, e.ctx, e.steamID)
	files, unmapped := compareCloud(list, local, e.ufs, e.ctx, e.steamID, e.base)
	return files, unmapped, nil
}

func unsupportedStatus(reason string, cl *cloudStatusClient) map[string]any {
	st := map[string]any{
		"state": "unsupported", "message": reason,
		"counts":    cloudStatusCounts{},
		"conflicts": []cloudStatusConflict{},
		"checkedAt": time.Now().Unix(),
	}
	if cl != nil {
		st["client"] = cl
	}
	return st
}

func offlineStatus(err error, cl *cloudStatusClient) map[string]any {
	st := map[string]any{
		"state": "offline", "message": fmt.Sprintf("Steam couldn't be reached: %v", err),
		"counts":    cloudStatusCounts{},
		"conflicts": []cloudStatusConflict{},
		"checkedAt": time.Now().Unix(),
	}
	if cl != nil {
		st["client"] = cl
	}
	return st
}

func noRulesReason(ufs *cloudUFS) string {
	if ufs.HasUFS {
		return "this game's saves go through the Steamworks API, which Platypus can't sync from outside the game"
	}
	return "this game declares no Auto-Cloud save locations"
}

// runCloudStatus compares both sides. It writes no save file and uploads
// nothing; it records agreed files in the plugin's private baseline only.
func runCloudStatus(ctx context.Context, req request, rawCtx json.RawMessage) error {
	env, err := prepareCloud(ctx, req, "status", rawCtx)
	if err != nil {
		return err
	}
	defer env.close()
	if !env.ufs.HasRules {
		emit(map[string]any{"type": "result", "status": unsupportedStatus(noRulesReason(env.ufs), detectSteamClient())})
		return nil
	}
	files, unmapped, err := env.cloudCompare(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		emit(map[string]any{"type": "result", "status": offlineStatus(err, detectSteamClient())})
		return nil
	}
	env.noteSame(files)
	if err := env.saveIfDirty(); err != nil {
		return err
	}
	emit(map[string]any{"type": "result", "status": buildStatus(req.AppID, files, unmapped, cloudMessage(files, unmapped))})
	return nil
}

func cloudMessage(files []cloudPlanFile, unmapped int) string {
	var dl, up, cf int
	for _, f := range files {
		switch f.Want {
		case "download":
			dl++
		case "upload":
			up++
		case "conflict":
			cf++
		}
	}
	var suffix string
	if unmapped > 0 {
		suffix = fmt.Sprintf(" %d cloud file(s) can't be placed on this device.", unmapped)
	}
	switch {
	case cf > 0:
		return fmt.Sprintf("%d file(s) changed on this device and in the cloud.", cf) + suffix
	case dl > 0 && up > 0:
		return fmt.Sprintf("%d file(s) waiting to download. Uploading saves isn't enabled yet.", dl) + suffix
	case dl > 0:
		return fmt.Sprintf("%d file(s) waiting to download.", dl) + suffix
	case up > 0:
		return "Uploading saves isn't enabled yet." + suffix
	}
	if suffix != "" {
		return "saves are in sync." + suffix
	}
	return "saves are in sync."
}

// reportProgress emits the cloud file progress the plugin forwards as the
// cloud.progress notification ({id, jobId, phase, done, total}).
func reportProgress(appID uint32, jobID, phase string, done, total int) {
	emit(map[string]any{"type": "progress", "phase": phase,
		"app_id": appID, "job_id": jobID, "done": done, "total": total})
}

// takeCloudDeletion backs up the local file, removes it and drops its
// baseline entry: a resolve-remote on a file the cloud deleted.
func (e *cloudEnv) takeCloudDeletion(f *cloudPlanFile) error {
	if f.LocalInfo.Found {
		if err := e.backupLocal(f.Local, f.Rel); err != nil {
			return err
		}
		if err := os.Remove(f.Local); err != nil {
			return err
		}
	}
	delete(e.base.Files, f.Rel)
	delete(e.base.Choices, f.Rel)
	e.dirty = true
	f.Want = "same"
	return nil
}

// pullFiles downloads every download-wanted file (or the remote choices),
// verifies SHA-1 before writing, backs up replaced locals, and records only
// actually written files in the baseline. A resolve-remote on a file the
// cloud deleted backs up the local file, removes it and drops its baseline
// entry instead of downloading. The baseline saves once per run through the
// caller's deferred saveIfDirty.
func (e *cloudEnv) pullFiles(ctx context.Context, req request, files []cloudPlanFile, only map[string]bool) (int, error) {
	var want []int
	for i := range files {
		if files[i].Want != "download" {
			continue
		}
		if only != nil && !only[files[i].Rel] {
			continue
		}
		want = append(want, i)
	}
	for n, i := range want {
		select {
		case <-ctx.Done():
			return n, ctx.Err()
		default:
		}
		reportProgress(req.AppID, req.JobID, "download", n, len(want))
		f := &files[i]
		if f.Remote.Persist == 2 {
			// The listing already says deleted: take the cloud deletion
			// without a download call, with a backup of what was here.
			// Only a resolve-remote reaches this path.
			if err := e.takeCloudDeletion(f); err != nil {
				return n, err
			}
			continue
		}
		dl, err := clientFileDownload(e.cm, e.appID, f.CloudName, e.timeout)
		if err != nil {
			return n, callErr("cloud download", err)
		}
		if dl.Deleted {
			// The download itself reports an explicit delete: same
			// handling as the listing flag above.
			if err := e.takeCloudDeletion(f); err != nil {
				return n, err
			}
			continue
		}
		raw, err := fetchCloudBytes(ctx, dl)
		if err != nil {
			return n, err
		}
		if _, err := e.writeCloudFile(f.Local, f.Rel, raw, dl.Time); err != nil {
			return n, err
		}
		e.base.Files[f.Rel] = cloudBaselineFile{SHA: hex.EncodeToString(dl.SHA), Size: dl.RawSize, Local: f.Local}
		delete(e.base.Choices, f.Rel)
		e.dirty = true
		f.Want = "same"
	}
	reportProgress(req.AppID, req.JobID, "download", len(want), len(want))
	return len(want), nil
}

// applyChoices validates resolve choices like the echo plugin does (a path
// nobody listed is -32602) and settles them: remote downloads with a backup,
// local records "keep this device's copy" for the later push — no upload
// happens in this task.
func (e *cloudEnv) applyChoices(files []cloudPlanFile, choices map[string]string) (remote map[string]bool, err error) {
	byRel := map[string]int{}
	for i := range files {
		if files[i].Want == "conflict" {
			byRel[files[i].Rel] = i
		}
	}
	remote = map[string]bool{}
	for path, side := range choices {
		if side != "local" && side != "remote" {
			return nil, fmt.Errorf("invalid params: choice for %q must be local or remote", path)
		}
		i, ok := byRel[path]
		if !ok {
			return nil, fmt.Errorf("invalid params: %q is not a conflict", path)
		}
		if side == "remote" {
			remote[path] = true
			files[i].Want = "download" // download with backup, then baseline
			continue
		}
		f := &files[i]
		var localSHA string
		if f.LocalInfo.Found {
			localSHA = hex.EncodeToString(f.LocalInfo.SHA[:])
		}
		if e.base.Choices == nil {
			e.base.Choices = map[string]cloudChoice{}
		}
		e.base.Choices[path] = cloudChoice{Side: "local", LocalSHA: localSHA,
			CloudSHA: hex.EncodeToString(f.Remote.SHA)}
		f.Want = "upload"
		e.dirty = true // the choice must survive through saveIfDirty
	}
	return remote, nil
}

// runCloudPull downloads what changed only in the cloud. Conflicts are never
// overwritten; resolve carries remote choices for the listed ones.
func runCloudPull(ctx context.Context, req request, rawCtx json.RawMessage, rawChoices json.RawMessage) error {
	if raw, ok := loadJobResult(req.AppID, req.JobID); ok {
		emit(map[string]any{"type": "result", "status": json.RawMessage(raw)})
		return nil
	}
	env, err := prepareCloud(ctx, req, "pull", rawCtx)
	if err != nil {
		return err
	}
	defer env.close()
	defer env.saveIfDirty() // every exit path keeps what was written (item 5)
	if !env.ufs.HasRules {
		return finishJob(req, unsupportedStatus(noRulesReason(env.ufs), detectSteamClient()))
	}
	files, unmapped, err := env.cloudCompare(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return finishJob(req, offlineStatus(err, detectSteamClient()))
	}
	env.noteSame(files)
	var only map[string]bool
	if len(rawChoices) > 0 && string(rawChoices) != "null" {
		var choices map[string]string
		if err := json.Unmarshal(rawChoices, &choices); err != nil {
			return fmt.Errorf("invalid params: choices must be an object")
		}
		only, err = env.applyChoices(files, choices)
		if err != nil {
			return err
		}
	}
	reportProgress(req.AppID, req.JobID, "check", 0, 1)
	if _, err := env.pullFiles(ctx, req, files, only); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	reportProgress(req.AppID, req.JobID, "check", 1, 1)
	if env.backedUp {
		pruneBackups(env.appID, 5)
	}
	return finishJob(req, buildStatus(req.AppID, files, unmapped, cloudMessage(files, unmapped)))
}

// runCloudPush computes the status without uploading anything: uploads wait
// for the push phase (issue #6). The answer still obeys the repeated jobId.
func runCloudPush(ctx context.Context, req request, rawCtx json.RawMessage) error {
	if raw, ok := loadJobResult(req.AppID, req.JobID); ok {
		emit(map[string]any{"type": "result", "status": json.RawMessage(raw)})
		return nil
	}
	env, err := prepareCloud(ctx, req, "push", rawCtx)
	if err != nil {
		return err
	}
	defer env.close()
	if !env.ufs.HasRules {
		return finishJob(req, unsupportedStatus(noRulesReason(env.ufs), detectSteamClient()))
	}
	files, unmapped, err := env.cloudCompare(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return finishJob(req, offlineStatus(err, detectSteamClient()))
	}
	return finishJob(req, buildStatus(req.AppID, files, unmapped, cloudMessage(files, unmapped)))
}

// parseChoices reads resolve choices before any CM work: shapes are refused
// without a listing, paths once the comparison exists.
func parseChoices(rawChoices json.RawMessage) (map[string]string, error) {
	var choices map[string]string
	if len(rawChoices) == 0 || string(rawChoices) == "null" {
		return nil, fmt.Errorf("invalid params: choices are required")
	}
	if err := json.Unmarshal(rawChoices, &choices); err != nil {
		return nil, fmt.Errorf("invalid params: choices must be an object")
	}
	for path, side := range choices {
		if side != "local" && side != "remote" {
			return nil, fmt.Errorf("invalid params: choice for %q must be local or remote", path)
		}
	}
	return choices, nil
}

// runCloudResolve settles the listed conflicts: remote downloads with a
// backup, local keeps this device's copy for the later push.
func runCloudResolve(ctx context.Context, req request, rawCtx json.RawMessage, rawChoices json.RawMessage) error {
	if raw, ok := loadJobResult(req.AppID, req.JobID); ok {
		emit(map[string]any{"type": "result", "status": json.RawMessage(raw)})
		return nil
	}
	choices, err := parseChoices(rawChoices)
	if err != nil {
		return err
	}
	env, err := prepareCloud(ctx, req, "resolve", rawCtx)
	if err != nil {
		return err
	}
	defer env.close()
	defer env.saveIfDirty() // every exit path keeps choices and downloads
	if !env.ufs.HasRules {
		// No rules means no conflicts: any choice path is refused here,
		// before the unsupported answer. Offline stays offline below.
		if len(choices) > 0 {
			paths := make([]string, 0, len(choices))
			for path := range choices {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			return fmt.Errorf("invalid params: %q is not a conflict", paths[0])
		}
		return finishJob(req, unsupportedStatus(noRulesReason(env.ufs), detectSteamClient()))
	}
	files, unmapped, err := env.cloudCompare(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return finishJob(req, offlineStatus(err, detectSteamClient()))
	}
	env.noteSame(files)
	only, err := env.applyChoices(files, choices)
	if err != nil {
		return err
	}
	reportProgress(req.AppID, req.JobID, "check", 0, 1)
	if _, err := env.pullFiles(ctx, req, files, only); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	reportProgress(req.AppID, req.JobID, "check", 1, 1)
	if env.backedUp {
		pruneBackups(env.appID, 5)
	}
	return finishJob(req, buildStatus(req.AppID, files, unmapped, cloudMessage(files, unmapped)))
}

// finishJob emits the result and records it for the repeated jobId.
func finishJob(req request, st map[string]any) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	saveJobResult(req.AppID, req.JobID, json.RawMessage(raw))
	emit(map[string]any{"type": "result", "status": st})
	return nil
}

// runCloud runs the cloud subcommand: op status|pull|push|resolve.
func runCloud(ctx context.Context, req request, op string, rawCtx, rawChoices json.RawMessage) error {
	switch op {
	case "status":
		return runCloudStatus(ctx, req, rawCtx)
	case "pull":
		return runCloudPull(ctx, req, rawCtx, rawChoices)
	case "push":
		return runCloudPush(ctx, req, rawCtx)
	case "resolve":
		return runCloudResolve(ctx, req, rawCtx, rawChoices)
	default:
		return fmt.Errorf("unknown cloud op %q", op)
	}
}
