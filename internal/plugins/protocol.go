package plugins

import (
	"errors"

	"platypus/internal/jsonrpc"
	"platypus/internal/types"
)

// Messages of protocol api 1. docs/plugins/protocol.md describes them for
// plugin authors; keep the two in sync.

// CodeAuthRequired is the error code a plugin returns when a request needs
// the user to log in first.
const CodeAuthRequired = -32001

// CodeUnavailable is the error code for "not right now": a borrowed game in
// use, a region lock, the store's service down. The message is for the user;
// asking again later may work. It is not a login problem (CodeAuthRequired)
// and not "the plugin doesn't offer this" (-32601).
const CodeUnavailable = -32002

// rpcCode is the JSON-RPC error code in err, or 0 when it isn't one.
func rpcCode(err error) int {
	var re *jsonrpc.Error
	if errors.As(err, &re) {
		return re.Code
	}
	return 0
}

// IsAuthRequired reports whether err is the plugin saying the user must log in.
func IsAuthRequired(err error) bool {
	return rpcCode(err) == CodeAuthRequired
}

// IsUnavailable reports whether err is the plugin saying "unavailable right
// now"; its message says why.
func IsUnavailable(err error) bool {
	return rpcCode(err) == CodeUnavailable
}

// LibraryGame is one entry of library.list.
type LibraryGame struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Installed       bool   `json:"installed"`
	InstallPath     string `json:"installPath,omitempty"`
	Version         string `json:"version,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable,omitempty"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
	// Installable is false for owned games Platypus can't install through this
	// plugin (third-party launcher, no Windows/Linux build); Note says why.
	Installable bool   `json:"installable"`
	Note        string `json:"note,omitempty"`
	// Native marks games with a Linux build: Platypus runs them directly, with
	// no Proton runner and none of the Proton-only settings.
	Native bool `json:"native,omitempty"`
	// ClientLaunch says the store client owns launch and compatibility data (1.17).
	ClientLaunch bool   `json:"clientLaunch,omitempty"`
	CoverURL     string `json:"coverUrl,omitempty"`  // portrait
	HeroURL      string `json:"heroUrl,omitempty"`   // wide background
	BannerURL    string `json:"bannerUrl,omitempty"` // wide card
	// Offline says whether the game launches without a network (protocol
	// 1.15, issue #44): "ok", "network" or "unknown". Absent means
	// "unknown". The conformance suite rejects other values.
	Offline string `json:"offline,omitempty"`
	// CloudSync says the store itself syncs this game's saves (protocol
	// 1.15, issue #44). gameHasCloudSync uses it for plugin games, alongside
	// the cloud-saves capability rule.
	CloudSync bool `json:"cloudSync,omitempty"`
	// LastPlayed is when the store last saw the game run, in unix seconds
	// (protocol 1.15): the library's "last played" for stores whose client
	// launches games outside Platypus. 0 means unknown.
	LastPlayed int64 `json:"lastPlayed,omitempty"`
	// Status is the store's suggested status for the game (protocol 1.16,
	// issue #51): "backlog", "playing", "finished" or "abandoned". A
	// suggestion only: the user's own value always wins. Absent means none.
	Status string `json:"status,omitempty"`
	// UserNotes is the store's suggested notes for the game: plain text, at
	// most 8192 bytes (protocol 1.16, issue #51). A suggestion only.
	UserNotes string `json:"userNotes,omitempty"`
	// Features narrows the capabilities the plugin offers for this game
	// (names from the registry). Absent (nil) means all of them; an empty
	// list means none. Names the plugin doesn't offer at runtime are ignored.
	Features []string `json:"features,omitempty"`
	// SharedBy marks a game the user borrows from a family member (1.8,
	// the `family` capability). Absent for the user's own games.
	SharedBy *types.SharedBy `json:"sharedBy,omitempty"`
	// Copies lists the family copies of a borrowed game, when members differ
	// (1.8, the `family` capability: docs/plugins/protocol.md, "Family"). Read
	// only for a game with SharedBy; the user's own copy means no SharedBy.
	Copies []LibraryCopy `json:"copies,omitempty"`
	// PreferredCopy is the owner id of the copy the user chose ("" for the
	// store's default, SharedBy).
	PreferredCopy string `json:"preferredCopy,omitempty"`
}

// LibraryCopy is one family member's copy of a borrowed game (LibraryGame.copies).
type LibraryCopy struct {
	OwnerID   string       `json:"ownerId"`
	OwnerName string       `json:"ownerName"`
	DLC       []LibraryDLC `json:"dlc,omitempty"`
	Note      string       `json:"note,omitempty"`
	Available bool         `json:"available"`
	InUse     bool         `json:"inUse,omitempty"`
	// Exclude says why part of the copy isn't lent (1.11), shown as text.
	Exclude string `json:"exclude,omitempty"`
}

// LibraryDLC is an add-on one copy has (LibraryCopy.dlc). Kind is "dlc"
// (the default), "soundtrack" or "other" (1.11).
type LibraryDLC struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

type LibraryList struct {
	Games []LibraryGame `json:"games"`
}

// GameSaveContext supplies store-owned save roots and Wine identity (1.17).
type GameSaveContext struct {
	Roots       map[string]string `json:"roots,omitempty"`
	StoreUserID string            `json:"storeUserId,omitempty"`
	Prefix      string            `json:"prefix,omitempty"`
	Windows     bool              `json:"windows,omitempty"`
	Proton      bool              `json:"proton,omitempty"`
	WinUser     string            `json:"winUser,omitempty"`
}

type ClientStatus struct {
	State string `json:"state"`
	Label string `json:"label,omitempty"`
	Progress *float64 `json:"progress,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`
	Total int64 `json:"total,omitempty"`
}

// CompatLookupGame is one game in a batched compat.lookup request (1.17).
type CompatLookupGame struct {
	ID         string   `json:"id"`
	Store      string   `json:"store"`
	StoreAppID string   `json:"storeAppId"`
	SteamAppID string   `json:"steamAppId,omitempty"`
	Platforms  []string `json:"platforms,omitempty"`
	InstallDir string   `json:"installDir,omitempty"`
	// Online allows ProtonDB ratings (and Steam Deck verdicts); ProtectionOnline
	// allows PCGamingWiki and AreWeAntiCheatYet (issue #150). Both are false
	// when the machine is offline.
	Online           bool `json:"online"`
	ProtectionOnline bool `json:"protectionOnline"`
	StoreDRM         bool `json:"storeDRM,omitempty"`
}

// CompatBadge is a provider supplied compatibility badge.
type CompatBadge struct {
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Tone      string `json:"tone,omitempty"`
	Link      string `json:"link,omitempty"`
	Source    string `json:"source"`
	Detail    string `json:"detail,omitempty"`
	CheckedAt int64  `json:"checkedAt"`
}

// CompatFact is one game's answer and its refresh interval.
type CompatFact struct {
	ID     string        `json:"id"`
	Badges []CompatBadge `json:"badges"`
	Report *CompatReport `json:"report,omitempty"`
	Links  []struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	} `json:"links,omitempty"`
	Notes      []string    `json:"notes,omitempty"`
	Wiki       *CompatWiki `json:"wiki,omitempty"`
	TTLSeconds int         `json:"ttlSeconds"`
}

// CompatWiki carries the attribution facts required for PCGamingWiki content.
type CompatWiki struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	FetchedAt  int64  `json:"fetchedAt"`
	LicenceURL string `json:"licenceUrl"`
}

// CompatReport is a provider's report count and rating summary for one game.
type CompatReport struct {
	Count int    `json:"count"`
	State string `json:"state"` // "rated" | "few" | "none"
	Tier  string `json:"tier,omitempty"`
}

// CompatLookupResult is the compat.lookup batch response.
type CompatLookupResult struct {
	Facts []CompatFact `json:"facts"`
}

// RunnerInfo is one installed entry returned by runner.list.
type RunnerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
	Default bool   `json:"default,omitempty"`
	Path    string `json:"path,omitempty"`
	System  bool   `json:"system,omitempty"`
	Custom  bool   `json:"custom,omitempty"`
	// SizeBytes is the runner's size on disk, 0 when unknown (issue #142).
	SizeBytes int64 `json:"size_bytes,omitempty"`
}

// LaunchSpec answers game.launchSpec. Either Platypus runs Exe through its
// runner pipeline, or (Delegate == "store-client") Platypus runs Command, which
// asks the store's own client to start the game, and follows the game through
// Find.
type LaunchSpec struct {
	Exe       string            `json:"exe,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	NeedsWine bool              `json:"needsWine,omitempty"`
	// Redact lists argument prefixes (e.g. "-AUTH_PASSWORD=") whose values
	// are hidden in the command preview and the launch log.
	Redact []string `json:"redact,omitempty"`

	Delegate     string    `json:"delegate,omitempty"`
	Client       string    `json:"client,omitempty"` // client name for messages
	Command      []string  `json:"command,omitempty"`
	Find         *FindSpec `json:"find,omitempty"`
	StartTimeout int       `json:"startTimeout,omitempty"` // seconds
}

// FindSpec identifies a delegated game's process: one whose command line
// (/proc/<pid>/cmdline) contains every string in Cmdline.
type FindSpec struct {
	Cmdline []string `json:"cmdline"`
}

// GameDetails answers game.details: the game page's metadata. A plugin
// without that data returns an empty struct, like Steam stores without a
// metadata source.
type GameDetails struct {
	Summary      string        `json:"summary,omitempty"`
	Screenshots  []string      `json:"screenshots,omitempty"` // http(s) URLs
	Genres       []string      `json:"genres,omitempty"`
	Developers   []string      `json:"developers,omitempty"`
	Publishers   []string      `json:"publishers,omitempty"`
	ReleaseDate  string        `json:"releaseDate,omitempty"`
	HeroURL      string        `json:"heroUrl,omitempty"` // wide background
	Achievements *Achievements `json:"achievements,omitempty"`
	// The compat fields below are protocol 1.10 (CompatVersion): Platypus reads
	// them only from a plugin that speaks it.
	SteamAppID string   `json:"steamAppId,omitempty"` // the game's Steam AppID, for ratings
	Platforms  []string `json:"platforms,omitempty"`  // "linux", "windows", "macos"
	DRM        *DRM     `json:"drm,omitempty"`
	// Terms are the agreements the user accepts before an install (1.11, the
	// `terms` capability: docs/plugins/protocol.md, "Terms").
	Terms []Term `json:"terms,omitempty"`
	// Media and richer facts below are protocol 1.12 (MediaVersion): Platypus
	// reads them only from a plugin that speaks it. DescriptionHTML and the
	// requirements are sanitized HTML (internal/sanitize); the UI renders
	// them as markup. URLs are https (or the local art server); videos never
	// autoplay with sound.
	DescriptionHTML string        `json:"descriptionHtml,omitempty"`
	Website         string        `json:"website,omitempty"`
	StoreURL        string        `json:"storeUrl,omitempty"`
	Languages       []string      `json:"languages,omitempty"`
	Controller      string        `json:"controller,omitempty"` // "full" | "partial" | ""
	Requirements    *Requirements `json:"requirements,omitempty"`
	AgeRating       string        `json:"ageRating,omitempty"`
	Tags            []string      `json:"tags,omitempty"`
	Videos          []Video       `json:"videos,omitempty"`
}

// Requirements are the store's system requirements (game.details,
// protocol 1.12).
type Requirements struct {
	Minimum     string `json:"minimum,omitempty"`
	Recommended string `json:"recommended,omitempty"`
}

// Video is one trailer or clip (game.details, protocol 1.12).
type Video struct {
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Title     string `json:"title,omitempty"`
	MIME      string `json:"mime,omitempty"`
}

// CompatVersion is the first protocol revision whose game.details carries the
// compat fields (SteamAppID, Platforms, DRM).
var CompatVersion = Version{Major: 1, Minor: 10}

// MediaVersion is the first protocol revision whose game.details carries the
// media and richer facts (DescriptionHTML, Website, StoreURL, Languages,
// Controller, Requirements, AgeRating, Tags, Videos).
var MediaVersion = Version{Major: 1, Minor: 12}

// UnreadCountsVersion is the first protocol revision with the unread snapshot
// of chat (chat.unreadCounts, the optional method of the `chat` capability).
// Platypus calls it only on a plugin that speaks it; an older plugin keeps
// today's behaviour, with badges set by the chat.unread notifications alone.
var UnreadCountsVersion = Version{Major: 1, Minor: 13}

// LocaleVersion is the first protocol revision with the user's locale
// (initialize {locale}, the locale.changed notification, issue #39).
// Platypus sends locale.changed only to a plugin that speaks it; an older
// plugin gets the new locale on its next start.
var LocaleVersion = Version{Major: 1, Minor: 14}

// OfflineVersion is the first protocol revision with offline launches
// (LibraryGame.offline/cloudSync, the local-art capability, gameIdPrefix,
// the executable param of game.launchSpec, optional requires; issue #44).
// Platypus reads those fields only from a plugin that speaks it; an older
// plugin keeps today's behaviour.
var OfflineVersion = Version{Major: 1, Minor: 15}

// StatusVersion is the first protocol revision with a store's per-game status
// and notes hints (LibraryGame.status/userNotes, issue #51). A suggestion
// only: the user's own value always wins, a hint fills an empty field only.
var StatusVersion = Version{Major: 1, Minor: 16}

// ProtectionOnlineVersion is the first protocol revision whose compat.lookup
// splits the online state in two (CompatLookupGame.online and
// protectionOnline, issue #150). A provider that predates it knows only
// `online` and gates every source on it, so Platypus sends it the
// intersection of both switches instead (see requestCompatFacts).
var ProtectionOnlineVersion = Version{Major: 1, Minor: 18}

// DRM is what the store says about a game's protection. Every field is a
// pointer or optional: nil (or "") means the store said nothing, and Platypus
// shows no badge for it. Plugins map their own store fields to these:
//
//   - Epic: OwnershipToken → DenuvoLikely; CanRunOffline=false → OnlineRequired;
//     ThirdPartyManagedApp → Launcher (ubisoft-connect, ea-app, …).
//   - GOG: DRMFree true by policy.
type DRM struct {
	DRMFree        *bool  `json:"drmFree,omitempty"`
	OnlineRequired *bool  `json:"onlineRequired,omitempty"`
	Launcher       string `json:"launcher,omitempty"`
	DenuvoLikely   *bool  `json:"denuvoLikely,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// Term is one agreement a store asks the user to accept (GameDetails.terms).
// Version changes when the text does: the user accepts each version again.
type Term struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url,omitempty"`
	Version string `json:"version"`
}

// Achievements in game.details are store-level: what the store shows about
// the game. Whether the user unlocked one is the plugin's business; Platypus
// shows the list as "highlighted by the store".
type Achievements struct {
	Total int           `json:"total"`
	List  []Achievement `json:"list"`
}

// Achievement is one achievement as the store describes it.
type Achievement struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	Icon          string  `json:"icon,omitempty"` // http(s) URL
	GlobalPercent float64 `json:"globalPercent,omitempty"`
}

type InstallSize struct {
	DownloadBytes int64 `json:"downloadBytes"`
	DiskBytes     int64 `json:"diskBytes"`
}

// JobUpdate is the params of job.progress, job.done and job.failed.
type JobUpdate struct {
	JobID   string   `json:"jobId"`
	Phase   string   `json:"phase,omitempty"`
	Bytes   int64    `json:"bytes,omitempty"`
	Total   int64    `json:"total,omitempty"`
	Speed   float64  `json:"speed,omitempty"` // bytes per second
	Percent *float64 `json:"percent,omitempty"`
	Error   string   `json:"error,omitempty"`
	// Installed, on job.done of an install or update, says which version the
	// plugin put on disk. Platypus appends it to the game's install ledger.
	Installed *types.Installed `json:"installed,omitempty"`
	// Verify, on job.done of a game.verify job, is what the check found.
	Verify *types.VerifyReport `json:"verify,omitempty"`
}
