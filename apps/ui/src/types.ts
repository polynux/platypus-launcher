// The UI's vocabulary. Backend data shapes live in exactly one place: the
// generated bridge (./gen, from `go run ./cmd/gen-ts`). This module aliases
// those types so the UI imports one vocabulary, and adds only what the UI
// owns: view unions, frontend-only settings fields, and refinements the wire
// types as plain strings (store and status unions the UI switches on).
import type { chat, social, types as gen } from './gen/models';

// Storefronts Platypus knows by name; store plugins may report others.
export type StoreType = 'steam' | 'epic' | 'gog' | 'ubisoft' | 'custom' | (string & {});

export type ProtonVersion = gen.ProtonVersion;

export type SyncMode = 'default' | 'esync' | 'fsync' | 'ntsync' | 'off';
export type DxLayer = 'dxvk' | 'wined3d';
export type Upscaler = 'native' | 'fsr1';
export type HudPreset = 'off' | 'fps only' | 'minimal' | 'full';

export type GameSettings = gen.GameSettings & {
  // Frontend-side Proton controls (DESIGN §5). The backend has no fields for
  // these yet; they are folded into env_vars before preview/launch
  // (see lib/settings.ts), so the Go side receives real, honest env vars.
  proton_flags?: string[];
  sync_mode?: SyncMode;
  dx_layer?: DxLayer;
  upscaler?: Upscaler;
  mangohud_preset?: HudPreset;
  shader_cache?: 'on' | 'off';
  winetricks?: string[];
};

export type GameFixInfo = gen.GameFixInfo;
export type Achievement = gen.Achievement;
export type Achievements = gen.Achievements;
export type ProgressAchievement = gen.ProgressAchievement;
export type AchievementProgress = gen.AchievementProgress;
export type RedeemResult = gen.RedeemResult;
export type GameDetails = gen.GameDetails;
export type Tweak = gen.Tweak;

export type SessionState = 'starting' | 'running' | 'stopping' | 'stopped' | 'failed';
export type SessionEvent = gen.SessionEvent;

/** A game as the UI holds it: the bridge's game, with the UI's settings. `features`
 *  stays nullable here though the bridge types it as a list: Go sends null for
 *  an empty list, and null/absent means "defer to the plugin" while [] means
 *  "none" (see lib/capabilities). */
export type Game = Omit<gen.Game, 'settings' | 'features'> & {
  settings: GameSettings;
  features?: string[] | null;
};

/** The user's own status for a game (issue #51): '' means none. */
export type GameStatus = '' | 'backlog' | 'playing' | 'finished' | 'abandoned';

/** A game's status and notes as the setters answer them (issue #51): the pair
 *  the game page shows, so a save updates it without a rescan. */
export type GameStatusInfo = { status: GameStatus; user_notes: string };

/** A collection (issue #45): a manual list of game ids, or a smart filter over library facts. */
export type Collection = gen.Collection;
export type SmartFilter = gen.SmartFilter;

/** The kinds of art a game can be customized with. */
export type ArtKind = 'cover' | 'banner' | 'logo' | 'icon' | 'background';
export type Customization = gen.Customization;
/** One online art search hit: a Steam app whose art applies to a sideloaded game. */
export type ArtCandidate = gen.ArtCandidate;

export type InstallLocation = gen.InstallLocation;
export type InstallFolderCheck = gen.InstallFolderCheck;
export type GameFolder = gen.GameFolder;
export type DiskUsage = gen.DiskUsage;

export type SharedBy = gen.SharedBy;
export type GameCopy = gen.GameCopy;
export type GameDLC = gen.GameDLC;
export type CopyComparison = gen.CopyComparison;
export type CopyDiff = gen.CopyDiff;
export type GameTerm = gen.GameTerm;
export type TermRef = gen.TermRef;
export type FamilyMember = gen.FamilyMember;

/** A family.warning: a borrowed game is about to be taken back. */
export interface FamilyWarning {
  source: string;
  game_id: string;
  message: string;
  seconds_left: number;
}

/** A plugin setting's kind. Core drops kinds it doesn't know. */
export type SettingKind = 'bool' | 'select' | 'text' | 'number';
export type SettingScope = 'plugin' | 'game';
/** A value of a plugin setting: on/off, text, a number, or an option id. */
export type SettingValue = boolean | string | number;

export interface SettingOption {
  id: string;
  label: string;
}

/** One option a plugin offers (the `settings` capability). `default` is null
 *  when the plugin gave none. */
export interface SettingField {
  id: string;
  kind: SettingKind;
  label: string;
  description?: string;
  scope: SettingScope;
  default: SettingValue | null;
  options?: SettingOption[];
  placeholder?: string;
  min?: number;
  max?: number;
}

export interface SettingGroup {
  id: string;
  label: string;
  description?: string;
  fields: SettingField[];
}

/** One scope of a plugin's settings, with the values the user chose. */
export interface SettingsView {
  groups: SettingGroup[];
  values: Record<string, SettingValue>;
}

/** What a save of settings answers. The values are kept either way; `applied`
 *  is false when the plugin didn't take them now, and `message` says why. */
export type SettingsSaved = gen.SettingsSaved;

export type SourceWarning = gen.SourceWarning;
export type Library = gen.Library;

/** The one-time Hydra→Platypus migration notice (issue #14). Null when there
 *  is nothing to show. Go sends nil slices as null; normalize at the edge. */
export type MigrationNotice = gen.MigrationNotice;

export type PluginStatus =
  | 'ready' | 'invalid' | 'unapproved' | 'changed' | 'disabled' | 'missing-requirements'
  | 'needs-newer-platypus' | 'too-old';

export type PluginRequirement = gen.PluginRequirement;

/** One declared capability as the Plugins view lists it. */
export type PluginCapability = gen.PluginCapability;
export type StorePlugin = gen.StorePlugin;

export type AuthStatus = gen.AuthStatus;
export type AuthField = gen.AuthField;
export type AuthPrompt = gen.AuthPrompt;

export type JobKind = 'install' | 'update' | 'uninstall' | 'verify' | 'move' | 'backup' | 'restore' | 'remove';
/** The jobs a game page starts from its install panel (the rest have their own actions). */
export type GameJobKind = 'install' | 'update' | 'uninstall';
export type JobState = 'queued' | 'running' | 'paused' | 'done' | 'failed' | 'canceled';
export type Job = gen.Job;
export type VerifyReport = gen.VerifyReport;
export type MoveCheck = gen.MoveCheck;
export type SaveBackup = gen.SaveBackup;
export type SavePath = gen.SavePath;
export type BackupSettings = gen.BackupSettings;
export type RestorePreview = gen.RestorePreview;
/** Removing a sideloaded game: what to delete, what it would do, one target. */
export type RemoveOptions = gen.RemoveOptions;
export type RemoveCheck = gen.RemoveCheck;
export type RemoveTarget = gen.RemoveTarget;
/** One Wine prefix and the games using it (issue #52), for the prefix picker. */
export type PrefixShare = gen.PrefixShare;
/** A named Wine prefix template (issue #52) and what applying it did. */
export type PrefixPreset = gen.PrefixPreset;
export type PrefixApplyResult = gen.PrefixApplyResult;
/** What a backup-before-delete step did (issue #42). */
export type SaveGate = gen.SaveGate;
/** Whether after-session automatic backups run for a game and why. */
export type AutoBackupState = gen.AutoBackupState;

export type VersionTarget = gen.VersionTarget;
export type VersionChannel = gen.VersionChannel;
export type VersionBuildSource = 'store' | 'archive' | 'manual';
export type VersionBuild = gen.VersionBuild;
export type VersionLink = gen.VersionLink;
export type VersionPin = gen.VersionPin;
export type LedgerEntry = gen.LedgerEntry;
export type VersionInfo = gen.VersionInfo;

/** Platypus-wide settings, persisted in ~/.local/share/platypus/settings.json. */
export type LauncherSettings = gen.LauncherSettings;
export type ProtonRating = gen.ProtonRating;
export type CompatBadge = gen.CompatBadge;
export type WikiCredit = gen.WikiCredit;
export type Compat = gen.Compat;
export type CrashHint = gen.CrashHint;
export type CrashHints = gen.CrashHints;

export type ActivityType = 'session' | 'unlock' | 'fix' | 'update' | 'download' | 'add' | 'remove' | 'tweak' | 'cloud';

export interface ActivityEvent {
  id: number;
  ts: number; // epoch ms
  type: ActivityType;
  game?: string;
  detail: string;
  meta?: string;
}

export type View =
  | { name: 'dashboard' }
  // collection: the library shows one collection (id), or all games when absent.
  | { name: 'library'; collection?: string }
  | { name: 'game'; id: string; from: 'dashboard' | 'library' | 'fixes' }
  | { name: 'activity' }
  | { name: 'fixes' }
  | { name: 'runners' }
  | { name: 'storage' }
  | { name: 'plugins' }
  | { name: 'downloads' }
  | { name: 'settings' }
  // The embedded store view (issue #17): the plugin's official website in a
  // native view over the page. url is a deep link from a game page (inside
  // the plugin's domains); absent means the storefront.
  | { name: 'store'; plugin: string; url?: string };

// ---- Cloud saves (protocol 1.2; docs/plugins/protocol.md "Cloud saves") ----

export type CloudState = 'synced' | 'pending' | 'conflict' | 'unsupported' | 'offline' | 'error';
export type CloudMode = 'off' | 'ask' | 'auto';

export type CloudFile = gen.CloudFile;
export type CloudConflict = gen.CloudConflict;
export type CloudStatus = gen.CloudStatus;
export type CloudInfo = gen.CloudInfo;
export type CloudSettings = gen.CloudSettings;

export type CloudPrompt = 'conflict' | 'offline' | 'client-running';

/** The "cloud" core event. */
export interface CloudEvent {
  game_id: string;
  kind: 'progress' | 'status' | 'prompt' | 'pushed' | 'skipped' | 'failed' | 'canceled';
  prompt?: CloudPrompt;
  message?: string;
  status?: CloudStatus;
  job_id?: string;
  phase?: 'check' | 'download' | 'upload';
  done?: number;
  total?: number;
  /** The store client's name ("Steam"), for the client prompt and skips. */
  client?: string;
  /** The game's title, for a prompt about a game the library may not list. */
  title?: string;
}

// The install-options capability (protocol 1.5, docs/plugins/protocol.md,
// "Install options"). Core ignores groups of kinds it doesn't know.
export type OptionKind = 'select' | 'multi' | 'bool';

export interface OptionValue {
  id: string;
  label?: string;
  sizeBytes?: number;
  /** DLC the user doesn't own is shown greyed out and never installed. */
  owned: boolean;
  installed?: boolean;
  storeUrl?: string;
}

export interface OptionGroup {
  id: string;
  label?: string;
  kind: OptionKind;
  dlc?: boolean;
  values?: OptionValue[];
  /** A select's value id, or a bool's flag. */
  default?: string | boolean | null;
}

/** One entry per group: a value id (select), value ids (multi) or a flag (bool). */
export type InstallChoices = Record<string, string | string[] | boolean>;

export interface GameOptions {
  groups: OptionGroup[];
  chosen: InstallChoices;
}

// ---- Social: friends and own status (protocol 1.6; docs/plugins/protocol.md "Social") ----

export type FriendState = 'online' | 'offline' | 'away' | 'busy' | 'snooze' | 'looking' | 'invisible';
/** The statuses the user can set for themselves on a store. */
export type SocialStatus = 'online' | 'away' | 'busy' | 'snooze' | 'invisible';
export type SocialConnState = 'connecting' | 'online' | 'reconnecting' | 'replaced' | 'offline' | 'loggedOut';

export type SocialFriend = social.Friend;
export type SocialSelf = social.Self;
export type SocialView = social.View;

// ---- Chat: one-to-one messages with friends (protocol 1.9; docs/plugins/protocol.md "Chat") ----

export type ChatMessage = chat.Message;
export type ChatView = chat.View;

/** A conversation change; alert is true for a new message from a friend the user should hear about. */
export interface ChatEvent extends ChatView {
  alert: boolean;
  /** The backend evicted the conversation from memory: the UI drops its view. */
  dropped?: boolean;
}
