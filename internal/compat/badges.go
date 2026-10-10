package compat

import (
	"fmt"
	"time"

	"hydra-launcher/internal/types"
)

// Badges say only what a source said. An unknown fact gets no badge, and a
// guess (Denuvo, say) is labelled as one.
//
// TODO(compat-drm): PCGamingWiki is not used yet. Its DRM taxonomy is the
// best per-store source, but its content is CC BY-NC-SA and the Cargo API is
// closed to anonymous callers. Pending the user's decision on the licence and
// the product model; until then there is no PCGW provider here.

// launcherNames are the display names of the store launchers a plugin may
// name in drm.launcher. Any other value is shown as the plugin wrote it.
var launcherNames = map[string]string{
	"ubisoft-connect": "Ubisoft Connect",
	"ea-app":          "EA app",
	"origin":          "EA app",
	"steam":           "Steam",
	"gog-galaxy":      "GOG Galaxy",
}

// maxLauncher caps a launcher name a plugin sends.
const maxLauncher = 60

// StoreDRM turns a store's protection facts into badges. source names the
// store, checked is when Hydra read the facts.
func StoreDRM(d *types.DRM, source string, checked time.Time) []types.CompatBadge {
	if d == nil {
		return nil
	}
	out := []types.CompatBadge{}
	add := func(kind, label, detail string) {
		out = append(out, types.CompatBadge{Kind: kind, Label: label, Detail: detail, Source: source, CheckedAt: checked.Unix()})
	}
	if d.DRMFree != nil && *d.DRMFree {
		add("drm-free", "DRM-free", "The store sells it without DRM.")
	}
	if d.OnlineRequired != nil && *d.OnlineRequired {
		add("online", "Online required", "The store says the game can't run offline.")
	}
	if l := launcherName(d.Launcher); l != "" {
		add("launcher", "Needs "+l, fmt.Sprintf("The store says the game runs through %s.", l))
	}
	if d.DenuvoLikely != nil && *d.DenuvoLikely {
		add("denuvo", "Denuvo likely", "Inferred from the store's ownership check, not confirmed.")
	}
	return out
}

func launcherName(raw string) string {
	if n, ok := launcherNames[raw]; ok {
		return n
	}
	if len(raw) > maxLauncher {
		raw = raw[:maxLauncher]
	}
	return raw
}

// linuxStatus words an AreWeAntiCheatYet status for Linux and Proton.
var linuxStatus = map[string]string{
	"Supported": "supported",
	"Running":   "runs with caveats",
	"Broken":    "broken",
	"Planned":   "planned by the developer",
	"Denied":    "denied by the developer",
}

// AntiCheatBadges are the anti-cheat systems of an AreWeAntiCheatYet entry,
// each with its Linux status.
func AntiCheatBadges(g AntiCheatGame, checked time.Time) []types.CompatBadge {
	status, ok := linuxStatus[g.Status]
	if !ok {
		status = "unknown"
	}
	out := []types.CompatBadge{}
	for _, name := range g.AntiCheats {
		out = append(out, types.CompatBadge{
			Kind:      "anticheat",
			Label:     name,
			Detail:    fmt.Sprintf("Linux: %s. Last changed in the catalogue %s.", status, shortDate(g.DateChanged)),
			Source:    AWACYAttribution,
			CheckedAt: checked.Unix(),
		})
	}
	return out
}

// shortDate keeps the day of an ISO timestamp ("2024-01-19T04:40:56.000Z").
func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return "unknown"
}

// LocalBadges are the markers found in an installed folder. A marker says the
// system is present, not that it is enforced.
func LocalBadges(hits []LocalHit, checked time.Time) []types.CompatBadge {
	out := []types.CompatBadge{}
	for _, h := range hits {
		detail := fmt.Sprintf("Found %s in the install folder. It shows the system is installed, not that it is enforced.", h.Name)
		if h.Kind == "eos" {
			detail = fmt.Sprintf("Found %s in the install folder. Epic Online Services is a cross-play service, not DRM.", h.Name)
		}
		out = append(out, types.CompatBadge{
			Kind: h.Kind, Label: h.Label, Detail: detail, Source: "Installed files", CheckedAt: checked.Unix(),
		})
	}
	return out
}
