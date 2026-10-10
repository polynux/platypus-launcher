# Plan: launcher parity — capabilities and protocol additions

Status: proposed, written 2026-10-09. Nothing implemented. Evidence, sources and the feature inventory are in
[docs/research/launcher-parity.md](../research/launcher-parity.md) (referred to as "research §n").
Cloud saves and social already have their own plan: [steam-cloud-social.md](steam-cloud-social.md); this plan
only adds what is missing around them.

Rules that bind every item (product direction): Platypus replaces the official launchers; each store's feature
set is replicated in its plugin; core offers store-agnostic capabilities; nothing store-specific in core. Hard
limits: no DRM or licence bypass, no Steam API emulator, nothing that modifies game files, no client-identity
spoofing. Live checks use the user's real accounts and wait for their go-ahead.

## 1. Capability system changes (do first)

Today: a flat manifest list, a hard-coded `knownCapabilities`, unknown names make a manifest invalid, UI tests
`capabilities.includes(...)`, gating per plugin (research §5.1).

| # | Change | Why |
|---|---|---|
| C1 | **Capability registry as data** in `internal/plugins`: `name -> { since, methods[], optionalMethods[], surfaces[] }`. Manifest validation, conformance (`plugtest`) and the gate in `stores.Plugin` all read it. | Adding a capability becomes one table row plus its orchestrator; no scattered `if`. |
| C2 | **Unknown capability names are ignored with a warning**, not invalid (only a malformed name is). A manifest field `"needs": ["cloud-saves"]` lists capabilities core *must* know; if one is missing the plugin shows "needs a newer Platypus" and does not start. | An older Platypus can still list and approve newer plugins; a plugin can still refuse to run half-featured. |
| C3 | **Manifest = promise, `initialize` = reality.** `initialize` returns `capabilities` (a subset of the manifest) and `features` (depth flags per capability, e.g. `"versions": {"channels":true,"builds":true,"password":false}`). Replaces today's "equal to the manifest" rule with "subset of the manifest". Missing tool or logged-out state switch features off without changing files. | Honest UI; the manifest hash is not touched when a tool appears. |
| C4 | **Per-game availability:** `LibraryGame.features: ["versions","dlc","cloud"]` (optional, subset of capabilities). | The Versions control must not appear for a game with one build. If a plugin omits it, core treats every declared capability as available and handles empty results. |
| C5 | **Surfaces bound to capabilities.** A UI surface declares its capability (and per-game feature) and renders nothing otherwise; a declined/unavailable state carries a plugin-written reason (as `CloudStatus.message`). The Plugins view lists capabilities with their `features`. | Matches "UI surfaces appear only when a plugin offers them". |
| C6 | **Optional methods keep using `-32601`**, probed once per plugin run and cached. New error code `-32002` "unavailable right now" (borrowed game in use, region lock, service down) with a user-readable message; `-32001` stays "log in". | Distinguishes "not offered" from "not now". |
| C7 | **Orchestrators depend on narrow Go interfaces** (`stores.VersionStore`, `CloudStore`, `SocialStore`, …) that `stores.Plugin` implements over RPC when the capability is on. Built-in Steam implements none. | Core never mentions a store; capability tests use a fake plugin (extend `plugins/echo`). |

Sizes: C1-C3 and C6 are S-M together (manifest, process, plugtest, docs); C4-C5 S; C7 grows with each capability.

## 2. Proposed capabilities and namespaces

New or extended capabilities, in manifest form. Shapes only; field names are proposals and become fixed when
written into `docs/plugins/protocol.md` and `internal/plugtest`. All additions are additive to api 1.

| Capability | Methods (Platypus → plugin) | Core owns | Plugin owns |
|---|---|---|---|
| `install-options` | `game.options {id}`; `game.install`/`game.update` gain `options` | the install dialog, remembering choices | what the options are |
| `versions` | `versions.list`, `versions.unlock`; `game.install`/`game.update` gain `target` | picker UI, pinning, install ledger | channels/builds, passwords, archive |
| `verify` | `game.verify` (job) | the Verify button and progress | check and restore of the store's own files |
| `move` | `game.move` (job) | the Move dialog, space check | moving and rebasing records |
| `cloud-saves` | `cloud.*` (existing plan) | when, policy, conflict UI | how |
| `social` | `social.*` (existing plan) | friends summary, modal, events | session, presence |
| `achievements` | `achievements.list {id}` | game-page list | the store's data |
| `playtime` | `playtime.list` | merge with local time, display | store-side minutes |
| `redeem` | `redeem.submit {code}` | the "Redeem code" entry | the store's redemption |
| `family` | `family.members`; `LibraryGame.sharedBy`; error `-32002` on borrowed-in-use | badge, "shared with me" filter, messages | rules and state |
| `metadata` (separate plugin axis, later) | `metadata.search`, `metadata.get` | field merging | one source |

### 2.1 `install-options` (covers language, platform, install tags and DLC)

DLC is not a separate capability: stores differ in naming but not in shape, so it is an option group.

```
game.options { id } -> { groups: [
  { id: "language", label: "Language",  kind: "select",  values: [{ id: "en", label: "English" }], default: "en" },
  { id: "dlc",      label: "Add-ons",    kind: "multi",   dlc: true,
    values: [{ id: "123", label: "Season Pass", sizeBytes: 1048576, owned: true, installed: false }] },
  { id: "packs",    label: "Optional packs", kind: "multi", values: [...] } ] }
game.install / game.update { ..., options: { language: "en", dlc: ["123"], packs: [] } }
```

Group `kind`: `select`, `multi`, `bool`. `owned: false` DLC is shown greyed with the store link. Unowned content is
never installable. `game.installSize` takes the same `options` so the free-space check stays honest.

### 2.2 `versions` (research §3)

```
versions.list { id } -> {
  channels: [ { id: "beta", name: "beta", description: "…", updatedAt: "2026-…",
                buildId: "25240193", requires: "none"|"password", unlocked: bool, current: bool } ],
  builds:   [ { id: "58076094395251196", name: "1.5 (CS) GOG 0.2", publishedAt: "2024-11-05",
                channel: null, source: "store"|"archive"|"manual", available: bool } ],
  manual:   { label: "Steam manifest ids", hint: "one per depot, from SteamDB" } | null,
  pinned:   { kind: "build"|"channel", id: "…" } | null }
versions.unlock { id, channel, password } -> { unlocked }     # plugin keeps the password in PLATYPUS_PLUGIN_DATA
game.install / game.update { ..., target: { channel?: "beta", build?: "…", manual?: {…} } }
job.done { jobId, installed?: { build, channel, versionName, archiveRef } }   # feeds the ledger
```

- `channel` = moving pointer (Steam branch, GOG branch); `build` = immutable build. `source` says where the entry
  came from, so the UI can mark "from your install history" or "typed by you".
- Passwords never reach core's saved job queue: `versions.unlock` validates and stores them in the plugin; the
  `target` only names the channel. A resumed job (same `jobId`) re-uses the stored unlock.
- **Install ledger (core):** after each `job.done` core appends `{ store, game, build, channel, versionName,
  date, archiveRef }` to a per-game history. `archiveRef` is opaque to core (Epic: manifest file name saved by
  the plugin; Steam: the depot→manifest id map). The plugin lists ledger entries in `builds` with
  `source:"archive"` and says whether they can still be fetched. This is the only history Steam and Epic give.
- **Pinning:** core stores the pin in per-game settings, passes it as `target` on `game.update` and suppresses
  update prompts; the plugin reports `updateAvailable` relative to the pin.
- Per store: GOG fills `builds` from the content-system builds list and `channels` from branches; Steam fills
  `channels` from app info branches, `unlocked` via `CheckBetaPassword`, `manual` for old manifest ids,
  `archive` from the ledger; Epic fills only `archive` (and hides the capability for games with no entries);
  Ubisoft does not declare `versions`.
- Safety: before installing a different build the UI warns that cloud saves or online play may not match (a
  downgrade can corrupt saves); Platypus backs up nothing itself but `cloud.*` pull-before-launch still protects
  the cloud copy.

### 2.3 `verify` and `move`

```
game.verify { id, installDir, jobId, title? } -> {}  # job: progress, job.done, may report { repaired: n } via log
game.move   { id, from, to, jobId }            -> {}  # job: plugin moves (legendary move) or rebases its records
```

`verify` restores the store's own files from the store's own content (as `legendary verify`/`--repair` do); it
is not patching. Core asks before repairing (the plugin first reports the damage count through a `phase`).
`move`: core does the free-space and same-filesystem check; when a plugin has no `move`, core can still
relocate a folder and call `game.install` rebase later (open: Q-MOVE).

### 2.4 `achievements`, `playtime`, `redeem`

```
achievements.list { id } -> { total, unlocked, list: [{ id, name, description, icon, hidden, unlockedAt?, globalPercent? }] }
playtime.list {}         -> { games: [{ id, minutes, lastPlayed? }] }          # whole library in one call
redeem.submit { code }   -> { message, titles?: ["…"] }                         # failure = -32000 with a readable message
```

`achievements.list` supersedes the catalogue-only `game.details.achievements` when the user's progress is
available; stores that can't see it leave `unlockedAt` out. Unlocking achievements is never offered: it is the
game's job through the store SDK, and Platypus won't emulate that. Core merges playtime as `max(local, store)` in
display only and never writes it back (Steam's in-game reporting belongs to `social.setPlaying`).

### 2.5 `family` (Steam first; GOG/Epic/Ubisoft have no equivalent)

- `LibraryGame.sharedBy: { id, name }` marks borrowed titles; the library gets a "shared with me" filter.
- Launching a borrowed game that is in use: `game.launchSpec` fails with `-32002` "Family member X is playing
  this game"; during play the plugin raises a notification `family.warning { id, message, secondsLeft }` that
  core shows as a toast (Steam warns the borrower then disconnects them when the owner plays [research §1]).
- `family.members {}` -> `[{ id, name, role: "adult"|"child", avatarUrl, state }]` for a small Families
  section. Exclusions and parental limits are shown, not enforced by core.
- Steam-only details (which games are excluded, per-copy limits) stay in the Steam plugin.
- **Later:** some families have a locking mechanism: a family member or parent can lock games or the whole
  library (Steam Families' restrictions and "locked" state). Display only, for later: core would show the
  locked state the store reports and enforce nothing. It needs live discovery first (L-F1); not implemented.

### 2.6 Launch spec for store clients under Wine (needed for Ubisoft)

Today `delegate: "store-client"` runs `command` natively, with `command[0]` among the manifest's `requires`
(`internal/stores/launchspec.go`). Add:

```
{ "delegate": "wine-client", "client": "Ubisoft Connect",
  "exe": "<prefix>/drive_c/…/UbisoftConnect.exe" | "uplay://launch/635",
  "args": [...], "prefix": "client",           # core-owned prefix keyed per plugin
  "find": { "cmdline": ["Game.exe"] }, "startTimeout": 180 }
```

Core owns the prefix (`prefixes/p.<plugin>.client`), runner choice and sandbox rules (exe inside the prefix or an
allowed folder, same denylists as direct launches); the plugin only names what to start and how to recognise
the game. A `client.setup` job (plugin returns the installer to run in that prefix; the user confirms the
download) installs the client. `find` already matches `/proc/<pid>/cmdline`; under Wine the match strings are
Windows paths/exe names, which works but needs the live check in L-U1.

## 3. Roadmap

Sizes follow the cloud plan: S ≤ 1 day, M 2-3 days, L 4-6 days, with tests. "Why" = why at that position.

| Order | Item | Size | Why here | Depends on |
|---|---|---|---|---|
| 1 | **Capability system C1-C6** (registry, tolerant unknowns, initialize subset, per-game `features`, `-32002`) | M | Every later item registers through it; cheap now, expensive after five ad-hoc capabilities | nothing |
| 2 | **Cloud saves** P1-P4 (existing plan) | L+M | Fixes the user's real error; already designed, Phase 0 merged | 1 (registry), P0 |
| 3 | **Versions, part 1: ledger + GOG** (`versions.list/unlock`, `target`, pin, UI picker) | M | GOG exposes a public builds list and `gogdl --build`; real value with the least risk; proves the contract | 1 |
| 4 | **Verify + move** (`game.verify`, `game.move`) | S + S | Both wrap existing legendary/gogdl/Steam behaviour; frequent user needs | 1 |
| 5 | **install-options incl. DLC and language** | M | Needed for any GOG/Epic game with extras; removes "DLC missing" gaps | 1 |
| 6 | **Versions, part 2: Steam** (branches from app info, password branches, `manual` manifest ids, ledger-based previous build) | M | Builds on `steamcm`'s `ManifestRequestCode` and `CheckBetaPassword`; the user asked for it explicitly | 3 |
| 7 | **Social** (existing plan P5-P6) | L + M | Needs the persistent session (P0 done) and L4/L5 answers | 1 |
| 8 | **Playtime, achievements (with user progress), redeem** | S + M + S | Each is one call plus a small UI row; Steam redeem needs a purchase-adjacent decision (Q-REDEEM) | 1 |
| 9 | **wine-client launch + Ubisoft plugin, phase A** (local cache library, install/launch/uninstall via the client) | M + L | Highest uncertainty: do after the live check L-U1 and Q-U1/Q-U2 | 1, L-U1 |
| 10 | **Steam Families** (borrowed games, `-32002`, family section) | L | Same persistent session as social; its rules are server-side and need discovery | 7 |
| 11 | **Epic versions via archived manifests** | M | Only after the ledger exists; value limited to "previous install" | 3 |
| 12 | **Ubisoft phase B** (friends/playtime via web API) | M | Only if Q-U1 allows | 9 |
| Deferred | chat/voice, Workshop (needs the real client), parental enforcement, metadata plugin axis, EOS overlay management, store deep links beyond `storeUrl` | | not core, or blocked by the hard limits | |

Small items to slip in whenever convenient (S each): `GameDetails.storeUrl` (store page deep link), a
"restricted account" banner from `auth.status`, `quota/bandwidth` setting in the download queue.

Rough totals: capability system ~3 days; versions (GOG then Steam) ~5 days; verify/move/options ~6 days;
Ubisoft A ~2 weeks including live iteration.

## 4. Live checks that need the user

| ID | What | For | Risk |
|---|---|---|---|
| L-V1 | GOG: list builds for one owned game, install an older build with `gogdl --build`, then update it; confirm `info` output shape | versions (GOG) | downloads a game; reversible |
| L-V2 | Steam: install a game from a non-default branch (`previous_release` or `beta`); then an old manifest id for one depot; check whether the CDN still serves it and whether a request code is issued for an old gid | versions (Steam) | downloads; a failed old manifest just errors |
| L-V3 | Epic: after a real game update, install from the manifest saved before the update (`legendary install --manifest`) | versions (Epic) | opportunistic; needs an update to happen |
| L-V4 | Steam: password branch flow on a game with a known beta password the user owns | versions (Steam) | none |
| L-U1 | Ubisoft Connect in a Wine prefix on this machine: does the login webview work under the user's Proton/Wine, where do `cache/configuration/configurations` and `ownership` land, does the cache list owned-but-uninstalled games, does `uplay://` reach the client from `wine start`, and does the `find` cmdline match work | Ubisoft | needs the user's Ubisoft login; no download needed for the first three |
| L-U2 | Ubisoft: `uplay://install/<id>` of a small game from Platypus; progress visibility | Ubisoft | downloads a game |
| L-F1 | Steam Families: what the CM sends for a family group (group membership, borrowed library, in-use state) | family | read-only, user's real group |
| L-S1 | Run each new method against real plugin binaries once the conformance suite passes | all | per item |

## User decisions (2026-10-09)

| Question | Decision |
|---|---|
| Q-U1 Ubisoft web API | Recommended: local cache only, no official app id. |
| Q-U2 Ubisoft downloads | Recommended: Ubisoft Connect does the downloading in a Platypus-managed Wine prefix. Ideally a light, headless setup that Platypus drives programmatically. |
| Q-V1 Steam rollback | OK (branches, passwords, install ledger, typed manifest id). The UI shows a link to the game's depots on SteamDB so the user can look up a manifest id themselves; Platypus never scrapes SteamDB. |
| Q-V2 Order | As in the roadmap. |
| Q-LIC Licence | Permissive: MIT or Apache-2.0 (user leaning away from GPL). Final pick pending. |
| Q-REDEEM | OK. |
| Q-SOCIAL | Per-store sections inside one friends modal. |
| Q-MOVE | Core moves the folder itself when the plugin has no `game.move`. |
| Q-OVERLAY | Overlays (Epic EOS, Steam) are managed by the plugin, which launches its games; core provides the settings surface for them. |
| Q-PARENT | Show state only, never enforce. |

**Implementation focus:** build each capability block in the launcher core (registry, protocol, orchestration,
UI surfaces) and validate it with the test plugin (`plugins/echo`), extended to fake every capability.
Real store plugins (Steam, GOG, Epic, Ubisoft) implement them later.

## 5. Open product questions for the user

- **Q-U1. Ubisoft web API.** Library, friends and playtime need Ubisoft Connect's own application id. Sending it
  from Platypus is "acting as the official client" (the Galaxy plugin does exactly that). Accept for this plugin,
  or stay on the local cache only (recommended)?
- **Q-U2. Ubisoft downloads.** Accept "the Ubisoft Connect client does the downloading, inside a shared Wine
  prefix Platypus manages", with no native downloader (recommended)?
- **Q-V1. Steam rollback.** Accept: Steam branches + passwords + Platypus's own install history + a typed manifest
  id, but no browsing of every historic build (Steam does not list them; SteamDB forbids scraping)?
- **Q-V2. Order.** Versions (GOG first) before verify/move/DLC, as in the roadmap, or the other way round?
- **Q-LIC. Platypus's licence.** No LICENSE file exists. MIT-licensed Playnite code can be ported with attribution
  under most choices; GPL tools stay external. Decide before porting the Ubisoft cache reader.
- **Q-REDEEM. Redeem keys in the app.** Acceptable for Steam/GOG/Epic/Ubisoft (the plugin forwards to the store)?
- **Q-SOCIAL. Merged friends.** One friends list per store (the Steam plan's sidebar summary) or a merged
  cross-store list? Recommend per-store sections inside one modal.
- **Q-MOVE. Move without plugin support.** Let core relocate a folder and only mark the game "moved", or require
  the plugin's `game.move`?
- **Q-OVERLAY. EOS overlay.** Offer `legendary eos-overlay` management in the Epic plugin (an Epic-supplied
  component for entitled users), or skip it?
- **Q-PARENT. Parental controls and Steam Families limits.** Show state only, never enforce, as drafted?

## 6. Checklist

- [ ] C1-C6 capability system (registry, `needs`, tolerant unknowns, initialize subset, per-game `features`, `-32002`) · protocol.md + plugtest
- [ ] Cloud saves P1-P4 (steam-cloud-social.md)
- [ ] Versions: ledger + GOG · L-V1 — core, protocol 1.3, UI and echo done (feat/versions-core); the GOG plugin side is not
- [x] Verify, move · core, protocol 1.4 (`game.verify`, `game.move`, optional `game.relocated`), UI and echo done (feat/verify-move); the store plugins' side is not
- [x] install-options + DLC · core, protocol 1.5, UI and echo done (feat/install-options); the store plugins' `game.options` (steam, gog, epic, amazon) are not
- [ ] Versions: Steam · L-V2 · L-V4 — needs the core above; the Steam plugin side is not done
- [ ] Social P5-P6 (steam-cloud-social.md)
- [ ] Playtime, achievements, redeem
- [ ] wine-client launch + Ubisoft phase A · L-U1 · L-U2
- [ ] Steam Families · L-F1 — core, protocol 1.8 (`family`, plugin `settings`), UI and echo done (feat/family-settings); the Steam plugin side is not
- [ ] Epic versions via archive · L-V3
- [ ] Ubisoft phase B (only if Q-U1 allows)
