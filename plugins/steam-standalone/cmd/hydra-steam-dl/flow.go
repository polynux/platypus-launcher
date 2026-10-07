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
	ref  DepotRef
	man  *depot.Manifest
	raw  []byte
	key  []byte
	code uint64
}

type session struct {
	cm      *steamcm.Session
	dataDir string
	req     request
	hosts   []depot.Host
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
	refs, osUsed, err := selectDepots(kv, branch, []string{"linux", "windows"})
	if err != nil {
		return nil, "", err
	}
	// Resolve shared depots against their source app's info.
	for i, r := range refs {
		if r.FromApp == 0 {
			continue
		}
		stoks, err := s.cm.PICSAccessTokens([]uint32{r.FromApp}, nil, 0)
		if err != nil {
			return nil, "", callErr("shared depot tokens", err)
		}
		skv, _, err := s.cm.ProductInfo(r.FromApp, stoks[r.FromApp], 0)
		if err != nil {
			return nil, "", callErr("shared depot info", err)
		}
		srefs, _, err := selectDepots(skv, branch, []string{"linux", "windows"})
		if err != nil {
			return nil, "", err
		}
		found := false
		for _, sr := range srefs {
			if sr.DepotID == r.DepotID {
				refs[i] = sr
				found = true
			}
		}
		if !found {
			return nil, "", fmt.Errorf("shared depot %d has no manifest", r.DepotID)
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
	client := depot.NewClient()
	var out []fetchedDepot
	for _, r := range refs {
		key, err := s.cm.DepotKey(r.DepotID, appID, 0)
		if err != nil {
			return nil, "", callErr(fmt.Sprintf("depot %d key", r.DepotID), err)
		}
		code, err := s.cm.ManifestRequestCode(appID, r.DepotID, r.GID, r.Branch, "", 30*time.Second)
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
	withTokens(ctx, s, appID, out)
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

func withTokens(ctx context.Context, s *session, appID uint32, depots []fetchedDepot) {
	for i, h := range s.hosts {
		// SteamCache hosts serve without tokens; ask for the rest once
		// per host using the first depot (tokens are per depot, but one
		// working token per host is enough to try).
		if len(depots) == 0 {
			return
		}
		tok, _, err := s.cm.CDNAuthToken(depots[0].ref.DepotID, h.Name, appID, 15*time.Second)
		if err != nil || tok == "" {
			continue
		}
		s.hosts[i].Token = tok
	}
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
