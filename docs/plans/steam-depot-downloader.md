# Plan: a Go Steam depot downloader for steam-standalone

Status: planned (milestone 7 of [launcher-plugins.md](launcher-plugins.md)).
Track progress in the checklist at the end; record anything that turns out
different from this plan under "Changes during implementation".

## Goal

Replace DepotDownloader in the `steam-standalone` plugin with our own Go
downloader that:

1. **Downloads at least as fast as the Steam client** on the same line. On the
   user's machine Steam reaches 325–711 Mbps; DepotDownloader manages
   1.4 MiB/s with 8 connections and 5 MiB/s with 24.
2. **Removes the .NET and DepotDownloader requirements.** The plugin should
   need only its own binaries (and `steam` for launch mode c).
3. Is **resumable, verifiable and incremental**: updates fetch only changed
   chunks, interrupted installs continue, and "verify files" repairs damage.

It downloads only what the logged-in account owns: Steam hands out depot keys
and manifest request codes only for licensed depots. No DRM or licence
bypass, and no Steam API emulators (same rule as the rest of the plugin plan).

## Why DepotDownloader is slow (the measured cause)

`~/.local/share/Steam/logs/content_log.txt` shows the Steam client:
- gets 32 download sources from `GetServersForSteamPipe` (CellID 14);
- uses only **9–11 connections**, raising the count while the rate keeps rising
  ("Increasing target number of download connections to 9 … 10 … 11");
- spreads them over **8–12 hosts at once** (`cache1-par1` … `cache12-par1`,
  some `ams1`).

DepotDownloader (v3.4.0, `CDNClientPool.cs`) sends every request to **one**
host: `GetConnection()` returns `servers[nextServer % servers.Count]`, and
`nextServer` only advances in `ReturnBrokenConnection()`. Adding connections
(`-max-downloads`) or forcing a region (`-cellid`) can't fix that. The cure is
a scheduler that spreads chunks over many hosts, which is the core of this plan.

## References

Study these to learn the protocol, then write our own code. Don't translate
files line by line: SteamKit2 is LGPL-2.1 and DepotDownloader is GPL-2.0, and
Hydra has no licence chosen yet.

| What | Where |
|---|---|
| CM connection, logon, service calls (ours, working) | `plugins/steam-standalone/bin/steam_cm.py` |
| Service RPCs: `GetServersForSteamPipe`, `GetManifestRequestCode`, `GetCDNAuthToken` | SteamKit2 `SteamKit2/Steam/Handlers/SteamContent/SteamContent.cs` |
| Depot keys (`ClientGetDepotDecryptionKey`), PICS product info and access tokens, licence list | SteamKit2 `SteamKit2/Steam/Handlers/SteamApps/SteamApps.cs` |
| CDN URLs, manifest download, timeouts | SteamKit2 `SteamKit2/Steam/CDN/Client.cs` |
| Chunk decrypt, decompress, checksum | SteamKit2 `SteamKit2/Steam/CDN/DepotChunk.cs` |
| Manifest binary format, filename decryption | SteamKit2 `SteamKit2/Types/DepotManifest.cs` |
| VZip (LZMA) and VSZa (zstd) containers | SteamKit2 `SteamKit2/Util/VZipUtil.cs`, `VZstdUtil.cs` |
| End-to-end flow: depot selection, updates, chunk reuse, validation | DepotDownloader `DepotDownloader/ContentDownloader.cs` |
| What to avoid: single-host pool | DepotDownloader `DepotDownloader/CDNClientPool.cs` |
| `.proto` definitions (content manifest, client/server messages, content system service) | SteamDatabase/Protobufs: `steam/content_manifest.proto`, `steam/steammessages_clientserver*.proto`, `steam/steammessages_contentsystem.steamclient.proto`, `steam/enums_clientserver.proto` |
| How the real client behaves on this account (hosts, connections, rates) | `~/.local/share/Steam/logs/content_log.txt` |
| Text KeyValues parser (PICS app info is text KV) | `internal/steam/kv.go` (ours) |

Repos: github.com/SteamRE/SteamKit, github.com/SteamRE/DepotDownloader,
github.com/SteamDatabase/Protobufs.

## Protocol facts (checked in the references above, 2026-10-07)

- **Manifest URL:** `depot/{depotId}/manifest/{manifestId}/5/{requestCode}`
  (`MANIFEST_VERSION = 5`; without a request code the last segment is
  omitted). **Chunk URL:** `depot/{depotId}/chunk/{chunkIdHex}`. The scheme is
  `http` or `https` according to the server entry. Some hosts need a CDN auth
  token from `ContentServerDirectory.GetCDNAuthToken` (`app_id`, `depot_id`,
  `host_name`).
- **Request codes** come from `ContentServerDirectory.GetManifestRequestCode`
  (`app_id`, `depot_id`, `manifest_id`, `app_branch`, `branch_password_hash`)
  and expire after about 5 minutes.
- **Server list:** `ContentServerDirectory.GetServersForSteamPipe` (`cell_id`,
  `max_servers`). DepotDownloader keeps types `SteamCache` and `CDN`, honours
  `allowed_app_ids`, and sorts by penalty, then `weighted_load`;
  `num_entries_in_client_list` repeats a server in the list.
- **Manifest file:** a zip with one entry. The entry is a sequence of
  `<uint32 magic><uint32 length><protobuf>` sections: payload `0x71F617D0`,
  metadata `0x1F4812BE`, signature `0x1B81B817`, end `0x32C415AB`. File names
  are encrypted when the metadata says so: base64, then the same AES scheme
  as chunks.
- **Chunk:** first 16 bytes are the IV encrypted with AES-256-ECB (depot key);
  the rest is AES-256-CBC with PKCS7. Then decompress by magic: `VSZa` = zstd,
  `VZa` = LZMA (VZip), `PK\x03\x04` = zip. Verify Adler-32 **seeded with 0**
  (not the usual 1) of the uncompressed data against `ChunkData.Checksum`.
- **VZip:** `'V' 'Z'` (0x5A56 LE), version `'a'`, uint32 CRC or timestamp,
  1 byte LZMA properties, uint32 dictionary size, LZMA stream, then a 10-byte
  footer: uint32 CRC, int32 decompressed size, `'z' 'v'` (0x767A LE).
- **Manifest data:** files carry name, flags (directory, executable, symlink
  with `LinkTarget`), total size and SHA-1 `FileHash`; chunks carry
  `ChunkID` (SHA-1), `Checksum`, `Offset`, `CompressedLength`,
  `UncompressedLength`. One chunk id can appear in several files.
- **Depot selection** (PICS app info, text KeyValues): `depots/<id>/config`
  with `oslist`, `osarch`, `language`, `lowviolence`; manifests at
  `depots/<id>/manifests/<branch>/gid`; `depotfromapp` points to another app's
  depot; password branches need `CheckAppBetaPassword`. PICS needs an access
  token per app (`ClientPICSAccessTokenRequest`) before `ClientPICSProductInfoRequest`.

Look up EMsg numbers in `enums_clientserver.proto`, not from memory.

## Architecture

- **Go packages** in this module (plugins already build from it, see
  `plugins/install.sh`):
  - `internal/steamcm`: a Go port of what `steam_cm.py` does (WebSocket CM,
    hello, logon with the refresh token, Multi/gzip, heartbeats, service
    calls), plus depot keys, PICS access tokens and product info, licences, the
    server list, request codes and CDN tokens.
  - `internal/depot`: the manifest parser, chunk decoder, CDN client,
    scheduler, file writer, verify and update logic. No CM code, so it can be
    tested on its own with a fake CDN.
- **Binary** `hydra-steam-dl`, built from `plugins/steam-standalone/cmd/hydra-steam-dl`
  into the plugin's `bin/`. `install.sh` has to learn to build Go helpers next
  to a non-Go exec; whole-folder hash pinning already covers the result.
- **Interface to the Python plugin:** subcommands that print JSON lines on
  stdout (`size`, `install`, `update`, `verify`). Progress lines carry bytes
  done, bytes total, current speed, connection count and host count; the last
  line is a result or an error with a code. The refresh token goes in through
  stdin or a 0600 file in `HYDRA_PLUGIN_DATA`, **never argv** (other users can
  read `/proc/<pid>/cmdline`), and is never logged.
- **Python plugin changes:** `install_size`, installs, updates and uninstalls
  call `hydra-steam-dl` instead of `depotdownloader`. Login stays on
  DepotDownloader until phase 6.

## Scheduler design (where the speed comes from)

- Fetch the server list once per job. Drop servers not allowed for the app,
  and keep the order Steam gives (penalty, then weighted load).
- **Many hosts, few connections each:** start with ~8 connections spread over
  the top N hosts (at most 2 per host to begin with). Like Steam, add a
  connection every few seconds while the measured rate keeps rising, and stop
  when it plateaus. Cap per host and overall (start with 4 per host and 32
  overall; tune live).
- **Per-host health:** track throughput and errors per host. Penalise and rest
  a host on timeouts, 5xx or a bad checksum, and move its queued chunks to
  others. Retry a chunk on another host, with backoff and a retry limit.
- **Keep-alive HTTP/1.1** with one `http.Transport` per host. Use timeouts
  similar to SteamKit's (10 s per request, 60 s for the body).
- **Pipeline:** network workers, then a CPU pool (`GOMAXPROCS`) to decrypt,
  decompress and checksum, then writers using `WriteAt` into preallocated
  files (no per-file lock). Bound the bytes in flight so memory stays flat.
- **Order:** files sorted by path, chunks by offset, so a partial install is
  mostly whole files. Download a chunk shared by several files once and copy
  it to each.

## Disk layout, resume and updates

- Write into the final install folder. Keep job state in
  `HYDRA_PLUGIN_DATA/jobs/<job>.json` (manifest ids and completed chunk ids,
  flushed every few seconds), and the installed manifest per depot in
  `HYDRA_PLUGIN_DATA/manifests/<app>/<depot>_<gid>.manifest`.
- **Resume:** reload the job state, re-check partially written chunks by
  checksum, and fetch only what's missing.
- **Update:** compare the old and new manifests. Reuse unchanged chunks by
  copying them from the old file, as DepotDownloader does, fetch new chunks,
  delete files that are gone, and apply flags (exec bit, symlinks).
- **Verify:** SHA-1 each file against `FileHash`. Re-fetch the chunks of
  mismatched files.
- Check free space against the manifest's total size before starting (the
  plugin's `game.installSize` already does this).

## Phases (one commit or more each, all on `feat/steam-depot-dl`)

1. **Offline core (`internal/depot`).** Chunk decoder (AES, VZip/LZMA, zstd,
   zip, Adler-32 seed 0), manifest parser with filename decryption, and test
   fixtures we generate ourselves: encrypt and compress with a test key, and
   build manifests from Go structs. No network.
2. **CM in Go (`internal/steamcm`).** Port `steam_cm.py`, then add depot
   keys, PICS (access token, then product info parsed with `internal/steam/kv.go`),
   licences, the server list, request codes and CDN tokens. Test against a
   fake CM: extend `testdata/fake_cm.py` or write a Go fake.
3. **Downloader and scheduler.** The CDN client, multi-host scheduler, writer,
   resume and verify. Test against an `httptest` fake CDN with several hosts,
   including a slow host, a failing host and bad chunks. Add a benchmark that
   shows multi-host beats single-host on throttled fake hosts.
4. **`hydra-steam-dl` binary and plugin wiring.** Subcommands, JSON progress,
   `install.sh` support, and the Python plugin switched over. The existing
   steam-standalone tests and the conformance suite must pass with the fake
   CDN. Keep DepotDownloader only for login.
5. **Live validation with the user** (ask before every download):
   - Install a small owned game. Compare every file's SHA-1 with the manifest,
     and do a test launch.
   - Install SUPERHOT (4.1 GiB) and record the speed, host count and
     connection count. Steam's own rate for the same game, from
     `content_log.txt`, is the target. "Done" means at least 50% of it; the
     aim is parity.
   - Test an update: install an older manifest, then update. If that isn't
     possible, verify after deleting and corrupting some files.
   - Cancel and resume mid-download, then uninstall.
6. **Drop DepotDownloader entirely** (optional, separate commits). Do the QR
   and password login in Go: `Authentication.BeginAuthSessionViaQR` /
   `ViaCredentials` and `PollAuthSessionStatus` over the CM, with platform type
   SteamClient. Store the refresh token ourselves, migrate the existing one out
   of DepotDownloader's `account.config`, and remove `depotdownloader` and
   `dotnet` from `requires`.

## Risks

- **CDN auth tokens and host types:** some hosts (third-party CDNs such as
  fastly/edgenext in the log) may need tokens or behave differently.
  Prefer `SteamCache` hosts, and fall back to the others.
- **Rate limits:** too many CM logons hit EResult 84. Use one CM session per
  job and reuse it; don't log on per chunk or per call.
- **Format drift:** manifest version 5 and the VZip/VSZa containers are stable
  today. Keep fixtures from a real manifest (header and structure only, no
  game content) to catch changes.
- **Memory:** chunks are ~1 MiB; bound the bytes in flight to 32 or more
  connections.
- **Licensing:** reimplement from the protocol facts above; don't copy
  SteamKit2 or DepotDownloader code.

## Checklist

- [ ] Phase 1: offline core (decoder, manifest, fixtures)
- [ ] Phase 2: CM in Go (logon, depot keys, PICS, server list, request codes, CDN tokens)
- [ ] Phase 3: downloader and scheduler (multi-host, resume, verify, updates) with fake CDN and benchmark
- [ ] Phase 4: `hydra-steam-dl` binary, install.sh, Python plugin switched over
- [ ] Phase 5: live validation (small game, SUPERHOT speed, update, resume, uninstall)
- [ ] Phase 6 (optional): login in Go, DepotDownloader and dotnet removed

## Changes during implementation

(none yet)
