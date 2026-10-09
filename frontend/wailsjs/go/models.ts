export namespace types {
	
	export class Achievement {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    unlocked: boolean;
	    unlocked_at: number;
	    global_percent: number;
	
	    static createFrom(source: any = {}) {
	        return new Achievement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.unlocked = source["unlocked"];
	        this.unlocked_at = source["unlocked_at"];
	        this.global_percent = source["global_percent"];
	    }
	}
	export class Achievements {
	    total: number;
	    unlocked: number;
	    list: Achievement[];
	
	    static createFrom(source: any = {}) {
	        return new Achievements(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.unlocked = source["unlocked"];
	        this.list = this.convertValues(source["list"], Achievement);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuthField {
	    id: string;
	    label: string;
	    secret?: boolean;
	    placeholder?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.secret = source["secret"];
	        this.placeholder = source["placeholder"];
	    }
	}
	export class AuthPrompt {
	    url?: string;
	    instructions?: string;
	    qr?: string;
	    fields: AuthField[];
	
	    static createFrom(source: any = {}) {
	        return new AuthPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.instructions = source["instructions"];
	        this.qr = source["qr"];
	        this.fields = this.convertValues(source["fields"], AuthField);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuthStatus {
	    loggedIn: boolean;
	    user?: string;
	    qr?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.user = source["user"];
	        this.qr = source["qr"];
	    }
	}
	export class VersionPin {
	    kind: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionPin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	    }
	}
	export class GameFixInfo {
	    source: string;
	    tier: string;
	    description: string;
	    winetricks: string[];
	    dll_overrides: Record<string, string>;
	    launch_args: string;
	
	    static createFrom(source: any = {}) {
	        return new GameFixInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.tier = source["tier"];
	        this.description = source["description"];
	        this.winetricks = source["winetricks"];
	        this.dll_overrides = source["dll_overrides"];
	        this.launch_args = source["launch_args"];
	    }
	}
	export class GameSettings {
	    runner_id: string;
	    wine_prefix: string;
	    launch_arguments: string;
	    env_vars: Record<string, string>;
	    use_gamemode: boolean;
	    use_mangohud: boolean;
	    use_gamescope: boolean;
	    gamescope_w: number;
	    gamescope_h: number;
	    gamescope_r: number;
	    gamescope_args: string;
	    eac_runtime: boolean;
	    battleye_runtime: boolean;
	    working_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new GameSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runner_id = source["runner_id"];
	        this.wine_prefix = source["wine_prefix"];
	        this.launch_arguments = source["launch_arguments"];
	        this.env_vars = source["env_vars"];
	        this.use_gamemode = source["use_gamemode"];
	        this.use_mangohud = source["use_mangohud"];
	        this.use_gamescope = source["use_gamescope"];
	        this.gamescope_w = source["gamescope_w"];
	        this.gamescope_h = source["gamescope_h"];
	        this.gamescope_r = source["gamescope_r"];
	        this.gamescope_args = source["gamescope_args"];
	        this.eac_runtime = source["eac_runtime"];
	        this.battleye_runtime = source["battleye_runtime"];
	        this.working_dir = source["working_dir"];
	    }
	}
	export class Game {
	    id: string;
	    source: string;
	    store: string;
	    store_app_id: string;
	    title: string;
	    cover_url: string;
	    banner_url: string;
	    hero_url: string;
	    install_dir: string;
	    executable: string;
	    is_installed: boolean;
	    settings: GameSettings;
	    settings_extra?: Record<string, any>;
	    fix_info?: GameFixInfo;
	    last_played_at?: number;
	    playtime_minutes?: number;
	    size_bytes?: number;
	    version?: string;
	    update_available?: boolean;
	    installable?: boolean;
	    note?: string;
	    outside_folder?: boolean;
	    native?: boolean;
	    features: string[];
	    pinned?: VersionPin;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.store = source["store"];
	        this.store_app_id = source["store_app_id"];
	        this.title = source["title"];
	        this.cover_url = source["cover_url"];
	        this.banner_url = source["banner_url"];
	        this.hero_url = source["hero_url"];
	        this.install_dir = source["install_dir"];
	        this.executable = source["executable"];
	        this.is_installed = source["is_installed"];
	        this.settings = this.convertValues(source["settings"], GameSettings);
	        this.settings_extra = source["settings_extra"];
	        this.fix_info = this.convertValues(source["fix_info"], GameFixInfo);
	        this.last_played_at = source["last_played_at"];
	        this.playtime_minutes = source["playtime_minutes"];
	        this.size_bytes = source["size_bytes"];
	        this.version = source["version"];
	        this.update_available = source["update_available"];
	        this.installable = source["installable"];
	        this.note = source["note"];
	        this.outside_folder = source["outside_folder"];
	        this.native = source["native"];
	        this.features = source["features"];
	        this.pinned = this.convertValues(source["pinned"], VersionPin);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameDetails {
	    summary: string;
	    screenshots: string[];
	    genres: string[];
	    developers: string[];
	    publishers: string[];
	    release_date: string;
	    hero_url?: string;
	    achievements?: Achievements;
	
	    static createFrom(source: any = {}) {
	        return new GameDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.screenshots = source["screenshots"];
	        this.genres = source["genres"];
	        this.developers = source["developers"];
	        this.publishers = source["publishers"];
	        this.release_date = source["release_date"];
	        this.hero_url = source["hero_url"];
	        this.achievements = this.convertValues(source["achievements"], Achievements);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class VersionTarget {
	    channel?: string;
	    build?: string;
	    manual?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new VersionTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.build = source["build"];
	        this.manual = source["manual"];
	    }
	}
	export class Job {
	    id: string;
	    kind: string;
	    source: string;
	    game_id: string;
	    store_game_id: string;
	    title: string;
	    state: string;
	    install_dir?: string;
	    phase?: string;
	    bytes: number;
	    total: number;
	    speed: number;
	    percent: number;
	    error?: string;
	    attempts: number;
	    created_at: number;
	    updated_at: number;
	    canceled_at?: number;
	    cleaned_at?: number;
	    leftover?: string;
	    target?: VersionTarget;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.game_id = source["game_id"];
	        this.store_game_id = source["store_game_id"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.install_dir = source["install_dir"];
	        this.phase = source["phase"];
	        this.bytes = source["bytes"];
	        this.total = source["total"];
	        this.speed = source["speed"];
	        this.percent = source["percent"];
	        this.error = source["error"];
	        this.attempts = source["attempts"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.canceled_at = source["canceled_at"];
	        this.cleaned_at = source["cleaned_at"];
	        this.leftover = source["leftover"];
	        this.target = this.convertValues(source["target"], VersionTarget);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LauncherSettings {
	    games_folder: string;
	    default_games_folder: string;
	    cancel_cleanup_days: number;
	
	    static createFrom(source: any = {}) {
	        return new LauncherSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games_folder = source["games_folder"];
	        this.default_games_folder = source["default_games_folder"];
	        this.cancel_cleanup_days = source["cancel_cleanup_days"];
	    }
	}
	export class LedgerEntry {
	    store: string;
	    game: string;
	    build: string;
	    channel?: string;
	    versionName?: string;
	    date: string;
	    archiveRef?: number[];
	
	    static createFrom(source: any = {}) {
	        return new LedgerEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.store = source["store"];
	        this.game = source["game"];
	        this.build = source["build"];
	        this.channel = source["channel"];
	        this.versionName = source["versionName"];
	        this.date = source["date"];
	        this.archiveRef = source["archiveRef"];
	    }
	}
	export class SourceWarning {
	    source: string;
	    message: string;
	    needs_auth?: boolean;
	    unavailable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SourceWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.message = source["message"];
	        this.needs_auth = source["needs_auth"];
	        this.unavailable = source["unavailable"];
	    }
	}
	export class Library {
	    games: Game[];
	    warnings: SourceWarning[];
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games = this.convertValues(source["games"], Game);
	        this.warnings = this.convertValues(source["warnings"], SourceWarning);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PluginCapability {
	    name: string;
	    on: boolean;
	    known: boolean;
	    orchestrated: boolean;
	    features: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new PluginCapability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.on = source["on"];
	        this.known = source["known"];
	        this.orchestrated = source["orchestrated"];
	        this.features = source["features"];
	    }
	}
	export class PluginRequirement {
	    bin: string;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginRequirement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bin = source["bin"];
	        this.hint = source["hint"];
	    }
	}
	export class ProtonVersion {
	    id: string;
	    name: string;
	    path: string;
	    is_system: boolean;
	    is_custom: boolean;
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProtonVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.is_system = source["is_system"];
	        this.is_custom = source["is_custom"];
	        this.is_default = source["is_default"];
	    }
	}
	export class SessionEvent {
	    game_id: string;
	    state: string;
	    pid?: number;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.state = source["state"];
	        this.pid = source["pid"];
	        this.message = source["message"];
	    }
	}
	
	export class StorePlugin {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    store: string;
	    kind?: string;
	    status: string;
	    problem?: string;
	    missing: PluginRequirement[];
	    requires: PluginRequirement[];
	    capabilities: string[];
	    declared: string[];
	    offers: PluginCapability[];
	    needs: string[];
	    warnings: string[];
	    initialized: boolean;
	    protocol?: string;
	    protocol_versions?: string;
	    transport?: string;
	    dir: string;
	    exec: string;
	    hash: string;
	    network: string[];
	    read: string[];
	    write: string[];
	    running: boolean;
	    gave_up?: boolean;
	    install_folder: string;
	    default_install_folder: string;
	    extra_folders: string[];
	
	    static createFrom(source: any = {}) {
	        return new StorePlugin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.store = source["store"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.problem = source["problem"];
	        this.missing = this.convertValues(source["missing"], PluginRequirement);
	        this.requires = this.convertValues(source["requires"], PluginRequirement);
	        this.capabilities = source["capabilities"];
	        this.declared = source["declared"];
	        this.offers = this.convertValues(source["offers"], PluginCapability);
	        this.needs = source["needs"];
	        this.warnings = source["warnings"];
	        this.initialized = source["initialized"];
	        this.protocol = source["protocol"];
	        this.protocol_versions = source["protocol_versions"];
	        this.transport = source["transport"];
	        this.dir = source["dir"];
	        this.exec = source["exec"];
	        this.hash = source["hash"];
	        this.network = source["network"];
	        this.read = source["read"];
	        this.write = source["write"];
	        this.running = source["running"];
	        this.gave_up = source["gave_up"];
	        this.install_folder = source["install_folder"];
	        this.default_install_folder = source["default_install_folder"];
	        this.extra_folders = source["extra_folders"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Tweak {
	    id: string;
	    title: string;
	    reason: string;
	    kind: string;
	    env: Record<string, string>;
	    wrappers?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Tweak(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	        this.kind = source["kind"];
	        this.env = source["env"];
	        this.wrappers = source["wrappers"];
	    }
	}
	export class VersionBuild {
	    id: string;
	    name?: string;
	    publishedAt?: string;
	    channel?: string;
	    source?: string;
	    available?: boolean;
	    current?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VersionBuild(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.publishedAt = source["publishedAt"];
	        this.channel = source["channel"];
	        this.source = source["source"];
	        this.available = source["available"];
	        this.current = source["current"];
	    }
	}
	export class VersionChannel {
	    id: string;
	    name?: string;
	    description?: string;
	    updatedAt?: string;
	    buildId?: string;
	    requires?: string;
	    unlocked?: boolean;
	    current?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VersionChannel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.updatedAt = source["updatedAt"];
	        this.buildId = source["buildId"];
	        this.requires = source["requires"];
	        this.unlocked = source["unlocked"];
	        this.current = source["current"];
	    }
	}
	export class VersionLink {
	    label: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.url = source["url"];
	    }
	}
	export class VersionManual {
	    label: string;
	    hint?: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionManual(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.hint = source["hint"];
	    }
	}
	export class VersionInfo {
	    channels: VersionChannel[];
	    builds: VersionBuild[];
	    manual?: VersionManual;
	    links: VersionLink[];
	    updateAvailable: boolean;
	    pinned?: VersionPin;
	    ledger: LedgerEntry[];
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channels = this.convertValues(source["channels"], VersionChannel);
	        this.builds = this.convertValues(source["builds"], VersionBuild);
	        this.manual = this.convertValues(source["manual"], VersionManual);
	        this.links = this.convertValues(source["links"], VersionLink);
	        this.updateAvailable = source["updateAvailable"];
	        this.pinned = this.convertValues(source["pinned"], VersionPin);
	        this.ledger = this.convertValues(source["ledger"], LedgerEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

