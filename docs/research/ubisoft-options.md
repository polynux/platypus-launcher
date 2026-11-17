# Research: options for a Ubisoft plugin (issue #10)

Status: research only, written 2026-11-17 on `docs/ubisoft-options-10`. No account
was used, nothing was downloaded, no code changed. Builds on
[launcher-parity.md](launcher-parity.md) §4 (written 2026-10-09); new evidence since
then is marked **(new)**. Confidence tags: **[src]** read in a primary source during
this research (linked). **[known]** general knowledge, not re-verified here.
**[LIVE]** needs a check with the user's account or a real Wine prefix (L-U1/L-U2 in
the parity plan).

Hard limits (unchanged): no DRM or licence bypass, no Steam-API-style emulators,
no game-file modification, no client-identity spoofing beyond what the user
approves, only what the user's own account is entitled to.

## 0. TL;DR

1. **Path 1 (client in a Platypus-managed prefix) is still the only shippable
   path.** Ubisoft Connect runs on Linux today under Wine/Proton (official Ubisoft
   Steam Deck login docs exist **[src]**), and every active project converges on
   the same architecture: the real client downloads/installs/launches, the
   launcher only orchestrates it and reads its on-disk state.
2. **(new)** Two 2026 projects prove more of the design space than was known in
   October: `xXJSONDeruloXx/ubi-cli-mvp` (MIT, TypeScript) validated live Demux
   ownership + manifest + slice downloads and a full 5,320-file game
   reconstruction **[src]**; `Starkka15/Optima` (GPL-3.0, Rust) validated the full
   loop — Demux download, seeding the official client's staging, official-client
   finalisation, `uplay://launch` — then deliberately moved launch *back* to the
   official client for DRM/ownership reasons **[src]**.
3. **(new)** `mubaraknumann/unifideck` (spec dated 2026-06-22) independently
   converged on exactly the parity-plan phase-A design: auth in the real client,
   library from local binary catalogs, install driven by the real client with
   honestly-indeterminate progress, `upc.exe uplay://launch/{id}/0` for launch
   **[src]**. Three independent implementations of the same shape is strong
   evidence it is the right shape.
4. **Path 2 (native downloads without the client) is technically demonstrated but
   not shippable for Platypus**: it works per-title, needs the client's own
   registration/finalisation step anyway (a raw reconstructed tree is *not*
   accepted as installed until the official client blesses it **[src]**), every
   game's DRM check still needs the client at launch, and the only complete
   implementation (Optima) is GPL-3.0, which Platypus (licence undecided, leaning
   MIT/Apache) cannot absorb.
5. **Path 3 (hybrid: web API for metadata/friends/playtime, client for
   install/launch) is viable but gated on Q-U1** (product decision: is Platypus
   allowed to send Ubisoft Connect's own app id?). The reference implementation
   (`melcom-creations/galaxy-integration-uplay`, MIT, actively maintained into
   2026 **[src]**) shows exactly what the web API gives and how fragile it is
   (app-id rotation, 403s, schema drift — fixed repeatedly in 2026 releases).
6. Recommendation: **phase A as planned** (local-cache library, `wine-client`
   launch delegate, client-driven install/uninstall), **phase B only after Q-U1**,
   **no native downloader**. Concrete issues proposed in §6.

---

## 1. Path 1 — Ubisoft Connect in a Wine/Proton prefix Platypus manages

### 1.1 It works today, officially acknowledged

- **Ubisoft documents Steam Deck login.** Ubisoft's own help site has "Logging
  into Ubisoft Connect on your Steam Deck" (article 000108819) **[src:
  ubisoft.com/en-ca/help/.../000108819]**. A vendor publishing first-party
  Proton-login instructions is the strongest possible "works today" signal.
- **Setup guides are current.** PulseGeek published "Ubisoft Connect on Steam
  Deck With Proton: Setup" 2026-02-16 **[src]**; decky.net published a Uplay
  troubleshooting guide ~Dec 2025 **[src]**; pimylifeup and Windows Central
  document the Steam-shortcut + Proton-Experimental flow **[src]**.
- **ProtonDB:** e.g. Assassin's Creed Origins reports playability with tinkering
  (Experimental, secondary-launcher friction noted) **[src: protondb.com/app/582160]**.

### 1.2 What each launcher does (recipe for Platypus to follow)

- **Lutris installer `ubisoft-connect-latest`** (script version published ~Apr
  2026) **[src: lutris.net/games/install/5228/view]**:
  - installer URL: `https://ubistatic3-a.akamaihd.net/orbit/launcher_installer/UbisoftConnectInstaller.exe`
  - `create_prefix`, then winetricks `d3dcompiler_43` + `corefonts`,
  - writes `settings.yaml` disabling the overlay
    (`overlay.enabled: false … user.closebehavior: CloseBehavior_Close`),
  - runs the installer with `wineexec args: /S` (silent).
  - So: **silent install is `/S`**, deps are two winetricks verbs + overlay off.
    Platypus's `client.setup` job can do exactly this.
- **Heroic wiki "How to install Ubisoft Connect on Linux and Mac"** **[src]**:
  add Connect as a (non-Epic) game in its own prefix, install games inside it,
  add per-game entries launching `uplay://launch/<id>`; recent notes recommend
  latest GE-Proton. Heroic 2.20 had a known install-button bug **[src:
  launcher-parity.md §2.2]**; Heroic issue #5805 (2026) records Legendary
  0.20.43 failing to launch Ubisoft games bought on Epic **[src]** — relevant:
  Epic-sold Ubisoft titles need Connect *inside that game's prefix* (Heroic
  issue #2479, 2023) **[src]**.
- **Bottles** ships a curated installer; breakage reports are about shortcuts
  not recognised (issue #4400, ~Feb 2026) and client updates breaking the
  curated recipe (issue #2648, 2023) **[src]** — i.e. maintenance is "track
  upstream", not "fix once".
- **Faugus launcher**: install-problem threads as recent as May 2026
  (linuxguides.de) **[src]** — installer friction is ongoing but solvable.
- **Steam + Proton directly**: the IBBoard gist documents running Uplay from an
  AC:Liberation install under Proton **[src]**; Steam Deck flows add Connect as
  a non-Steam shortcut with Proton Experimental **[src: pimylifeup]**.

### 1.3 `uplay://` protocol and client CLI surface

| URL / command | Meaning | Source |
|---|---|---|
| `uplay://launch/<id>/0` (also `/1` for multiplayer, e.g. AC IV `565/0`, `565/1`) | launch installed game | **[src: RizSavio/UplayGameIDs]**; unifideck launches `upc.exe uplay://launch/{id}/0` **[src: ubisoft-store-spec.md §7]** |
| `uplay://install/<id>` | open client's install confirmation for the product | **[src: Playnite `UplayGameController.cs`]**; validated live 2026-07-12: `uplay://install/82` opened the official language confirmation directly **[src: ubi-cli-mvp DEMUX_JOURNAL.md]** |
| `uplay://uninstall/<id>` | uninstall via client, then poll installed list | **[src: Playnite]** |
| `UbisoftConnectInstaller.exe /S` | silent client install | **[src: Lutris installer]** |
| installer URL `ubistatic3-a.akamaihd.net/orbit/launcher_installer/…` | pinned official client | **[src: Lutris installer]**; ubi-cli-mvp pins + SHA-256-verifies a CDN build **[src]** |

Limits found live: `uplay://install/109` was *accepted* but showed no confirmed
non-interactive install action against an already-installed product, and no
supported language/destination/EULA/auto-confirm URI parameters were found in any
reviewed source — the client confirmation dialogs stay manual **[src: ubi-cli-mvp
research.md]**. So `game.install` progress through the client is honestly
**indeterminate** (unifideck models `download_phase = "manual"` with Cancel =
SIGTERM the client **[src: ubisoft-store-spec.md §6]**). Platypus's job model
should do the same rather than fake a percentage.

### 1.4 Known breakages (recurring themes, 2025–2026)

- Login webview fails on some systems; reported workaround
  `WEBKIT_DISABLE_DMABUF_RENDERER=1` **[src: Heroic wiki via launcher-parity.md]**.
- Black window on start (Lutris forums, Jul 2025) **[src]**; CDN 403s during
  install (Lutris #6254, Nov 2025) **[src]**; "installer yields paths the
  launcher can't find" (Lutris #6252, Nov 2025) **[src]**; install script
  reported broken (Lutris #6599, Mar 2026, closed as duplicate) **[src]**.
- Client updates periodically break third-party recipes (Bottles #2648)
  **[src]**; Ubisoft moved cache/user-data locations before (`<InstallDir>\cache`
  → `%LOCALAPPDATA%\Ubisoft Game Launcher\cache`, caught by galaxy-integration
  v2.0.3) **[src: CHANGELOG.md]**.
- Anti-cheat titles (e.g. Division 2 + EAC loop, Mar 2025) remain the hard edge
  **[src: Lutris forums]** — same class of limitation as any Wine story, not
  Ubisoft-specific.
- Conclusion: breakage is real but operational (recipes, webview flags, cache
  paths), not structural. Mitigation is version-pinned installer + `setup --check`
  style validation (ubi-cli-mvp's approach **[src]**) and reading cache dirs with
  fallbacks.

### 1.5 Linux compatibility verdict

**Compatible today** for the single-player catalogue via any Wine/Proton runner,
with first-party Steam Deck docs. No kernel anti-cheat story (same as the rest of
Platypus's Wine surface).

## 2. Path 2 — direct protocol access without the client

### 2.1 The two back ends

Confirmed by every reverse-engineering source: (a) public HTTPS API
`public-ubiservices.ubi.com`, (b) the launcher's socket/protobuf "Demux" service
`dmx.upc.ubisoft.com:443` (TLS 1.2) **[src: ubi-cli-mvp research.md; YoobieRE]**.

### 2.2 Demux/UPC protocol

- Working handshake (validated live Mar 2026) **[src: ubi-cli-mvp
  DEMUX_JOURNAL.md]**: TLS 1.2 → `getPatchInfoReq` → adopt
  `clientVersion = latestVersion` (hard-coded `10931` in the old Node package is
  stale) → `authenticateReq { clientId: "uplay_pc", token.ubiTicket }` → open
  `ownership_service` → `initializeReq { getAssociations: true, protoVersion: 7,
  useStaging: false, ubiTicket, ubiSessionId }` → ownership enumeration,
  ownership tokens, download-service init, signed `.manifest` / `.metadata` /
  `.licenses` URLs.
- Reference implementations: **UplayDB/UplayKit** (C#, MIT, last push 2025-09 —
  maintenance: slow/inactive) **[src: github.com/UplayDB/UplayKit]**;
  **YoobieRE/ubisoft-demux-node** (no licence stated → **no reuse**), plus
  `ubisoft-dmx-decode` and `uplay-install-reverse` (research notes) **[src:
  YoobieRE org]**.
- **What works today:** ownership listing, ownership tokens, signed manifest +
  metadata + licences retrieval, slice-URL derivation (deterministic CDN prefix
  from `fileHashToPathChar`), zstd (and older zlib-framed) slice download,
  per-file reconstruction with SHA-1/SHA-256 verification, resume via
  signed-URL refresh — all validated live against owned titles (Assassin's
  Creed Origins product 3539; Splinter Cell product 109: full 5,320-file /
  ~2.5 GB tree reconstructed in ~6 min, byte-identical re-verify) **[src:
  ubi-cli-mvp DEMUX_JOURNAL.md]**.
- Maintenance status: UplayKit quiet; the live edge is the two 2026 CLI
  projects (ubi-cli-mvp MIT, Optima GPL-3.0). Demux drifts (client-version
  bumps, `Ubi-Challenge` header on login reported by unifideck **[src:
  ubi-cli-mvp research.md §9]**); expect to re-validate the handshake every few
  months.

### 2.3 Community downloaders and manifest datasets

- **UplayDB/UplayApps** (C#, **MIT**): `Downloader` (CLI: downloads owned
  games/CDN chunks/manifests, verify), `Dumperv2` (ownership/manifest dumps),
  `DenuvoTicket` tool, `UbiServices` info gatherer **[src:
  github.com/UplayDB/UplayApps]**. Actively structured, licence-clean for
  behaviour reference.
- **UplayDB/UplayManifests**: community dataset — `gamelist.json`,
  `manifestlist.json`, `productservice.json` (SpaceId/AppId), `productconfig.json`,
  raw `.manifest` fixtures + `.txt` renderings (256 commits) **[src]**.
- **YoobieRE/ubisoft-manifest-downloader**: WIP CLI for downloading *old
  versions* of Connect games **[src]** — the only "versions" lead for Ubisoft;
  treat as experimental (see §5: parity plan records versions as unsupported).
- **xXJSONDeruloXx/ubi-cli-mvp** (TypeScript, **MIT**, Mar–Jul 2026, 33+ commits):
  the most complete public Demux download pipeline; honest about the frontier
  (per-title reconstruction variance, no launcher-grade update engine) **[src]**.
- **Starkka15/Optima** (Rust, **GPL-3.0**, active 2026): ownership-backed client
  — login, list, CDN download with progress/resume, writes Connect's own install
  records (`uplay_install.manifest`, `uplay_install.state`), launches through the
  official client **[src: github.com/Starkka15/Optima]**. The project that got
  furthest *abandoned clientless launch* (replacing Ubisoft loader DLLs with
  shims) and moved launch back into official Connect.

### 2.4 The public web (UbiServices) API

Documented largely through the Galaxy plugin and Lutris **[src: ubi-cli-mvp
research.md §§2–3,5]**:

- Sessions: `POST /v3/profiles/sessions` (`Basic base64(email:password)`,
  `Ubi-AppId`, `Ubi-RequestedPlatformType: uplay`, `{rememberMe}`); 2FA via
  `ubi_2fa_v1` + `Ubi-2faCode`; refresh `PUT …/sessions` (`Ubi_v1 t=`), fallback
  `rm_v1 t=` (Lutris behaviour) **[src]**.
- Identity: `GET /v3/users/{userId}`; library fallback: `POST
  /v1/profiles/me/uplay/graphql` → `viewer.games.nodes` (Lutris query includes
  `spaceId`, name, image URLs) **[src]**.
- App-id/genome pairs in the wild: Lutris `82b650c0-…/42d07c95-…`;
  `ubi-cli` `314d4fef-…/85c31714-…`; demux-node `f68a4bb5-…`; Galaxy plugin uses
  the Club pair (`CLUB_APPID b8fde481-…`, `CLUB_GENOME_ID fbd6791c-…`) for web
  login **[src: FriendsOfGalaxy consts.py, ubi-cli-mvp research.md §3]**.
- Machine-readable surface: `Phoenix-Assassins/ac4-docs` publishes an OpenAPI
  YAML for `public-ubiservices.ubi.com` **[src]**; `UplayDB/UbiServices` gathers
  endpoint notes (MIT) **[src]**.
- Friends: `api-ubiservices.ubi.com/v2/profiles/me/friends` (used by the Galaxy
  plugin) **[src: launcher-parity.md §1]**; playtime/stats via Club titles +
  stats endpoints (`stats.py`, `test_import_game_times.py` in the Galaxy
  plugin) **[src]**.
- Maintenance: the Galaxy integration needed app-id updates (PR #34), 403 fixes
  after identifier rotation (v2.0.1), and repeated 2026 ownership/login fixes
  up to v2.0.12 (entitlement endpoint now authoritative; Club GraphQL kept as
  fallback) **[src: melcom-creations CHANGELOG.md]**. Expect the same treadmill.

### 2.5 Why native downloads don't remove the client

1. **Registration:** a reconstructed tree is *not* an install. Live-tested:
   Connect still offered "Download" until `connect-seed` copied payloads into
   Connect-created staging + official finalisation wrote the install
   manifest/state **[src: ubi-cli-mvp DEMUX_JOURNAL.md 2026-07-12]**. Platypus
   must not fabricate those records.
2. **DRM:** manifests carry `licenses` (e.g. Origins: 1 licence,
   `denuvo_eula`) **[src: ubi-cli-mvp DEMUX_JOURNAL.md]**; titles ship Denuvo
   and/or Ubisoft's own VMProtect-based protection (TechPowerUp/Ars on AC:
   Origins **[src]**). Checks run inside the official client/game handshake;
   Optima keeps launch in official Connect for exactly this reason **[src]**.
3. **Offline** exists in Connect (Ubisoft help 000063188) **[src: pcgamingwiki /
   ubisoft.com]** — the client story covers it.
4. Result: a native downloader saves bandwidth plumbing but keeps 100% of the
   client dependency at install *and* launch, while adding Demux drift, ToS
   surface (§2.6), and a GPL-licence wall around the best implementation.

### 2.6 Risk: ToS, bans, DRM (summary, not legal advice)

- Ubisoft Terms of Use (updated **2026-01-26**) provide temporary locks and
  permanent bans for violations **[src: ubisoft.com/legal/terms; help articles
  000066757, 000066783, 000109437]**. Automation/unauthorised-access clauses
  live in that document and the EULA (2023) — read the current text before
  building, and re-read on client updates.
- Observed practice: community downloaders and Galaxy integrations have
  operated for years; Lutris performs web login with its *own* app id and
  refreshes sessions like a client **[src]**. That is precedent, not permission:
  credential-based automation + unofficial app ids is the highest-risk
  combination (ban-able account action on the user's *real* game library).
- The phase-A design minimises this surface: the user logs into the **official
  client**, Platypus never handles the password, never sends an app id, never
  downloads a byte outside Connect. Any phase-B web-API work re-opens the
  question — hence Q-U1.

## 3. Path 3 — hybrid (web API metadata + client install/launch)

What each side would carry, mapped to protocol capabilities:

| Capability | Phase A (client + cache) | Phase B (web API, needs Q-U1) |
|---|---|---|
| `library` | local `configurations` + `ownership/{userId}` + registry installs | entitlements endpoint as authority, Club GraphQL fallback (Galaxy v2.0.9 pattern) |
| `install`, `launch` | `uplay://` + prefix, always | unchanged |
| `auth` | "signed into Connect in prefix P" (file evidence) | ticket login + refresh (`Ubi_v1`/`rm_v1`) |
| `achievements` (user progress) | catalogue from cache config | Club challenges endpoints |
| `playtime` | local session tracker only | Club titles/stats import |
| `social` (friends/presence) | out | friends API + presence |
| `cloud-saves` | client does it on launch/exit; plugin no-op | unchanged (client) |
| `versions`, `verify` | not offered (client-only) | unchanged |

Web-login UX constraint (parity plan §4, still true): the Galaxy flow ends on a
page whose session data lives in `localStorage` (`PRODloginData`,
`PRODrememberMe`), extracted via injected JS after redirect to
`connect.ubisoft.com/change_domain/` **[src: FriendsOfGalaxy consts.py
AUTH_JS]**. Platypus has no embedded web view; `auth.begin` ("open URL, paste
fields") cannot reach `localStorage`. Options remain: (1) paste-from-devtools
(poor), (2) system-web-engine helper (new code, new risk), (3) **client-side
login only** (recommended — the unifideck/Optima/ubi-cli-mvp consensus: first
auth is interactive official-client UI, and CLI-ticket → Connect-secure-storage
handoff was investigated and found unsupported **[src: ubi-cli-mvp
research.md]**).

## 4. Clean-room path

Constraints that keep this on the right side of interoperability practice
(high level, **not legal advice** — settle Q-LIC and take real advice before
porting anything):

- **Document from behaviour, not from code.** The 2026 projects already model
  this: ubi-cli-mvp's fresh-auth trace recorded only file paths/sizes/hashes,
  process names, DNS names — no keystrokes, memory, TLS payloads, or headers —
  and deleted raw observations **[src: research.md]**. Imitate that method for
  L-U1/L-U2.
- **Reuse matrix (licence-first):** MIT — Playnite + PlayniteExtensions
  (cache schema, `uplay://` controller pattern, DLC filters), UplayKit,
  UplayApps, UplayManifests data, ubi-cli-mvp, galaxy-integration-uplay,
  unifideck spec (docs, check its licence) — portable **with attribution**,
  subject to Q-LIC. GPL-3.0 — legendary/gogdl/nile/Heroic (external processes
  only, as today), **Optima (cannot port; behaviour reference only)**. No
  licence — YoobieRE Node/TS clients (**reference only, no reuse**).
- **EU:** Directive 2009/24/EC arts. 5–6 (observe/study/test; decompilation
  strictly for interoperability by a lawful user, limited to necessary parts,
  not reused to make an infringing copy) **[src: eur-lex.europa.eu/eli/dir/2009/24]**.
- **US:** 17 U.S.C. §1201(f) reverse-engineering exception for interoperability
  (lawfully obtained copy, circumvention only as necessary, sharing limited to
  enabling interoperability) **[src: law.cornell.edu/uscode/text/17/1201]**;
  case law (Sega v. Accolade; Sony v. Connectix) and the EFF reverse-engineering
  FAQ describe the fair-use contour **[src]**. Ubisoft's contractual terms may
  purport to restrict this; the interplay is exactly what counsel is for.
- Never copy: app ids/secrets as credentials, installer binaries (link/pin the
  vendor URL + SHA-256, as Lutris and ubi-cli-mvp do), any DRM circumvention.

## 5. Capability mapping for the first plugin (phase A)

Manifest sketch (protocol 1.x, cf. `docs/plugins/protocol.md`):

```json
{
  "id": "ubisoft-connect",
  "protocol": { "versions": ["1.0"], "transport": "jsonrpc-stdio" },
  "capabilities": ["library", "install", "launch", "auth", "discard"],
  "requires": [{ "bin": "wine", "hint": "…" }],
  "install_dir": "ubisoft"
}
```

- `library.list`: parse `<prefix>/…/Ubisoft Game Launcher/cache/configuration/configurations`
  (protobuf-ish records + embedded YAML product blob; Playnite `Uplay.cs`
  `GetLocalProductCache` **[src: PlayniteExtensions UplayLibrary]**; unifideck
  `parser.py` documents the `0x0A`-framed record scan **[src]**) ∩
  `cache/ownership/{userId}` for owned ids; installed state from Wine
  `system.reg` `HKLM\SOFTWARE\(WOW6432Node\)\Ubisoft\Launcher\Installs\<id>\InstallDir`
  **[src: Playnite `UplayLibrary.cs`; Galaxy v2.0.1 64-bit registry fix]**;
  DLC filtering (`root.addons`, `is_ulc`, `third_party_platform`,
  `start_game == null`) **[src: Playnite]**; art from config records/store CDN.
  Fail with "start the client at least once" when the cache is absent **[src:
  Playnite]**.
- `game.install` / `game.update` (`update` = re-run install via client):
  `uplay://install/<id>` in the managed prefix + watch install registry;
  **indeterminate progress** (`download_phase = "manual"` pattern **[src:
  unifideck §6]**). `game.uninstall`: `uplay://uninstall/<id>` + poll.
- `game.launchSpec`: the **`wine-client` delegate** from parity-plan §2.6
  (`exe` + `args: [uplay://launch/<id>/0]` + core-owned per-plugin prefix +
  `find: {cmdline}` matching Windows exe names — needs L-U1 verification).
- `auth.*`: client-side only — status = prefix credential evidence
  (`ConnectSecureStorage.dat` + `user.dat` shape/mtime, cf. unifideck §12:
  freshness by **mtime, never size** **[src]**); begin/logout = open/close the
  client (logout deletes or signs out the auth prefix).
- Explicitly **not** declared in phase A: `versions` (no public tracks;
  YoobieRE's old-version downloader is WIP **[src]**), `verify`/`move`
  (client-only), `cloud-saves` (client syncs on launch/exit; plugin declares
  nothing), `achievements`/`playtime`/`social` (phase B), `redeem`, `family`.
- **Prefix model** (adopt unifideck's, validated in production): one shared
  per-plugin client prefix (Platypus already plans `prefixes/p.<plugin>.client`
  **[src: parity plan §2.6]**); per-game prefixes are out of scope for phase A —
  Heroic/Ubisoft notes say all games share one prefix. Credential propagation
  across prefixes is a solved-but-subtle problem (MachineGuid identity, token
  rotation as a baton — unifideck §§7/12 **[src]**); a single shared prefix
  avoids it entirely. **Do not clone authenticated prefixes as templates**
  (ubi-cli-mvp proved clones invalidate the source session **[src]**).
- **New schema-drift watch:** a 2026 community report of an encrypted `.vdat`
  library cache (CrowIsTaken/PlayniteExtensions PR #31) **[src:
  codeberg.org]** — unverified here; if Ubisoft migrates the cache format, the
  local reader breaks and phase B's entitlement endpoint becomes the fallback.
  Track during L-U1.

## 6. Recommendation and proposed issues

**Recommendation:** build phase A exactly as the parity plan describes, with
three adjustments from this research: (a) indeterminate install progress (no
fake %), (b) single shared client prefix (no per-game credential fan-out),
(c) pin + SHA-256-verify the Connect installer and validate with a
`setup --check`-style offline inspection. Defer phase B to after Q-U1; do not
build a native downloader (the client remains mandatory at both ends, and the
best implementation is GPL).

Phases:

- **Phase A1 (core):** `wine-client` launch delegate + `client.setup` job shape
  (parity plan §2.6). Delivers: any client-based plugin can launch.
- **Phase A2 (plugin):** local-cache `library.list` + installed detection +
  `uplay://` install/uninstall/launch. Delivers: owned library, installs,
  launches. Needs L-U1 then L-U2 with a real account.
- **Phase A3 (hardening):** installer pinning/verification, cache-path
  fallbacks, login-webview flags, offline launch. Delivers: survives client
  updates.
- **Phase B (gated on Q-U1):** web-API `social` + `playtime` + user
  `achievements` via the Galaxy-plugin endpoint set. Delivers: friends modal,
  accurate playtime.

Proposed issues to file (titles + scope):

1. **Core: `wine-client` launch delegate + `client.setup` job** — implement
   parity-plan §2.6 (`exe`/`args`/`prefix`/`find`/`startTimeout`); `command`
   runs under the runner like a normal Wine launch; prefix keyed per plugin.
2. **Ubisoft plugin: local-cache library reader** — parse `configurations` +
   `ownership/{userId}` + `system.reg` installs in the managed prefix; DLC
   filters; "start the client once" empty state; needs Q-LIC before porting
   MIT schema code.
3. **Ubisoft plugin: managed prefix + Connect install** — Lutris recipe
   (`/S`, `d3dcompiler_43`, `corefonts`, overlay off), pinned installer URL +
   SHA-256, `setup --check` offline validation.
4. **Ubisoft plugin: client-driven install/launch/uninstall** — `uplay://`
   flows with indeterminate progress + cancel; `find`-based game-process
   tracking (L-U1); small-game L-U2.
5. **Live checks L-U1/L-U2 tracking** — login webview under our Wine/Proton,
   cache/ownership locations, `uplay://` reachability via `wine start`,
   cmdline `find` matching; then one small install.
6. **Ubisoft phase B (blocked: Q-U1): friends/playtime/achievements via web
   API** — Galaxy-plugin endpoint set, ticket auth, localStorage-login UX
   problem stated; do not start before the product decision.
7. **Docs: Ubisoft ToS/licence pre-check (blocks phase B, informs phase A)** —
   close Q-LIC (repo has no LICENSE file), record the interoperability method
   rules from §4, re-check ToS text current at build time.
