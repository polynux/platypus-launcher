package plugins

import (
	"sync"

	"hydra-launcher/internal/types"
)

// Catalog is the static list the Plugins view shows until store plugins are
// wired to the UI (milestone 3). Toggles are remembered for the session only.
type Catalog struct {
	mu      sync.RWMutex
	plugins []types.PluginInfo
}

func NewCatalog() *Catalog {
	return &Catalog{
		plugins: []types.PluginInfo{
			{
				ID:          "store-steam",
				Name:        "Steam Library Integrator",
				Author:      "Hydra Team",
				Version:     "1.0.0",
				Description: "Automatically parses local Steam libraries (libraryfolders.vdf & appmanifest_*.acf)",
				Enabled:     true,
				Store:       "steam",
			},
			{
				ID:          "store-epic",
				Name:        "Epic Games Connector (Legendary)",
				Author:      "Hydra Team",
				Version:     "0.9.2",
				Description: "Integrates with Legendary CLI to sync Epic Games Store library and launch via Proton",
				Enabled:     true,
				Store:       "epic",
			},
			{
				ID:          "store-gog",
				Name:        "GOG Galaxy Connector (Gogdl)",
				Author:      "Hydra Team",
				Version:     "0.8.5",
				Description: "Syncs DRM-free GOG library, cloud saves and automatic Linux Wine wrappers",
				Enabled:     true,
				Store:       "gog",
			},
			{
				ID:          "store-ubisoft",
				Name:        "Ubisoft Connect Bridge",
				Author:      "Hydra Team",
				Version:     "0.5.1",
				Description: "Manages Ubisoft Connect launcher prefix and authenticated token launches",
				Enabled:     false,
				Store:       "ubisoft",
			},
			{
				ID:          "fix-protondb",
				Name:        "ProtonDB Compatibility Feed",
				Author:      "Community",
				Version:     "1.2.0",
				Description: "Fetches live user reports, ratings, and launch tweaks from ProtonDB API",
				Enabled:     true,
				Store:       "fixes",
			},
			{
				ID:          "fix-lutris",
				Name:        "Lutris Community Recipes",
				Author:      "Lutris Team",
				Version:     "1.0.0",
				Description: "Imports community winetricks scripts and registry tweaks directly from lutris.net",
				Enabled:     true,
				Store:       "fixes",
			},
		},
	}
}

func (m *Catalog) ListPlugins() []types.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy so callers can't mutate manager state outside the lock.
	out := make([]types.PluginInfo, len(m.plugins))
	copy(out, m.plugins)
	return out
}

func (m *Catalog) TogglePlugin(id string, enabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.plugins {
		if m.plugins[i].ID == id {
			m.plugins[i].Enabled = enabled
			return true
		}
	}
	return false
}
