# Research: Steam Cloud saves, friends and online status in Hydra

Status: research only, written 2026-10-08 on `research/steam-social`. Nothing here
was tested against the real Steam network. Everything below comes from protocol
definitions, other projects' source, public docs, and the official Steam client's
**own local log files** (read-only, no account call). The plan that builds on this is
[docs/plans/steam-cloud-social.md](../plans/steam-cloud-social.md).

Legend: **[src]** = read in a primary source (linked). **[inferred]** = reasoned from
sources, not confirmed. **[LIVE]** = needs a live test with the user's account.

Sources used most often (all fetched during this research):

- SteamDatabase Protobufs, the same definitions SteamKit2 is generated from:
  [`steammessages_cloud.steamclient.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_cloud.steamclient.proto),
  [`steammessages_clientserver_friends.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_clientserver_friends.proto),
  [`steammessages_clientserver_login.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_clientserver_login.proto),
  [`steammessages_clientserver.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_clientserver.proto),
  [`steammessages_clientserver_2.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_clientserver_2.proto),
  [`steammessages_client_objects.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/steammessages_client_objects.proto),
  [`enums_clientserver.proto`](https://github.com/SteamDatabase/Protobufs/blob/master/steam/enums_clientserver.proto).
- SteamKit2 enums: [`SteamLanguage.cs`](https://github.com/SteamRE/SteamKit/blob/master/SteamKit2/SteamKit2/Base/Generated/SteamLanguage.cs).
- node-steam-user: [README](https://github.com/DoctorMcKay/node-steam-user/blob/master/README.md),
  [`components/09-logon.js`](https://github.com/DoctorMcKay/node-steam-user/blob/master/components/09-logon.js),
  [`friends.js`](https://github.com/DoctorMcKay/node-steam-user/blob/master/components/friends.js),
  [`apps.js`](https://github.com/DoctorMcKay/node-steam-user/blob/master/components/apps.js).
- **Madeira** ([willfaust/Madeira PR #130](https://github.com/willfaust/Madeira/pull/130), merged 2026-10-03,
  files `docs/STEAM_CLOUD.md`, `app/Madeira/SteamCloud.swift`, `app/Madeira/SteamOwnedLibrary.swift`):
  the only open-source third-party app found that syncs Steam Cloud **without the Steam client**, over its own CM
  session, with the exact methods this document recommends. Its code is the best wire-level reference.
- Valve's [Steam Cloud docs](https://partner.steamgames.com/doc/features/cloud) and the
  [Steamworks API overview](https://partner.steamgames.com/doc/sdk/api).
- Public PICS mirror used only to read **app metadata** (no account involved):
  `https://api.steamcmd.net/v1/info/<appid>`.
- Heroic / legendary / gogdl source: [`launcher.ts`](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/blob/main/src/backend/launcher.ts),
  [`legendary/games.ts`](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/blob/main/src/backend/storeManagers/legendary/games.ts),
  [`gog/games.ts`](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/blob/main/src/backend/storeManagers/gog/games.ts),
  [legendary `core.py`](https://github.com/derrod/legendary/blob/master/legendary/core.py),
  [gogdl `saves.py`](https://github.com/Heroic-Games-Launcher/heroic-gogdl/blob/main/gogdl/saves.py).

---

## 0. TL;DR

1. **Cloud sync is doable over the CM we already speak.** The unified `Cloud` service (151 authed calls,
   `Cloud.GetAppFileChangelist#1`, `ClientFileDownload#1`, `BeginAppUploadBatch#1`, `ClientBeginFileUpload#1`,
   `ClientCommitFileUpload#1`, `CompleteAppUploadBatchBlocking#1`) is what Madeira uses; file bytes move over
   plain HTTP(S) to hosts Steam names in the reply. No new transport.
2. **The "Steam Cloud error" the user got was a real save conflict, not interference by Hydra.** The official
   client's `cloud_log.txt` shows it attached to the game process (through the Steamworks pipe), ran AutoCloud at exit
   with no launch record and no baseline, and found local saves that differ from cloud saves dated Dec 2024.
   Details in section 2.
3. **The official client and a Hydra CM session coexist** as long as their `obfuscated_private_ip` "logon IDs"
   differ. Hydra uses a **fixed** `LoginID = 2718281` and a **random machine ID per logon**. Both need fixing before
   Hydra holds a long-lived session next to short-lived ones (section 3).
4. **Friends and presence need a CM session; the Web API is not a substitute.** The friends list arrives
   unprompted after logon (`CMsgClientFriendsList`, EMsg 767); persona states arrive only once the session sets a
   persona state (`CMsgClientChangeStatus`, 716). No keyless Web API lists friends.
5. **Architecture:** keep the Steam session in the Steam plugin (the token must stay there), add generic
   `cloud.*` and `social.*` namespaces to the plugin protocol, and let core own *when* (launch/exit hooks), the UI
   and the conflict choice. This generalises to GOG and Epic, whose tools only offer "newer timestamp wins".

---

## 1. Steam Cloud on the wire

### 1.1 Two storage models inside one backend

Steam Cloud stores, per `(account, appid)`, a flat set of files named by a **relative path string** plus metadata.
Two kinds of apps write there:

- **Auto-Cloud**: the developer lists save locations in app metadata (`ufs` in PICS appinfo). The *client* finds the
  files, uploads at exit and downloads at launch. The game does not know. [src: [Valve docs](https://partner.steamgames.com/doc/features/cloud):
  "Steam will automatically sync the groups of files when the application launches and exits."]
- **ISteamRemoteStorage API**: the game calls `FileWrite`/`FileRead` through `steam_api`, which talks to the running
  Steam client; files live in `userdata/<accountid>/<appid>/remote/` and the cloud path has **no `%Root%` prefix**.
  [src: Madeira maps a path with no placeholder to `userdata/<account>/<app>/remote`, `SteamCloud.swift` `location()`.]

A third-party launcher can sync **both** as plain files, because the cloud side is just a file set:

- Auto-Cloud: the local location is computed from `ufs` (section 1.4).
- Remote-storage API apps: there is no `steam_api` client to write into `userdata/.../remote`, so Hydra would have to
  **own that directory** (create `<hydra data>/steam-remote/<appid>/` and point the game at it). That requires the game to run
  against a Steam client, which Hydra's direct launch does not provide, so API-only games are **out of scope until
  something supplies the Steamworks IPC** — which we must not emulate. [inferred] Such games already need the real
  client (Hydra's "client required" mode), and the real client then does their cloud sync itself. Hydra can still
  *download* such files for backup. Open question for the user (plan, Q-A).

### 1.2 Service methods, field shapes and order

All are `ServiceMethodCallFromClient` (EMsg 151) with `target_job_name = "Cloud.<Method>#1"`, exactly like
the `Player.GetPlayerLinkDetails#1` / `Authentication.*` calls already in `internal/steamcm`. Field numbers are from
`steammessages_cloud.steamclient.proto` [src]; usage order is from Madeira's `SteamOwnedLibrary.swift` [src] and
marked **(M)** where only Madeira confirms it.

| Step | Method | Request fields (number) | Response fields |
|---|---|---|---|
| List | `Cloud.GetAppFileChangelist#1` | `appid`=1, `synced_change_number`=2 (0 = whole list) **(M)** | `current_change_number`=1, `files`=2 [`CCloud_AppFileInfo`], `is_only_delta`=3, `path_prefixes`=4 (strings), `machine_names`=5, `app_buildid_hwm`=6 |
| (file entry) | | | `file_name`=1, `sha_file`=2 (20 raw bytes, SHA-1), `time_stamp`=3 (unix s), `raw_file_size`=4, `persist_state`=5 (0 persisted, 1 forgotten, 2 deleted), `platforms_to_sync`=6, `path_prefix_index`=7 (into `path_prefixes`), `machine_name_index`=8, `reupload_requested`=9 |
| Download | `Cloud.ClientFileDownload#1` | `appid`=1, `filename`=2 (**prefix + name**, e.g. `%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot`), `realm`=3, `force_proxy`=4 | `file_size`=2 (wire), `raw_file_size`=3, `sha_file`=4, `time_stamp`=5, `is_explicit_delete`=6, `url_host`=7, `url_path`=8, `use_https`=9, `request_headers`=10 [{name=1,value=2}], `encrypted`=11 |
| Upload begin | `Cloud.BeginAppUploadBatch#1` | `appid`=1, `machine_name`=2, `files_to_upload`=3 (repeated), `files_to_delete`=4, `client_id`=5 (u64, stable per install), `app_build_id`=6 | `batch_id`=1, `app_change_number`=4 |
| Per file | `Cloud.ClientBeginFileUpload#1` | `appid`=1, `file_size`=2, `raw_file_size`=3, `file_sha`=4 (bytes), `time_stamp`=5, `filename`=6, `platforms_to_sync`=7 (default 0xFFFFFFFF), `cell_id`=9, `can_encrypt`=10, `upload_batch_id`=13 | `encrypt_file`=1, `block_requests`=2 [`ClientCloudFileUploadBlockDetails`] |
| (block) | | | `url_host`=1, `url_path`=2, `use_https`=3, `http_method`=4, `request_headers`=5, `block_offset`=6, `block_length`=7, `explicit_body_data`=8, `may_parallelize`=9 |
| (HTTP) | PUT/POST each block's bytes to `https://url_host/url_path` with the given headers | | 2xx = ok |
| Per file | `Cloud.ClientCommitFileUpload#1` | `transfer_succeeded`=1, `appid`=2, `file_sha`=3, `filename`=4 | `file_committed`=1 |
| Upload end | `Cloud.CompleteAppUploadBatchBlocking#1` (or the notification variant `CompleteAppUploadBatch`) | `appid`=1, `batch_id`=2, `batch_eresult`=3 (1 OK, 2 Fail) | empty |
| Delete | `Cloud.ClientDeleteFile#1` | `appid`=1, `filename`=2, `is_explicit_delete`=3, `upload_batch_id`=4 | empty |
| Quota | `Cloud.ClientGetAppQuotaUsage#1` | `appid`=1 | `existing_files`, `existing_bytes`, `max_num_files`, `max_num_bytes` |
| Enumerate (old) | `Cloud.EnumerateUserFiles#1` | `appid`=1, `extended_details`=2, `count`=3, `start_index`=4 | `files` [`CCloud_UserFile`], `total_files`=2 |
| Single file | `Cloud.GetSingleFileInfo#1` | `app_id`=1, `file_name`=2 | `sha_file`, `time_stamp`, `raw_file_size`, `is_explicit_delete` |

Notes:

- **`GetAppFileChangelist` is the modern list call**; `EnumerateUserFiles` is the older web-UGC style list
  (`CCloud_UserFile`: `ugcid`, `url`, `file_sha` as a *string*, `flags`). Prefer the changelist: it returns raw SHA-1
  bytes, timestamps, `persist_state`, and **path prefixes split out** (`path_prefix_index` into `path_prefixes`;
  the full cloud path is `prefix + file_name`). `synced_change_number` > 0 requests a delta (`is_only_delta`). [src]
- `BeginAppUploadBatch`/`ClientBeginFileUpload`/`Commit...` is a **batch**: begin names the whole set, each file is
  begun/uploaded/committed, then the batch is completed with `batch_eresult`. A failure completes the batch with
  `batch_eresult = 2` (Madeira does `complete(false)` in its catch). [src: M]
- **Download decoding** (M): if `file_size != raw_file_size` or the body starts with `PK\x03\x04`, the body is a
  single-entry ZIP (stored or deflate); unpack it, then **verify SHA-1 against the listing** before writing.
  `encrypted = true` files need `Cloud.GetClientEncryptionKey#1` (`key`, `crc`); Madeira refuses them. [src]
- **Upload blocks** (M): `http_method == 3` is sent as POST, anything else as PUT; headers from the reply are copied verbatim; a block with
  `explicit_body_data` carries its own body; otherwise body = file bytes `[block_offset, block_offset + block_length)`.
  Madeira uploads blocks serially; `may_parallelize` is the hint to do otherwise. The numeric `EHTTPMethod` values were
  not found in the protobuf repo's `enums.proto`; verify against SteamKit2 `EHTTPMethod` when implementing. [inferred]
- `appid` is `uint32`; file sizes are `uint32` (max 4 GiB per file, Valve's own soft limits are 100 MB per write call and 256 MB per
  file [src: Valve docs]).
- **Quotas** come from `ufs.quota` and `ufs.maxnumfiles` in appinfo (SUPERHOT: quota 524288 B, 5 files).

### 1.3 Change numbers and the conflict model

What the official client keeps, per app, in `~/.local/share/Steam/userdata/<accountid>/<appid>/remotecache.vdf`
(read from this machine for SUPERHOT, names anonymised) [src: local file]:

```
"322500" {
  "ChangeNumber" "0"          // last global change number this machine synced to
  "ostype"       "-203"       // EOSType.LinuxUnknown: the platform the paths were last computed for
  "SUPERHOT_Team/SUPERHOT/super.hot" {
     "root" "12"              // numeric root id (12 = WinAppDataLocalLow)
     "size" "3705"  "localtime" "<unix>"  "time" "<unix>"  "remotetime" "1734286632"
     "sha" "<sha1 hex>"  "syncstate" "4"  "persiststate" "0"  "platformstosync2" "-1" } }
```

and in `logs/cloud_log.txt`: `Attempting to sync from local change number '0' to global change number '1' (full)`.

So the official model is **three-way**:

- the cloud has a global **change number** per app (`current_change_number`; `BeginAppUploadBatch` returns
  `app_change_number`; every successful upload batch bumps it; online clients are told with
  `CloudClient.NotifyAppStateChange` carrying `appid` + `app_change_number` [src: proto]);
- each client remembers the change number it last synced to, plus per-file SHA/time it last saw (`remotecache.vdf`);
- a file is **changed locally** if its SHA differs from the remembered one, **changed remotely** if the cloud SHA
  differs from the remembered one. Changed on one side only: copy it across. Changed on both, or **no remembered
  state**: **conflict**. The log wording is `has local changes but mismatch - creating a conflict. Local/Remote SHA: ...`
  followed by `Upload failed due to conflicts in build list`. [src: local `cloud_log.txt`]
- timestamps are informational (the dialog shows both dates); SHA-1 + the baseline decide.

A third-party client cannot share `remotecache.vdf` and must not edit it. Hydra needs its **own baseline**
(`path -> last identical SHA`), exactly what Madeira does (`steam-cloud-sync.json`): changed on one side = copy; changed on
both or no record = **leave alone and ask**. Madeira also never deletes in the cloud and backs up any local file a
download replaces, and notes "Steam keeps no copy of a cloud file an upload replaces" (so an overwrite must be a
conscious choice). [src: M `docs/STEAM_CLOUD.md`]

**Session signalling used by the official client** (in the same proto; Madeira does *not* use them, so the exact semantics are
**[LIVE]**):

- `Cloud.SignalAppLaunchIntent#1` (`appid`, `client_id`, `machine_name`, `ignore_pending_operations`, `os_type`,
  `device_type`) returns `pending_remote_operations` [`CCloud_PendingRemoteOperation`: `operation`, `machine_name`,
  `client_id`, `time_last_updated`, `os_type`, `device_type`]. `ECloudPendingRemoteOperation` = `None 0`,
  `AppSessionActive 1`, `UploadInProgress 2`, `UploadPending 3`, `AppSessionSuspended 4`. This is how Steam warns
  "this game is still running / uploading on another device" before launch. [src]
- `Cloud.SuspendAppSession#1` / `ResumeAppSession#1` (`appid`, `client_id`, `machine_name`, `cloud_sync_completed`)
  and the notification `Cloud.SignalAppExitSyncDone#1` (`appid`, `client_id`, `uploads_completed`, `uploads_required`) mark the
  exit side. [src]
- `Cloud.ClientConflictResolution#1` (notification: `appid`, `chose_local_files`) reports the user's conflict
  choice to Steam. [src]

Hydra should send these (it is a "device" like any other) so that the user's Steam client and other devices see accurate
state, but this is optional for a first version and **[LIVE]**-verified before enabling.

### 1.4 Auto-Cloud (`ufs`) to real paths

PICS appinfo, section `ufs` (already fetched by `internal/steamcm.ProductInfo`; the plugin currently ignores it).
Real examples from the public mirror [src]:

```
SUPERHOT (322500)       "ufs": { "quota": 524288, "maxnumfiles": 5,
  "savefiles":     { "0": {root: WinAppDataLocalLow, path: "SUPERHOT_Team/SUPERHOT", pattern: "*.hot"},
                     "1": {root: WinAppDataLocalLow, path: "SUPERHOT_Team/SUPERHOT", pattern: "*.cfg"} },
  "rootoverrides": { "0": {os: MacOS, oscompare: "=", root: WinAppDataLocalLow, useinstead: MacAppSupport,
                           pathtransforms: {"0": {find: "SUPERHOT_Team/SUPERHOT", replace: "unity.SUPERHOT_Team.SUPERHOT"}}},
                     "1": {os: Linux,  oscompare: "=", root: WinAppDataLocalLow, useinstead: LinuxHome,
                           pathtransforms: {"0": {find: "SUPERHOT_Team/SUPERHOT", replace: ".config/unity3d/SUPERHOT_Team/SUPERHOT"}}} } }
Hades (1145360)         savefiles {root: WinMyDocuments, path: "Saved Games/Hades", pattern: "Profile*.sav"}; MacOS override only
Stardew Valley (413150) savefiles {root: WinAppDataRoaming, path: "StardewValley/Saves", pattern: "*", recursive: "1"};
                        Linux override: LinuxHome + transform "StardewValley/Saves" -> ".config/StardewValley/Saves"
Terraria (105600)       "ufs": { quota, maxnumfiles } only -> uses the Cloud API (remote storage) -> no Auto-Cloud rules
```

Fields per `savefiles` entry: `root`, `path` (subdirectory, may contain `{64BitSteamID}` / `{Steam3AccountID}`), `pattern`
(glob), `recursive` (`"1"`), `platforms` (a sub-block of OS names; **absent = all platforms**). Per `rootoverrides` entry: `root`,
`os`, `oscompare`, `useinstead`, `addpath`, `pathtransforms` ({`find`,`replace`}). [src: Valve docs; Madeira parses `root`/`os`/`useinstead`/`addpath`;
the `pathtransforms` and `oscompare` fields are visible in the mirror data above, and not handled by Madeira]. `quota`/`maxnumfiles`
also appear. Extra `ufs` keys exist (`hidecloudui`, `ignoreexternalfiles`) [inferred from experience; check against more appinfo samples].

**Cloud path = `%Root%` + relative path.** The file in the cloud list for SUPERHOT is named
`%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot` (Madeira's `split()` takes `%Root%rest`; the official client's own
`remotecache.vdf` stores the key `SUPERHOT_Team/SUPERHOT/super.hot` with `"root" "12"`). The *same cloud name* is used on every
OS: the **platform decides where it lives locally**, through `rootoverrides`.

Root ids and names (numeric `root` in `remotecache.vdf`; names from
[SteamCloudFileManager's ROOT_PATH_MAPPING.md](https://github.com/Fldicoahkiin/SteamCloudFileManager/blob/master/ROOT_PATH_MAPPING.md),
a Rust tool that talks to the real client; consistent with `root=12` for `WinAppDataLocalLow` in the local file above):

| Name | id | Windows/Proton | Native Linux |
|---|---|---|---|
| (none, remote storage) | 0 | `userdata/<acct>/<app>/remote/` | same |
| `GameInstall` | 1 | game's install dir | game's install dir |
| `WinMyDocuments` | 2 | `…/Users/<u>/Documents` | (override) |
| `WinAppDataLocal` | 3 | `…/AppData/Local` | (override) |
| `WinAppDataRoaming` | 4 | `…/AppData/Roaming` | (override) |
| `WinSavedGames` | 9 | `…/Saved Games` | (override) |
| `SteamCloudDocuments` | 11 | `…/Documents/Steam Cloud/<user>/<game>` | `~/.SteamCloud/<user>/<game>` |
| `WinAppDataLocalLow` | 12 | `…/AppData/LocalLow` | (override) |
| `LinuxHome` / `LinuxXdgDataHome` / `LinuxXdgConfigHome` | 14–16 | – | `~`, `$XDG_DATA_HOME`, `$XDG_CONFIG_HOME` |
| `MacHome`/`MacAppSupport`/`MacDocuments` | 6–8 | – | – |

**Mapping algorithm** (the official client's observed behaviour in `cloud_log.txt` + Madeira's `location()`):

1. Take each cloud path, split `%Root%` / rest.
2. Decide the **client OS** the game runs as: `linux` for a native build, `windows` for Proton. Hydra knows this
   (`needsWine`, `native` in the launch spec, `internal/runner`).
3. If the OS is `linux` (or macOS) and a `rootoverrides` entry with matching `root` + `os` exists: new root =
   `useinstead`, then apply `addpath` (prefix) and `pathtransforms` (string find/replace on the relative part).
   The official client's log for SUPERHOT confirms the result for native Linux:
   `Evaluating rule 0 with root="WinAppDataLocalLow" path="SUPERHOT_Team/SUPERHOT" pattern="*.hot"` ->
   `Found 2 files that match ~/.config/unity3d/SUPERHOT_Team/SUPERHOT/*.hot`. [src: local `cloud_log.txt`]
4. Otherwise for `windows` (Proton): the root maps into the **Wine prefix** user profile; rule `rootoverrides` for Windows
   (rare, Madeira handles `os=Windows`) applies the same way.
5. Replace `{64BitSteamID}` / `{Steam3AccountID}` (low 32 bits) in the path. Match case-insensitively on Proton (Wine is
   case-insensitive; Madeira resolves each path component case-insensitively). Reject any component `..`, `.`, containing `:`,
   absolute paths, or > 32 components. [src: M]

**Where the Wine profile is in Hydra**, from `internal/runner/launcher.go` (read in this repo): the prefix is
`~/.local/share/hydra/prefixes/<game.ID>` (or `game.Settings.WinePrefix`), exported as both `WINEPREFIX` and
`STEAM_COMPAT_DATA_PATH`. With Proton, `STEAM_COMPAT_DATA_PATH` is the *compat data dir*; Proton puts the real Wine prefix at
`<compatdata>/pfx` and names the user `steamuser`. So the profile for `WinAppDataLocalLow` is expected at
`<prefix>/pfx/drive_c/users/steamuser/AppData/LocalLow` **[LIVE]** — I could not inspect a Proton prefix here, and which
Windows profile name is live (`steamuser`, or `$USER` as Madeira's Wine does) must be discovered, not assumed. Madeira's `userFolder()`
heuristic (try `$USER`, then passwd name, then newest folder under `users/` ignoring `Public`) is a good fallback. Whether `Documents`,
`Saved Games` etc. are real directories or symlinks into the host home depends on the Proton build [unknown]: resolve with
`EvalSymlinks` and refuse targets outside the prefix or the user's home.

**Native vs Proton is also a *Steam-level* concern**: the official client logs `Detected app platform change since last sync - from
'linux' to 'windows'` followed by `Backup of AutoCloud files in preparation for path changes` / `Restore ... after path changes` (seen for app 7,
Steam client itself) [src: local `cloud_log.txt`]. Because both platforms share one cloud name, a game run native and later under Proton
should see its saves moved across. Hydra should treat a platform switch the same way: back up before changing roots.

### 1.5 Prior art: how others do cloud saves

| Project | Store | Mechanism | Conflict handling |
|---|---|---|---|
| **Heroic -> legendary** | Epic | `legendary sync-saves --skip-upload` before launch, `--skip-download` after exit, `--save-path`, `-y` | `SaveGameStatus` is **timestamp only** (`LOCAL_NEWER`/`REMOTE_NEWER`/`SAME_AGE`/`NO_SAVE`): newer wins; without `-y` it asks "Download cloud save?" / "Upload local save?". No third-party conflict state. [src: `legendary/core.py` `check_savegame_state`, `cli.py` `sync_saves`, Heroic `launcher.ts` lines 146-156 and 291-306] |
| **Heroic -> gogdl** | GOG | `gogdl save-sync <location> <id> --os <platform> --ts <last timestamp> --name <location name> [--skip-upload|--skip-download]`, once per cloud location GOG's API lists; Heroic persists the printed timestamp per `(game, location)` | `SyncClassifier` compares file mtimes with the stored sync timestamp; if both sides have newer files it logs "Files in conflict force downloading or uploading of files" and does nothing unless forced. Heroic has no conflict UI. [src: `gogdl/saves.py`, Heroic `gog/games.ts` `syncSaves`] |
| **Madeira** | Steam | Own CM session; `GetAppFileChangelist` -> compare SHA-1 -> download/upload batch; own baseline; never deletes; backs up before replacing; asks on conflicts | Per-file choice dialog; "upload and close" |
| **SteamCloudFileManager** (Rust), **CloudKit** (SteamKit), **steamCloudSaveDownloader** | Steam | Browse/download/upload files; the Rust tool drives the Steamworks SDK (needs the Steam client); CloudKit uses SteamKit directly | n/a (manual) |
| **Lutris** | – | no first-party Steam Cloud support found | – |

Pattern to copy: **pre-launch = pull only; post-exit = push only; mismatched/baseline-less = ask; log counts, not file names.**

Pattern to avoid: Heroic's "sync always on, conflict = silently skip" gives the user no way to see or fix a conflict, which is the
exact situation that produced the Steam notification here.

---

## 2. Why "Steam Cloud error" appeared (official client running)

### 2.1 What actually happened (evidence)

Read-only from `~/.local/share/Steam/logs/` on this machine (account id removed; **these are local log files, not an account request**):

`gameprocess_log.txt` / `console_log.txt`:

```
[2026-10-08 13:25:17] Game process added : AppID 322500 "", ProcID 222157, IP 0.0.0.0:0
[2026-10-08 13:25:17] AppID 322500 adding PID 222157 as a tracked process
[2026-10-08 13:25:27] Game process removed: AppID 322500 "", ProcID 222157
[2026-10-08 13:25:35] AppID 322500 no longer tracking PID 222157, exit code -1
```

`cloud_log.txt` (same day, trimmed):

```
13:25:17 [AppID 322500] Starting sync (init,)
13:25:17 [AppID 322500] AutoCloud checking local state for user <acct>
13:25:17 [AppID 322500] Init cache request complete
13:25:35 [AppID 322500] Starting sync (up,AC Exit,)
13:25:35 [AppID 322500] Running AutoCloud on exit. Looking for new and updated files
13:25:35 [AppID 322500]   Evaluating rule 0 with root="WinAppDataLocalLow" path="SUPERHOT_Team/SUPERHOT" pattern="*.hot"
13:25:35 [AppID 322500]   Found 2 files that match /home/<u>/.config/unity3d/SUPERHOT_Team/SUPERHOT/*.hot
13:25:35 [AppID 322500]   No launch record found. Skipping looking for files added/removed while the game was running
13:25:35 [AppID 322500] Attempting to sync from local change number '0' to global change number '1' (full)
13:25:35 [AppID 322500] SUPERHOT_Team/SUPERHOT/super.hot already marked as conflicting. Local/Remote SHA: 1476e694… / bcb7ad8c…
13:25:35 [AppID 322500] Sync conflict for file SUPERHOT_Team/SUPERHOT/super.hot
13:25:35 [AppID 322500] Upload failed due to conflicts in build list
```

The previous session (2026-10-07 20:36–20:39) shows the same sequence with
`SUPERHOT_Team/SUPERHOT/super.hot has local changes but mismatch - creating a conflict`. The official client's own cache for the app has
`ChangeNumber 0` and the cloud copy's `remotetime 1734286632` (= **2024-12-15**), while the local files are from October 2026.

### 2.2 Reading it

- The running Steam client **did** attach to the game. Hydra's `steam-standalone` plugin launches mode-2 games directly with
  `SteamAppId`/`SteamGameId` set (`plugins/steam-standalone/bin/hydra-steam:1457-1463`). The game's `SteamAPI_Init` finds the running
  client (on Linux through `~/.steam/steam.pipe` / `steam.pid`, which exist and are live on this machine) and the client then
  treats the process like one it started: tracks the PID, shows the user "playing", and runs the **Auto-Cloud sync for the app**.
  Valve's docs describe the attach only as "when you launch your app from Steam itself... automatically have the App ID"
  and the `steam_appid.txt` override [src: [steam_api docs](https://partner.steamgames.com/doc/sdk/api)]; the pipe behaviour
  and the "client syncs a game it didn't launch" part are **inferred from the logs above**, not from a Valve document.
- But the client did **not** do its normal launch sequence: no pre-launch download, and `No launch record found` (nothing
  recorded the set of files at game start), local change number `0`. At exit it compared the files the game wrote with the cloud and found
  *different SHA-1s* with no baseline, so it created a conflict and aborted the upload. A genuine data conflict: the account's cloud
  has older SUPERHOT saves (Dec 2024, from another machine or profile) and this machine has newer, different ones.
  Steam shows that as the "Steam Cloud error" toast; no conflict dialog appears because it does not own the launch.
- **Hydra did nothing wrong on the wire**: it has no cloud code today. The error is a consequence of "official client running + direct launch of
  a Steamworks game", combined with no one reconciling the two saves *before* launch.

### 2.3 What this means for Hydra

1. **Detect the official client** (read-only): `~/.steam/steam.pid` (and `/proc/<pid>/comm == steam`) plus `~/.steam/steam.pipe`.
   Also `pgrep -x steam`. No interference needed.
2. **Two writers on one save directory is the danger.** If Hydra pulls the cloud saves before launch and the attached official client then
   runs its own exit sync with *no baseline* (ChangeNumber 0, no record), it may still report a conflict. Because the client's exit sync starts
   within seconds of the process ending (log: process removed 13:25:27, exit sync 13:25:35), Hydra's post-exit push cannot be relied on to go first. Whether a
   Hydra pre-pull makes the client's exit sync benign (local SHA == cloud SHA at start; after play local != baseline) is **[LIVE]**.
3. **Safety risk for Proton games:** the attached client computes Windows roots in *its own* compat data
   (`steamapps/compatdata/<appid>/pfx`), not in Hydra's prefix (`~/.local/share/hydra/prefixes/<id>`). It will find **no files**
   there, and the log shows what it does with files it knew about and no longer finds: `File ... no longer matches any patterns (2).
   Removing from cloud` then `FileForget(...)` (here ignored because of pending cloud changes). Files the client has never seen are
   not touched, but a Proton-launched game with a known baseline could have cloud copies *forgotten* (`persist_state = 1`). [src: log; the
   consequence for a known-baseline Proton case is **[LIVE]**]. This is why "do nothing while the official client is running and the game links `steam_api`" is not a safe default
   for Proton games either.
4. **Defer vs warn.** Recommended policy (details in the plan): if the official client is running and the game would attach to it, Hydra
   **does not run its own cloud sync for that launch**, tells the user why, and offers "Launch through Steam" (the existing
   `delegate: store-client` mode, where Steam does the full sequence including its conflict dialog). Hydra runs its own sync only when
   the official client is **not** running or the game does not link Steamworks (`mode: none`). This needs the user's decision (plan, Q-B).

---

## 3. Session conflicts and what the logon flags do

### 3.1 What Hydra sends today

`internal/steamcm/wire.go: logonBody` [src: repo]: protocol 65580, `client_package_version` 1771, `client_os_type` = `LinuxUnknown`,
`obfuscated_private_ip` = `LoginID` (a **constant**, `2718281`) in both the legacy field 2 and `CMsgIPAddress` field 11, `should_remember_password=1`,
`machine_id` = a **fresh random** KeyValues blob (`BB3`/`FF2`/`3B3` = SHA-1 of random nonces) on *every* logon, `chat_mode=2`, `machine_name="Hydra"`,
`supports_rate_limit_response=1`, `access_token` = refresh token. Sessions today are short-lived (library listing, depot download jobs).

### 3.2 What Steam does with two logons of one account

From node-steam-user's README and source [src]:

> `logonID` — a 32-bit integer to identify this login. The official Steam client derives this from your machine's private IP (it's the
> `obfuscated_private_ip` field in `CMsgClientLogOn`). If you try to logon twice to the same account from the same public IP with the same
> `logonID`, the first session will be kicked with reason `SteamUser.EResult.LogonSessionReplaced`.

Forum reports back this up ("multiple sessions with differing logon IDs work fine"; "you can only have one logon instance *playing a game*
at the same time") [src: dev.doctormckay.com topics 4715, 1518; search results, not re-verified in source]. So:

- **Hydra vs the official client:** different logon IDs, they coexist. The official client's ID is a function of its real private IP, Hydra's is
  `2718281`; collision probability is negligible. The existing comment in `steamcm.go` ("a fixed value keeps a logon from kicking an
  existing session") is right for that purpose.
- **Hydra vs Hydra: BROKEN design once sessions overlap.** All Hydra sessions share `LoginID = 2718281`. Today they never overlap.
  A persistent social session plus a depot download or a cloud sync (each does `Dial`) would kick each other with
  `LogonSessionReplaced (34)` and make the long-lived one reconnect-loop. **Fix: a distinct `obfuscated_private_ip` per role** (e.g. social = base+1, cloud
  = base+2, depot job = random per process). **[LIVE]** that two Hydra logons with different IDs both stay up.
- **Machine ID:** random per logon means Steam sees a new device each time. With a refresh token this does not trigger a Guard challenge in
  our tests so far, but a persistent social session should send a **stable** `machine_id` (node-steam-user derives one from the account name;
  `09-logon.js: createMachineID`). Stable also reduces "new device" signals. **[LIVE]**.
- **`LoggedInElsewhere (6)`** is delivered as `CMsgClientLoggedOff.eresult` (EMsg 757) when the account is taken over by a session that cannot coexist, and
  `AlreadyLoggedInElsewhere (50)` / `PasswordRequiredToKickSession (49)` exist for the logon response [src: `SteamLanguage.cs`]. Treat 6 and 34 as "back off, don't fight":
  do not reconnect in a tight loop; reconnect with a delay and only if the user opted in. The existing `Dial` already maps EResults to messages.

### 3.3 "Playing on another computer"

- Steam allows **one session to be playing at a time per account**. `CMsgClientPlayingSessionState { playing_blocked (2), playing_app (3) }`
  (EMsg 9600) is sent right after logon if another session is playing, and whenever it changes; `CMsgClientKickPlayingSession { only_stop_game }` (9601)
  forcibly stops the other session. node-steam-user wraps this as `playingState` / `kickPlayingSession` / `gamesPlayed(apps, force)`. [src]
- A Hydra session that is **logged on but not reporting any game** does not block the official client from launching anything. A session that *does* report a game
  (`ClientGamesPlayed`) will make the official client's next launch say "Steam is currently being used to play X elsewhere" (and vice versa: our report is
  `playing_blocked` while the official client plays). Hydra must **never** use `kickPlayingSession`/`force`; if blocked, just skip reporting
  and show "Playing in Steam". [inferred from the message definitions; the exact dialog text is **[LIVE]**]

### 3.4 Logon flags that matter

| Field | Meaning | Hydra choice |
|---|---|---|
| `obfuscated_private_ip` (11, legacy 2) | logon ID; collision = replaced session | unique per role (see above) |
| `client_os_type` (7) | shown as platform in some UIs | keep `LinuxUnknown` (-203), matches `remotecache.vdf "ostype"` |
| `machine_name` (96) | name Steam shows in "devices / playing on" | "Hydra" (maybe `Hydra on <host>`) |
| `machine_id` (30) | device identity | stable per install |
| `chat_mode` (33) | 2 = new chat | keep 2 (needed to receive modern chat notifications; chat out of scope) |
| `ui_mode` (32) / `CMsgClientCurrentUIMode` (5597) | icon beside status in friends list | leave default; optional |
| `launcher_type` (31) | 0 default, `ELauncherType` has `ClientUI 5`, `Headless 6` ... | keep default; do not claim to be `ClientUI` |
| `client_instance_id` (100) / response (27) | per-session id returned by the CM | store only in memory |
| `should_remember_password` (8) | refresh-token logon | already 1 |

There is **no logon flag that makes Steam treat Hydra as a "web client" that never kicks anything** other than the distinct logon ID. A *web*
logon (`web_logon_nonce`, `client_os_type = 4294966596` (-700 Web), `ui_mode = 4`) is what steamcommunity's chat uses in node-steam-user
(`09-logon.js` lines 104-117) but needs a nonce from the website; not applicable.

---

## 4. Friends and presence

### 4.1 Messages (all via the same CM session; protobuf bodies)

| Purpose | EMsg | Body (fields) |
|---|---|---|
| Friends list push (after logon; also incremental updates) | `ClientFriendsList` 767 | `CMsgClientFriendsList { bincremental=1, friends=2 [{ulfriendid(fixed64)=1, efriendrelationship=2}], max_friend_count=3, active_friend_count=4, friends_limit_hit=5 }`. `efriendrelationship`: 0 None, 1 Blocked, 2 RequestRecipient (incoming request), **3 Friend**, 4 RequestInitiator, 5 Ignored, 6 IgnoredFriend. The list also contains **clan (group) SteamIDs**; filter by SteamID type (type 7 = Clan). |
| Persona data push | `ClientPersonaState` 766 | `CMsgClientPersonaState { status_flags=1, friends=2 [Friend] }`, `Friend { friendid=1, persona_state=2, game_played_app_id=3, game_server_ip=4, game_server_port=5, persona_state_flags=6, online_session_instances=7, persona_set_by_user=10, player_name=15, query_port=20, steamid_source=25, avatar_hash=31 (bytes), last_logoff=45, last_logon=46, last_seen_online=47, clan_rank=50, game_name=55, gameid=56 (fixed64), game_data_blob=60, clan_data=64, clan_tag=65, rich_presence=71 [{key,value}], broadcast_id=72, game_lobby_id=73, watching_*=74-77, is_community_banned=78, player_name_pending_review=79, avatar_pending_review=80, on_steam_deck=81, other_game_data=82, gaming_device_type=83 }` |
| Ask for persona data | `ClientRequestFriendData` 815 | `{ persona_state_requested=1 (flags), friends=2 (fixed64 list) }` |
| Set own state | `ClientChangeStatus` 716 | `{ persona_state=1, player_name=2, is_auto_generated_name=3, high_priority=4, persona_set_by_user=5, persona_state_flags=6, need_persona_response=7, is_client_idle=8 }`; reply `ClientPersonaChangeResponse` 5584 `{ result, player_name }` |
| Own "in game" | `ClientGamesPlayed` 742 (also `…WithDataBlob` 5410 as used by node-steam-user, `…NoDataBlob` 715) | `CMsgClientGamesPlayed { games_played=1 [GamePlayed{ game_id(fixed64)=2, game_extra_info=7, process_id=9, game_os_platform=25, game_build_id=26, compat_tool_id=27, compat_tool_cmd=28, beta_name=30, … }], client_os_type=2, cloud_gaming_platform=3, recent_reauthentication=4 }`; an **empty list = stop playing** |
| Rich presence | `ClientRichPresenceUpload` 7501 / `Request` 7502 / `Info` 7503 | game-supplied key/value; out of scope |
| UI mode icon | `ClientCurrentUIMode` 5597 | `{ uimode, chat_mode }` |
| Add / remove friend, nickname, groups | `ClientAddFriend` 791 …, `ClientRemoveFriend`, `ClientSetPlayerNickname`, `ClientFriendsGroupsList` 5553 | optional, not v1 |
| Playing state | `ClientPlayingSessionState` 9600 / `ClientKickPlayingSession` 9601 | see 3.3 |
| Friends who play game | `ClientGetFriendsWhoPlayGame` 5488 | optional |

Enums [src: `SteamLanguage.cs`]: `EPersonaState` = Offline 0, Online 1, Busy 2, Away 3, Snooze 4, LookingToTrade 5, LookingToPlay 6, **Invisible 7**.
`EClientPersonaStateFlag` = Status 1, PlayerName 2, QueryPort 4, SourceID 8, Presence 16, LastSeen 64, UserClanRank 128, GameExtraInfo 256,
GameDataBlob 512, ClanData 1024, Facebook 2048, RichPresence 4096, Broadcast 8192, Watching 16384. node-steam-user's `getPersonas` requests
`Status|PlayerName|QueryPort|SourceID|Presence|Metadata|LastSeen|UserClanRank|GameExtraInfo|GameDataBlob|ClanData|Facebook|RichPresence|Broadcast|Watching`.
(`Metadata` = 32 is marked "removed" in node-steam-user's `EClientPersonaStateFlag.js` and absent from SteamKit2; leave it out.)

**Behaviour documented by node-steam-user** [src README, `setPersona`]: "You need to call this after you logon or else you'll show up as offline.
You won't receive any persona data about your friends if you don't go online." And in `friends.js` (the `ClientFriendsList` handler): after the
non-incremental list, it calls `getPersonas(<all friends>)`, i.e. `ClientRequestFriendData` with the flags above, then waits for `ClientPersonaState`.
Persona pushes then arrive **unprompted** as friends' state changes (including `game_played_app_id`, `gameid`, `game_name`).

Avatars are **not** in the CM messages: only `avatar_hash` (20 bytes). The image URL is built from it (convention: 
`https://avatars.steamstatic.com/<hex hash>_full.jpg`) [inferred; verify the CDN path when implementing]. Names, avatars and `last_logoff` are public-profile-ish
data and fine to cache in memory/disk for the UI.

### 4.2 The own-status problem (read-only presence)

To receive friend data the session must call `ClientChangeStatus`. If the user is running the official client at the same time, **one
account with two sessions has one persona** [inferred]. `CMsgClientPersonaState.Friend.online_session_instances` (7) and `persona_set_by_user` (10)
suggest Steam aggregates per-session states, but the **rule (does Hydra's `Invisible` hide the user while Steam says Online? does Hydra's `Online`
override the user's `Away`?) is unknown** and decides the feature's UX. **[LIVE]**, plan live check L4. Safe candidates for a first version:
(a) Hydra requests **Invisible (7)** to read friends without changing visibility, if the aggregation lets the official session win; (b) only go online
when the user explicitly chooses a status in Hydra.

### 4.3 In-game presence

Reporting `ClientGamesPlayed` with the app's real `game_id` is what the official client does and what makes friends see "In-game: X" and
credits playtime. For a game the user owns, started by Hydra via its own pipeline, that is the same observable behaviour as the official client's. It does
**not** make the game believe anything (no API emulation), but it does record playtime on the account and take the "playing" slot
(3.3). This is a product + policy call, listed in the plan (Q-D). A non-Steam-style `game_id` (high bits set, `game_extra_info` = title) shows the title to friends
without crediting the real app's playtime; the official client uses that for "Add a Non-Steam Game". [src: node-steam-user `gamesPlayed` doc: string = non-Steam game by name]

### 4.4 Can the Web API cover it without a persistent connection?

Probably not for the same fidelity, and the keyless surface is very small:

- `ISteamUser/GetFriendList` and `GetPlayerSummaries` (personastate, `gameid`, `gameextrainfo`) need a **Web API key** and the target's
  profile/friend list to be public [src: Valve wiki/search results]. A launcher cannot ship a key per user and must not ask for one.
- The keyless method list (`GET https://api.steampowered.com/ISteamWebAPIUtil/GetSupportedAPIList/v1/`, fetched during this research) has no friends/presence
  method. Methods that accept a user `access_token` (as `IPlayerService/GetOwnedGames` does in `hydra-steam`) exist for some `IPlayerService`
  calls, e.g. `GetFriendsGameplayInfo` (per-app, in `steammessages_player.steamclient.proto`) and `GetPlayerLinkDetails` (already used for the persona name).
  Whether any `IFriendsList…` service accepts the access token over HTTP is **unknown [LIVE]**; I found no definition in the protobuf repo for a
  `FriendsList` service. Do not plan on it.
- Conclusion: **friends list + persona states = CM session.** A cheap "own profile name/avatar" can keep using `GetPlayerLinkDetails` (already done).

---

## 5. Risks and unknowns

| # | Risk / unknown | Why it matters | Mitigation / test |
|---|---|---|---|
| R1 | Fixed `LoginID` makes overlapping Hydra sessions kick each other (`LogonSessionReplaced`) | breaks the persistent social session or running downloads | unique ID per role; L1 |
| R2 | Random `machine_id` per logon | looks like a new device each time | stable per install; L1 |
| R3 | Official client attaches to Hydra-launched Steamworks games and runs its own cloud sync with no baseline | the exact error the user saw; possible conflict even after Hydra's pull; for Proton, client looks in its own compatdata and may *forget* cloud files it knew | detect + defer policy; L3 |
| R4 | Upload overwrites cloud copies irreversibly ("Steam keeps no copy of a cloud file an upload replaces", Madeira) | user data loss | upload only when baseline proves cloud == last synced, or explicit user choice; keep local backup of what a download replaces; never delete in cloud |
| R5 | Game still writing at exit (Unity flush, child processes) | partial files uploaded | wait for process-group exit + two stable stat/SHA passes 2 s apart (Madeira does this) |
| R6 | Proton profile path/symlink layout unknown (`steamuser` vs `$USER`, `Documents` symlinks) | wrong directory => "nothing to sync" or writes outside the prefix | discover + EvalSymlinks + confine to prefix/home; L2 |
| R7 | `pathtransforms`/`oscompare` semantics only seen in data, not documented | wrong Linux mapping | implement from observed examples, verify against the official client's `cloud_log.txt` lines `Found N files that match <path>` (it prints the resolved path!) for ~5 games; L2 |
| R8 | Remote-storage-API (non-Auto-Cloud) games need the Steamworks IPC to write into `…/remote` | cannot be fed legitimately without a Steam client | out of scope for direct launch; client-mode games use Steam |
| R9 | Encrypted cloud files (`encrypted`, `can_encrypt`) | Madeira refuses | refuse + tell the user; implement `GetClientEncryptionKey` later if seen |
| R10 | Presence aggregation with the official client unknown | could flip the user's visible status | L4; default Invisible/read-only; opt-in status control |
| R11 | `ClientGamesPlayed` credits playtime and blocks the official client from playing another game | surprising side effects | opt-in; never `force`/kick; Q-D |
| R12 | Third-party CM clients and Valve's terms | account risk is unknown; SteamKit/steam-user/Madeira exist, no ban reports found, but no policy statement either | tell the user; keep behaviour identical to the official client's, no spoofing of client identity (`launcher_type` etc.) |
| R13 | Unit/field numbers verified against protobufs, not against a live CM | typos | build fakes from the same protos; first live call is read-only (`GetAppFileChangelist`) |
| R14 | Tokens in logs | hard rule | reuse the existing "token never leaves the package" pattern; log appids and counts only (Madeira logs app ids, counts, no paths) |

---

## 6. What needs a live test with the user

Everything below uses the user's real account and so needs their go-ahead; none was done here.

- **L1 Logon coexistence.** Official client running; Hydra logs on with two different logon IDs plus a stable machine ID; confirm neither side gets
  `ClientLoggedOff`, and the official client stays logged in. Repeat with equal IDs to confirm the kick (`34`), then stop.
- **L2 Read-only cloud listing.** `Cloud.GetAppFileChangelist` for 3-5 owned Auto-Cloud games (SUPERHOT, Hades, Stardew Valley …); compare
  the computed local path for each file with the `Found N files that match <path>` lines the official client logged for the same games, native and Proton.
- **L3 Attach behaviour.** With the official client running and Hydra's cloud code *off*: launch an Auto-Cloud game; then with Hydra having
  pre-pulled the cloud copy first. Check `cloud_log.txt` for a conflict at exit in each case. Do it with a throwaway-save game first.
- **L4 Presence aggregation.** Official client Online; Hydra logs on and sets Invisible; ask a friend (or a second account) what they see;
  vice versa. Then Hydra reports Away while the official client is Online.
- **L5 Playing session.** Hydra reports `ClientGamesPlayed` for an owned app; start another game in the official client; observe the dialog;
  stop in both orders.
- **L6 Upload round trip** on a disposable save: batch begin -> block PUT/POST -> commit -> complete; confirm the official client sees the new change number and
  file. Confirm what happens to `pending_remote_operations` during/after.
- **L7 Download of a zipped file**, of a large file, of an `encrypted` file (to learn the frequency).
