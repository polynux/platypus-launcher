package plugins

// Messages of protocol api 1. docs/plugins/protocol.md describes them for
// plugin authors; keep the two in sync.

// CodeAuthRequired is the error code a plugin returns when a request needs
// the user to log in first.
const CodeAuthRequired = -32001

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
	CoverURL    string `json:"coverUrl,omitempty"`  // portrait
	HeroURL     string `json:"heroUrl,omitempty"`   // wide background
	BannerURL   string `json:"bannerUrl,omitempty"` // wide card
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
}
