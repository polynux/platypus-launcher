package types

type StoreType string

const (
	StoreSteam   StoreType = "steam"
	StoreEpic    StoreType = "epic"
	StoreGOG     StoreType = "gog"
	StoreUbisoft StoreType = "ubisoft"
	StoreCustom  StoreType = "custom"
)

type ProtonVersion struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsSystem  bool   `json:"is_system"`
	IsCustom  bool   `json:"is_custom"`
	IsDefault bool   `json:"is_default"`
}

type GameSettings struct {
	RunnerID        string            `json:"runner_id"`        // e.g. "Proton - Experimental", "wine-system"
	WinePrefix      string            `json:"wine_prefix"`      // Custom wine prefix path
	LaunchArguments string            `json:"launch_arguments"` // -novid -fullscreen
	EnvVars         map[string]string `json:"env_vars"`         // e.g. PROTON_ENABLE_NVAPI=1
	UseGameMode     bool              `json:"use_gamemode"`     // gamemoderun
	UseMangoHud     bool              `json:"use_mangohud"`     // mangohud
	UseGamescope    bool              `json:"use_gamescope"`    // gamescope wrapper
	GamescopeW      int               `json:"gamescope_w"`      // gamescope width
	GamescopeH      int               `json:"gamescope_h"`      // gamescope height
	GamescopeR      int               `json:"gamescope_r"`      // gamescope refresh rate
	GamescopeArgs   string            `json:"gamescope_args"`   // e.g. -f --adaptive-sync
	EACRuntime      bool              `json:"eac_runtime"`      // EasyAntiCheat runtime binding
	BattlEyeRuntime bool              `json:"battleye_runtime"` // BattlEye runtime binding
	WorkingDir      string            `json:"working_dir"`
}

type GameFixInfo struct {
	Source       string            `json:"source"`        // "lutris", "protondb", "community"
	Tier         string            `json:"tier"`          // "platinum", "gold", "silver", "bronze", "borked"
	Description  string            `json:"description"`   // Brief note
	Winetricks   []string          `json:"winetricks"`    // e.g. ["vcrun2019", "corefonts"]
	DLLOverrides map[string]string `json:"dll_overrides"` // e.g. {"dxgi": "n,b"}
	LaunchArgs   string            `json:"launch_args"`   // Recommended launch options
}

// Sources that aren't store plugins. Any other Game.Source is a plugin id.
const (
	SourceSteam  = "steam"
	SourceCustom = "custom"
)

type Game struct {
	ID string `json:"id"`
	// Source is where the game comes from: "steam", "custom" or a store
	// plugin id. Store is the storefront for display, and several sources may
	// share one (a plugin can serve Steam games too).
	Source      string       `json:"source"`
	Store       StoreType    `json:"store"`
	StoreAppID  string       `json:"store_app_id"`
	Title       string       `json:"title"`
	CoverURL    string       `json:"cover_url"`
	BannerURL   string       `json:"banner_url"`
	HeroURL     string       `json:"hero_url"`
	InstallDir  string       `json:"install_dir"`
	Executable  string       `json:"executable"`
	IsInstalled bool         `json:"is_installed"`
	Settings    GameSettings `json:"settings"`
	FixInfo     *GameFixInfo `json:"fix_info,omitempty"`
	LastPlayed  int64        `json:"last_played_at,omitempty"` // unix seconds
	PlaytimeMin int          `json:"playtime_minutes,omitempty"`
	SizeBytes   int64        `json:"size_bytes,omitempty"` // installed size, if known
	Version     string       `json:"version,omitempty"`
	// Plugin games only.
	UpdateAvailable bool   `json:"update_available,omitempty"`
	Installable     bool   `json:"installable,omitempty"`    // Hydra can install it through the plugin
	Note            string `json:"note,omitempty"`           // e.g. "Needs the EA app"
	OutsideFolder   bool   `json:"outside_folder,omitempty"` // installed outside the folders Hydra lets the plugin launch from
	// Native marks plugin games with a Linux build: they run without Proton,
	// so the runner pick and Proton-only settings don't apply to them.
	Native bool `json:"native,omitempty"`
}

// Library is the merged library. A source that fails to list (a plugin that
// is logged out or crashed) adds a warning instead of failing the scan.
type Library struct {
	Games    []Game          `json:"games"`
	Warnings []SourceWarning `json:"warnings"`
}

type SourceWarning struct {
	Source    string `json:"source"`
	Message   string `json:"message"`
	NeedsAuth bool   `json:"needs_auth,omitempty"`
}

// StorePlugin is a discovered store plugin as the Plugins view shows it.
type StorePlugin struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Author       string              `json:"author"`
	Description  string              `json:"description"`
	Store        string              `json:"store"`
	Status       string              `json:"status"` // see plugins.Status
	Problem      string              `json:"problem,omitempty"`
	Missing      []PluginRequirement `json:"missing"`
	Requires     []PluginRequirement `json:"requires"`
	Capabilities []string            `json:"capabilities"`
	Dir          string              `json:"dir"`
	Exec         string              `json:"exec"`
	Hash         string              `json:"hash"`
	Network      []string            `json:"network"`
	Read         []string            `json:"read"`
	Write        []string            `json:"write"`
	Running      bool                `json:"running"`
	// Where its games install: the Games folder subfolder, or the user's override.
	InstallFolder        string   `json:"install_folder"`
	DefaultInstallFolder string   `json:"default_install_folder"`
	ExtraFolders         []string `json:"extra_folders"` // other folders the user allowed launching from
}

type PluginRequirement struct {
	Bin  string `json:"bin"`
	Hint string `json:"hint"`
}

// AuthStatus is a plugin's login state.
type AuthStatus struct {
	LoggedIn bool   `json:"loggedIn"`
	User     string `json:"user,omitempty"`
}

// AuthPrompt is what the login dialog shows: a page to open, if any, then the
// fields to fill in.
type AuthPrompt struct {
	URL          string      `json:"url,omitempty"`
	Instructions string      `json:"instructions,omitempty"`
	Fields       []AuthField `json:"fields"`
}

type AuthField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Secret      bool   `json:"secret,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// LauncherSettings are Hydra-wide settings, persisted in settings.json.
type LauncherSettings struct {
	GamesFolder        string `json:"games_folder"` // store plugins install into <GamesFolder>/<plugin>
	DefaultGamesFolder string `json:"default_games_folder"`
}

// Achievement is one Steam achievement. Unlocked/UnlockedAt are only known when
// the Steam client has cached the user's progress locally.
type Achievement struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Icon          string  `json:"icon"`
	Unlocked      bool    `json:"unlocked"`
	UnlockedAt    int64   `json:"unlocked_at"`    // unix seconds, 0 if locked/unknown
	GlobalPercent float64 `json:"global_percent"` // share of all players who have it
}

type Achievements struct {
	Total    int           `json:"total"`
	Unlocked int           `json:"unlocked"` // -1 when progress is unknown
	List     []Achievement `json:"list"`     // may be a subset of Total
}

// GameDetails is the on-demand data shown on the game page.
type GameDetails struct {
	Summary      string        `json:"summary"`
	Screenshots  []string      `json:"screenshots"`
	Genres       []string      `json:"genres"`
	Developers   []string      `json:"developers"`
	Publishers   []string      `json:"publishers"`
	ReleaseDate  string        `json:"release_date"`
	HeroURL      string        `json:"hero_url,omitempty"` // wide art, when the store has better than the library entry's
	Achievements *Achievements `json:"achievements"`
}

// Tweak is a launch setting Hydra suggests for every game, based on the
// detected hardware and session, independent of the ProtonDB tier.
type Tweak struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Reason   string            `json:"reason"`
	Kind     string            `json:"kind"` // "recommended" | "troubleshoot"
	Env      map[string]string `json:"env"`
	Wrappers []string          `json:"wrappers,omitempty"` // "gamemode", "mangohud"
}

// SessionEvent is emitted on the "session" Wails event when a launched game
// changes state.
type SessionEvent struct {
	GameID  string `json:"game_id"`
	State   string `json:"state"` // "starting" | "running" | "stopped" | "failed"
	PID     int    `json:"pid,omitempty"`
	Message string `json:"message,omitempty"`
}

// Job is an install, update or uninstall run by a store plugin.
type Job struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"` // "install" | "update" | "uninstall"
	Source      string  `json:"source"`
	GameID      string  `json:"game_id"`       // Hydra's game id
	StoreGameID string  `json:"store_game_id"` // the plugin's game id
	Title       string  `json:"title"`
	State       string  `json:"state"` // "queued" | "running" | "done" | "failed" | "canceled"
	InstallDir  string  `json:"install_dir,omitempty"`
	Phase       string  `json:"phase,omitempty"`
	Bytes       int64   `json:"bytes"`
	Total       int64   `json:"total"`
	Speed       float64 `json:"speed"`   // bytes per second
	Percent     float64 `json:"percent"` // 0-100, -1 when unknown
	Error       string  `json:"error,omitempty"`
	Attempts    int     `json:"attempts"`
	CreatedAt   int64   `json:"created_at"` // unix ms
	UpdatedAt   int64   `json:"updated_at"`
}
