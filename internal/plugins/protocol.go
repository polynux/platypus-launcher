package plugins

import (
	"errors"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/types"
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
	// Installable is false for owned games Hydra can't install through this
	// plugin (third-party launcher, no Windows/Linux build); Note says why.
	Installable bool   `json:"installable"`
	Note        string `json:"note,omitempty"`
	// Native marks games with a Linux build: Hydra runs them directly, with
	// no Proton runner and none of the Proton-only settings.
	Native    bool   `json:"native,omitempty"`
	CoverURL  string `json:"coverUrl,omitempty"`  // portrait
	HeroURL   string `json:"heroUrl,omitempty"`   // wide background
	BannerURL string `json:"bannerUrl,omitempty"` // wide card
	// Features narrows the capabilities the plugin offers for this game
	// (names from the registry). Absent (nil) means all of them; an empty
	// list means none. Names the plugin doesn't offer at runtime are ignored.
	Features []string `json:"features,omitempty"`
	// SharedBy marks a game the user borrows from a family member (1.8,
	// the `family` capability). Absent for the user's own games.
	SharedBy *types.SharedBy `json:"sharedBy,omitempty"`
}

type LibraryList struct {
	Games []LibraryGame `json:"games"`
}

// LaunchSpec answers game.launchSpec. Either Hydra runs Exe through its
// runner pipeline, or (Delegate == "store-client") Hydra runs Command, which
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
}

// Achievements in game.details are store-level: what the store shows about
// the game. Whether the user unlocked one is the plugin's business; Hydra
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
	// plugin put on disk. Hydra appends it to the game's install ledger.
	Installed *types.Installed `json:"installed,omitempty"`
	// Verify, on job.done of a game.verify job, is what the check found.
	Verify *types.VerifyReport `json:"verify,omitempty"`
}
