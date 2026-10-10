package types

// Cloud saves (protocol 1.2, capability `cloud-saves`; docs/plugins/protocol.md,
// "Cloud saves"). The first group is the wire format between Platypus and a store
// plugin; the second is what the orchestrator tells the UI.

// Cloud sync states, as CloudStatus.State carries them.
const (
	CloudStateSynced      = "synced"
	CloudStatePending     = "pending"
	CloudStateConflict    = "conflict"
	CloudStateUnsupported = "unsupported"
	CloudStateOffline     = "offline"
	CloudStateError       = "error"
)

// Cloud sync modes, per plugin and per game.
const (
	CloudOff  = "off"  // Platypus never syncs this game
	CloudAsk  = "ask"  // sync clean cases, ask on a conflict or when the store is unreachable
	CloudAuto = "auto" // like ask, but go on without asking once the user confirmed an unreachable store for the game
)

// CloudContext is what Platypus knows about the launch a call is for. The plugin
// maps its store's save locations through it and never learns Platypus's prefix
// layout.
type CloudContext struct {
	// OS is the client OS of the launch: "linux" for a native build, "windows"
	// for a game run through Wine or Proton.
	OS         string `json:"os"`
	Home       string `json:"home"`
	XDGData    string `json:"xdgData"`
	XDGConfig  string `json:"xdgConfig"`
	InstallDir string `json:"installDir"`
	// PrefixDir is the Wine prefix root (the folder holding drive_c) for a
	// windows launch; empty for a native one.
	PrefixDir string `json:"prefixDir,omitempty"`
	// WinUser is the Windows user name inside the prefix ("steamuser" under Proton).
	WinUser string `json:"winUser,omitempty"`
}

// CloudStatus is the plugin's answer to every cloud.* request.
type CloudStatus struct {
	State     string          `json:"state"`
	Message   string          `json:"message,omitempty"` // shown to the user as is
	Counts    CloudCounts     `json:"counts"`
	Conflicts []CloudConflict `json:"conflicts,omitempty"`
	CheckedAt int64           `json:"checkedAt,omitempty"` // unix seconds
	BackupDir string          `json:"backupDir,omitempty"`
	// Stamp is an opaque fingerprint of the local save files. Platypus compares
	// two readings to know the game stopped writing; absent, it compares the
	// counts and conflicts.
	Stamp string `json:"stamp,omitempty"`
	// Client reports the store's own client. While it runs, Platypus does not sync.
	Client *CloudClient `json:"client,omitempty"`
}

// CloudCounts is the per-file tally of a comparison.
type CloudCounts struct {
	Same     int `json:"same"`
	Download int `json:"download"`
	Upload   int `json:"upload"`
	Conflict int `json:"conflict"`
	Unmapped int `json:"unmapped"`
}

// CloudConflict is one file that changed on both sides.
type CloudConflict struct {
	Path   string    `json:"path"`
	Local  CloudFile `json:"local"`
	Remote CloudFile `json:"remote"`
}

// CloudFile describes one side of a conflict.
type CloudFile struct {
	Size    int64  `json:"size"`
	Mtime   int64  `json:"mtime"` // unix seconds
	Machine string `json:"machine,omitempty"`
}

// CloudClient says whether the store's official client is running, and
// whether the plugin can start this game through it (game.launchSpec with
// via "client").
type CloudClient struct {
	Running   bool   `json:"running"`
	Name      string `json:"name"`
	CanLaunch bool   `json:"canLaunch,omitempty"`
}

// CloudProgress is the cloud.progress notification.
type CloudProgress struct {
	ID    string `json:"id"`
	JobID string `json:"jobId"`
	Phase string `json:"phase"` // "check", "download" or "upload"
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// CloudInfo is the game page's view of a game's cloud saves.
type CloudInfo struct {
	// Mode is the effective mode; PluginMode the plugin's default and GameMode
	// the game's own choice ("" = follow the plugin).
	Mode       string `json:"mode"`
	PluginMode string `json:"plugin_mode"`
	GameMode   string `json:"game_mode"`
	// Status is the last answer this session, nil before the first check.
	Status *CloudStatus `json:"status"`
	// Busy names the job running for the game ("pull", "push", "resolve"), if any.
	Busy string `json:"busy,omitempty"`
	// Running says the game is running now: its saves are not synced until it exits.
	Running bool `json:"running"`
}

// CloudSettings are the saved modes: the plugins' defaults and the games' own choices.
type CloudSettings struct {
	Plugins map[string]string `json:"plugins"`
	Games   map[string]string `json:"games"`
}

// Kinds of CloudEvent.
const (
	CloudEvProgress = "progress" // a job moved: phase, done, total
	CloudEvStatus   = "status"   // a new status is known (check, pull, resolve)
	CloudEvPrompt   = "prompt"   // a launch waits for the user: see Prompt
	CloudEvPushed   = "pushed"   // a post-exit push finished
	CloudEvSkipped  = "skipped"  // a sync was left out on purpose; Message says why
	CloudEvFailed   = "failed"   // a sync failed; the game's saves are not in the cloud
	CloudEvCanceled = "canceled"
)

// Prompts of a launch that waits for the user.
const (
	CloudPromptConflict = "conflict"       // saves changed on both sides
	CloudPromptOffline  = "offline"        // the store couldn't be reached, or the sync failed
	CloudPromptClient   = "client-running" // the store's client is running and can start the game
)

// CloudEvent is emitted on the "cloud" Wails event.
type CloudEvent struct {
	GameID  string       `json:"game_id"`
	Kind    string       `json:"kind"`
	Prompt  string       `json:"prompt,omitempty"`
	Message string       `json:"message,omitempty"`
	Status  *CloudStatus `json:"status,omitempty"`
	JobID   string       `json:"job_id,omitempty"`
	Phase   string       `json:"phase,omitempty"`
	Done    int64        `json:"done,omitempty"`
	Total   int64        `json:"total,omitempty"`
	// Client is the store client's name, for the client-running prompt and skips.
	Client string `json:"client,omitempty"`
	// Title is the game's title, so a prompt can be shown without the library.
	Title string `json:"title,omitempty"`
}

// Normalize makes a plugin's status safe to act on: a state this Platypus
// doesn't know becomes an error that says so, and negative counts are zeroed.
func (s *CloudStatus) Normalize() {
	switch s.State {
	case CloudStateSynced, CloudStatePending, CloudStateConflict, CloudStateUnsupported, CloudStateOffline, CloudStateError:
	default:
		s.Message = "The plugin reported an unknown cloud state " + quoteState(s.State) + "."
		s.State = CloudStateError
	}
	c := &s.Counts
	c.Same, c.Download, c.Upload, c.Conflict, c.Unmapped = max(c.Same, 0), max(c.Download, 0), max(c.Upload, 0), max(c.Conflict, 0), max(c.Unmapped, 0)
}

func quoteState(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return `"` + s + `"`
}

// ValidCloudMode reports whether m is one of the sync modes.
func ValidCloudMode(m string) bool { return m == CloudOff || m == CloudAsk || m == CloudAuto }
