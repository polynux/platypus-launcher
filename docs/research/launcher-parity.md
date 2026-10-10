# Research: launcher parity — what Platypus's core must offer plugins to replace Steam, Epic, GOG and Ubisoft

Status: research only, written 2026-10-09 on `research/launcher-parity`. No account was used, nothing was
downloaded. The plan built on this is [docs/plans/launcher-parity.md](../plans/launcher-parity.md).

Confidence tags: **[src]** read in a primary source during this research (linked). **[known]** general knowledge
of the product, not re-verified here. **[inferred]** reasoned from sources. **[LIVE]** needs a check with the user's
account or a real Wine prefix.

Hard limits that apply to every row below (from the product direction and `docs/plugins/protocol.md`): no DRM or
licence bypass, no Steam API emulator, nothing that modifies game files, no spoofing of client identity, only what
the user's own account is entitled to.

---

## 0. TL;DR

1. **Most store features reduce to about ten store-agnostic capabilities** (library, install lifecycle, DLC,
   versions, cloud saves, social, achievements, playtime, redeem, family). Cloud and social are already designed
   ([steam-cloud-social.md](../plans/steam-cloud-social.md)); the rest are small or medium once the capability
   system is table-driven (§5).
2. **Version selection is genuinely possible on GOG, partly on Steam, barely on Epic, unknown on Ubisoft.** GOG
   publishes a public builds list per product and OS (verified live, no login) and Heroic already ships "pin a
   build" on top of `gogdl --build` [src]. Steam lists *branches* (and their manifest ids) in app info, but no
   store lists *past* manifests; older ones need ids from outside (SteamDB, the user) or from Platypus's own install
   history [src/inferred]. Epic serves only the live manifest; legendary can install from a manifest you supply
   but has no history, and Heroic marks version pinning "not implemented" for Epic [src].
3. **Ubisoft is a different kind of plugin.** There is no maintained open-source downloader or CLI. The realistic
   Linux design is the Playnite/Galaxy-integration one: let the Ubisoft Connect client (in a Wine prefix Platypus
   manages) do the downloading, and have the plugin import the library, start games through `uplay://` URLs and
   track the process tree. Library import can come either from the client's local product cache (no login, but
   needs the client run once) or from Ubisoft's web API (needs a web login and Ubisoft Connect's own application
   ids; see risk U1).
4. **Capabilities today are a flat manifest list that core hard-codes** (`knownCapabilities`, a manifest with an
   unknown name is invalid, and the UI tests `plugin.capabilities.includes(...)`). That works for six names and
   will not scale to fifteen: it needs namespaces with versions, per-game availability, and tolerance for
   capabilities newer than the running core (§5).
5. **Licences matter for reuse.** Playnite and PlayniteExtensions are MIT (their Ubisoft/Epic/GOG/Steam library
   code can be ported with attribution). legendary, gogdl, nile and Heroic are GPL-3.0: keep calling them as
   separate executables (as today), do not copy their code. UplayKit and `galaxy-integration-uplay` are MIT. The
   Platypus repo has no LICENSE file yet, which should be settled before any port.

---

## 1. Feature inventory

Class: **CORE** = store-agnostic capability, core defines the contract and UI, plugins fill it. **PLUGIN** =
store-specific, lives and dies with its plugin (core may offer a generic container, e.g. a "link" or a form).
**OUT** = out of scope, reason given. "Today" is what Platypus has now. Cells for GOG mean GOG Galaxy 2.0 [known];
for Epic the Epic Games Launcher (EGL).

| Feature | Steam | Epic (EGL) | GOG Galaxy | Ubisoft Connect | Class | Platypus today / note |
|---|---|---|---|---|---|---|
| Library list with art | yes | yes | yes | yes | CORE | `library` |
| Collections, favourites, hidden | yes (cloud-synced) | categories (limited) | tags, filters | no | CORE, local to Platypus | Platypus-side tags beat copying each store's; importing Steam collections is a plugin extra |
| Install, update, uninstall | yes | yes | yes | yes (client) | CORE | `install`, `game.update`, `game.uninstall` |
| Verify / repair | "verify integrity" | verify, repair [src: `legendary verify`, `install --repair`] | repair [known]; `gogdl download` re-checks files [inferred] | "verify files" | CORE | new `game.verify` job; no method today |
| Move install | yes (folder move) | `legendary move` [src] | no | no | CORE | Platypus can move the folder and ask the plugin to rebase its records: `game.relocate` |
| Install options (language, optional packs, platform) | language, beta | install tags / selective download [src: `--install-tag`, `--reset-sdl`], platform | language, extras | language | CORE (generic option list) | none: `game.install` has only `installDir` |
| DLC | owned DLC auto-installed per depot | `--with-dlcs`, `--skip-dlcs` [src] | `gogdl download --dlcs` [src] | add-ons in the product cache [src] | CORE | no DLC concept; needs `dlc` listing + selection |
| Redistributables / prerequisites | `_CommonRedist` via client | prereq in manifest | `gogdl redist` [src] | installed by client | PLUGIN | internal to `game.install`; under Wine the runner may need them |
| Versions, branches, rollback | beta branches, password branches; no UI for old builds | none | rollback to older builds | none | CORE (new) | §3 |
| Cloud saves | Steam Cloud | `sync-saves`, `list-saves`, `clean-saves` [src] | `gogdl save-sync` [src] | Ubisoft cloud saves (client) [known] | CORE | designed: `cloud.*` |
| Achievements | per game | per game (EOS) | Galaxy achievements | Ubisoft Club challenges | CORE (read-only) | `game.details.achievements` shows a catalogue; per-user unlock state not defined |
| Friends, presence | CM session | friends service | Galaxy friends | Ubisoft friends API [src: `api-ubiservices.ubi.com/v2/profiles/me/friends` in the Galaxy plugin] | CORE (summary list + status) | designed: `social.*` |
| Chat, voice, groups | yes | yes | yes | yes | PLUGIN, deferred | large per-store protocols; not core |
| Family sharing | Steam Families (6 members, per-copy limit) [src: Steam support FAQ via search] | none [inferred] | none [known] | none [inferred] | PLUGIN (Steam), with a small core hook | entries need an `owner`/`sharedBy` field and a "borrowed" launch rule |
| Parental controls | Steam Families (adult/child, playtime limits) | account-level controls | none | none | PLUGIN, deferred | only detect and show "restricted"; Platypus cannot enforce a child account's limits |
| In-game overlay | injected by the client | EOS overlay, managed by `legendary eos-overlay` [src] | Galaxy overlay | Ubisoft overlay | OUT, except EOS overlay install | the Steam/Galaxy/Ubisoft overlays are injected by their clients; Platypus has its own MangoHud/gamescope overlay. EOS overlay is an Epic-supplied component the user's account is entitled to; optional plugin feature |
| Store page, purchases | yes | yes | yes | yes | PLUGIN (deep link only) | `storeUrl` in game details; no in-app checkout (payment, tax, regional law) |
| Wishlist, store browse | yes | yes | yes | yes | OUT | link out |
| Redeem a key or code | `Store.RegisterKey` / web | redeem code | redeem code (web) | redeem code (web) | CORE (one text field) | `redeem` method returning a message |
| Playtime | `playtime_forever`, client reports | in launcher | local + cloud | Club "time played" [src: Galaxy plugin imports it] | CORE | Platypus measures locally (`session.Tracker`); store-side playtime import is new |
| Launch options, env, args | per-game string | per-game | per-game | no | CORE (already Platypus's) | Platypus has per-game settings; importing a store's launch options is a plugin extra |
| Controller config (Steam Input) | Steam Input, workshop layouts | no | no | no | OUT | needs the client's input layer; "Launch through Steam" keeps it working |
| Mods / Workshop | Steam Workshop (needs `ISteamUGC`, i.e. the client) | no | no | no | PLUGIN, deferred; direct launch OUT | games find subscribed items through the Steamworks API; without an emulator that only works when the real client launches the game |
| Notifications | yes | yes | yes | yes | CORE | Platypus toasts exist; plugin-pushed events (friend online, achievement) are new |
| Offline mode | client offline mode | `launch --offline` [src] | GOG games are DRM-free; client not required | client offline mode | CORE | cache library, skip cloud/social, per-plugin "offline launch" flag |
| Auto-update, bandwidth limit, schedule | yes | yes | yes | yes | CORE | queue exists; limits and schedule absent |
| Account and profile | name, avatar | name | name, avatar | name, avatar | CORE | `auth.status.user` |
| Remote Play / streaming | Remote Play | no | no | no | OUT | large separate product |
| News, community hub, reviews | yes | yes | yes | yes | OUT | link out |

Reading the table: core needs about ten new capabilities beyond today's `library/install/launch/auth/discard`:
`verify`, `move` (relocate), `install-options` (which also carries DLC and language), `versions`, `cloud-saves`,
`social`, `achievements`, `playtime`, `redeem`, `family`. Everything else is either already core-owned, a plain link, or out.

---

## 2. How others do it

### 2.1 Playnite and PlayniteExtensions (MIT)

Repos: [JosefNemec/Playnite](https://github.com/JosefNemec/Playnite) (MIT),
[JosefNemec/PlayniteExtensions](https://github.com/JosefNemec/PlayniteExtensions) (MIT) with library plugins for
Amazon, Battle.net, Epic, GOG, Humble, itch.io, Rockstar, Steam, **Uplay**, Xbox [src: repo tree].

Plugin model, per the [SDK docs](https://api.playnite.link/docs/tutorials/extensions/libraryPlugins.html) [src]:

- **Three plugin types**: `GenericPlugin` (events, UI hooks), `LibraryPlugin` (`GetGames` returns library
  entries; optional `Client`, settings view), `MetadataPlugin` (fields from other sources). A metadata source
  is *not* part of a store plugin.
- **Controllers** instead of RPC verbs: a library plugin returns `InstallController`, `PlayController` and
  `UninstallController` objects for a game (`GetInstallActions`, `GetPlayActions`, `GetUninstallActions`); the
  controller raises `InvokeOnInstalled(GameInstallationData)`, `InvokeOnStarted`, `InvokeOnStopped(session
  length)`, `InvokeOnUninstalled`. This is the same shape as Platypus's jobs plus session tracking.
- Plugins are in-process .NET: no isolation, no capability gating beyond which virtual members are overridden.

What it teaches Platypus:

1. **Metadata is a separate plugin axis.** Platypus already treats ProtonDB as built in and `game.details` as optional
   per store; a `metadata` capability (SteamGridDB, IGDB) in its own plugin would keep store plugins small.
2. **The "client does the work" pattern is first-class.** The Ubisoft controllers don't download anything: install
   is `uplay://install/<id>` and uninstall `uplay://uninstall/<id>`, then a watcher polls the installed list until
   the game appears [src: `UplayGameController.cs`]. Play is `uplay://launch/<id>`, wait for
   `UbisoftGameLauncher` to run, then watch the install directory's process tree for the game.
3. **The library is read from the client's disk state, not from the web.** `Uplay.GetLocalProductCache()` parses
   `%LOCALAPPDATA%\Ubisoft Game Launcher\cache\configuration\configurations` (a protobuf whose records hold a YAML
   product description) and fails with "start the client at least once" when absent [src: `Uplay.cs`]. Installed
   games come from `HKLM\SOFTWARE\ubisoft\Launcher\Installs\<id>\InstallDir` [src: `UplayLibrary.cs`]. DLC are
   filtered by `root.addons`, `is_ulc`, `third_party_platform != null` and `start_game == null`.

Reusable (MIT, with attribution in the file header and a NOTICE entry): the protobuf/YAML schema in
`UplayLibrary/Models/ProductSchema.cs` and `LocalCache.cs`, and the DLC/third-party filtering rules. Must build:
reading `system.reg` of a Wine prefix instead of the Windows registry, a Go/Python protobuf reader, and everything
about running the client in Wine.

### 2.2 Heroic (GPL-3.0) and the CLIs it wraps

[HeroicGamesLauncher](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher) splits each store into a
"store manager" (`src/backend/storeManagers/{legendary,gog,nile,sideload,zoom}`: games, library, user, setup)
[src]. That is the same decomposition as Platypus's plugin methods, which is some evidence the protocol's shape is
right. Relevant findings:

- **GOG version pinning exists.** `gog/games.ts` passes `--build <id>` and `--branch <name>` to `gogdl`, stores a
  per-game private-branch password (`privateBranchesStore`), records `buildId`/`versionEtag` on install, and sets
  `pinnedVersion` so updates are skipped [src: `gog/games.ts`, `gog/library.ts`]. The UI is a "modify install"
  modal for GOG only.
- **Epic version pinning does not exist**: `changeVersionPinnedStatus` throws "not implemented on Legendary
  Library Manager" [src: `legendary/library.ts`].
- **Cloud saves**: `legendary sync-saves` and `gogdl save-sync` (already analysed in
  [steam-cloud-social.md](steam-cloud-social.md) §1.5).
- **Ubisoft is a shim.** Heroic's wiki describes adding Ubisoft Connect as a game in a Wine prefix, installing games
  inside it, then adding per-game entries that launch `uplay://launch/<id>`; recent notes recommend the latest
  GE-Proton, warn about a login webview that fails on some systems (`WEBKIT_DISABLE_DMABUF_RENDERER=1` is a
  reported workaround), and note an install-button bug in Heroic 2.20
  ([wiki](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/wiki/How-to-install-Ubisoft-Connect-on-Linux-and-Mac),
  [Lutris forum](https://forums.lutris.net/t/cant-login-to-ubisoft-connect/13673)) [src via search].

CLI facts that matter here (all [src], from `--help`/source):

| Tool | Licence | Relevant surface |
|---|---|---|
| [legendary](https://github.com/legendary-gl/legendary) | GPL-3.0 | `install/update/repair`, `verify`, `move`, `import`, `list-files --manifest`, `install --manifest <uri>`, `--platform`, `--install-tag`, `--with-dlcs`, `launch --offline --dry-run`, `sync-saves`, `list-saves`, `clean-saves`, `eos-overlay`, `egl-sync`, `info` |
| [gogdl](https://github.com/Heroic-Games-Launcher/heroic-gogdl) | GPL-3.0 | `auth`, `download` (`--branch`, `--password`, `--build`, `--lang`, `--dlcs`, `--dlc-only`), `info`/size (returns the builds list and available branches), `redist`, `launch`, `save-sync`, `save-clear`, `lang-match` |
| [nile](https://github.com/imLinguin/nile) | GPL-3.0 | Amazon Games; already wrapped |

GPL consequence: spawning these as external programs and parsing their output is how the bundled plugins already
work and is fine. Copying their code into Platypus would impose GPL on Platypus; so don't. Platypus's own licence is
undecided (no LICENSE file in the repo), which should be settled first.

### 2.3 Ubisoft tooling

| Project | Licence | What it is | Use for Platypus |
|---|---|---|---|
| [PlayniteExtensions UplayLibrary](https://github.com/JosefNemec/PlayniteExtensions/tree/master/source/Libraries/UplayLibrary) | MIT | local-cache library import, `uplay://` install/launch/uninstall | **Port the data model and rules**; Windows-registry and file paths need Wine equivalents |
| [melcom-creations/galaxy-integration-uplay](https://github.com/melcom-creations/galaxy-integration-uplay) | MIT, active (pushed 2026-09) | GOG Galaxy plugin: web login, entitlements, friends, club titles, playtime, stats | **Best reference for the web API**: endpoints, headers and the `Ubi_v1 t=`/`rm_v1 t=` ticket refresh flow |
| [UplayDB/UplayKit](https://github.com/UplayDB/UplayKit) | MIT, last push 2025-09 | C# client for `dmx.upc.ubisoft.com` (the protobuf "demux" service the client uses), incl. `Authenticate`, `GetPatch` | Reference for ownership and download manifests; **not** verified to download; unmaintained-ish |
| [YoobieRE/ubisoft-demux-node](https://github.com/YoobieRE/ubisoft-demux-node) | none stated | Node client for the same demux API | Reference only (no licence = no reuse) |
| Lutris installer scripts | GPL-3.0 | run Ubisoft Connect in Wine | Reference for the Wine dependency set |

I found **no maintained open-source tool that downloads Ubisoft games outside Ubisoft Connect** [LIVE/negative
result; searches on GitHub and the web]. UplayKit's demux client shows ownership and patch queries exist, but
whether it can fetch game content was not established here. So a Platypus Ubisoft plugin should not promise native
downloads (see §4).

### 2.4 Reuse summary

| Need | Reuse (with attribution) | Build |
|---|---|---|
| Ubisoft product cache schema, DLC filter | Playnite `UplayLibrary` (MIT) | Go reader, Wine `system.reg` parse |
| Ubisoft web API calls | `galaxy-integration-uplay` (MIT) | Platypus-side login dialog, token storage in the plugin |
| Library plugin concepts, controller events | Playnite SDK (design only) | none (already jobs + session tracking) |
| GOG version list and pin semantics | Heroic design (GPL: reimplement from the public API) | `versions.*` over `gogdl info` and the content-system builds API |
| Steam branch and manifest listing | SteamKit/DepotDownloader behaviour; Platypus's `steamcm` already has `ManifestRequestCode(branch, passwordHash)` and `CheckBetaPassword` | listing, password UX, rollback bookkeeping |
| Epic old builds | none | nothing is possible without outside manifests |

---

## 3. Game version selection

Goal: a store-agnostic "install a different build" capability. What each store technically allows:

### 3.1 Steam

- **Branches are listed in app info** (`depots.branches.<name>` with `buildid`, `description`, `timeupdated`,
  and `pwdrequired` for password branches) and each depot's `manifests.<branch>.gid` gives that branch's manifest id.
  Verified on a public app via the PICS mirror `api.steamcmd.net/v1/info/620`: branches `public`, `beta`,
  `previous_release`, `demo_viewer`, each with its own `buildid` and depot `gid` [src].
- **Password branches**: `ClientCheckAppBetaPassword` (EMsg 5450) returns the decryption key for the branch;
  Platypus's `steamcm.CheckBetaPassword` exists [src: `internal/steamcm/content.go`]. DepotDownloader exposes this as
  `-branch` and `-branchpassword` [src: its README via search].
- **Old manifests**: the Steam client never lists past manifests for a depot; only each branch's *current* one.
  DepotDownloader can download an arbitrary `-manifest <id>` given `-depot` [src: README via search], and the
  manifest request code (`ContentServerDirectory.GetManifestRequestCode`, required since 2022, valid minutes,
  requires owning the depot) is what authorises it [src: SteamDB FAQ via search; Platypus's `ManifestRequestCode`].
  Where do old ids come from? SteamDB's depot history (no API, scraping forbidden [src: SteamDB FAQ]), the user, or
  **Platypus's own record of what it installed before**. That last source is the only one Platypus can offer reliably.
- **Does the CDN still serve old chunks?** Usually yes for recent builds, not guaranteed [inferred]; Valve can
  retire content. A rollback must fail cleanly on a missing manifest or chunk. [LIVE] with a real older manifest.
- **Decryption key**: the depot key is per depot, not per manifest, so a fresh key request works for any manifest
  [src: TEK Steam Client README via search: "depot decryption keys are never updated"].
- **Playing an old build**: the game's Steam Cloud saves and online features may break against an old build;
  Platypus must warn, not block.

### 3.2 GOG

- **Public builds list, no login needed**: `GET https://content-system.gog.com/products/<id>/os/<windows|osx|linux>/builds?generation=<1|2>[&password=…]`
  returns `items[]` with `build_id`, `branch` (null = default), `version_name`, `tags`, `public`,
  `date_published`, `generation`, `link` (manifest). Checked live for one product: `total_count 13`, `count 4` for
  generation 2, versions `1.5`, `1.5 (A)`, `1.5 (CS) GOG 0.2` going back to 2017 [src: live call to the public
  endpoint]. Generations 1 and 2 are listed separately; `gogdl` picks by build.
- **gogdl**: `download <id> --build <build_id> --branch <name> --password <pw>`; the build is matched on
  `build_id`; no `--build` = latest of the default branch; `info` returns the builds and
  `available_branches` [src: `gogdl/args.py`, `dl/managers/manager.py`, `v2.py`]. Heroic stores the pin.
- **So GOG supports full history** as far as the store publishes it (GOG keeps builds; some old ones are
  removed). Legacy generation-1 builds work through gogdl's v1 manager.

### 3.3 Epic

- Epic serves the **live manifest per platform and label**; `legendary install` downloads from it. Flags that can
  change what is installed: `--manifest <uri|path>` (use a given manifest), `--old-manifest`/`--delta-manifest`
  (for patch testing), `--base-url`, `--platform` [src: `legendary --help`].
- No command lists past versions, and Epic exposes no history API [src: absence in legendary's help; Heroic: "not
  implemented"]. A rollback only works when Platypus *kept* a manifest file from an earlier install (legendary stores
  manifests on disk; `legendary cleanup --keep-manifests` retains them [src]) and the CDN still serves its chunks.
  Platypus can therefore offer **"go back to what I installed before"** for Epic if the plugin archives the manifest
  at install time, and nothing else. [LIVE] to check chunk retention.

### 3.4 Ubisoft

- The client has "patch tracks" (the demux `GetPatchInfo` takes a `trackId`, default `"DEFAULT"`; UplayKit
  exposes `GetPatch(trackId, trackType)` [src: `DemuxSocketExt.cs`]). Whether tracks other than `DEFAULT` exist
  publicly, or whether the client can install old versions, is unknown. Treat as **unsupported** until proven.

### 3.5 Proposed store-agnostic shape

The shape (details in the plan): `versions.list { id }` returns *channels* and *builds*; `versions.install` is a
normal install job with a `target`. Differences between stores are expressed as data, not branches in core:

- `kind`: `channel` (a moving pointer: Steam branch, GOG branch) or `build` (an immutable build: GOG build id, a
  Steam manifest set). Core shows channels as a dropdown and builds as a history list.
- `requires`: `none`, `password` (branch password, never stored by core), `manifest-id` (user types or pastes it),
  `archived` (only builds Platypus/the plugin saw).
- `pinned`: the install should not auto-update. Core keeps "pinned build X" in game settings and tells the plugin
  on `game.update`.
- Honest capability: a plugin lists `versions` only if it can do at least one of channel or build selection; the
  per-game answer comes from `versions.list` (empty channels are fine).

Core also needs a **local install ledger** (store, game id, installed build/channel, date, manifest reference the
plugin returned) that outlives updates. It is what turns "no history API" into "rollback to previous" for Steam
and Epic. The ledger lives in core because it is store-agnostic, but plugins decide what an `archiveRef` means.

---

## 4. Ubisoft on Linux

What a Platypus Ubisoft plugin can realistically do:

| Capability | Feasible? | How | Risk |
|---|---|---|---|
| Library import | yes | (a) read the client's local product cache in the Wine prefix [src: Playnite]; requires the client to have been run and logged in once; (b) web API with a web login [src: Galaxy plugin] | U1, U2 |
| Installed games | yes | Wine `system.reg` key `HKLM\SOFTWARE\(WOW6432Node\)ubisoft\Launcher\Installs` [src: Playnite uses it on Windows; the Wine path is **[LIVE]**] | low |
| Launch | yes | start `UbisoftConnect.exe` / `uplay://launch/<id>` in the plugin's prefix, then follow the game's process tree (Platypus's `store-client` delegate with a Wine-aware `find`) | needs a launch-spec extension (§plan) |
| Install, update, uninstall | via the client | `uplay://install/<id>`, then watch the install list [src: Playnite]; progress shown by the client's own window, so `job.progress` is coarse | U3 |
| Native download | **no** | no maintained open-source tool; would need the demux protocol and an ownership token [src: UplayKit exists, content download unverified] | U4, ToS |
| Cloud saves | via the client | Ubisoft Connect syncs on launch/exit when launched through it; Platypus does nothing | none |
| Friends, achievements, playtime | web API | `api-ubiservices.ubi.com` friends and club endpoints [src: Galaxy plugin] | U1 |
| Verify, versions | no | client-only | n/a |

**Auth flow, as the Galaxy plugin does it** [src: `consts.py`, `backend.py`]:

1. Open `https://connect.ubisoft.com/login?appId=<id>&genomeId=<id>…` in a web view and wait for the redirect to
   `connect.ubisoft.com/change_domain/…`; JS injected into the page reads `localStorage` for `userId`,
   `nameOnPlatform`, `ticket`, `rememberMeTicket`, `sessionId`.
2. Call `public-ubiservices.ubi.com/v3/profiles/sessions` with `Authorization: Ubi_v1 t=<ticket>` and
   `Ubi-AppId`; refresh the ticket before its `refreshTime`; renew via the `rm_v1 t=<rememberMeTicket>` header.
3. Entitlements from `…/ubiconnect/entitlement/api/entitlements`, details by `spaceIds`.

For Platypus (Wails, no embedded web view guarantee, Linux): the plugin protocol's `auth.begin` already supports
"open this URL in the browser, then paste fields". Ubisoft's flow ends on a page whose data lives in
`localStorage`, which a browser tab can't hand over. Options: (1) the user pastes the ticket from the browser's dev
tools (poor), (2) a small helper using a system web engine or a local redirect, (3) rely on the client's own
login in the Wine prefix and read its caches (no Platypus login at all). Option 3 is the cheapest and the safest.

**Risks**

- **U1: client identity.** The API rejects arbitrary app ids; the Galaxy plugin reads `Ubi-AppId` from the Ubisoft
  Connect executable and notes the login id must differ from it. Sending Ubisoft Connect's own app id from
  another program is "acting as the official client". The Steam plan forbids spoofing client identity; the same
  stance should apply here. **Needs a product decision (Q-U1).** Until then use only the local cache.
- **U2: schema drift.** The product cache is a private format; the Galaxy plugin repeatedly patches it.
- **U3: UI fragility.** Install via `uplay://` needs the client window and a working login webview under
  Wine/Proton; Heroic and Lutris threads show login failures. [LIVE] on this machine's Proton.
- **U4: terms of service** for any direct download.
- **U5: Wine prefix isolation.** Heroic notes all games share one prefix; Platypus's per-game prefixes would need a
  dedicated shared "Ubisoft Connect prefix" per plugin.

**Recommendation**: Ubisoft plugin, phase A (library from local cache, launch/install/uninstall through the
client, installed detection) — useful and low-risk. Phase B (web API for friends and playtime) only after Q-U1.
No native downloads.

---

## 5. The capability system

### 5.1 Today [src: `internal/plugins/manifest.go`, `process.go`, `plugtest.go`, `stores/plugin.go`, frontend]

- The manifest lists capability names: `library`, `install`, `launch`, `auth`, `cloud-saves`, `discard`.
- `knownCapabilities` is a hard-coded map; an unknown name makes the manifest **invalid**. A plugin written for a
  newer Platypus therefore cannot even be listed by an older one.
- `initialize` returns `capabilities` too; the conformance suite requires it to equal the manifest's. The manifest
  is what the UI uses (`plugin.capabilities.includes('install')` in `GamePage.tsx`, `PluginsView.tsx`).
- Gating is per capability, not per method: `Installable: lg.Installable && s.has("install")`. A missing optional
  method is signalled by `-32601`, used for `game.details`.
- Capabilities are global per plugin. There is no way to say "versions work for this game only", which is what
  the reality is (a Steam game with no beta branches; a GOG game with one build; an Epic game with no archive).

### 5.2 Design forces

1. New feature = new RPC namespace + new UI surface + (sometimes) a core orchestrator (cloud, social).
2. Plugins and core ship separately; both directions of mismatch must degrade gracefully.
3. Users should see features appear only where a plugin really offers them, and see why a missing one is missing.
4. The same capability differs in depth by store (GOG builds vs Steam channels); depth must be data.
5. Store-specific replication must stay in the plugin so it can be dropped.

### 5.3 Proposal (details in the plan)

1. **Namespaced, versioned capabilities.** Names stay flat strings but may carry a version suffix:
   `cloud-saves`, `versions@1`. Core knows a registry (`capability -> { methods, minProtocol, surfaces }`) as
   data, not scattered `if` statements.
2. **Unknown capabilities are tolerated.** An unrecognised name is ignored with a warning on the Plugins view
   ("this plugin offers X, which this Platypus doesn't use"); only a malformed name is invalid. Plugins can mark a
   capability `required` to refuse an older core.
3. **`initialize` is the truth, the manifest is the promise.** The manifest declares the maximum (shown to the
   user before approval, hashed); `initialize` returns the subset actually available right now with feature flags
   (`{"versions": {"channels": true, "builds": true, "password": true}}`), so a missing tool or a logged-out state
   switches features off without editing files. Conformance: initialize ⊆ manifest.
4. **Per-game availability** through optional fields on `LibraryGame` / `game.details`
   (`features: ["versions","dlc","cloud"]`), so the Versions control appears only for games that have something to
   choose. Failing that, the method answers an empty result.
5. **Per-method negotiation stays JSON-RPC's `-32601`.** Optional methods inside a declared capability are
   probed once and cached.
6. **UI surfaces are bound to the registry.** A surface (cloud row, versions menu, friends summary, DLC list,
   redeem entry, family badge) declares the capability it needs and renders nothing otherwise. A disabled-state
   explanation ("Steam Cloud needs a login") comes from the plugin through the state it returns.
7. **Core orchestrators are capability clients, not store code.** `cloudsync`, `social`, `versions` talk to
   `Store` interface extensions (`stores.CloudStore`, …) that `stores.Plugin` implements by RPC and that the
   built-in Steam path may not implement at all. Adding a store never needs core changes; adding a capability
   does, once.

---

## 6. Risks and unknowns

| ID | Risk | Mitigation |
|---|---|---|
| R1 | Capability sprawl: ten new names tempt plugins to declare things half-done | Conformance suite per capability; `initialize` reports reality |
| R2 | Steam rollback relies on old manifests existing on the CDN and on ids Platypus can't list | Ledger-based "previous install" rollback; manual manifest id; clear failure |
| R3 | Epic rollback only via archived manifests | Plugin archives at install; otherwise hide `versions` |
| R4 | Ubisoft private formats and login webview under Wine | Local-cache-first, client does the work, [LIVE] on this machine |
| R5 | Ubisoft web API needs the client's app id (U1) | Product decision; ship without |
| R6 | GPL code must not be copied | External processes only; settle Platypus's licence |
| R7 | Family and parental rules are server-side and change | Plugin reads the state and shows it; Platypus enforces nothing it can't verify |
| R8 | Launching a store client inside Wine needs a launch-spec extension the current `store-client` mode lacks (`command[0]` must be in `requires`, runs natively) | Plan §"Core changes" |
| R9 | Workshop/Steam Input need the real client; a direct launch can't provide them without an emulator | Keep "Launch through Steam"; mark capabilities `OUT` |

## 7. Sources

- Platypus: `docs/plugins/protocol.md`, `docs/plans/launcher-plugins.md`, `docs/plans/steam-cloud-social.md`,
  `internal/plugins/manifest.go`, `internal/steamcm/content.go`, `plugins/*/plugin.json`.
- Playnite SDK: <https://api.playnite.link/docs/tutorials/extensions/libraryPlugins.html>,
  <https://api.playnite.link/docs/tutorials/extensions/gameActions.html>,
  <https://github.com/JosefNemec/PlayniteExtensions/tree/master/source/Libraries/UplayLibrary>.
- Heroic: <https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher> (`src/backend/storeManagers/*`),
  <https://github.com/Heroic-Games-Launcher/heroic-gogdl> (`gogdl/args.py`, `dl/managers/*`),
  <https://github.com/legendary-gl/legendary> (README `--help`), <https://github.com/imLinguin/nile>,
  <https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/wiki/How-to-install-Ubisoft-Connect-on-Linux-and-Mac>.
- Ubisoft: <https://github.com/melcom-creations/galaxy-integration-uplay> (`src/backend.py`, `consts.py`),
  <https://github.com/UplayDB/UplayKit>, <https://github.com/YoobieRE/ubisoft-demux-node>.
- GOG content system (live): `https://content-system.gog.com/products/<id>/os/<os>/builds?generation=2`;
  build database <https://www.gogdb.org>.
- Steam: <https://api.steamcmd.net/v1/info/620> (PICS mirror), <https://github.com/SteamRE/DepotDownloader>,
  <https://steamdb.info/faq>, <https://www.steamdb.com/en/articles/steam-family-sharing-complete-guide>,
  Steam Support Steam Families FAQ.
