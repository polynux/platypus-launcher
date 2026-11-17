# Research: Steam client parity gaps in Platypus

Status: research only, written 2026-11-17 on `docs/steam-parity-gaps`. Nothing
here changes code. Live probes were strictly read-only: a CM logon + PICS
read through the worktree-built helper with
`PLATYPUS_PLUGIN_DATA=$HOME/.local/share/platypus-live/steam`, one anonymous
public-mirror read, and keyless Web API calls. No token, account name,
steamid, family id or friend name was printed, logged or committed; account
identifiers below appear only as `<redacted>`.

Legend: **[src]** read in a primary source (linked). **[live]** confirmed by a
read-only probe during this research. **[inferred]** reasoned from sources, not
confirmed. **[LIVE]** needs a live check before building.

Read first (all in this repo): `docs/plugins/protocol.md` (1.18),
`plugins/steam-standalone` (`bin/platypus-steam`, `cmd/platypus-steam-dl`),
`internal/steamcm`, `docs/plans/*`, `docs/research/steam-cloud-social.md`,
`docs/research/launcher-parity.md`, and the issue list (`gh issue list`,
200 rows, 2026-11-17).

## 0. What's already done (don't re-propose)

Library, install/update/uninstall/verify/move (move via core fallback +
`game.relocated`), branches + beta passwords (incl. `versions.unlock`,
issue #176), family sharing + in-use refusal + `family.warning` (#178, #189),
cloud saves pull/push (issue #6 open but the mechanism is built),
friends/status/chat/unread counts (issue #7 closed), `achievements.list`,
`playtime.list`, terms/EULA listing (#9), game media facts (1.12),
`game.clientStatus` (1.18), `game.saveContext`, storefront embed (#17),
client-required launch mode. Open issues that already cover a gap are named
per row: #11 (install options, verify, move, achievements, playtime, redeem),
#10 (Ubisoft), #6 (cloud), #5 (Epic versions), #2 (PCGamingWiki permission).

Method inventory of `steam-standalone` (from `METHODS` in
`bin/platypus-steam`, [src: file]): everything in protocol up to 1.18
**except** `game.move`, `game.options`, `redeem.submit`,
`settings.schema`/`settings.apply`, `game.acceptTerms`, and any
`compat-facts`/`runner`/`local-art` surface (those last three are other
plugins' capabilities). `library.list` entries carry `clientLaunch` but none
of the 1.15/1.16 hints (`offline`, `cloudSync`, `lastPlayed`, `status`,
`userNotes`). `game.details` carries the 1.12 media facts but no 1.10 compat
facts (`steamAppId`, `platforms`, `drm`).

---

## 1. DLC management (ownership display, per-DLC install/remove)

What it is: the Steam client shows each game's DLC, which are owned, and
installs/removes them independently. Platypus installs a game's depots as one
blob with no DLC concept; issue #11 tracks the generic `install-options` half.

Core support: **yes, protocol-ready.** `install-options` (1.5) defines DLC
groups (`kind: multi`, `dlc: true`, per-value `owned`/`installed`/`storeUrl`)
and `options` on install/update/installSize. The plugin just doesn't
implement `game.options`.

How to build [src: public PICS mirror + repo code]:

- Global DLC list: PICS appinfo `extended.listofdlc` (comma-separated DLC
  appids). **[live]** `api.steamcmd.net/v1/info/377160` returns
  `extended.listofdlc: 404090,435870,…` alongside `developer`, `publisher`.
- Depot→DLC mapping: PICS appinfo `depots.<id>.dlcappid`. **[live]** the same
  app shows ~50 depots, each with `manifests.public` and either no
  `dlcappid` (base game) or one naming its DLC app (e.g. depots tagged
  `490650`, `480631`, `435870`…). Filtering an install to depots whose
  `dlcappid` is unset or owned is exactly the downloader change needed.
- Ownership: the CM already sends `CMsgClientLicenseList` after logon and
  `internal/steamcm` already captures it (`Session.Licenses()`,
  `session.go:41-47` [src]). Mapping licences (package ids) to DLC appids is
  PICS package info (`PICSAccessTokens` already takes package ids,
  `content.go:93-104` [src]).
- Base-game depots sometimes have no `dlcappid` while DLC depots always do
  [inferred from the mirror sample above]; treat "no `dlcappid`" as base.

Account-write? Listing is read-only. Installing/removing DLC writes only game
files (same as installs today). No live test needed for listing; an install
test is a normal download.

Size: **M** (plugin `game.options` + depot filter in `runInstall`/`runSize` +
ledger `archiveRef` already carries depot→manifest maps). Value: **high** —
DLC is the most visible "my game is incomplete" gap.

## 2. Workshop / UGC (subscribe, download, update)

What it is: Subscribe in the client auto-downloads/updates mods into the
game's `workshop/` folder. Platypus has nothing: no protocol capability, no
plugin code (`grep workshop|ugc` hits only the launcher-parity plan [src]).

Core support: **no.** Needs a new capability (shape below) with UI for
subscriptions per game.

How to build [src: SteamDatabase Protobufs, DepotDownloader workshop guide]:

- `PublishedFile` service (`steammessages_publishedfile.steamclient.proto`
  [src], verified 2026-11-17): `Subscribe#1`, `Unsubscribe#1`,
  `GetDetails#1` (returns `file_size`, `hcontent_file`, `time_updated`,
  `consumer_appid`), `QueryFiles#1` (browse/search),
  `GetItemInfo#1` (update checks), `AreFilesInSubscriptionList#1`,
  `SetSubscriptionListFromCollection#1` (whole collections in one call),
  plus `PublishedFileClient.NotifyFileSubscribed/Unsubscribed/Deleted`
  pushes for live updates. All are `ServiceMethodCallFromClient` (EMsg 151),
  the transport `internal/steamcm` already speaks.
- Bytes: DepotDownloader's `-pubfile <id> -app <appid>` downloads any item
  over the content system without the client, anonymous-first, ownership-gated
  for paid games ([src: depotdownloader.com/guides/workshop]). The Go helper
  already speaks that content protocol, so this is reuse, not new transport.
  Caveat from the same guide: items expose **no past versions** and delisted
  means gone — sync is current-only.
- Placement is per-game (GMod `addons/`, Source `maps/` …): needs a small
  per-app rule table, defaulting to `<install>/workshop/<fileid>`.

Account-write? **Yes**: `Subscribe`/`Unsubscribe` change the account. Live
tests need explicit approval. Read-only probes (QueryFiles/GetDetails) need
none.

Size: **L** (new protocol capability + core UI + plugin sync loop). Value:
**high** — mods are everyday Steam use; maps/servers are the headless case
that needs Platypus most.

## 3. Shader pre-caching

What it is: the client pre-downloads Vulkan pipeline caches so games don't
stutter. Platypus downloads no shader content; first runs compile cold.

Core support: no.

How to build: entry point is the official client's own task vocabulary —
`bin/platypus-steam` maps `game.clientStatus` task `CheckShaderDepotManifest`
to `"shaders"` ([src: `bin/platypus-steam:4010`]), i.e. the client fetches
per-app shader depot manifests over the same content system the helper
already speaks. Exact depot selection (per-app shader depots vs shared
driver pipelines) is **not established here** — marked investigation step,
then a `settings`/toggle-gated background fetch into the prefix. **[LIVE]**
to confirm which depots/manifests the client requests for one owned game.

Account-write? No (content download only). Size: **M** after investigation.
Value: **medium** (matters on Deck-class GPUs; invisible otherwise).

## 4. Downloads: bandwidth limit, scheduling, region

What it is: client settings for rate limits, auto-update windows/schedules,
and the download region picker.

Core support: partial. The queue exists; no limits/schedule in core, no
capability for region. The plugin's scheduler already honours the client's
region via `PLATYPUS_STEAM_CELLID` override ([src: plugin README]).

How to build: all three are **core-owned, store-agnostic** (launcher-parity
§1 already classes auto-update/bandwidth/schedule as CORE [src]). Region can
ride the missing `settings` capability (§9): a `select` of CM server-list
regions. No CM/Web API work needed.

Account-write? No. Size: **M** (core) + **S** (region setting). Value:
**medium**.

## 5. Offline mode

What it is: the client works offline from cached licences/content; Platypus's
core never blocks a launch offline (protocol 1.15 [src]), but the plugin
never tells core what it knows.

Core support: **yes.** `LibraryGame.offline` (`ok`/`network`/`unknown`) and
`cloudSync` only need to be sent; core shows the "needs a connection" chip
and serves library snapshots while offline ([src: protocol §1.15]).

How to build: plugin-side only — set `offline: "ok"` for direct-launch games
(the helper needs no CM once manifests/keys are cached… PlayToken caveat:
licence checkouts for client-required games need the client; those get
`"network"`), `cloudSync: false` (Platypus owns cloud), and backfill
`lastPlayed` from `playtime.list`'s `rtime_last_played` (already parsed,
`bin/platypus-steam:2640` [src]). No protocol change.

Account-write? No. Size: **S**. Value: **medium-high** (cheap, kills a whole
class of "can I play on the train" questions).

## 6. News, events and announcements per game

What it is: client/overlay news hub per game.

Core support: no, but core toasts + the game page exist as surfaces; a link
is enough for v1.

How to build: `ISteamNews/GetNewsForApp/v2` is **keyless** ([src:
partner.steamgames.com/doc/webapi]). **[live]** `…/GetNewsForApp/v2/?appid=620&count=2`
returns items with `title`, `url`, `contents`, `feedlabel` with no auth. Either
a plugin method under a tiny capability or just the existing `storeUrl` +
community link-out. Read-only, no account involved.

Size: **S**. Value: **low-medium**.

## 7. Friend invites and "join game"

What it is: invite a friend to your game, see an invite, join a friend's
server from the friends list.

Core support: **no.** `social` (1.6) covers list/status/playing only; chat
(1.9) is 1:1 text. No invite/join surface.

How to build [inferred, needs verification]: at CM level this is the
game-invite signal plus a connect string the game consumes (`+connect <ip>`
for Source-style games; lobby games differ per title). `node-steam-user`
`components/friends.js` (current master, fetched 2026-11-17) exposes
`addFriend`/`removeFriend`/group invites but **no** `inviteToGame`/`joinGame`
[src], so there is no drop-in reference — the EMsg and payload need
identifying from SteamKit2/SteamDatabase (`steammessages_clientserver_friends`
family) **[LIVE]**. Joining itself is a `game.launchSpec` with extra args,
which the protocol already allows.

Account-write? Invites write (social action, needs approval to live-test);
listing an incoming invite is read-only. Size: **M**. Value: **medium**
(multiplayer users only, but it's the friends list's main action).

## 8. Screenshots (local view, upload)

What it is: F12 screenshots, per-game shelf, upload/share.

Core support: partial — `game.details.screenshots` is store marketing art,
not user shots; no user-screenshot surface.

How to build: local-first. The client's on-disk layout
(`userdata/<id3>/760/remote/<appid>/screenshots/`, grid + full) is readable
without login [inferred — verify against a machine with the client installed,
**[LIVE]**]; show thumbnails via the same local-file route as `local-art`.
Upload stays with the client (`ISteamScreenshots` is an in-process SDK API
[src: Steamworks API reference lists `ISteamScreenshots`]). No new CM work.

Account-write? Local view: no. Upload: yes (deferred to client). Size:
**S/M**. Value: **low-medium**.

## 9. `settings` capability (region, toggles) + redeem

Two small protocol gaps in one row because both are "plugin never implemented
an existing method":

- `settings.schema`/`settings.apply` (1.8): the download-region select (§4),
  a shader-cache toggle (§3), per-game flags. Core renders the form; the
  plugin persists under `PLATYPUS_PLUGIN_DATA`. No account writes. **S**.
- `redeem.submit` (1.7, open via issue #11): key redemption at CM level is
  the `ClientRegisterKey` path (SteamKit EMsg family) [inferred — verify
  against SteamKit2 before building, **[LIVE]**]. Writes to the account:
  needs explicit approval to live-test. **S**.

Value: **medium** (settings is an enabler; redeem unblocks key buyers).

## 10. Compat facts in `game.details` + per-game hints

What it is: ProtonDB/Steam Deck badges need per-game facts; Platypus has the
`linux-compat` provider but the Steam plugin withholds two trivial fields.

Core support: **yes** (1.10 compat fields; 1.16 status/notes; `compat.lookup`
already sends `storeAppId`, so Steam games are matchable today [src:
protocol]).

How to build: send `steamAppId` (= the appid, always known) and `platforms`
(PICS appinfo `common` carries the OS list — `oslist` [inferred, one PICS
read to confirm]) from `game_details`. `drm` stays absent (unknown, correctly
shown as no badge). `status`/`userNotes` suggestions: Steam publishes no
per-user status; leave unset. All read-only. **S**. Value: **medium**
(unblocks accurate badges with ~10 lines).

## 11. Parental controls (Steam Families youth settings)

What it is: child accounts with playtime limits and approval flows.

Core support: `family` (1.8) has members/copies/warnings only.

How to build: the plugin already speaks `IFamilyGroupsService`
(`GetFamilyGroup`, `GetSharedLibraryApps` [src: `family_fetch_group`]). Group
responses carry roles/permissions; read-only display ("managed account —
limits enforced by the Steam client") is a small extension of the existing
fetch. Enforcement stays OUT (Platypus must not bypass a parent's limits).

Account-write? No (display only). Size: **S/M**. Value: **medium** for
affected households, low otherwise.

## 12. Multi-account switching, Steam Guard app confirmations

Login (QR/password/Guard code) is done. What's missing: switching between
accounts without full logout/login, and mobile-authenticator confirmations
(trades/market). Confirmations are trade-API surface with no launcher value —
OUT. A switcher is core UX over existing `auth.*` (clear token, begin new
login); the plugin needs no change. **S**, value **low-medium**.

## Explicitly OUT (do not build)

- **Steam Input / controller layouts standalone.** `ISteamInput` is an
  in-process SDK API driven by the running client ([src: Steamworks
  ISteamInput docs]); configs are workshop items consumed through it.
  Platypus stance (unchanged from launcher-parity §1): keep working via
  "Launch through Steam". Documented, not built.
- **Remote Play / streaming.** A separate product (encode/capture/input
  loop); no public CM primitive to reuse. OUT.
- **In-game overlay.** Injected by the client; Platypus has MangoHud/gamescope
  (§1 table already OUT [src]).
- **App tickets / running Steamworks-DRM games without the client.** Against
  repo rules ("no Steam API emulators", protocol §"Out of scope" [src]).
  Launch mode 3 (real client, hidden) is the answer and already exists.
- **Trading cards / inventory / badges / market.** No CM path fit for a
  launcher; Web APIs need a publisher key or web login cookies. Deferred; link
  out to the community pages.
- **Game recording / timeline.** `ISteamTimeline` is in-game SDK surface
  ([src: API reference]); nothing for a launcher to own.
- **Store checkout / purchases / wishlist writes.** By product direction
  (embedded `storefront` view exists, issue #17 closed); money movement never
  goes through core.
- **Chat extras** (group chats, voice, images/history search): protocol 1.9
  deliberately excludes them [src].

---

## Prioritised top 10 (value × feasibility)

| # | Gap | Why this rank |
|---|---|---|
| 1 | DLC ownership + per-DLC install (§1) | High value; protocol + data + licences all exist — pure plugin work |
| 2 | Offline/cloudSync/lastPlayed flags (§5) | ~20 lines, protocol-ready, answers offline play |
| 3 | `settings` capability: region select, shader toggle (§9) | Small enabler for §§3–4 |
| 4 | Compat facts in details: `steamAppId`/`platforms` (§10) | ~10 lines, fixes badges |
| 5 | `redeem.submit` (§9) | Small, account-write, unblocks key buyers |
| 6 | News/events link-out (§6) | Keyless API, tiny |
| 7 | Workshop subscriptions + updates (§2) | Highest remaining user value, but L |
| 8 | Shader pre-caching fetch (§3) | Needs depot investigation first |
| 9 | Friend invites / join game (§7) | Needs EMsg identification first |
| 10 | Bandwidth limit + update schedule (core) (§4) | Core-owned, benefits every store |

## GitHub issue drafts

### 1. Steam: DLC ownership display and per-DLC install
**Scope:** implement `game.options` with a `dlc` group: enumerate
`extended.listofdlc` + per-depot `dlcappid` from PICS, resolve ownership via
the post-logon licence list (`Session.Licenses()`), filter depots at
install/update/size time; changing options on an installed game is a
`game.update` with new options (protocol 1.5).
**Acceptance:** a game with DLC (e.g. Fallout 4, depots tagged per-DLC [live])
shows owned/unowned add-ons; unchecking an owned DLC removes its depots on
update; unowned DLC is refused with the store link; `installSize` counts the
selection.

### 2. Steam: send offline/cloudSync/lastPlayed library hints (1.15)
**Scope:** `library.list` sets `offline: "ok"` for direct-launch games,
`"network"` for client-required ones, `cloudSync: false`, `lastPlayed` from
`rtime_last_played` (already parsed for playtime). No core change.
**Acceptance:** offline launcher shows the "needs connection" chip only on
client games; snapshots serve the library with no network.

### 3. Steam: implement `settings` capability (region, shader toggle)
**Scope:** `settings.schema` with a download-region select (default: current
cell behaviour incl. `PLATYPUS_STEAM_CELLID` override) and feature flags for
later work (shader cache); `settings.apply` persists to plugin data.
**Acceptance:** region change alters CM server-list selection on next
install; values round-trip through core; secrets never touch settings.

### 4. Steam: send compat facts in `game.details`
**Scope:** add `steamAppId` (== appid) and `platforms` (from PICS appinfo
`common`, confirm field name with one PICS read) to `game_details`; leave
`drm` absent.
**Acceptance:** `linux-compat` badges resolve without title matching; no
guessed DRM badge appears.

### 5. Steam: implement `redeem.submit`
**Scope:** confirm the `ClientRegisterKey` CM path against SteamKit2, then
`redeem.submit { code }` → register + human message; map refusal to `-32000`.
Account-write: live tests need explicit approval; never log the code (core
already redacts to first/last two chars).
**Acceptance:** valid key grants the game (approved test), rejected key shows
the store's message, no code in logs.

### 6. Steam: per-game news via keyless ISteamNews
**Scope:** link-out first (community announcements URL in details/store
page); optionally a small method feeding core toasts, using
`ISteamNews/GetNewsForApp/v2` (verified keyless). Read-only.
**Acceptance:** game page links open that game's news; no auth, no account
involved.

### 7. Steam: Workshop subscriptions (new `workshop` capability)
**Scope:** protocol proposal + plugin: `PublishedFile.Subscribe/Unsubscribe/
GetDetails/QueryFiles/GetItemInfo` + `PublishedFileClient` notifications over
the existing CM session; content fetch reusing the depot-download path
(DepotDownloader `-pubfile` pattern, anonymous-first); per-app placement
rules defaulting to `<install>/workshop/<fileid>`; `SetSubscriptionListFrom-
Collection` for collections. Subscribe/Unsubscribe are account-write (approval
required to live-test); no past-version support (upstream limitation).
**Acceptance:** subscribing downloads current files into the game folder,
updates arrive on launch path, unsubscribing removes them, collections
resolve in one call.

### 8. Steam: shader pre-caching download
**Scope:** first identify the depot/manifest set behind the client's
`CheckShaderDepotManifest` task for one owned game (live, read-only), then a
settings-gated background fetch into the prefix at install/update end.
**Acceptance:** documented depot mapping + cold-start frametime improvement on
a Fossilize/RADV title; toggle in settings.

### 9. Steam: friend game invites and join
**Scope:** identify the invite/join EMsg + payload from SteamKit2/
SteamDatabase (`node-steam-user` has no such API — verified), extend `social`
with invite notifications, implement join as `launchSpec` with connect args
for supporting games.
**Acceptance:** incoming invite surfaces as a toast with Join; joining starts
the game with the right connect string where the game supports it.

### 10. Core: download bandwidth limit and update schedule
**Scope:** core-owned (all stores): rate cap on the job downloader path +
auto-update windows in settings; per-job progress already exists.
**Acceptance:** capped downloads hold the configured rate; updates outside the
window wait; works with the Steam plugin unchanged.
