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
- The UI shows badges generically: a badge has a `kind` (`rating`, `verdict`, `protection`, `note`), a label, a
  tone/tier for colour, a link and a source with its date; the existing `types.CompatBadge` is most of it.
- Smart filters filter on badges (`kind` + value) instead of named ProtonDB/Deck fields. Existing smart collections
  are migrated (a filter on tier "platinum" becomes a filter on the ProtonDB plugin's `rating` badge).
- Matching stays as today: by Steam AppID when known, never by title.
- Order: ProtonDB first (#55), then Steam Deck, then AreWeAntiCheatYet, PCGamingWiki and local anti-cheat detection.
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
