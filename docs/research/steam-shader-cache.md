# Steam shader pre-caching: how the client fetches it, and what Platypus downloads

Status: investigation done 2026-11-17 (issue #205). All live probes were
read-only: one CM logon + PICS reads, `Shader.*` service calls, manifest
request codes, depot keys and manifest fetches over the CDN. No launch, no
purchase, no account write. Downloads during research: two manifests
(~193 KB total); deleted afterwards.

## Sources

- SteamDatabase/Protobufs `steammessages_shader.steamclient.proto` [src]:
  `service Shader` with `GetShaderReportingCadence`, `RegisterShader`,
  `SendShader`, `GetBucketManifest(appid, gpu_desc, driver_desc) ->
  (manifestid, depotsize, bucketid)`, `GetStaleBucket(gpu_desc,
  driver_desc) -> (bucketid, appid, manifestid, gpu_desc, driver_desc,
  depot_encryption_key)`, `ReportExternalBuild`,
  `ReportBucketProcessingComplete`.
- SteamKit2 `SteamMsgShader.cs` (generated): job names
  `"Shader.GetBucketManifest#1"` / `"Shader.GetStaleBucket#1"` [src].
- Steam EResult enum (SteamKit2 `SteamLanguage.cs`): 42 = `NoMatch` [src].
- The client's own vocabulary: `bin/platypus-steam` maps the client task
  `CheckShaderDepotManifest` to `game.clientStatus` `"shaders"` [src].
- Public behaviour: steam-for-linux #11547 ("Downloading N chunks for
  depot 2357570 (manifest…)", "starting commit from
  …/steamapps/shadercache/2357…"), #13647 ("Committed bucket 70016911
  (AppID 2073850) from <old> to <new> … bytes" in `shader_log.txt`;
  on-disk `shadercache/2073850/nvidiav1/…/steamapp_merged_shader_cache.bin`),
  #13215 (toggling pre-caching abandons bucket tracking metadata;
  `steam_pipeline_cache.foz` files), #13197 (kisak-valve: the subcomponent
  "distribute[s] hardware independent shaders"), ValveSoftware/Fossilize
  (`.foz` database format; `fossilize_replay`), nixos-steam
  `fossilize-filters.md` (record → upload → replay pipeline).
- Live probes below [live], run from this worktree with
  `PLATYPUS_PLUGIN_DATA=<scratch>` (machine seed only; the live data dir
  was never modified, tokens never printed).

## The working read-only path [live]

For one owned game, the client obtains that machine's shader cache as:

1. `Shader.GetBucketManifest(appid, gpu_desc, driver_desc)` → `(manifestid,
   depotsize, bucketid)`. No match for the pair is EResult 42 `NoMatch`;
   a known pair with no published bucket for the app answers success with
   `manifestid = 0` (nothing to download).
2. `ContentServerDirectory.GetManifestRequestCode(app_id=appid,
   depot_id=appid, manifest_id, branch="public")` → code. **The shader
   depot id is the app id itself** — PICS lists no depot with id == appid
   for any scanned owned app (Half-Life, Portal, Portal 2, CS2, Garry's
   Mod, …: `hasSelfDepot=false` everywhere), so selection bypasses PICS
   entirely.
3. `ClientGetDepotDecryptionKey(depot_id=appid, app_id=appid)` → 32-byte key.
4. CDN `DownloadManifest(depot=appid, manifest, code)` → manifest whose
   files are `fozpipelinesv6/steam_pipeline_cache.foz` (+ the dir entry);
   filenames are encrypted, decrypted with the depot key.
5. Chunk download over the normal CDN hosts/scheduler, committed under
   `steamapps/shadercache/<appid>/` (client behaviour, [src] above).

Verified live for 6 owned apps (Portal, Portal 2, CS2, Half-Life, A Hat
in Time, The Talos Principle): every app answers a `SteamSwarm /
G7:VulkanPipelinesV6` bucket (e.g. CS2: manifest `1654897073518120278`,
6 638 351 633 bytes; Half-Life: 731 500 bytes). The CS2 and Half-Life
swarm manifests were fetched and parsed; filenames decrypt to
`fozpipelinesv6/steam_pipeline_cache.foz` with matching sizes.

## How (app, GPU, driver) is identified [live]

`gpu_desc` / `driver_desc` are the client's own bucket keys, not free
text: human-readable guesses (`"NVIDIA GeForce GTX 1060" / "580.196.07"`)
answer `NoMatch`. The exact wire strings live in the local Steam client's
`config/config.vdf`, `ShaderCacheManager` section:

```
"CurrentBucketGPU"    "llvmpipe;llvmpipe;c3ccdae58dbfd620;c2946dde828a076c;SteamSwarm;SteamSwarm;SteamSwarm"
"CurrentBucketDriver" "L7:411d…;L7:496c…;L2:82b1…;L2:4122…;G7:VulkanPipelinesV6;G7:CompatAudioTCMediaV1;G7:CompatVideoTCMediaV1"
```

positional pairs: `(llvmpipe, L7:…)` ×2 (GL), two 16-hex GPU hashes with
`L2:` drivers (NVIDIA vk_64/vk_32 — the hashes match the on-disk
`nvidiav1/GLCache/<driver-hash>/<gpu-hash>/` layout), and the three
`SteamSwarm / G7:` shared buckets. The pairs were confirmed live:
the machine's own NVIDIA pair returns a real bucket for CS2
(manifest `2560673956878643153`, 29 233 bytes) and success + `manifest 0`
(no published bucket) for the others; an llvmpipe pair returns success +
`manifest 0` for Portal. `GetShaderReportingCadence` (no params) answers
fine on the same session.

`GetStaleBucket` answers `AccessDenied` for every tried pair, including
the machine's own: that is the background volunteer-work queue (stale
buckets to re-process, paired with `ReportBucketProcessingComplete`),
not the download path, and is not needed.

## What the client does that Platypus does NOT (deliberately)

After the bytes land, the client replays the `.foz` databases with
`fossilize_replay` into driver caches (`nvidiav1/…`,
`mesa_shader_cache…`, `fozpipelinesv6/…`) and points the game at them
(`__GL_SHADER_DISK_CACHE_PATH`, `MESA_GLSL_CACHE_DIR`, … — see local
`shader_log.txt`: "Setting MESA_GLSL_CACHE_DIR=…",
"Processing FOZ cache file at …", "Writing NVIDIA cache of 1 buckets").
Platypus downloads the buckets and runs nothing: the `.foz` files wait in
`shadercache/<appid>/` for the client (or a future replay step) to
process. Download-only therefore saves bandwidth on the next client run
but does not by itself warm the driver caches.

## Implementation consequences (issue #205 step 2)

- Pairs to query, in order: always `(SteamSwarm, G7:VulkanPipelinesV6)`
  (universal; holds the bulk — CS2 6.6 GB swarm vs 29 KB GPU bucket),
  plus the positional `CurrentBucketGPU/Driver` pairs from the local
  Steam client's `config.vdf` when the client has run here. No bucket-hash
  computation is needed.
- Per pair: `NoMatch`/empty-manifest are soft skips; anything else aborts
  the pair with an error. Depot and key app ids are both the app id;
  no branch, no DLC filtering, no PICS.
- Destination mirrors the client: `shadercache/<appid>/` next to the game
  install (a sibling of the store dir's game folders, like
  `steamapps/shadercache` is a sibling of `steamapps/common`).
- Gated behind the plugin's `settings` toggle (default off); multi-GB
  swarm buckets (CS2 6.6 GB, A Hat in Time 8.3 GB) must not ride every
  install silently. Skip-if-current via stored bucket manifest gids.
