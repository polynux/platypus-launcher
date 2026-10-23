# ADR 0001: a modular core, platform and compatibility specifics in plugins

- Status: accepted (2026-10-16)
- Issues: #73 (epic), #55 (compat providers), #74 (runners), #75 (audit)

## Context

Store plugins already keep each store out of the core. Everything else that is specific to Linux, to Wine/Proton
or to one data source is still compiled into the core:

| Area | Where | Size |
|---|---|---|
| ProtonDB, Steam Deck, AreWeAntiCheatYet, PCGamingWiki, local anti-cheat detection | `internal/compat` | ~3.3k lines; 48 call sites in `app_compat.go`, 14 in `app.go` |
| Runner discovery, selection, launch wrapping, prefixes, umu, wineserver, native/PE detection | `internal/runner`, `app_umu.go`, prefix code in `internal/app` | ~1.3k lines + app glue in 7 files |
| Crash hints and the winetricks fix | `internal/crashhints`, `internal/winetricks`, `app_crashhints.go` | small, but `winetricks` is also imported by `dataplug` and `settings` |
| Steam client specifics | `internal/steam` imported by `internal/app` and `internal/savebackup` | to audit (#75) |

The data model follows the same shape: `types.Compat` has `ProtonDB` and `Deck` fields, `GameFixInfo` carries the
Deck verdict for the cards, and smart collections (#45) filter on the ProtonDB tier and the Deck verdict by name.

The protocol doc says today: "Platypus owns the runner pipeline (Proton/Wine, gamemode, gamescope, MangoHud,
per-game settings)". This ADR proposes to reverse that for the Wine/Proton parts.

Goal (user, 2026-10-16): a mostly modular launcher, so that a future Windows port ships the same core without the
Linux-specific parts, and Linux users can drop or replace them.

## Decision (proposed)

### 1. New plugin kinds are capabilities, not new manifest types

A plugin is still a manifest + a process speaking the versioned JSON-RPC protocol, approved by the user and checked
by the conformance suite. What it provides is its capability list. Two new capabilities:

- `compat-facts`: facts about games from any store (ratings, verdicts, protection, links).
- `runner`: runs games on this machine (lists runners, wraps launch specs, prefix actions, crash hints).

`store` becomes optional in the manifest: a plugin with no store capability (`library`, `install`, …) provides no
games and needs no `store` id. Fixes/tweaks data plugins stay as they are (`docs/plugins/data-plugins.md`).

Why not a second plugin system: approval, hashing, bundling, process supervision, version negotiation and the
conformance suite already exist and are tested; a new kind should reuse all of it.

### 2. `compat-facts`: batched, cached by the core, generic in the UI

```
compat.lookup { games: [{ id, store, storeAppId, steamAppId?, platforms?, installDir? }] }
  -> { facts: [{ id, badges: [CompatBadge], links: [{label, url}], notes?, ttlSeconds }] }
```

- Batched: the library has hundreds of games; the core sends what changed, never one call per card.
- The core owns the cache (per plugin, per game, with the plugin's TTL), offline behaviour and background refresh,
  as `internal/compat` does today. Rendering never waits for a plugin.
- The UI shows badges generically: a badge has a `kind` (`rating`, `verdict`, `protection`, `note`, plus source
  kinds such as `drm`, `anticheat`, `drm-free`, `denuvo`, `launcher`, `online` and `drm-removed`), a label, a
  tone/tier for colour, a link and a source with its date; the existing `types.CompatBadge` is most of it.
- Smart filters filter on badges (`kind` + value) instead of named ProtonDB/Deck fields. Existing smart collections
  are migrated (a filter on tier "platinum" becomes a filter on the ProtonDB plugin's `rating` badge).
- Matching stays as today: by Steam AppID when known, never by title.
- Order: ProtonDB first (#55), then Steam Deck, then AreWeAntiCheatYet, PCGamingWiki and local anti-cheat detection.
- Step 4 completed (2026-10-16): all compatibility sources now run in the bundled `linux-compat` plugin; the core
  renders generic facts and owns the per-plugin cache. Store DRM hints remain core-side because they come from
  `game.details`.
- One bundle for now: a single bundled `linux-compat` plugin holds every source, each still credited on its own in
  its badges. Splitting it per source later is a packaging change, not a protocol one.

### 3. `runner`: the core runs processes, the runner plugin decides how

The core keeps: per-game settings storage, the launch flow, starting and stopping processes, session tracking,
playtime and logs. Without any runner plugin, the core runs a store's direct spec as is (a native game on Linux, any
game on Windows).

The runner plugin wraps native games too: gamemode, gamescope and MangoHud are Linux-only and move with Wine/Proton,
so `wrap` is called for every launch, native or not.

A runner plugin answers:

```
runner.list {} -> { runners: [{ id, name, kind, version, default? }] }
runner.options {} -> settings schema (same shape as the 1.8 `settings` capability)
runner.wrap { spec, game, runnerId, options, tweaks, prefixesFolder, preview } -> { command, env, cwd, find? }
runner.actions { game, runnerId } -> [{ id, label }]   +   runner.runAction { id, game, runnerId }
runner.crashHints { game, runnerId, log } -> [CrashHint]
```

- `wrap` is called for every launch and every settings preview: one RPC, no slower than today's in-process call.
- Tweaks from data plugins (env, DLL overrides, winetricks verbs) are passed to `wrap`; the runner applies them.
- The env denylist for store specs stays (`WINE*`, `PROTON_*`, `STEAM_COMPAT_*`, …): only a runner plugin may set
  those keys. Because a runner wraps every game, `runner` is limited to bundled/first-party plugins (an allowlist like
  `gameIdPrefix`) until the bubblewrap sandbox exists.
- The per-game runner settings UI is generated from `runner.options`, replacing the hand-made runner panel.
- Crash hints move with the runner: they read Wine/Proton logs. The gaps of #46 are closed there (Steam-client
  launches, umu games).

### 4. Linux builds bundle the Linux plugins

The bundled-plugin mechanism (`PLATYPUS_BUNDLED_PLUGINS`, `BundledPluginIDs`) ships the compat and runner plugins with
Linux builds, approved by default, so users see no change. A Windows build would bundle neither.

### 5. "Out of the core" is checked, not claimed

The packages can stay in this repository and be imported by plugin binaries (as `internal/steam` is by the steam
plugins). Each move is done when:

```sh
go list -deps ./cmd/platypus-core | grep -E 'platypus/internal/(compat|runner|crashhints|winetricks)'   # empty
GOOS=windows go build ./cmd/platypus-core                                                             # builds
```

The second check is the epic's end state (#75 lists what still fails until then); both go into the test scripts once
they pass. On 2026-10-16 the first lists `compat`, `runner`, `crashhints`, `winetricks` and `steam`, and the Windows
build stops in `internal/relocate` (`syscall.Stat`) and `internal/session` (process groups, `syscall.Kill`): those two
are core code that needs per-OS files (build tags), not plugins.

## Order of work

## Audit: remaining Steam and Linux specifics in the core (#75)

Checked against `go list -deps ./cmd/platypus-core` and the imports/call sites
listed below (2026-10-22). “Plugin” means the owning store/runner plugin;
“generic capability” means a reusable core or desktop host boundary.

| Item | Files and verified use | Destination and reason |
|---|---|---|
| Steam client, install roots, library metadata, and artwork | `internal/steam/*`; imported/called by `internal/app/app.go` (`sideloadSteam` client, `IsClientLaunch` in the session wrap), `app_gameinfo.go` (`sideloadSteamClient`, Steam app IDs), `app_presets.go` (`IsClientLaunch`), `app_library_snapshot.go` (Steam art URL mapping), `app_artwork.go` (Steam art lookup), and `app_backups.go` (`RootFor`); save discovery in `internal/savebackup/resolve.go` also imports it | **Plugin**: Steam-owned client details, metadata, artwork, and user paths belong to the Steam plugin; expose save locations/art through plugin results instead of core imports. |
| Steam URI launch fallback | `internal/steam/launch.go` returns `xdg-open steam://rungameid/...`; the steam plugin owns this launch spec | **Plugin**: the URI scheme and desktop opener are Steam launch behavior. |
| Cloud sync orchestration | `internal/cloudsync/*`; `internal/app/app_cloud.go` sends `cloud.*` calls to any ready store plugin with `cloud-sync`, while app code owns conflict state, launch gate, jobs, and user modes | **Keep**: generic capability fed by per-store plugins, not Steam-shaped. |
| Friends/social orchestration | `internal/social/*`; `internal/app/app_social.go` enumerates ready plugins declaring `social`, sends `social.*` calls, and scopes state by plugin ID | **Keep**: generic capability fed by store plugins. |
| Chat orchestration | `internal/chat/*`; `internal/app/app_chat.go` sends `chat.*` calls to capability providers and validates friends through `social` | **Keep**: generic capability fed by store plugins. |
| Launcher data/config/cache paths | `internal/paths/paths.go` applies `PLATYPUS_*`, XDG variables, and Linux `~/.local/share`, `~/.config`, `~/.cache` defaults; consumed by core startup, `cloudsync/context.go`, and save backup path resolution | **Generic capability**: retain central path resolution but add OS-native defaults behind the resolver for Windows/macOS; XDG compatibility remains Linux-specific. |
| Steam save roots and XDG save contexts | `internal/paths/SteamRoots`; `internal/savebackup/resolve.go` recognizes Steam and XDG roots; `internal/cloudsync/context.go` and `internal/savebackup` resolve XDG save locations | **Plugin** for Steam roots (above); **generic capability** for user save locations, with OS-specific known-folder providers. |
| Native executable detection and Linux scripts | Runner plugin `plugins/runner/main.go` handles `runner.detectExecutable`; its runner implementation uses `internal/runner/native.go` to identify ELF, MZ/PE, AppImage, and script formats. Core `internal/app/app.go` still invokes `/bin/sh` for non-executable scripts and shebang files | **Plugin** for detection; **generic capability** for launching a script through a host-provided shell. `/bin/sh` is Linux-specific and currently blocks portable direct launches. |
| Linux mount accounting | `internal/app/app_diskusage.go` reads `/proc/self/mountinfo` to group game folders; `internal/jobs/space_unix.go` uses `syscall.Statfs` for free/total bytes | **Generic capability**: provide disk-volume enumeration and space queries per OS; Windows space reporting currently returns an explicit unavailable error. |
| Linux process discovery and teardown | `internal/session/session.go` discovers client launches by scanning `/proc/<pid>/cmdline`; `internal/session/stop_unix.go` walks `/proc` for descendants and launch tokens and signals process groups | **Generic capability**: retain session tracking in core but implement process enumeration, group/job cleanup, and stop signals per OS. Windows currently kills only the tracked process. |
| Filesystem root and protected system directories | `internal/app/app_remove.go`, `app_discard.go`, `app_installdir.go`, and `internal/plugins/manager.go` reject `/`, Linux system directories, and `/usr` paths in destructive/install/plugin checks | **Generic capability**: safety checks should use platform roots and protected locations, not `/` string prefixes. |
| Open file/folder and external links | `internal/app/app_plugins.go`, `app_crashhints.go` and `app_diskusage.go` execute `xdg-open`; `internal/steam/launch.go` does likewise; `apps/desktop/src/main.ts` already routes web URLs through Electron `shell.openExternal` | **Generic capability**: use the desktop host's open-file/open-directory/open-URL interface across OSes; core `xdg-open` calls are Linux-only. |
| Desktop tray | `apps/desktop/src/main.ts`, `tray.ts`, and tray locales create an Electron `Tray`; code detects creation failure and minimizes instead of hiding | **Keep**: Electron supplies the cross-platform host integration and the existing fallback handles systems without a tray host. |
| Desktop keyboard shortcut policy | `apps/desktop/src/main.ts`, `shortcuts.ts` suppress reload/inspect shortcuts in production; no global OS shortcut registration is present | **Keep**: generic shell behavior, with platform-independent Electron input handling. |
| Desktop data directory | `apps/desktop/src/userdata.ts` resolves Electron `userData` from XDG config conventions and sets it in `main.ts` | **Generic capability**: keep the desktop shell owner, but use Electron's native default on non-Linux platforms (currently the helper specifically mirrors XDG behavior). |
| Windows build boundaries | `internal/relocate`, `internal/session`, `internal/plugins/process`, `internal/jobs`, `internal/app` writability checks, and `cmd/platypus-core` required OS-specific implementations. `plugins/...` now cross-builds too. | **Keep**: these are generic core mechanisms; per-OS files preserve Unix behavior and provide Windows fallbacks. Windows job-object/descendant stopping, ACL-aware writable checks, volume space reporting, handle inheritance/protocol stdout separation remain missing. |

The Windows check now builds both the core and Go plugins. `plugins/...` succeeds
with the current tree; it is included in `check:windows` so future plugin build
failures are caught with the core.

Follow-up issues: #80 (Steam specifics into the Steam plugin), #81 (host-neutral script launches), #82 (platform-native paths and save locations), #83 (disk and protected-path checks), #84 (open files/URLs through the desktop host), #85 (Windows process supervision and core stdio).

| Step | Status (2026-10-16) |
|---|---|
| #55 ProtonDB provider | Complete; bundled in `linux-compat` |
| #73 step 4 remaining compat sources | Complete; bundled in `linux-compat`, with facts rendered by kind |
| #74 runner plugin | In progress (2026-10-17): the core has no runner, crash-hint or winetricks imports; `runner.options` covers every per-game runner setting; the settings panel still hand-made (open question in #74) |

1. This ADR, reviewed and amended.
2. Protocol: `store` optional, the `compat-facts` and `runner` capability names in the registry, conformance support.
3. #55: ProtonDB as the first `compat-facts` plugin; generic badges in the UI; smart filter migration.
4. The other compat sources, until `internal/compat` leaves the core.
5. #74: the Wine/Proton/umu runner plugin, with crash hints; the sandbox fake Proton (#66) and the e2e crash scenario
   (#69) move with it and must keep passing.
6. #75: the audit and the `GOOS=windows` build check.

Each step lands with `bun run test:all` and `PLATYPUS_TEST_SLOW=1 bun run test:e2e` green.

## Consequences

- Positive: the core shrinks to launcher concepts; Linux specifics can be replaced or dropped; third parties can add
  compat sources without touching the core; Windows becomes a packaging question for the core.
- Cost: a protocol extension (minor version bump), a data-model change for compat (badges instead of named fields,
  with a migration of saved smart filters), and moving tested code behind an RPC boundary. The e2e suite is what
  makes the runner move safe.
- Risk: launch latency and failure modes (a crashed runner plugin must not leave a game half-started); the plugin
  supervisor's restart and error paths need tests for the runner kind.

## Answers (user, 2026-10-16)

1. Native-game wrappers (gamemode, gamescope, MangoHud) move into the runner plugin: `wrap` is called for native games
   too.
2. Runner plugins are bundled/first-party only until the bubblewrap sandbox exists.
3. Compat sources ship as one bundled plugin for now.
4. The per-game runner settings UI is generated from the runner's `runner.options` schema.
