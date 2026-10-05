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

type Game struct {
	ID          string       `json:"id"`
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
}

type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Store       string `json:"store"`
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
