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
	    fix_info?: GameFixInfo;
	    last_played_at?: number;
	    playtime_minutes?: number;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
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
	        this.fix_info = this.convertValues(source["fix_info"], GameFixInfo);
	        this.last_played_at = source["last_played_at"];
	        this.playtime_minutes = source["playtime_minutes"];
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
	
	
	export class PluginInfo {
	    id: string;
	    name: string;
	    author: string;
	    version: string;
	    description: string;
	    enabled: boolean;
	    store: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.author = source["author"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.store = source["store"];
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

}

