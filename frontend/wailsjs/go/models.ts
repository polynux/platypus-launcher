export namespace chat {
	
	export class Message {
	    id: string;
	    from: string;
	    text: string;
	    sentAt: number;
	    status?: string;
	    clientId?: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.from = source["from"];
	        this.text = source["text"];
	        this.sentAt = source["sentAt"];
	        this.status = source["status"];
	        this.clientId = source["clientId"];
	    }
	}
	export class View {
	    seq: number;
	    plugin: string;
	    friendId: string;
	    open: boolean;
	    loaded: boolean;
	    more: boolean;
	    unread: number;
	    typing: boolean;
	    messages: Message[];
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.plugin = source["plugin"];
	        this.friendId = source["friendId"];
	        this.open = source["open"];
	        this.loaded = source["loaded"];
	        this.more = source["more"];
	        this.unread = source["unread"];
	        this.typing = source["typing"];
	        this.messages = this.convertValues(source["messages"], Message);
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

export namespace social {
	
	export class Connection {
	    state: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}
	export class Game {
	    id?: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	    }
	}
	export class Friend {
	    id: string;
	    name: string;
	    avatarUrl?: string;
	    state: string;
	    game?: Game;
	    lastSeen?: number;
	
	    static createFrom(source: any = {}) {
	        return new Friend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.avatarUrl = source["avatarUrl"];
	        this.state = source["state"];
	        this.game = this.convertValues(source["game"], Game);
	        this.lastSeen = source["lastSeen"];
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
	
	export class Self {
	    name: string;
	    avatarUrl?: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Self(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.avatarUrl = source["avatarUrl"];
	        this.state = source["state"];
	    }
	}
	export class View {
	    seq: number;
	    plugin: string;
	    enabled: boolean;
	    showPlaying: boolean;
	    running: boolean;
	    connection: Connection;
	    self?: Self;
	    friends: Friend[];
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.plugin = source["plugin"];
	        this.enabled = source["enabled"];
	        this.showPlaying = source["showPlaying"];
	        this.running = source["running"];
	        this.connection = this.convertValues(source["connection"], Connection);
	        this.self = this.convertValues(source["self"], Self);
	        this.friends = this.convertValues(source["friends"], Friend);
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
	export class ProgressAchievement {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    hidden: boolean;
	    unlocked: boolean;
	    unlocked_at: number;
	    global_percent?: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressAchievement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.hidden = source["hidden"];
	        this.unlocked = source["unlocked"];
	        this.unlocked_at = source["unlocked_at"];
	        this.global_percent = source["global_percent"];
	    }
	}
	export class AchievementProgress {
	    total: number;
	    unlocked: number;
	    list: ProgressAchievement[];
	
	    static createFrom(source: any = {}) {
	        return new AchievementProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.unlocked = source["unlocked"];
	        this.list = this.convertValues(source["list"], ProgressAchievement);
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
	export class CloudClient {
	    running: boolean;
	    name: string;
	    canLaunch?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CloudClient(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.name = source["name"];
	        this.canLaunch = source["canLaunch"];
	    }
	}
	export class CloudFile {
	    size: number;
	    mtime: number;
	    machine?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.size = source["size"];
	        this.mtime = source["mtime"];
	        this.machine = source["machine"];
	    }
	}
	export class CloudConflict {
	    path: string;
	    local: CloudFile;
	    remote: CloudFile;
	
	    static createFrom(source: any = {}) {
	        return new CloudConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.local = this.convertValues(source["local"], CloudFile);
	        this.remote = this.convertValues(source["remote"], CloudFile);
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
	export class CloudCounts {
	    same: number;
	    download: number;
	    upload: number;
	    conflict: number;
	    unmapped: number;
	
	    static createFrom(source: any = {}) {
	        return new CloudCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.same = source["same"];
	        this.download = source["download"];
	        this.upload = source["upload"];
	        this.conflict = source["conflict"];
	        this.unmapped = source["unmapped"];
	    }
	}
	
	export class CloudStatus {
	    state: string;
	    message?: string;
	    detail?: string;
	    counts: CloudCounts;
	    conflicts?: CloudConflict[];
	    checkedAt?: number;
	    backupDir?: string;
	    stamp?: string;
	    client?: CloudClient;
	
	    static createFrom(source: any = {}) {
	        return new CloudStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.counts = this.convertValues(source["counts"], CloudCounts);
	        this.conflicts = this.convertValues(source["conflicts"], CloudConflict);
	        this.checkedAt = source["checkedAt"];
	        this.backupDir = source["backupDir"];
	        this.stamp = source["stamp"];
	        this.client = this.convertValues(source["client"], CloudClient);
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
	export class CloudInfo {
	    mode: string;
	    plugin_mode: string;
	    game_mode: string;
	    status?: CloudStatus;
	    busy?: string;
	    running: boolean;
	    skipped?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.plugin_mode = source["plugin_mode"];
	        this.game_mode = source["game_mode"];
	        this.status = this.convertValues(source["status"], CloudStatus);
	        this.busy = source["busy"];
	        this.running = source["running"];
	        this.skipped = source["skipped"];
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
	export class CloudSettings {
	    plugins: Record<string, string>;
	    games: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new CloudSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plugins = source["plugins"];
	        this.games = source["games"];
	    }
	}
	
	export class WikiCredit {
	    title: string;
	    url: string;
	    fetched_at: number;
	    licence_url: string;
	
	    static createFrom(source: any = {}) {
	        return new WikiCredit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	        this.fetched_at = source["fetched_at"];
	        this.licence_url = source["licence_url"];
	    }
	}
	export class CompatBadge {
	    kind: string;
	    label: string;
	    detail: string;
	    source: string;
	    checked_at: number;
	
	    static createFrom(source: any = {}) {
	        return new CompatBadge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.source = source["source"];
	        this.checked_at = source["checked_at"];
	    }
	}
	export class ProtonRating {
	    tier: string;
	    confidence: string;
	    total: number;
	    fetched_at: number;
	    stale?: boolean;
	    no_reports?: boolean;
	    few_reports?: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtonRating(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tier = source["tier"];
	        this.confidence = source["confidence"];
	        this.total = source["total"];
	        this.fetched_at = source["fetched_at"];
	        this.stale = source["stale"];
	        this.no_reports = source["no_reports"];
	        this.few_reports = source["few_reports"];
	        this.url = source["url"];
	    }
	}
	export class Compat {
	    steam_app_id?: string;
	    steam_app_id_source?: string;
	    native: boolean;
	    protondb?: ProtonRating;
	    ratings_online: boolean;
	    drm: CompatBadge[];
	    drm_notes?: string;
	    anticheat: CompatBadge[];
	    wiki?: WikiCredit;
	
	    static createFrom(source: any = {}) {
	        return new Compat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steam_app_id = source["steam_app_id"];
	        this.steam_app_id_source = source["steam_app_id_source"];
	        this.native = source["native"];
	        this.protondb = this.convertValues(source["protondb"], ProtonRating);
	        this.ratings_online = source["ratings_online"];
	        this.drm = this.convertValues(source["drm"], CompatBadge);
	        this.drm_notes = source["drm_notes"];
	        this.anticheat = this.convertValues(source["anticheat"], CompatBadge);
	        this.wiki = this.convertValues(source["wiki"], WikiCredit);
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
	
	export class CopyDiff {
	    owner_id: string;
	    only_here: number;
	    only_baseline: number;
	    shared: number;
	    soundtracks_only_here: number;
	    soundtracks_only_baseline: number;
	    soundtracks_shared: number;
	
	    static createFrom(source: any = {}) {
	        return new CopyDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner_id = source["owner_id"];
	        this.only_here = source["only_here"];
	        this.only_baseline = source["only_baseline"];
	        this.shared = source["shared"];
	        this.soundtracks_only_here = source["soundtracks_only_here"];
	        this.soundtracks_only_baseline = source["soundtracks_only_baseline"];
	        this.soundtracks_shared = source["soundtracks_shared"];
	    }
	}
	export class CopyComparison {
	    baseline: string;
	    diffs: CopyDiff[];
	
	    static createFrom(source: any = {}) {
	        return new CopyComparison(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseline = source["baseline"];
	        this.diffs = this.convertValues(source["diffs"], CopyDiff);
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
	
	export class Customization {
	    title: string;
	    sort_title: string;
	    art: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Customization(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.sort_title = source["sort_title"];
	        this.art = source["art"];
	    }
	}
	export class DRM {
	    drm_free?: boolean;
	    online_required?: boolean;
	    launcher?: string;
	    denuvo_likely?: boolean;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new DRM(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.drm_free = source["drm_free"];
	        this.online_required = source["online_required"];
	        this.launcher = source["launcher"];
	        this.denuvo_likely = source["denuvo_likely"];
	        this.notes = source["notes"];
	    }
	}
	export class FamilyMember {
	    id: string;
	    name: string;
	    role: string;
	    avatar_url?: string;
	    state?: string;
	
	    static createFrom(source: any = {}) {
	        return new FamilyMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.avatar_url = source["avatar_url"];
	        this.state = source["state"];
	    }
	}
	export class GameDLC {
	    id: string;
	    name: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new GameDLC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	    }
	}
	export class GameCopy {
	    owner_id: string;
	    owner_name: string;
	    dlc?: GameDLC[];
	    note?: string;
	    available: boolean;
	    in_use?: boolean;
	    exclude?: string;
	
	    static createFrom(source: any = {}) {
	        return new GameCopy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner_id = source["owner_id"];
	        this.owner_name = source["owner_name"];
	        this.dlc = this.convertValues(source["dlc"], GameDLC);
	        this.note = source["note"];
	        this.available = source["available"];
	        this.in_use = source["in_use"];
	        this.exclude = source["exclude"];
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
	export class SharedBy {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SharedBy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
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
	    playtime_local_minutes?: number;
	    playtime_store_minutes?: number;
	    size_bytes?: number;
	    version?: string;
	    update_available?: boolean;
	    installable?: boolean;
	    note?: string;
	    outside_folder?: boolean;
	    native?: boolean;
	    features: string[];
	    pinned?: VersionPin;
	    shared_by?: SharedBy;
	    copies?: GameCopy[];
	    preferred_copy?: string;
	    copy_compare?: CopyComparison;
	    store_title?: string;
	    sort_title?: string;
	    logo_url?: string;
	    icon_url?: string;
	    background_url?: string;
	    customized?: string[];
	
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
	        this.playtime_local_minutes = source["playtime_local_minutes"];
	        this.playtime_store_minutes = source["playtime_store_minutes"];
	        this.size_bytes = source["size_bytes"];
	        this.version = source["version"];
	        this.update_available = source["update_available"];
	        this.installable = source["installable"];
	        this.note = source["note"];
	        this.outside_folder = source["outside_folder"];
	        this.native = source["native"];
	        this.features = source["features"];
	        this.pinned = this.convertValues(source["pinned"], VersionPin);
	        this.shared_by = this.convertValues(source["shared_by"], SharedBy);
	        this.copies = this.convertValues(source["copies"], GameCopy);
	        this.preferred_copy = source["preferred_copy"];
	        this.copy_compare = this.convertValues(source["copy_compare"], CopyComparison);
	        this.store_title = source["store_title"];
	        this.sort_title = source["sort_title"];
	        this.logo_url = source["logo_url"];
	        this.icon_url = source["icon_url"];
	        this.background_url = source["background_url"];
	        this.customized = source["customized"];
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
	    steam_app_id?: string;
	    platforms?: string[];
	    drm?: DRM;
	
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
	        this.steam_app_id = source["steam_app_id"];
	        this.platforms = source["platforms"];
	        this.drm = this.convertValues(source["drm"], DRM);
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
	
	export class GameFolder {
	    path: string;
	    is_default: boolean;
	    exists: boolean;
	    free_bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new GameFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.is_default = source["is_default"];
	        this.exists = source["exists"];
	        this.free_bytes = source["free_bytes"];
	    }
	}
	export class OptionValue {
	    id: string;
	    label?: string;
	    sizeBytes?: number;
	    owned: boolean;
	    installed?: boolean;
	    storeUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new OptionValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.sizeBytes = source["sizeBytes"];
	        this.owned = source["owned"];
	        this.installed = source["installed"];
	        this.storeUrl = source["storeUrl"];
	    }
	}
	export class OptionGroup {
	    id: string;
	    label?: string;
	    kind: string;
	    dlc?: boolean;
	    values?: OptionValue[];
	    default?: any;
	
	    static createFrom(source: any = {}) {
	        return new OptionGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.dlc = source["dlc"];
	        this.values = this.convertValues(source["values"], OptionValue);
	        this.default = source["default"];
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
	export class GameOptions {
	    groups: OptionGroup[];
	    chosen: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new GameOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], OptionGroup);
	        this.chosen = source["chosen"];
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
	
	export class GameTerm {
	    id: string;
	    title: string;
	    url?: string;
	    version: string;
	    accepted_at?: number;
	
	    static createFrom(source: any = {}) {
	        return new GameTerm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.version = source["version"];
	        this.accepted_at = source["accepted_at"];
	    }
	}
	export class InstallFolderCheck {
	    folder: string;
	    free_bytes: number;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstallFolderCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.free_bytes = source["free_bytes"];
	        this.exists = source["exists"];
	    }
	}
	export class InstallLocation {
	    folder: string;
	    kind: string;
	    root?: string;
	    free_bytes: number;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstallLocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.kind = source["kind"];
	        this.root = source["root"];
	        this.free_bytes = source["free_bytes"];
	        this.exists = source["exists"];
	    }
	}
	export class VerifyReport {
	    checked: number;
	    damaged: number;
	    missing: number;
	    repaired: number;
	
	    static createFrom(source: any = {}) {
	        return new VerifyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.damaged = source["damaged"];
	        this.missing = source["missing"];
	        this.repaired = source["repaired"];
	    }
	}
	export class VersionTarget {
	    channel?: string;
	    build?: string;
	    manual?: Record<string, string>;
	    archiveRef?: number[];
	
	    static createFrom(source: any = {}) {
	        return new VersionTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.build = source["build"];
	        this.manual = source["manual"];
	        this.archiveRef = source["archiveRef"];
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
	    repair?: boolean;
	    destination?: string;
	    verify?: VerifyReport;
	    options?: Record<string, any>;
	
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
	        this.repair = source["repair"];
	        this.destination = source["destination"];
	        this.verify = this.convertValues(source["verify"], VerifyReport);
	        this.options = source["options"];
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
	    ratings_online: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LauncherSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games_folder = source["games_folder"];
	        this.default_games_folder = source["default_games_folder"];
	        this.cancel_cleanup_days = source["cancel_cleanup_days"];
	        this.ratings_online = source["ratings_online"];
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
	export class MoveCheck {
	    to: string;
	    need_bytes: number;
	    free_bytes: number;
	    plugin_moves: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MoveCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.to = source["to"];
	        this.need_bytes = source["need_bytes"];
	        this.free_bytes = source["free_bytes"];
	        this.plugin_moves = source["plugin_moves"];
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
	export class RedeemResult {
	    message: string;
	    titles: string[];
	
	    static createFrom(source: any = {}) {
	        return new RedeemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.titles = source["titles"];
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
	export class SettingOption {
	    id: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class SettingField {
	    id: string;
	    kind: string;
	    label: string;
	    description?: string;
	    scope: string;
	    default: any;
	    options?: SettingOption[];
	    placeholder?: string;
	    min?: number;
	    max?: number;
	
	    static createFrom(source: any = {}) {
	        return new SettingField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.scope = source["scope"];
	        this.default = source["default"];
	        this.options = this.convertValues(source["options"], SettingOption);
	        this.placeholder = source["placeholder"];
	        this.min = source["min"];
	        this.max = source["max"];
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
	export class SettingGroup {
	    id: string;
	    label: string;
	    description?: string;
	    fields: SettingField[];
	
	    static createFrom(source: any = {}) {
	        return new SettingGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.fields = this.convertValues(source["fields"], SettingField);
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
	
	export class SettingsSaved {
	    applied: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsSaved(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applied = source["applied"];
	        this.message = source["message"];
	    }
	}
	export class SettingsView {
	    groups: SettingGroup[];
	    values: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new SettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], SettingGroup);
	        this.values = source["values"];
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
	export class TermRef {
	    id: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new TermRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
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

