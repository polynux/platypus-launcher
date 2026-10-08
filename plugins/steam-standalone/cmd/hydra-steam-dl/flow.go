// Shared CM + CDN flow for the subcommands: log on, pick depots from
// PICS, fetch manifests, and download with the multi-host scheduler.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"hydra-launcher/internal/depot"
	"hydra-launcher/internal/steamcm"
)

type fetchedDepot struct {
	ref   DepotRef
	man   *depot.Manifest
	raw   []byte
	key   []byte
	code  uint64
	hosts []depot.Host
}

type session struct {
	cm      *steamcm.Session
	dataDir string
	req     request
	hosts   []depot.Host
	tokens  map[tokenKey]string
}

func dataDir() string {
	if d := os.Getenv("HYDRA_PLUGIN_DATA"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hydra-steam-standalone")
}

// cellID mirrors the plugin's download region: HYDRA_STEAM_CELLID wins,
// else the Steam client's CellIDServerOverride, else 0 (CM decides).
func cellID() uint32 {
	if env := strings.TrimSpace(os.Getenv("HYDRA_STEAM_CELLID")); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			return uint32(n)
		}
	}
	home, _ := os.UserHomeDir()
	re := regexp.MustCompile(`"CellIDServerOverride"\s+"(\d+)"`)
	for _, rel := range []string{".local/share/Steam/config/config.vdf", ".steam/steam/config/config.vdf"} {
		raw, err := os.ReadFile(filepath.Join(home, rel))
		if err != nil {
			continue
		}
		if m := re.FindSubmatch(raw); m != nil {
			if n, err := strconv.Atoi(string(m[1])); err == nil && n > 0 {
				return uint32(n)
			}
		}
	}
	return 0
}

func dialCM(ctx context.Context, token string) (*steamcm.Session, error) {
	s, err := steamcm.Dial(ctx, token)
	if err != nil {
		if ce, ok := err.(*steamcm.CMError); ok {
			return nil, cmErr{msg: ce.Error(), auth: ce.Auth}
		}
		return nil, err
	}
	return s, nil
}

func callErr(op string, err error) error {
	if ce, ok := err.(*steamcm.CMError); ok {
		return cmErr{msg: fmt.Sprintf("%s: %s", op, ce.Error()), auth: ce.Auth}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// fetchDepots resolves the app's depots for the branch and downloads every
// manifest: PICS tokens, product info (linux first, windows fallback),
// depot keys, server list, request codes and manifest bodies.
func fetchDepots(ctx context.Context, s *session) ([]fetchedDepot, string, error) {
	branch := s.req.Branch
	if branch == "" {
		branch = "public"
	}
	appID := s.req.AppID
	toks, err := s.cm.PICSAccessTokens([]uint32{appID}, nil, 0)
	if err != nil {
		return nil, "", callErr("product info tokens", err)
	}
	kv, _, err := s.cm.ProductInfo(appID, toks[appID], 0)
	if err != nil {
		return nil, "", callErr("product info", err)
	}
	// Linux depots first, Windows fallback — the platform used decides
	// `native`, like the old `-os linux` then `-os windows` runs.
	var refs []DepotRef
	var osUsed string
	for _, osname := range []string{"linux", "windows"} {
		if r, err := selectDepots(kv, appID, branch, osname); err == nil {
			refs, osUsed = r, osname
			break
		}
	}
	if len(refs) == 0 {
		return nil, "", fmt.Errorf("no depots for this platform")
	}
	// Resolve shared depots: the manifest gid comes straight from the
	// source app's entry (DepotDownloader recurses the same way), and
	// the depot key is keyed to the source app unless the game itself
	// is free to download.
	free := kv.Get("common") != nil && kv.Get("common").String("freetodownload") == "1"
	for i, r := range refs {
		if r.FromApp == 0 {
			refs[i].KeyAppID = appID
			continue
		}
		gid, err := sharedManifestGID(ctx, s, r.DepotID, r.FromApp, branch, 0)
		if err != nil {
			return nil, "", err
		}
		refs[i].GID = gid
		if free {
			refs[i].KeyAppID = appID
		} else {
			refs[i].KeyAppID = r.FromApp
		}
	}
	servers, err := s.cm.ServersForSteamPipe(cellID(), 32, 0)
	if err != nil {
		return nil, "", callErr("server list", err)
	}
	hosts := depotHosts(servers, appID)
	if len(hosts) == 0 {
		return nil, "", fmt.Errorf("Steam returned no download hosts for this app")
	}
	s.hosts = hosts
	// Branch passwords (STEAM-4): check once against the app, then send
	// SHA1(password) with every request-code call. Wrong passwords fail
	// here with a distinct message instead of "no depots".
	pwHash := branchPasswordHash(s.req)
	if pwHash != "" {
		if err := s.cm.CheckBetaPassword(appID, s.req.BranchPassword, 30*time.Second); err != nil {
			return nil, "", fmt.Errorf("wrong branch password for %q: %w", branch, err)
		}
	}
	client := depot.NewClient()
	var out []fetchedDepot
	for _, r := range refs {
		keyApp := r.KeyAppID
		if keyApp == 0 {
			keyApp = appID
		}
		key, err := s.cm.DepotKey(r.DepotID, keyApp, 0)
		if err != nil {
			return nil, "", callErr(fmt.Sprintf("depot %d key", r.DepotID), err)
		}
		code, err := s.cm.ManifestRequestCode(keyApp, r.DepotID, r.GID, r.Branch, pwHash, 30*time.Second)
		if err != nil {
			return nil, "", callErr(fmt.Sprintf("depot %d request code", r.DepotID), err)
		}
		raw, err := fetchManifest(ctx, client, hosts, r.DepotID, r.GID, code)
		if err != nil {
			return nil, "", err
		}
		man, err := depot.ParseManifest(raw)
		if err != nil {
			return nil, "", fmt.Errorf("depot %d manifest didn't parse: %w", r.DepotID, err)
		}
		if man.Meta.FilenamesEncrypted {
			if err := man.DecryptFilenames(key); err != nil {
				return nil, "", fmt.Errorf("depot %d filenames: %w", r.DepotID, err)
			}
		}
		out = append(out, fetchedDepot{ref: r, man: man, raw: raw, key: key, code: code})
	}
	// CDN tokens for hosts that need them (third-party CDNs); the fake
	// CDN ignores the query, real ones may require it.
	withTokens(ctx, s, out)
	return out, osUsed, nil
}

// depotHosts keeps SteamCache and CDN types, honours allowed_app_ids, and
// keeps Steam's order (penalty, then weighted load). The VHost carries the
// download hostname when present, else the host.
func depotHosts(servers []steamcm.Server, appID uint32) []depot.Host {
	var out []depot.Host
	for _, sv := range servers {
		if sv.Type != "SteamCache" && sv.Type != "CDN" {
			continue
		}
		if len(sv.AllowedAppIDs) > 0 {
			ok := false
			for _, a := range sv.AllowedAppIDs {
				if a == appID {
					ok = true
				}
			}
			if !ok {
				continue
			}
		}
		name := sv.Host
		if sv.VHost != "" {
			name = sv.VHost
		}
		scheme := "http"
		if sv.UseHTTPS {
			scheme = "https"
		}
		out = append(out, depot.Host{Name: name, BaseURL: scheme + "://" + name})
	}
	return out
}

func withTokens(ctx context.Context, s *session, depots []fetchedDepot) {
	for i := range depots {
		depots[i].hosts = s.hostsForDepot(ctx, depots[i].ref)
	}
}

// hostsForDepot copies the server list for one depot, attaching that
// depot's CDN auth tokens (tokens are per depot and host). Results are
// cached on the session so one job asks once per pair.
func (s *session) hostsForDepot(ctx context.Context, r DepotRef) []depot.Host {
	hosts := append([]depot.Host{}, s.hosts...)
	keyApp := r.KeyAppID
	if keyApp == 0 {
		keyApp = s.req.AppID
	}
	for i, h := range hosts {
		ck := tokenKey{host: h.Name, depot: r.DepotID}
		if tok, ok := s.tokens[ck]; ok {
			hosts[i].Token = tok
			continue
		}
		if s.tokens == nil {
			s.tokens = map[tokenKey]string{}
		}
		tok, _, err := s.cm.CDNAuthToken(r.DepotID, h.Name, keyApp, 15*time.Second)
		if err != nil || tok == "" {
			continue
		}
		s.tokens[ck] = tok
		hosts[i].Token = tok
	}
	return hosts
}

type tokenKey struct {
	host  string
	depot uint32
}

// sharedManifestGID resolves a shared depot's manifest gid straight from
// the source app's entry, recursing like DepotDownloader when the source
// entry itself proxies elsewhere. Depths stay tiny; self-references stop.
func sharedManifestGID(ctx context.Context, s *session, depotID, appID uint32, branch string, depth int) (uint64, error) {
	if depth > 3 {
		return 0, fmt.Errorf("shared depot %d resolution too deep", depotID)
	}
	toks, err := s.cm.PICSAccessTokens([]uint32{appID}, nil, 0)
	if err != nil {
		return 0, callErr("shared depot tokens", err)
	}
	kv, _, err := s.cm.ProductInfo(appID, toks[appID], 0)
	if err != nil {
		return 0, callErr("shared depot info", err)
	}
	entry := kv.Get("depots", fmt.Sprint(depotID))
	if entry == nil {
		return 0, fmt.Errorf("shared depot %d has no entry in app %d", depotID, appID)
	}
	if gidStr := entry.String("manifests", branch, "gid"); gidStr != "" {
		var gid uint64
		fmt.Sscanf(gidStr, "%d", &gid)
		if gid != 0 {
			return gid, nil
		}
	}
	if from := entry.String("depotfromapp"); from != "" {
		var src uint32
		fmt.Sscanf(from, "%d", &src)
		if src != 0 && src != appID {
			return sharedManifestGID(ctx, s, depotID, src, branch, depth+1)
		}
	}
	return 0, fmt.Errorf("shared depot %d has no manifest", depotID)
}

func fetchManifest(ctx context.Context, client *depot.Client, hosts []depot.Host, depotID uint32, gid, code uint64) ([]byte, error) {
	var last error
	for _, h := range hosts {
		raw, err := client.DownloadManifest(ctx, h, depotID, gid, code)
		if err != nil {
			last = err
			continue
		}
		return raw, nil
	}
	if last == nil {
		last = fmt.Errorf("no hosts")
	}
	return nil, fmt.Errorf("manifest download failed: %w", last)
}

func progress(total uint64) (func(depot.Progress), *uint64, *uint64) {
	var done, peak uint64
	return func(p depot.Progress) {
		done = p.DoneBytes
		emit(map[string]any{"type": "progress", "bytes": p.DoneBytes, "total": total,
			"speed": p.BytesPerSec, "connections": p.Connections, "hosts": p.Hosts})
	}, &done, &peak
}

func hexChunk(id []byte) string { return hex.EncodeToString(id) }
