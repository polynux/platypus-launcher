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
	// RunMode is how a sideloaded game runs (issue #47): "" or "auto" means
	// the detected value (ELF/shebang/AppImage = native, MZ/.msi/.bat/.lnk =
	// Wine), "native" forces a Linux run, "wine" forces Wine/Proton.
	RunMode string `json:"run_mode,omitempty"`
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

// DetectResult is how a sideloaded executable would run by default
// (issue #47): content detection, before the per-game run_mode override.
type DetectResult struct {
	Native bool   `json:"native"`
	Kind   string `json:"kind"`
}

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
	// ReleaseYear is the year the game came out, 0 when unknown. Sideloaded
	// games carry what the user set; store games only what the library
	// already knows without extra lookups.
	ReleaseYear int `json:"release_year,omitempty"`
	// SettingsExtra carries saved per-game settings keys the backend
	// doesn't model (UI-only controls like sync_mode), so
	// sideloaded games don't lose them in the emitted library (CORE-9).
	SettingsExtra map[string]any `json:"settings_extra,omitempty"`
	FixInfo       *GameFixInfo   `json:"fix_info,omitempty"`
	CompatBadges  []CompatBadge  `json:"compat_badges,omitempty"`
	LastPlayed    int64          `json:"last_played_at,omitempty"` // unix seconds
	// PlaytimeMin is the playtime shown and sorted by: max(local, store) for
	// a plugin game with a playtime report (playtime capability, protocol 1.7),
	// else the local value. The two parts ride along for the tooltip.
	PlaytimeMin int `json:"playtime_minutes,omitempty"`
	// PlaytimeLocalMin is nil when Platypus has no local record (plugin games);
	// PlaytimeStoreMin is nil without a store report, and 0 is a real report.
	PlaytimeLocalMin *int   `json:"playtime_local_minutes,omitempty"`
	PlaytimeStoreMin *int   `json:"playtime_store_minutes,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"` // installed size, if known
	Version          string `json:"version,omitempty"`
	// Plugin games only.
	UpdateAvailable bool   `json:"update_available,omitempty"`
	Installable     bool   `json:"installable,omitempty"`    // Platypus can install it through the plugin
	Note            string `json:"note,omitempty"`           // e.g. "Needs the EA app"
	OutsideFolder   bool   `json:"outside_folder,omitempty"` // installed outside the folders Platypus lets the plugin launch from
	// Native marks plugin games with a Linux build: they run without Proton,
	// so the runner pick and Proton-only settings don't apply to them.
	Native bool `json:"native,omitempty"`
	// ClientLaunch says the store client must launch the game and owns its compatibility data.
	ClientLaunch bool `json:"client_launch,omitempty"`
	// Offline says whether the game launches without a network (protocol
	// 1.15, issue #44): "ok", "network" or "unknown" ("" means unknown,
	// the pre-1.15 default). The core never disables Play because of it.
	Offline string `json:"offline,omitempty"`
	// CloudSync says the store itself syncs this game's saves (protocol
	// 1.15, issue #44). gameHasCloudSync uses it for plugin games.
	CloudSync bool `json:"cloud_sync,omitempty"`
	// Features are the plugin capabilities on offer for this game (names from
	// the plugin capability registry): the plugin's per-game list, or every
	// capability the plugin offers when it gave none. Never omitted for plugin
	// games, so an empty list ("nothing") differs from null (Steam, sideloaded:
	// not capability-driven).
	Features []string `json:"features"`
	// Pinned is the version the user holds the game at (the `versions`
	// capability): its update prompt is off and updates install this.
	Pinned *VersionPin `json:"pinned,omitempty"`
	// Status is where the user stands with the game (issue #51): "backlog",
	// "playing", "finished" or "abandoned", "" for none. It is the user's
	// own, never the store's.
	Status string `json:"status,omitempty"`
	// UserNotes are the user's own notes on the game, plain text. They are
	// not Note: that is the store's install note ("Needs the EA app").
	UserNotes string `json:"user_notes,omitempty"`
	// StoreStatus/StoreNotes are the store's status and notes suggestions
	// (protocol 1.16, issue #51), carried to applyGameStatus and kept off
	// the wire: the UI sees the merged Status/UserNotes above.
	StoreStatus string `json:"-"`
	StoreNotes  string `json:"-"`
	// SharedBy is the family member who lends the game (the `family`
	// capability). Nil for the user's own games.
	SharedBy *SharedBy `json:"shared_by,omitempty"`
	// Copies are the family copies of a borrowed game, when they differ
	// (protocol 1.8). PreferredCopy is the owner id the user chose, "" for
	// the store's default (SharedBy).
	Copies        []GameCopy `json:"copies,omitempty"`
	PreferredCopy string     `json:"preferred_copy,omitempty"`
	// CopyCompare sets the borrowed game's copies against the baseline copy
	// (nil with fewer than two copies). Computed by core, see stores/copydiff.go.
	CopyCompare *CopyComparison `json:"copy_compare,omitempty"`
	// What the user changed about the game's look (Platypus's own customization,
	// never the plugin's): StoreTitle is the title the store gave when Title is
	// the user's, SortTitle orders the library, the *URL fields are the user's
	// images and Customized names every part that is theirs (title, sort_title
	// and the kinds of art). Plugins only ever see StoreTitle.
	StoreTitle    string   `json:"store_title,omitempty"`
	SortTitle     string   `json:"sort_title,omitempty"`
	LogoURL       string   `json:"logo_url,omitempty"`
	IconURL       string   `json:"icon_url,omitempty"`
	BackgroundURL string   `json:"background_url,omitempty"`
	Customized    []string `json:"customized,omitempty"`
}

// OriginalTitle is the title the store gave the game: what a plugin is sent
// and what the user's own title replaces.
func (g Game) OriginalTitle() string {
	if g.StoreTitle != "" {
		return g.StoreTitle
	}
	return g.Title
}

// Customization is the user's look for one game, as the editor shows it: the
// title and sort title they set, and the address of each image they set.
type Customization struct {
	Title     string            `json:"title"`
	SortTitle string            `json:"sort_title"`
	Art       map[string]string `json:"art"`
	// ArtSource remembers where the art came from when Platypus found it
	// online (issues #36, art fallbacks), e.g. "steam:12345", "gog:1308320804"
	// or "lutris:rivage", with ArtSourceName the store name for the
	// attribution line. Empty when the art is the store's own or picked by
	// hand.
	ArtSource     string `json:"art_source,omitempty"`
	ArtSourceName string `json:"art_source_name,omitempty"`
}

// ArtCandidate is one online art search hit (issues #36, art fallbacks):
// art the user can apply to a sideloaded game. Source names where the art
// comes from: "steam", "gog" or "lutris" (Lutris mirrors IGDB covers). ID is
// the source's id: the Steam app id as a string, the GOG product id, or the
// Lutris slug. AppID keeps the Steam app id for backward compatibility (0
// for non-Steam hits). Year is the release year when known.
type ArtCandidate struct {
	Source   string `json:"source"`
	ID       string `json:"id"`
	AppID    int    `json:"app_id"`
	Name     string `json:"name"`
	Year     int    `json:"year,omitempty"`
	ThumbURL string `json:"thumb_url"`
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
	// Unavailable marks a source that answered "unavailable right now"
	// (plugin error -32002): not a login problem, worth trying again later.
	Unavailable bool `json:"unavailable,omitempty"`
	// Stale marks games served from the last good listing or the on-disk
	// snapshot while the source failed (issue #44): the library survives
	// an offline restart.
	Stale bool `json:"stale,omitempty"`
}

// StorePlugin is a discovered plugin as the Plugins view shows it. Kind is
// "store" (a program, needs approval) or "data" (JSON only, always on).
type StorePlugin struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	Author      string              `json:"author"`
	Description string              `json:"description"`
	Store       string              `json:"store"`
	Kind        string              `json:"kind,omitempty"`
	Status      string              `json:"status"` // see plugins.Status
	Problem     string              `json:"problem,omitempty"`
	Missing     []PluginRequirement `json:"missing"`
	Requires    []PluginRequirement `json:"requires"`
	// Capabilities is what the plugin offers right now: its last
	// initialize's answer, or the declared list until it has run. UI
	// surfaces test this, through the UI's capability helper.
	Capabilities []string `json:"capabilities"`
	// Declared is the manifest's list: the promise the user approved.
	Declared []string `json:"declared"`
	// Offers lists every declared capability with its state and depth flags,
	// for the Plugins view.
	Offers []PluginCapability `json:"offers"`
	// Needs are the capabilities Platypus must know for the plugin to run.
	Needs []string `json:"needs"`
	// Warnings are things that don't stop the plugin, e.g. capabilities this
	// Platypus ignores because it doesn't know them.
	Warnings []string `json:"warnings"`
	// Initialized is true once the plugin has answered initialize in this
	// session; Protocol is the negotiated version ("1.1").
	Initialized bool   `json:"initialized"`
	Protocol    string `json:"protocol,omitempty"`
	// ProtocolVersions and Transport are what the manifest declares.
	ProtocolVersions string   `json:"protocol_versions,omitempty"`
	Transport        string   `json:"transport,omitempty"`
	Dir              string   `json:"dir"`
	Exec             string   `json:"exec"`
	Hash             string   `json:"hash"`
	Network          []string `json:"network"`
	Read             []string `json:"read"`
	Write            []string `json:"write"`
	Running          bool     `json:"running"`
	// GaveUp is true when the plugin crashed MaxCrashes times in a row: it
	// fails fast until retried. The Plugins view offers a Retry action.
	GaveUp bool `json:"gave_up,omitempty"`
	// Where its games install: the Games folder subfolder, or the user's override.
	InstallFolder        string   `json:"install_folder"`
	DefaultInstallFolder string   `json:"default_install_folder"`
	ExtraFolders         []string `json:"extra_folders"` // other folders the user allowed launching from
	// IconURL is the address the UI draws the plugin's logo from ("" when it
	// has none): a locally served file from the plugin folder, never the
	// network. The sidebar Sources and the Plugins view show it, with a
	// monogram tile as fallback.
	IconURL string `json:"icon_url,omitempty"`
	// StorefrontURL is the store's official website the launcher embeds
	// (issue #17), "" when the plugin declares none. StorefrontDomains are
	// the plain hostnames it belongs to (checkout may wander outside them).
	StorefrontURL     string   `json:"storefront_url,omitempty"`
	StorefrontDomains []string `json:"storefront_domains,omitempty"`
}

// PluginCapability is one declared capability as the Plugins view shows it.
type PluginCapability struct {
	Name string `json:"name"`
	// On: the plugin offers it right now (see StorePlugin.Capabilities).
	On bool `json:"on"`
	// Known: this Platypus has it in its registry; unknown names are ignored.
	Known bool `json:"known"`
	// Orchestrated: core has code that uses it. Known but not orchestrated
	// capabilities are recognised and not used yet.
	Orchestrated bool `json:"orchestrated"`
	// Features are the plugin's depth flags for the capability.
	Features map[string]bool `json:"features"`
}

type PluginRequirement struct {
	Bin  string `json:"bin"`
	Hint string `json:"hint"`
}

// AuthStatus is a plugin's login state.
type AuthStatus struct {
	LoggedIn bool   `json:"loggedIn"`
	User     string `json:"user,omitempty"`
	// QR carries the current QR login code as block-art text while a QR
	// login is pending, so the dialog can redraw it when Steam refreshes it.
	QR string `json:"qr,omitempty"`
}

// AuthEvent reports a plugin's login state after a background refresh found
// it changed (issue #28): the Plugins view asked once, shows the cached
// state, and updates when this arrives on the "auth" event.
type AuthEvent struct {
	Plugin string     `json:"plugin"`
	Status AuthStatus `json:"status"`
}

// AuthPrompt is what the login dialog shows: a page to open, if any, a QR
// code to scan, if any, then the fields to fill in. QR holds the code as
// block-art text ("\u2588" = dark module, two chars per module, one row per
// module) for the dialog to draw as square modules.
type AuthPrompt struct {
	URL          string      `json:"url,omitempty"`
	Instructions string      `json:"instructions,omitempty"`
	QR           string      `json:"qr,omitempty"` // QR code as block-art text
	Fields       []AuthField `json:"fields"`
}

type AuthField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Secret      bool   `json:"secret,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// LauncherSettings are Platypus-wide settings, persisted in settings.json.
type LauncherSettings struct {
	GamesFolder        string `json:"games_folder"` // store plugins install into <GamesFolder>/<plugin>
	DefaultGamesFolder string `json:"default_games_folder"`
	// CancelCleanupDays is how long a canceled download's partial data is
	// kept before Platypus deletes it; 0 deletes it right away.
	CancelCleanupDays int `json:"cancel_cleanup_days"`
	// RatingsOnline is the "Fetch ratings online" switch (see compat.go).
	RatingsOnline bool `json:"ratings_online"`
	// ArtworkOnline is the "Find artwork online" switch for sideloaded games
	// (issue #36): the background lookup after an add and on-demand search.
	ArtworkOnline bool `json:"artwork_online"`
	// GameInfoOnline is the "Find game info online" switch for sideloaded
	// games (issue #38): the on-demand description and details lookup.
	GameInfoOnline bool `json:"game_info_online"`
	// ChatDesktopNotify is the "Desktop notifications for chat" switch (issue
	// #12): a system notification for a new chat message when the window
	// isn't focused. In-app toasts are unaffected.
	ChatDesktopNotify bool `json:"chat_desktop_notify"`
	// PrefixesFolder is where Wine prefixes are created, one folder per
	// game (see runner.PrefixPath). Empty means the default below.
	PrefixesFolder string `json:"prefixes_folder"`
	// DefaultPrefixesFolder is <data>/prefixes, used when PrefixesFolder
	// is empty.
	DefaultPrefixesFolder string `json:"default_prefixes_folder"`
	// Language is the UI language the user picked (issue #39): "" means
	// System, resolved in the shell from the OS languages.
	Language string `json:"language"`
	// MinimizeToTray is the "Minimize to tray while a game runs" switch
	// (issue #49), off by default. The shell reads it and hides the window
	// to the tray while a game session runs.
	MinimizeToTray bool `json:"minimize_to_tray"`
	// Theme is the picked theme's id (issue #93): "" is the built-in default.
	Theme string `json:"theme"`
}

// ThemeInfo is one theme in the Settings picker (issue #93). Status is "ok"
// (can be picked), "newer" (needs a newer Platypus) or "invalid" (its
// theme.toml can't be used). Warnings list what was ignored or fell back.
type ThemeInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Schema   int      `json:"schema"`
	Supports []string `json:"supports"`
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
}

// ThemeTokens is a theme's resolved tokens (issue #93): every token has a
// value, the built-in one where the theme's is missing or invalid. The UI sets
// them as CSS custom properties.
type ThemeTokens struct {
	ID       string            `json:"id"`
	Tokens   map[string]string `json:"tokens"`
	Wash     []WashLayer       `json:"wash"`
	Variants ThemeVariants     `json:"variants"`
}

// ThemeVariants gives the generated UI bridge a closed set of catalog axes.
type ThemeVariants struct {
	Settings   ThemeSettingsVariants   `json:"settings"`
	Section    ThemeSectionVariants    `json:"section"`
	ViewHeader ThemeViewHeaderVariants `json:"viewHeader"`
	EmptyState ThemeEmptyStateVariants `json:"emptyState"`
	ListRow    ThemeListRowVariants    `json:"listRow"`
}

type ThemeSettingsVariants struct {
	Presentation string `json:"presentation"`
	Navigation   string `json:"navigation"`
}

type ThemeSectionVariants struct {
	Chrome string `json:"chrome"`
}

type ThemeViewHeaderVariants struct {
	Actions  string `json:"actions"`
	Subtitle string `json:"subtitle"`
	Size     string `json:"size"`
}

type ThemeEmptyStateVariants struct {
	Chrome string `json:"chrome"`
	Icon   string `json:"icon"`
}

type ThemeListRowVariants struct {
	Media   string `json:"media"`
	Density string `json:"density"`
}

type ThemeCatalogEntry struct {
	Entry string             `json:"entry"`
	Since string             `json:"since"`
	Axes  []ThemeCatalogAxis `json:"axes"`
	// Kind is "block" or "region" for layout entries (issue #96); older
	// component and container entries leave it empty.
	Kind string `json:"kind,omitempty"`
	// Required marks a block the layout can't hide: a layout that leaves it
	// out still gets it, in Region, with a warning.
	Required bool   `json:"required,omitempty"`
	Region   string `json:"region,omitempty"`
	// Attrs lists the per-block layout attributes the catalog declares (e.g.
	// "limit" on the activity feed). Anything else is ignored with a warning.
	Attrs []string `json:"attrs,omitempty"`
}

type ThemeCatalogAxis struct {
	Name    string   `json:"name"`
	Values  []string `json:"values"`
	Default string   `json:"default"`
	Since   string   `json:"since"`
}

// WashLayer is one radial gradient of the background wash: where it sits,
// how big it is, its colour, and where it fades out.
type WashLayer struct {
	At    string `json:"at"`
	Size  string `json:"size"`
	Color string `json:"color"`
	Fade  string `json:"fade"`
}

// ThemeLayout is a view's validated block layout (issue #96): the regions in
// the order the theme lists them, each with its blocks and rows. The UI maps
// the names onto the view's block components; a nil layout means the theme
// ships none and the UI uses its built-in arrangement.
type ThemeLayout struct {
	View    string              `json:"view"`
	Regions []ThemeLayoutRegion `json:"regions"`
}

// ThemeLayoutRegion is one named place in a view (e.g. "main", "side").
type ThemeLayoutRegion struct {
	Name  string            `json:"name"`
	Nodes []ThemeLayoutNode `json:"nodes"`
}

// ThemeLayoutNode is one block in a region, or one row grouping blocks
// side by side. Exactly one of Block and Row is set.
type ThemeLayoutNode struct {
	Block string            `json:"block,omitempty"`
	Attrs map[string]string `json:"attrs,omitempty"`
	Row   []ThemeLayoutNode `json:"row,omitempty"`
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
	// Compat facts from the plugin (protocol 1.10): the Steam AppID hint, the
	// store's platforms and DRM. Nil when the plugin says nothing.
	SteamAppID string   `json:"steam_app_id,omitempty"`
	Platforms  []string `json:"platforms,omitempty"`
	DRM        *DRM     `json:"drm,omitempty"`
	// Media and richer facts (protocol 1.12): sanitized HTML for the
	// description and requirements, links, languages, controller support,
	// age rating, tags and videos. The UI renders DescriptionHTML and
	// Requirements as markup; core sanitizes them (internal/sanitize) so
	// only the allowlist reaches the page.
	DescriptionHTML string `json:"description_html,omitempty"`
	Website         string `json:"website,omitempty"`
	StoreURL        string `json:"store_url,omitempty"`
	// InfoSource is the online source of a sideloaded game's details
	// (issue #38): "steam", "gog" or "lutris". Empty for store games and
	// for sideloaded games with no linked info. InfoSourceName is the
	// store name for the attribution line ("Steam", "GOG", "Lutris / IGDB").
	InfoSource     string `json:"info_source,omitempty"`
	InfoSourceName string `json:"info_source_name,omitempty"`
	// InfoStale marks details served from the cache while offline
	// (issue #44): the fetch was skipped, so the info may be out of date.
	InfoStale    bool              `json:"info_stale,omitempty"`
	Languages    []string          `json:"languages,omitempty"`
	Controller   string            `json:"controller,omitempty"` // "full" | "partial" | ""
	Requirements *GameRequirements `json:"requirements,omitempty"`
	AgeRating    string            `json:"age_rating,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Videos       []GameVideo       `json:"videos,omitempty"`
}

// GameRequirements are the store's system requirements, sanitized HTML or
// plain text, as the game page's min/recommended tabs show them.
type GameRequirements struct {
	Minimum     string `json:"minimum,omitempty"`
	Recommended string `json:"recommended,omitempty"`
}

// GameVideo is one trailer or clip of the game page's media strip.
type GameVideo struct {
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Title     string `json:"title,omitempty"`
	MIME      string `json:"mime,omitempty"`
}

// DRM is the store's protection facts for a game (see plugins.DRM).
type DRM struct {
	DRMFree        *bool  `json:"drm_free,omitempty"`
	OnlineRequired *bool  `json:"online_required,omitempty"`
	Launcher       string `json:"launcher,omitempty"`
	DenuvoLikely   *bool  `json:"denuvo_likely,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// Tweak is a launch setting Platypus suggests for every game, based on the
// detected hardware and session, independent of the ProtonDB tier.
type Tweak struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Reason   string            `json:"reason"`
	Kind     string            `json:"kind"` // "recommended" | "troubleshoot"
	Env      map[string]string `json:"env"`
	Wrappers []string          `json:"wrappers,omitempty"` // "gamemode", "mangohud"
}

// SessionEvent is emitted on the "session" core event when a launched game
// changes state.
type SessionEvent struct {
	GameID  string `json:"game_id"`
	State   string `json:"state"` // "starting" | "running" | "stopped" | "failed"
	PID     int    `json:"pid,omitempty"`
	Message string `json:"message,omitempty"`
}

// NetState is what GetNetState returns (issue #44): whether the machine
// looks online, and when the current state began (unix seconds, 0 when the
// state never changed). The UI shows a small "Offline" indicator in the
// library toolbar while offline.
type NetState struct {
	Online bool  `json:"online"`
	Since  int64 `json:"since"`
}

// Job is an install, update or uninstall run by a store plugin.
type Job struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"` // "install" | "update" | "uninstall"
	Source      string  `json:"source"`
	GameID      string  `json:"game_id"`       // Platypus's game id
	StoreGameID string  `json:"store_game_id"` // the plugin's game id
	Title       string  `json:"title"`
	State       string  `json:"state"` // "queued" | "running" | "paused" | "done" | "failed" | "canceled"
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
	// CanceledAt is when the user canceled the job (unix ms). Its partial
	// download is deleted CleanupAfter later; until then it can be resumed.
	CanceledAt int64 `json:"canceled_at,omitempty"`
	// CleanedAt is when the canceled job's leftovers were deleted, or
	// forgotten when the plugin can't delete them (unix ms; 0 = not yet).
	CleanedAt int64 `json:"cleaned_at,omitempty"`
	// Leftover says what may remain on disk after a cleanup the plugin
	// couldn't perform, or why the last cleanup attempt failed.
	Leftover string `json:"leftover,omitempty"`
	// Target is the version the install or update asked for (the `versions`
	// capability). It names a channel or build and never carries a password.
	Target *VersionTarget `json:"target,omitempty"`
	// Repair says a verify job restores the damaged files it finds (the
	// second run, after the user agreed to it).
	Repair bool `json:"repair,omitempty"`
	// Destination is where a move takes the game (InstallDir is where it is).
	Destination string `json:"destination,omitempty"`
	// DeletePrefix and DeleteFiles say what a remove job deletes: the
	// sideloaded game's Wine prefix and/or its game files (issue #43). At
	// least one is set. AllowNoSaves lets the save gate pass when no known
	// save paths apply (a remove, or an uninstall after the user checked
	// "delete without a save backup", issue #42).
	DeletePrefix bool `json:"delete_prefix,omitempty"`
	DeleteFiles  bool `json:"delete_files,omitempty"`
	AllowNoSaves bool `json:"allow_no_saves,omitempty"`
	// Verify is what a verify job found, once it is done.
	Verify *VerifyReport `json:"verify,omitempty"`
	// Options are the install-option choices the install or update sends
	// (the `install-options` capability), kept with the job like Target.
	Options InstallOptions `json:"options,omitempty"`
}

// VerifyReport is what a game's files check found. Checked counts the files
// looked at; Damaged and Missing are the ones that were wrong or absent (in a
// repair run, what is still wrong afterwards); Repaired is what a repair run
// restored.
type VerifyReport struct {
	Checked  int `json:"checked"`
	Damaged  int `json:"damaged"`
	Missing  int `json:"missing"`
	Repaired int `json:"repaired"`
}

// MoveCheck is what a move would do, before it is queued: where the game ends
// up, what it needs and what the destination has free. PluginMoves says the
// plugin does the move itself (game.move) rather than Platypus.
type MoveCheck struct {
	To          string `json:"to"`
	NeedBytes   int64  `json:"need_bytes"`
	FreeBytes   int64  `json:"free_bytes"`
	PluginMoves bool   `json:"plugin_moves"`
}

// RemoveOptions are what a sideloaded game's removal deletes (issue #43):
// its Wine prefix and/or its game files. Neither means library only (no
// job). AllowNoSaves answers the save gate when no known save paths apply;
// CheckRemove ignores it.
type RemoveOptions struct {
	DeletePrefix bool `json:"delete_prefix"`
	DeleteFiles  bool `json:"delete_files"`
	AllowNoSaves bool `json:"allow_no_saves"`
}

// RemoveTarget is one folder a removal would delete: its kind ("prefix" or
// "files"), path and size, whether it is there, and whether it may go (with
// the reason when it may not). ReasonCode is the apperr code behind Reason,
// so the UI renders errors:<code>; ReasonParams carries its values.
type RemoveTarget struct {
	Kind         string            `json:"kind"`
	Path         string            `json:"path"`
	Bytes        int64             `json:"bytes"`
	Exists       bool              `json:"exists"`
	Allowed      bool              `json:"allowed"`
	Reason       string            `json:"reason,omitempty"`
	ReasonCode   string            `json:"reason_code,omitempty"`
	ReasonParams map[string]string `json:"reason_params,omitempty"`
}

// RemoveCheck is what removing a sideloaded game would do, before it is
// queued. Saves previews the save gate (issue #42): with no known save
// paths the UI asks for an explicit confirmation.
type RemoveCheck struct {
	Targets []RemoveTarget `json:"targets"`
	Saves   SaveGate       `json:"saves"`
}

// PrefixGame is one game using a Wine prefix (issue #52): its id and title.
type PrefixGame struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// PrefixShare is one Wine prefix and the games using it (issue #52): the
// picker lists these, and the remove dialog names the sharers.
type PrefixShare struct {
	Path  string       `json:"path"`
	Games []PrefixGame `json:"games"`
}

// PrefixPreset is a named template for a Wine prefix (issue #52): DLL
// overrides, winetricks verbs, environment variables, a Windows version and
// launch arguments. Applying one merges the env and DLL overrides into the
// game's settings (the game's own values always win) and records the rest
// as ordinary per-game settings; winetricks verbs and the Windows version
// are stored and shown, never executed.
type PrefixPreset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Builtin marks the hardcoded read-only presets: they live in the
	// backend, not in settings.json, and can't be saved over or deleted.
	Builtin        bool              `json:"builtin"`
	DLLOverrides   map[string]string `json:"dll_overrides,omitempty"`   // e.g. {"xaudio2_7": "n,b"}
	Winetricks     []string          `json:"winetricks,omitempty"`      // e.g. ["d3dx9", "corefonts"]
	Env            map[string]string `json:"env,omitempty"`             // e.g. {"PROTON_ENABLE_NVAPI": "1"}
	WindowsVersion string            `json:"windows_version,omitempty"` // win7/win8/win81/win10/win11, "" means win10
	LaunchArgs     string            `json:"launch_args,omitempty"`     // e.g. "-novid"
}

// PrefixApplyResult is what applying a preset did (issue #52): the merged
// env vars and DLL overrides, the preset env keys the game's own values
// kept, and the recorded pending winetricks verbs and Windows version.
type PrefixApplyResult struct {
	Env            map[string]string `json:"env"`
	Overrides      string            `json:"overrides"`
	Skipped        []string          `json:"skipped"`
	Verbs          []string          `json:"verbs"`
	WindowsVersion string            `json:"windows_version"`
}

// Relocation says core moved a game's folder: the folder the plugin reports
// (From) and where the game is now (To). It is kept until the plugin reports
// the new folder itself (game.relocated) or the game is moved again.
type Relocation struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// MigrationNotice is the one-time notice shown after the Hydra→Platypus data
// move (internal/migrate, issue #14): what happened and what to do. Nil (from
// MigrationNotice) means there is nothing to show.
type MigrationNotice struct {
	// Moved: the old folder was renamed. Copied: it was copied and the old
	// one kept. One of them is always true when a notice exists.
	Moved  bool `json:"moved"`
	Copied bool `json:"copied"`
	// SetAside lists plugin builds that predate the rename, moved out of
	// plugins/ until they are reinstalled and approved again.
	SetAside []string `json:"set_aside"`
	// Warnings are things that did not finish (failed copies, paths not
	// rewritten), in plain words.
	Warnings []string `json:"warnings"`
}

// InstallLocation is a place the install dialog offers: Folder is the install
// folder the plugin is sent (games land in a folder of their own inside it).
// Kind is "default" (the store's install folder), "library" (inside one of the
// user's game folders; Root names it) or "recent" (a folder picked by hand
// before). FreeBytes is -1 when the disk can't be read.
type InstallLocation struct {
	Folder    string `json:"folder"`
	Kind      string `json:"kind"`
	Root      string `json:"root,omitempty"`
	FreeBytes int64  `json:"free_bytes"`
	Exists    bool   `json:"exists"`
}

// InstallFolderCheck is what Platypus found out about a folder the user picked
// to install into, before anything is queued.
type InstallFolderCheck struct {
	Folder    string `json:"folder"`
	FreeBytes int64  `json:"free_bytes"`
	Exists    bool   `json:"exists"`
}

// GameFolder is one of the user's library folders (Settings → Game folders).
type GameFolder struct {
	Path      string `json:"path"`
	IsDefault bool   `json:"is_default"`
	Exists    bool   `json:"exists"`
	FreeBytes int64  `json:"free_bytes"` // -1 when the disk can't be read
}

// DiskUsage is the snapshot behind the Storage view (issue #50): one entry
// per drive holding game folders, and every installed game by size. Scanning
// says a background pass is still measuring; GeneratedAt is unix seconds.
type DiskUsage struct {
	Drives      []DriveUsage `json:"drives"`
	Games       []GameSize   `json:"games"`
	Scanning    bool         `json:"scanning"`
	GeneratedAt int64        `json:"generated_at"`
}

// DriveUsage is one drive's share: what Platypus games use (GamesBytes, from
// the game files and prefixes measured separately), what else is on it
// (OtherBytes, derived from the filesystem totals, never scanned) and what
// is free.
type DriveUsage struct {
	ID         string   `json:"id"`
	Mount      string   `json:"mount"`
	Folders    []string `json:"folders"`
	TotalBytes int64    `json:"total_bytes"` // -1 when the disk can't be read
	FreeBytes  int64    `json:"free_bytes"`  // -1 when the disk can't be read
	GamesBytes int64    `json:"games_bytes"`
	OtherBytes int64    `json:"other_bytes"`
}

// GameSize is one installed game's disk share. FromPlugin means the figure
// is what the store reported (no walk); Measured means Platypus walked the
// disk, which outranks the store. PrefixBytes is the Wine prefix measured
// separately (0 for native games).
type GameSize struct {
	GameID      string    `json:"game_id"`
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	Store       StoreType `json:"store"`
	InstallDir  string    `json:"install_dir"`
	SizeBytes   int64     `json:"size_bytes"`
	PrefixBytes int64     `json:"prefix_bytes"`
	FromPlugin  bool      `json:"from_plugin"`
	Measured    bool      `json:"measured"`
}
