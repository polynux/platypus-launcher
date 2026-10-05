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
	InstallDir  string       `json:"install_dir"`
	Executable  string       `json:"executable"`
	IsInstalled bool         `json:"is_installed"`
	Settings    GameSettings `json:"settings"`
	FixInfo     *GameFixInfo `json:"fix_info,omitempty"`
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
