package types

import "encoding/json"

// The `versions` capability (docs/plugins/protocol.md, "Versions"). These
// shapes are both the plugin's wire format and what the UI receives, so the
// field names are the protocol's.

// Version channel and build vocabularies.
const (
	ChannelOpen     = "none"     // anyone may switch to it
	ChannelPassword = "password" // locked until versions.unlock accepted a password

	BuildStore   = "store"   // listed by the store
	BuildArchive = "archive" // from Hydra's install history (the ledger)
	BuildManual  = "manual"  // typed by the user

	PinBuild   = "build"
	PinChannel = "channel"
)

// VersionTarget names what game.install/game.update should install. A channel
// is a moving pointer (a Steam or GOG branch); a build is immutable; manual is
// whatever the plugin's `manual` hint asks the user to type (Steam: depot →
// manifest id). Passwords are never part of a target: versions.unlock hands
// them to the plugin, which keeps them.
type VersionTarget struct {
	Channel string            `json:"channel,omitempty"`
	Build   string            `json:"build,omitempty"`
	Manual  map[string]string `json:"manual,omitempty"`
	// ArchiveRef is the install history's reference for Build, added by Hydra
	// for a build it remembers; the UI never sends it.
	ArchiveRef json.RawMessage `json:"archiveRef,omitempty"`
}

// VersionChannel is a moving pointer to a build.
type VersionChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	BuildID     string `json:"buildId,omitempty"`
	// Requires is "none" or "password".
	Requires string `json:"requires,omitempty"`
	Unlocked bool   `json:"unlocked,omitempty"`
	// Current: the installed game follows this channel.
	Current bool `json:"current,omitempty"`
}

// VersionBuild is one immutable build.
type VersionBuild struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Channel     string `json:"channel,omitempty"`
	// Source says where the entry came from: "store", "archive" (Hydra's
	// install history) or "manual" (typed by the user).
	Source string `json:"source,omitempty"`
	// Available is false for a build the plugin knows but can't fetch now.
	// Absent means true; Hydra fills it in.
	Available *bool `json:"available,omitempty"`
	// Current: this build is the installed one.
	Current bool `json:"current,omitempty"`
}

// VersionManual asks the user to type something the store doesn't list.
type VersionManual struct {
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// VersionLink points at a page that helps the user pick a version, such as the
// game's depots on SteamDB. Hydra only opens it in the browser.
type VersionLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// VersionPin is the version a game is held at.
type VersionPin struct {
	Kind string `json:"kind"` // "build" | "channel"
	ID   string `json:"id"`
}

// Target is the pin as a game.update target.
func (p VersionPin) Target() VersionTarget {
	if p.Kind == PinChannel {
		return VersionTarget{Channel: p.ID}
	}
	return VersionTarget{Build: p.ID}
}

// LedgerEntry is one install Hydra performed: a row of the per-game install
// history. ArchiveRef is opaque to Hydra (Epic: a saved manifest's name; Steam:
// the depot → manifest ids); the plugin reads it back to refetch the build.
type LedgerEntry struct {
	Store       string          `json:"store"` // plugin id
	Game        string          `json:"game"`  // Hydra's game id
	Build       string          `json:"build"`
	Channel     string          `json:"channel,omitempty"`
	VersionName string          `json:"versionName,omitempty"`
	Date        string          `json:"date"` // RFC 3339, UTC
	ArchiveRef  json.RawMessage `json:"archiveRef,omitempty"`
}

// Installed is the `installed` object of job.done: what the plugin just put
// on disk. It feeds the install ledger.
type Installed struct {
	Build       string          `json:"build"`
	Channel     string          `json:"channel,omitempty"`
	VersionName string          `json:"versionName,omitempty"`
	ArchiveRef  json.RawMessage `json:"archiveRef,omitempty"`
}

// VersionInfo answers versions.list, and is what the UI shows. The plugin
// fills the first part; Hydra adds the ledger and its own pin.
type VersionInfo struct {
	Channels []VersionChannel `json:"channels"`
	Builds   []VersionBuild   `json:"builds"`
	Manual   *VersionManual   `json:"manual"`
	Links    []VersionLink    `json:"links"`
	// UpdateAvailable is the plugin's verdict relative to the pin: with a pin,
	// whether the installed version differs from it; without one, whether a
	// newer build exists.
	UpdateAvailable bool `json:"updateAvailable"`
	// Pinned is Hydra's pin, never the plugin's echo of it.
	Pinned *VersionPin `json:"pinned"`
	// Ledger is the game's install history, oldest first.
	Ledger []LedgerEntry `json:"ledger"`
}
