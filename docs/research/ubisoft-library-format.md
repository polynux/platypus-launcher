# Research: Ubisoft Connect's local library files (issue #192)

Status: clean-room format note, written 2026-11-17 for issue #192. No Ubisoft
account was used and no Ubisoft file was downloaded or included; every claim is
from public documentation cited inline, not from reading another project's
implementation. The plugin's reader (`plugins/ubisoft-connect/bin/platypus-ubisoft`)
is written from this note alone, because the project licence (issue #197) is not
settled and no code may be ported from any project, MIT or otherwise.

Confidence: **[src]** read in the cited public source for this note. **[inf]**
inferred from those sources, not directly stated by one. **[LIVE]** needs a real
Wine prefix and a signed-in account (L-U1, `ubisoft-options.md` §5).

Two independent 2026 reports and the client's own behaviour agree on the shape:
the client keeps a plaintext library cache under the prefix, a second per-account
file limits it to what the account owns, and the install registry says which of
those products are installed. In October 2026 the client moved the cache to an
encrypted `.vdat` container, which this note also describes.

## 1. Where the files live

Inside a Wine/Proton prefix (`WINEPREFIX`):

```
<prefix>/system.reg                                   HKEY_LOCAL_MACHINE [src: wine registry doc]
<prefix>/drive_c/users/<user>/AppData/Local/Ubisoft Game Launcher/cache/
    configuration/configurations                      plaintext product catalogue
    configuration/configurations.vdat                 the 2026 encrypted replacement
    ownership/<userId>                                the account's owned products
    ownership/<userId>.vdat                           the 2026 encrypted replacement
    settings                                          client settings (unused)
```

Evidence:

- Lutris issue #6999 (2026-11-01) logs
  `C:/users/steamuser/AppData/Local/Ubisoft Game Launcher/cache/configuration/configurations.vdat`
  and states there is no extensionless `configurations` file in that prefix
  [src: github.com/lutris/lutris/issues/6999].
- A GOG Galaxy Uplay integration PR (2023) records that the client started
  keeping `configurations`, `ownership` and `settings` in
  `AppData/Local/Ubisoft Game Launcher/cache` instead of the launcher install
  folder's `cache` [src: github.com/FriendsOfGalaxy/galaxy-integration-uplay/pull/51].
- The unifideck store spec (2026-06-22) names the same paths under
  `drive_c/users/steamuser/AppData/Local/Ubisoft Game Launcher/`: 
  `cache/configuration/configurations`, `cache/ownership/{userId}`, and the
  credential files `ConnectSecureStorage.dat` / `user.dat`
  [src: github.com/mubaraknumann/unifideck/blob/staging/docs/ubisoft-store-spec.md §9].
- SteamAll issue #1 (2026) reports the same `.vdat` location, ~290 KB, and no
  extensionless file [src: github.com/Koisuji02/SteamAll/issues/1].

**[inf]** The per-user folder name is the Wine user the client runs as (Playnite
and unifideck both show `steamuser` for Proton prefixes); a prefix can hold
several user profiles, so a reader that wants the signed-in account should probe
each `users/*` folder. The legacy in-install-folder `cache/` is still worth
probing for older clients [src: Galaxy PR #51].

The ownership file's `<userId>` is the Ubisoft user id: the file is per account,
written when the client is signed in [src: unifideck spec §9; Playnite's reader is
named after the same user id, per `ubisoft-options.md` §5]. Its exact spelling is
**[LIVE]**.

## 2. `configurations`: the product catalogue

Container: a protobuf message (`ConfigurationCache`) whose repeated field 1 holds
one `Configuration` per product, each with:

| field | type | meaning |
|---|---|---|
| 1 | uint32 | product id (the numeric Ubisoft product id, also the install-registry key) |
| 2 | uint32 | uplay id (secondary numeric id, not needed for this reader) |
| 3 | string | the product's YAML configuration blob |

[src: github.com/UplayDB/Uplay-Protobufs, `protos/configuration.proto` (MIT; a
schema-only dump of the client's messages, no client code).]

The framing follows from that schema: the file is `ConfigurationCache`, so the
repeated `Configuration` messages are length-delimited field 1 entries (a `0x0A`
tag, a varint length, then the record). unifideck's spec describes the same
framing independently: "records are framed by a `0x0A` marker + varint size;
fields use `0x08`/`0x10`/`0x1A` markers, with an embedded YAML blob"
[src: unifideck spec §5]. `0x08` is field 1 varint, `0x10` field 2 varint, `0x1A`
field 3 length-delimited, which matches the schema field numbers above.

The YAML blob is a v2.0 product description. UplayDB's public dataset dumps one
entry per product as `{"ProductId": …, "Configuration": "<the YAML text>"}`, which
is exactly the (product id, YAML) pair of the message
[src: github.com/UplayDB/UplayManifests, `productconfig.json`]. The keys this
reader uses, all seen in that public dataset:

```yaml
version: 2.0
root:
  name: "Tom Clancy's Splinter Cell Conviction"   # display title
  is_visible: no                                  # hidden store entry
  is_ulc: yes                                      # Ubisoft "ULC" content (add-on), not a game
  start_game:
    online:
      executables:
        - path:
            relative: "Conviction_game.exe"        # the game's executable, relative to the install dir
    offline:
      executables: […]
  installer:
    game_identifier: Splinter Cell Conviction       # fallback title
  third_party_platform:
    name: Steam                                     # sold elsewhere (Steam/Epic), not a Ubisoft-owned game
  addons:
  - id: 96
    is_visible: no
    name: "… - Deniable Ops: Insurgency"
```

[src: the same `productconfig.json`; add-on entries were also observed there as
separate products whose `root` has `is_ulc: yes` and no `start_game`.]

### What the catalogue is filtered by

Games and add-ons are separate products in the same file. The reader keeps only
launchable games:

- `root.is_ulc: yes` marks ULC/add-on content, not a game [src: `productconfig.json`].
- A product with no `root.start_game` executable is not launchable (DLC, sound
  tracks, demos of removed games) [src: `productconfig.json`; the same test is in
  Playnite's Uplay reader per `ubisoft-options.md` §5].
- `root.third_party_platform` names another store (Steam, Epic); those products
  are owned and launched there, not by this Ubisoft library
  [src: `productconfig.json`; `ubisoft-options.md` §5 lists the same filter].
- `root.addons` lists add-on ids owned with the game; they are not separate
  library entries [src: `productconfig.json`].

Titles come from `root.name`, falling back to `root.installer.game_identifier`
[src: Galaxy PR #51 describes the same fallback order].

**[LIVE]** Whether `root.start_game.online.executables[0].path.relative` is
always the file the running game process is named after. The client itself starts
the game from this blob, so it is the best available source for a launch
`find`; the value can contain subdirectories, so only its basename is usable as a
process-name match.

## 3. `ownership/<userId>`: what the account owns

Container: protobuf `OwnershipCache`:

| field | type | meaning |
|---|---|---|
| 1 | repeated `OwnedGame` | one entry per owned product |
| 2 | repeated uint32 | the owned product ids |
| 3 | repeated uint32 | visible/installable product ids |

`OwnedGame`'s fields that matter here are `product_id` (1, required) and
`uplay_id` (2); it also carries `product_type` (6), `state` (7), `platform` (5),
`target_partner` (17) and `activation_type` (18), which this reader does not need
[src: github.com/UplayDB/Uplay-Protobufs, `protos/ownership_cache.proto`].

**[inf]** Field 1 is the authoritative per-product record and field 2 the same
set as plain ids; taking the union of both is safe and tolerant. Field 3 is a
superset that can include add-ons, so it is not used on its own.

The catalogue and the ownership file are intersected: a cached product
configuration is not proof of ownership (the client caches descriptions of
products it does not own), and only the pair gives the user's library
[src: `ubisoft-options.md` §5, the same two-file join].

## 4. Installs: the Wine registry

The client records each installed product in the machine registry:

```
HKLM\Software\Wow6432Node\Ubisoft\Launcher\Installs\<product id>\InstallDir = "C:\…"
```

with the unredirected `HKLM\Software\Ubisoft\Launcher\Installs\<id>\InstallDir`
also seen for 64-bit clients [src:
github.com/King4s/OptiScaler-GUI/blob/v2026.9.0/docs/store-formats.md §Ubisoft
Connect: numeric subkeys with an `InstallDir` string, both the Wow6432Node path
and a second unredirected path; `ubisoft-options.md` §5 cites Playnite and the
Galaxy 64-bit registry fix for the same]. The numeric subkey is the Ubisoft
product id, i.e. the same id as `Configuration.product_id` and the id used in
`uplay://` URLs [src: OptiScaler doc; `ubisoft-options.md` §1.3].

Why both views: Wine redirects 32-bit callers through `Wow6432Node` when that key
exists, but an installer may write either view explicitly, and a prefix can end
up with only one of them [src:
learn.microsoft.com/windows/win32/sysinfo/32-bit-and-64-bit-application-data-in-the-registry;
playportdev/playport docs/evidence/2026-10-11-wow64-registry-view.md, which
observes Wine's no-fallback redirection].

On Linux the whole machine hive is Wine's text file `<prefix>/system.reg`
[src: Wine documentation, `documentation/registry.sgml`: system.reg holds
HKEY_LOCAL_MACHINE, as human-readable text]. Its layout is the Windows `.reg`
text form: a `[Key\Path]` line per key (Wine appends a last-write timestamp), then
`"ValueName"="string"` lines, with backslashes in keys and string values escaped
as `\\` [src:
learn.microsoft.com/windows/win32/sysinfo/registry-files for the `.reg` text
format]. **[LIVE]** The exact escaping of the key lines and whether Wine writes
`#time=` comments in system.reg; the reader escapes both `\\` and `\"` and accepts
a key line with or without a trailing timestamp.

Installed state is therefore "the registry has an `InstallDir` for this product",
and the Linux path is the registry path with `C:` mapped to
`<prefix>/drive_c` (other drive letters resolve through `<prefix>/dosdevices/<x>:`,
the Wine drive mapping) [src: Wine documentation on the `dosdevices` directory;
**[inf]** for the mapping rule].

A game can be owned and not installed: it is in the catalogue and in the
ownership file, with no install key. The library lists it as owned, not
installed.

## 5. The October 2026 `.vdat` change

Two independent reports (2026-11: Lutris #6999 and SteamAll #1) and a Playnite
cache-decryptor PR describe the same migration: the client writes
`configurations.vdat` (and an `.vdat` ownership file) instead of the plaintext
files, and existing parsers find zero games
[src: github.com/lutris/lutris/issues/6999; github.com/Koisuji02/SteamAll/issues/1;
codeberg.org/CrowIsTaken/PlayniteExtensions/pulls/31].

The container is the client's own `versioned_storage` envelope:

```
FileRoot
  field 1: HeaderAndBody
    field 1: Header
      field 1: version (observed 1)
      field 2: EncryptionConfig
        field 1: scheme (observed 1 = AES-256-GCM)
        field 2: key_id (32 ASCII hex characters)
        field 3: iv (12 bytes)
        field 4: tag (16 bytes)
      field 3: HashConfig
    field 2: body = ciphertext
  field 2: hash
```

[src: the same PR body states the field layout and the AES-256-GCM parameters;
github.com/UplayDB/Uplay-Protobufs `protos/versioned_storage.proto` documents the
same message names and field numbers.]
The Lutris hexdump corroborates it: `0a <len> 0a <header len> 08 01 12 <len> 08 01
12 20 <32 hex chars> 1a 0c <12 bytes> 22 10 <16 bytes> …`, i.e. the envelope,
version 1, scheme 1, a 32-character key id, a 12-byte IV and a 16-byte tag, then
high-entropy data [src: lutris/lutris#6999].

The AES key is not in the cache: it lives at
`%LOCALAPPDATA%\Ubisoft Game Launcher\store\<keyId>.bin`, wrapped with Windows
DPAPI in the user's scope, and decrypts to the same protobuf payload the plaintext
file held [src: codeberg PR #31]. Reading it on Linux would need the prefix's
DPAPI user secrets, which does not exist here; the entitlement endpoint is the
documented fallback (`ubisoft-options.md` §5, phase B, gated on Q-U1). This plugin
therefore treats a `.vdat`-only cache as unreadable and says so.

Detection used by the reader, in order:

1. A plaintext `configurations` file exists: use it.
2. Only `configurations.vdat` / `ownership/*.vdat` exist: encrypted. The envelope
   walk (bounded varints, bounded lengths) is used only to log what was seen
   (version, scheme, key id presence); the payload is never guessed.
3. Plaintext files exist but do not parse (truncated, declared lengths past the
   end, no records): unreadable, same user-facing state as missing.
4. Neither file: the client was never started/signed in in this prefix.

All four cases answer the same empty state, because all four mean "Platypus
cannot read the library yet": one localized status message (a library warning
in the UI, code -32002), not an error per call, and no retry loop.

## 6. Launching

`uplay://launch/<product id>/0` opens the product through the official client
[src: `ubisoft-options.md` §1.3, citing RizSavio/UplayGameIDs and unifideck §7].
The id is the numeric product id used by the install registry, which is why
installed detection, the catalogue join and the launch URL all use the same
number. **[LIVE]** confirm on a real prefix that the client accepts the id from
`Configuration.product_id` (the alternative is `Configuration.uplay_id`, field 2,
which is zero or unrelated in the public dataset).

The game's `find` process name comes from the catalogue's `start_game` executable
basename (§2). A game that is installed but whose catalogue record is missing
cannot be launched safely and stays unlaunchable rather than launching the wrong
process.

## 7. What is still unverified (L-U1)

- Exact cache paths and per-user folder name in Platypus's own prefix; whether the
  client still writes the legacy in-install-folder `cache/`.
- The ownership file name (`<userId>`) and whether field 1, field 2 or both carry
  the owned set on a real account.
- The real `.vdat` files: envelope values beyond the two reports, whether both
  cache files are encrypted together, and whether an old plaintext file lingers
  after the migration.
- Whether `uplay://launch` takes `product_id` or `uplay_id`.
- Titles when `root.name` is empty (some catalogue entries have no name) and
  whether every launchable game really has an `online.executables` entry.
- Whether the client rewrites `configurations` on install/update, which the
  install watch relies on in addition to the install registry key.

Manual check on a real machine: install the client in a managed prefix, sign in
once, rescan the library (games must appear, DLC must not), install one game from
inside the client (it must flip to installed with its path), then launch it
through Platypus (`uplay://launch/<id>/0` must start the client's game). With a
client build that writes `.vdat`, the library must stay empty with the
"start the client and sign in" status instead of errors.
