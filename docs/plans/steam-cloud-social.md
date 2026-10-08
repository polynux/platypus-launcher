# Plan: Steam Cloud saves, friends list and online status

Status: proposed, written 2026-10-08. Nothing implemented. Findings, sources, message layouts and the
list of unknowns are in [docs/research/steam-cloud-social.md](../research/steam-cloud-social.md); this
plan only records decisions and work. Section and risk numbers (§, R#, L#) refer to that document.

Rules that bind every phase (from the task, and the existing plugin rules):

- Only the official Steam protocol, as the official client uses it. No DRM or licence bypass, no Steam API
  emulator (nothing that makes a game think it runs under Steam), no spoofing of client identity.
- Tokens and passwords are never logged, never in argv, never in test fixtures. Log appids and counts, not paths.
- Live checks use the user's real account, so each one waits for their go-ahead. Everything else is covered
  by fakes and runs in CI.

## Goal

1. **Cloud saves**: pull before launch, push after exit, never overwrite a differing save without asking.
2. **Friends list** with names, avatars, status, "in game".
3. **Online status**: see your own status, change it, optionally tell friends what you are playing.

The first one fixes the problem the user hit (§2: a real cloud conflict that the official client reported because
it attached to a Hydra-launched game). The other two share one new piece of infrastructure, a long-lived
CM connection.

## Decisions

| # | Decision | Why |
|---|---|---|
| D1 | **The Steam session lives in the Steam plugin**, not in core. A new Go daemon in the plugin folder, `hydra-steam-dl serve`, owns it; `bin/hydra-steam` (Python) proxies `cloud.*` / `social.*` to it. | The refresh token must stay with the plugin ("Hydra never stores credentials", protocol.md). Keeps core store-agnostic. Hashing, HTTP and zip are easier and faster in Go than in the Python exec. |
| D2 | **Two new protocol namespaces**, both capability-gated and additive to api 1: `cloud.*` (capability `cloud-saves`, already a known manifest capability) and `social.*` (new capability `social`). | GOG/Epic/Amazon get the same UI with their own tools behind `cloud.*` (§1.5). |
| D3 | **Core owns when and what the user sees; the plugin owns how.** Core calls `cloud.pull` before and `cloud.push` after a game; the plugin computes paths, compares, transfers, and returns data (counts, conflicts). Core shows state and passes the user's choice back with `cloud.resolve`. | Policy (is the official client running? ask or auto?) is the same for every store. Plugins stay small. |
| D4 | **Explicit `cloud.*` calls instead of generic `game.prelaunch` / `game.postexit` hooks.** | A hook would push policy into every plugin and cannot return conflicts as typed data. Core still sequences it as pre-launch / post-exit. |
| D5 | **Core passes a `CloudContext`** (client OS of the launch, home dir, XDG dirs, install dir, compat/prefix dir) with each call. The plugin never learns Hydra's prefix layout. | The mapping `%WinAppDataLocalLow%` -> real path depends on native vs Proton and on `internal/runner`'s prefix choice (§1.4). |
| D6 | **Own baseline, never the official client's files.** Per game: `path -> last SHA-1 identical on both sides`, stored under the plugin's `HYDRA_PLUGIN_DATA`. Rules: changed on one side = copy; changed on both or no record = conflict, ask; never delete in the cloud; back up any local file a download replaces. | Madeira's proven model (§1.3) and the only safe one, since `remotecache.vdf` is the official client's private state. |
| D7 | **When the official Steam client is running and a Hydra-launched game would attach to it, Hydra does not sync that game itself.** It shows why and offers "Launch through Steam". Otherwise Hydra syncs. (Needs the user's agreement, Q-B.) | Two writers on one save folder produce exactly the error seen (§2.3). |
| D8 | **Unique logon ID per role and a stable machine ID** before any persistent session exists. | The fixed `LoginID = 2718281` makes two overlapping Hydra sessions kick each other (§3.2, R1). |
| D9 | **Presence is opt-in and conservative**: friends list read-only first; own status control second; "in game" reporting last and off by default. | Aggregation with the official client's status is unknown (R10), and `ClientGamesPlayed` credits playtime and blocks the official client from playing (R11). |
| D10 | **API-only (non-Auto-Cloud) games are out of scope** for direct launch. They keep working through "Launch through Steam". | They need the Steamworks IPC to write `userdata/…/remote` (R8). |

## Protocol additions (documented in `docs/plugins/protocol.md` and enforced by `internal/plugtest`)

Manifest: `"capabilities": [..., "cloud-saves", "social"]`. `social` is added to `knownCapabilities` in
`internal/plugins/manifest.go`.

### `cloud.*` (Hydra -> plugin)

`CloudContext` (core builds it from the launch it is about to run, or just ran):

```json
{ "os": "linux" | "windows", "home": "/home/u", "xdgData": "…", "xdgConfig": "…",
  "installDir": "/home/u/Games/steam/SUPERHOT_322500",
  "prefixDir": "/home/u/.local/share/hydra/prefixes/p.steam-standalone.322500",
  "winUser": "steamuser" }
```

| Method | Params | Result |
|---|---|---|
| `cloud.status` | `{ id, ctx }` | `CloudStatus` (read-only, no file written, no upload) |
| `cloud.pull` | `{ id, ctx, jobId }` | `CloudStatus` after downloading what changed **only** in the cloud |
| `cloud.push` | `{ id, ctx, jobId }` | `CloudStatus` after uploading what changed **only** on this device |
| `cloud.resolve` | `{ id, ctx, jobId, choices: { "<path>": "local" \| "remote" } }` | `CloudStatus`; "remote" backs up the local file first, "local" replaces the cloud copy |

```json
CloudStatus = { "state": "synced" | "pending" | "conflict" | "unsupported" | "offline" | "error",
  "message": "shown to the user",
  "counts": { "same": 3, "download": 0, "upload": 1, "conflict": 0, "unmapped": 0 },
  "conflicts": [ { "path": "SUPERHOT_Team/SUPERHOT/super.hot",
      "local":  { "size": 3705, "mtime": 1791398361 },
      "remote": { "size": 3718, "mtime": 1734286632, "machine": "Deck" } } ],
  "checkedAt": 1791458728, "backupDir": "…" }
```

`unsupported` carries the reason (`no Auto-Cloud rules`, `encrypted cloud files`, `Steamworks API storage`). `offline` means the
store was unreachable; core then lets the game start after one confirmation. Notification `cloud.progress { id, jobId, phase, done, total }`
(phases `check`, `download`, `upload`) is shown on the game page and in Activity. `cloud.pull` and `cloud.push` obey `job.cancel` and must be idempotent for a
repeated `jobId`, like installs.

### `social.*`

Hydra -> plugin: `social.start`, `social.stop` (idempotent), `social.state` (current snapshot), `social.setStatus { state }`
(`online|away|busy|snooze|invisible`), `social.setPlaying { id \| null }`.

Plugin -> Hydra notifications: `social.connection { state: "connecting|online|reconnecting|replaced|offline|loggedOut", message? }`,
`social.self { name, avatarUrl, state }`, `social.friends { full: bool, friends: [Friend] }` (incremental after the first full push).

```json
Friend = { "id": "76561198000000000", "name": "…", "avatarUrl": "https://…",
  "state": "online|offline|away|busy|snooze|looking|invisible",
  "game": { "id": "p.steam-standalone.322500", "title": "SUPERHOT" }, "lastSeen": 1791398361 }
```

`game.id` uses Hydra's key (`p.<plugin>.<appid>`) when the app is in the library, else `{ "title": "…" }` only. Chat, groups, requests and
rich presence are out of scope.

### Core pieces

- `internal/cloudsync`: orchestrator (policy, official-client detection, per-game settings, events to the frontend).
  `App.LaunchGame` (`app.go:455`) gains the pre-launch step; `session.Tracker` (`internal/session`) gets an exit callback for the post-exit step.
- `internal/social`: owns the `social.*` subscription, caches the friends snapshot, emits Wails events.
- Settings: per plugin (`cloudDefault: off | ask | auto`, `socialEnabled`), per game override.

## Phases

Sizes are rough: S = ≤1 day, M = 2-3 days, L = 4-6 days of focused work, including tests. Dependencies in the right column.

### Phase 0: session foundations (M, no user-visible change) — done, merged in cde2cd3

Scope:

- `steamcm.Client`: a persistent connection type next to `Session`: one reader goroutine, job-id router for service
  replies, handler registry for pushes (`ClientFriendsList`, `ClientPersonaState`, `ClientLoggedOff` (757), `ClientPlayingSessionState` (9600),
  `CloudClient.NotifyAppStateChange` (EMsg 146, `target_job_name`)), **client-initiated heartbeats** at the logon response's `heartbeat_seconds`
  (today `Session` only answers heartbeats; a persistent session must send them), reconnect with backoff and jitter, give up on `LogonSessionReplaced (34)` / `LoggedInElsewhere (6)`
  and surface `replaced` instead of fighting. `Session.Call` keeps its signature (wrapper) so depot and auth code is untouched.
- Logon identity: `LoginID` becomes a parameter. Roles `social`, `cloud`, `content` derive different values; one-shot callers pick a random value
  per process. `machineID()` becomes stable per install (stored in `HYDRA_PLUGIN_DATA`) (R1, R2).
- Fake CM: make the Go fake importable (`internal/steamcm/steamcmtest`, moved out of `_test.go`) and add the pushes above plus a kick (`ClientLoggedOff` with 34 or 6).
  Keep `plugins/steam-standalone/testdata/fake_cm.py` in step for the Python harness.

Tests (fakes): heartbeat cadence from `heartbeat_seconds`; reconnect after a dropped socket; two logons with different IDs both survive; same ID => first gets 34 and does not
reconnect-loop; a push arriving between a request and its reply is not lost; `Call` still matches replies by `jobid_target`.

Live: **L1** (coexistence with the official client). Gate for every later phase.

Depends on: nothing. Delivers: D8, a stable base for P5+.

### Phase 1: cloud, read-only (M)

Scope:

- `internal/steamcloud` (Go): parse `ufs` from appinfo (the KV `ProductInfo` already returns), `Mapper` (root -> directory, overrides, `pathtransforms`, `{64BitSteamID}`/`{Steam3AccountID}`,
  case-insensitive resolution, path confinement), `Listing` (`GetAppFileChangelist` + `path_prefixes`), `Compare` (SHA-1 + size, local files that match the save rules but are not in the cloud).
- `hydra-steam-dl cloud status --app N --ctx …` and the `serve` daemon method; `bin/hydra-steam` `cloud.status`.
- Core: `cloud.*` types, manifest `social`/`cloud-saves` validation, conformance rules for the new methods in `internal/plugtest`.
- UI: a "Cloud saves" row on the game page (`GamePage.tsx`): state chip, counts, "Check now", and the reason for `unsupported`. No file is touched.

Tests (fakes): mapper table tests with the three real `ufs` blocks recorded in §1.4 (SUPERHOT, Hades, Stardew Valley) for native and Proton contexts; traversal
(`..`, absolute, `:`, 40 components) refused; fake `GetAppFileChangelist` with prefixes, deltas, `persist_state` 1/2; fake cloud for compare states; frontend test for the new row.

Live: **L2** (compare the mapper's resolved path with the `Found N files that match <path>` lines the official client logged for the same games; native and Proton). First real cloud call; read-only.

Depends on: P0 (can start in parallel for the mapper and compare code).

### Phase 2: pull before launch, with a baseline (M)

Scope:

- `ClientFileDownload` + HTTP GET + unzip (stored/deflate) + SHA-1 verify + atomic write + mtime set; backup of any replaced local file to `<plugin data>/cloud-backups/<app>/<time>/`;
  baseline file; refusal of `encrypted` files with a clear `unsupported`. Concurrency: bounded, per-file timeouts.
- `cloud.pull` end to end. Core: `LaunchGame` runs `cloud.pull` first when the game's setting allows; `ask` shows a dialog on conflict; `auto` proceeds past `offline`/`error` only after the user
  confirmed once ("launch anyway").
- Conflict UI v1: list with both dates and sizes and three choices per file (Keep this device / Keep cloud / Decide later); wired to `cloud.resolve` (remote = download with backup; local is applied in P3).
- Official-client policy (D7) in `internal/cloudsync`: detect through `~/.steam/steam.pid` (+ `/proc/<pid>/comm`) and `~/.steam/steam.pipe`; banner + "Launch through Steam" action; setting to override.

Tests (fakes): fake CM + httptest store; zip and plain bodies, bad SHA => nothing written, truncated body, partial failure leaves already-written files and records only those in the baseline;
conflict never overwritten; backup created; platform switch (native <-> Proton) maps the same cloud names to different directories and backs up before moving; core orchestration with a fake plugin (`plugins/echo` gets `cloud.*`
stubs); frontend dialog tests.

Live: **L3** (attach behaviour with and without a Hydra pre-pull; decides whether D7 can be relaxed), **L7** (zip, large, encrypted).

Depends on: P1.

### Phase 3: push after exit (L)

Scope:

- Post-exit hook in `session.Tracker`: when a direct launch's process group ends, wait for two stable passes 2 s apart (R5), then `cloud.push`.
- Upload: `BeginAppUploadBatch` (`machine_name`, stable `client_id`, `app_build_id` from appinfo) -> per file `ClientBeginFileUpload` -> HTTP blocks (honour `http_method`, headers, `explicit_body_data`,
  `block_offset/length`) -> `ClientCommitFileUpload` -> `CompleteAppUploadBatchBlocking` (`batch_eresult` 1/2; always complete the batch on failure). Upload only when the baseline proves the cloud copy is what this device last synced,
  or the user chose "Keep this device". Check quota with `ClientGetAppQuotaUsage`. `Cloud.ClientConflictResolution` notification after a user choice.
- Optional, behind a flag until **L6**: `SignalAppLaunchIntent` before launch (warn on `pending_remote_operations`), `SuspendAppSession`/`SignalAppExitSyncDone` after exit.
- Resilience: if Hydra quits while a game runs, remember `pendingPush` and run it at the next start; never block shutdown for longer than a few seconds.
- UI: post-exit toast ("Saved 2 files to Steam Cloud"), error state with Retry, Activity entry.

Tests (fakes): fake store with change numbers; upload round trip; abort mid-batch calls Complete with 2; upload refused when remote changed since baseline => conflict; quota exceeded; Hydra restarts with `pendingPush`; the
game still writing (file changes between the two stable passes) delays the push; `job.cancel` mid-upload.

Live: **L6** (round trip on a disposable save, and what the official client shows afterwards).

Depends on: P2.

### Phase 4: coexistence hardening and Windows/Proton edge cases (M)

Scope: `userFolder` discovery in a Proton prefix with symlink confinement (R6), `pathtransforms` for more games, `GameInstall`/`SteamCloudDocuments` roots, `platforms` filter in `savefiles`, `files_to_delete` (explicit deletes only on a
user choice), compat-tool switches, log hygiene review (R14), "cloud off for this game", import of a hand-moved prefix.

Tests: broaden the mapper corpus (record `ufs` for ~15 games, including `SteamCloudDocuments`, `{64BitSteamID}` paths, recursive globs, Windows overrides); symlinked `Documents`.

Live: repeat L2 on 5 more games. Depends on P3. Can run in parallel with P5.

### Phase 5: friends list and own status, read-only (L)

Scope:

- Plugin `social.start`: `Client` logs on with the `social` ID, sends `ClientChangeStatus` (default **Invisible (7)** so friends data flows without changing visibility; configurable), handles `ClientFriendsList` (filter relationship 3 and SteamID type != clan),
  `ClientRequestFriendData` with the flags from §4.1, accumulates `ClientPersonaState`, resolves avatars from `avatar_hash`, emits `social.friends` (full then incremental) and `social.connection`.
- Core `internal/social` + Wails events; frontend: Friends panel (online / in game / offline groups, search, "Playing SUPERHOT" links to the game page), own avatar + name in the sidebar with a status indicator.
- Settings: "Show Steam friends" (off by default), and what happens at login (start the session at app start, or when the panel opens).
- Backoff and the `replaced` state: a banner "Another Hydra session replaced this one" instead of reconnect fights.

Tests (fakes): scripted `ClientFriendsList` (full + incremental, blocked, request, clan IDs), persona pushes with game, name, avatar hash; reconnect re-requests data; `replaced` stops the loop; the plugin's JSON is checked by `plugtest`; frontend list/search/empty/offline states.

Live: **L4** (does Invisible hide the user while the official client says Online? what do friends see?). Do not ship the default until it passes.

Depends on: P0. Independent of P1-P4.

### Phase 6: change own status and "in game" (M)

Scope: status selector (Online/Away/Busy/Snooze/Invisible) with `social.setStatus`; persisted choice; optional "tell friends what I'm playing" (`social.setPlaying`, off by default, explicit consent text about playtime) tied to `session.Tracker`'s running state;
never `force`, never `kickPlayingSession`; handle `playing_blocked`; stop playing on exit/crash/Hydra quit.

Tests (fakes): `ClientGamesPlayed` sent on launch and cleared on exit/crash/quit; blocked state shown and not fought; status persists across reconnect.

Live: **L5** (playing-session interaction with the official client, both orders). Depends on P5.

### Phase 7: store-agnostic generalisation (M per store)

Scope: write `cloud.*` / `social.*` into `docs/plugins/protocol.md` as stable, then:

- **Epic (legendary)**: `cloud.status|pull|push` over `legendary sync-saves` (`--skip-upload`/`--skip-download`, `--save-path` from the game's `CloudContext`); legendary only knows "newer wins" (§1.5), so conflict detection is Hydra-side with the
  same baseline (hash the save dir before/after). 
- **GOG (gogdl)**: per cloud location `gogdl save-sync <location> <id> --os <platform> --ts <last> --name <name>`; the printed timestamp is the baseline; same Hydra-side conflict check.
- Frontend uses `CloudStatus` only; no per-store UI.

Tests: conformance suite gains `cloud.*` rules; fake legendary/gogdl scripts in testdata. Live: one game per store. Depends on P3.

### Phase 8: Web API fallback (only if wanted, S)

Own display name/avatar without a session through `Player.GetPlayerLinkDetails` (already used). Nothing else; §4.4 found no keyless friends API. Probably unnecessary.

## Order and sizes

| Order | Phase | Size | Why here |
|---|---|---|---|
| 1 | P0 session foundations | M | Gate for everything; fixes R1/R2 that already matter whenever downloads and library calls overlap |
| 2 | P1 cloud read-only | M | Zero-risk first live check (L2); gives the mapper and compare code the rest builds on |
| 3 | P2 pull before launch | M | Removes the cause of the user's error for the common case |
| 4 | P3 push after exit | L | The risky half (writes to the account); goes after reads are proven |
| 5 | P5 friends read-only | L | Independent of P1-P4; can run in parallel with P3/P4 |
| 6 | P4 hardening | M | Real-game coverage |
| 7 | P6 status + in-game | M | Opt-in, needs L4/L5 answers |
| 8 | P7 GOG/Epic | M each | Only after Steam's contract has proved itself |

Rough total for Steam cloud (P0-P4): ~4 weeks of focused work; social (P5-P6): ~2 weeks; each extra store (P7): ~3 days.

## Live checks that need the user

All use the real account and wait for approval; all are small and reversible except where noted.

| ID | What | Phase | Risk |
|---|---|---|---|
| L1 | Official client running; Hydra logs on with distinct logon IDs; confirm neither is kicked. Then equal IDs once to confirm `34` | P0 | Might sign the official client out for a moment |
| L2 | Read-only `GetAppFileChangelist` on 3-5 games; compare to the official client's `Found N files …` lines | P1 | None (read-only) |
| L3 | Launch an Auto-Cloud game with the official client running, with and without a Hydra pre-pull; read `cloud_log.txt` | P2 | Can create a Steam-side conflict like the user's; use a throwaway-save game |
| L4 | Presence aggregation (Invisible vs Online/Away), with a friend or a second account watching | P5 | Briefly shows an odd status |
| L5 | `ClientGamesPlayed` + official client launching another game, both orders | P6 | Playtime recorded; may stop a game |
| L6 | Upload round trip on a disposable save; check change number and the official client's view | P3 | Writes to the cloud; a bad batch could replace a save. Use a disposable game |
| L7 | Zip/large/encrypted downloads | P2 | None |

## Testing approach (summary)

- One importable fake CM (`steamcmtest`) with: Cloud service (in-memory file store, change numbers, prefixes, persist states, zip and plain bodies, an `httptest`
  server for block PUT/POST and GET with the headers Steam hands out), friends and persona pushes, kicks (6, 34), `ClientPlayingSessionState`, `NotifyAppStateChange`. The Python fake stays in step for the
  plugin harness (`FAKE_CM_STATE` knobs, as documented in `fake_cm.py`).
- Fixtures: recorded `ufs` blocks for the games in §1.4; no real saves, no real tokens, no real SteamIDs.
- `internal/plugtest` conformance: new rules for `cloud.*`/`social.*` shapes (extra keys rejected, like `library.list`).
- Core orchestration tested with a fake plugin (extend `plugins/echo`): pull before launch, push after exit, policy when the official client is "running" (a temp `steam.pid`), conflict flow, offline flow.
- Frontend: component tests for the cloud row, conflict dialog, friends panel and status selector under `frontend/src/components/__tests__`.
- Never run `wails dev` from CI or agents.

## User decisions (2026-10-08)

Answers to the open questions below; they override the recommendations where they differ.

| Question | Decision |
|---|---|
| Q-A API-only games (e.g. Terraria) | Launch them through the official Steam client. Making Hydra answer for the Steam client (Steamworks IPC) is deferred; if ever done: only with real account data, no game file changes, its own droppable plugin. |
| Q-B Official client running | D7 as recommended: Hydra does not sync that game and offers "Launch through Steam". |
| Q-C Automatic vs ask | Auto-sync clean cases; dialog only on conflicts. |
| Q-D In-game reporting | Report "in game" like Steam does (playtime credited). Goal is to replicate Steam's behaviour. Overrides D9's "off by default". |
| Q-E Status and connection | Follow Steam's status (not Invisible); connect at launcher start. |
| Q-F UI | Small friends summary section in the sidebar; clicking it opens a friends modal, like Steam. |
| Q-G SUPERHOT conflict | Skip. |
| Q-H Terms of service | Accepted. Everything Steam-specific stays in the Steam plugin so it can be dropped. |
| Q-I Save backups | Keep the last N backups per game. |

Further Steam plugin scope requested by the user:

- **Steam Families**: list games shared by family members, install and play them with Steam's own rules (one player per copy, the borrower is warned then disconnected when the owner plays, owner exclusions, parental limits). Same persistent session as social.
- **Game version selection** (store-agnostic core capability): install an older build. Steam via beta branches and older depot manifests.

Product direction: Hydra replaces the official launchers (Steam, Epic, GOG, Ubisoft). Store features are replicated in their plugins; the core offers store-agnostic capabilities (cloud saves, social, versions, ...).

## Open product questions for the user

- **Q-A. API-only games.** Games whose saves go through the Steamworks storage API (e.g. Terraria's `ufs` has quota only) cannot be synced by a direct launch. Accept "use Launch through Steam for those", or should Hydra at least download their cloud files for backup?
- **Q-B. Official client running.** Default to "Hydra does not sync that game and offers Launch through Steam" (recommended, §2.3), or to "Hydra syncs anyway and warns about a possible Steam-side conflict"? A third option is to refuse direct launch of Steamworks games while Steam is open.
- **Q-C. Automatic vs ask.** Default `cloudDefault`: `ask` on conflict only (recommended: auto for clean cases, dialog for conflicts), or always confirm?
- **Q-D. In-game reporting.** Should Hydra tell friends what you play and so credit playtime to the real app, or show non-Steam style "Playing SUPERHOT" without playtime, or nothing? Recommend off by default.
- **Q-E. Visible status.** Should Hydra's session default to Invisible (reads friends, changes nothing) or follow Steam's status (needs L4)? Should it connect at app start, or only when the Friends panel is open?
- **Q-F. Where in the UI.** Friends as a right-hand drawer, or a sidebar view? Status selector in the sidebar footer?
- **Q-G. The existing SUPERHOT conflict.** The cloud holds December 2024 saves, this machine has October 2026 ones. Which side do you want to keep? Hydra will not decide that for you, and the official client currently marks both files as conflicting.
- **Q-H. Terms of service.** Third-party CM clients are widespread (SteamKit, steam-user, Madeira) and no ban reports were found, but Valve has no public statement. Keep the behaviour identical to the official client's and accept that residual risk?
- **Q-I. Save backups.** Keep every replaced local file forever, the last N, or N days? Where?

## Out of scope

Chat and voice, groups and clan data, friend requests and nicknames, rich presence uploads, trading, Steam Input/overlay, achievements sync, Steamworks IPC of any kind, API-only cloud games for direct launches, deleting cloud files.

## Checklist

- [ ] P0 session foundations (`Client`, heartbeats, distinct logon IDs, stable machine ID, importable fake CM)
- [ ] L1
- [ ] P1 cloud read-only (mapper, listing, compare, status UI) · [ ] L2
- [ ] P2 pull before launch (+ baseline, backups, conflict dialog, official-client policy) · [ ] L3 · [ ] L7
- [ ] P3 push after exit · [ ] L6
- [ ] P4 hardening
- [ ] P5 friends read-only · [ ] L4
- [ ] P6 status and in-game · [ ] L5
- [ ] P7 Epic and GOG on the same contract
