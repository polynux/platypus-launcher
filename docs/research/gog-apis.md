# GOG APIs behind the GOG plugin

What `plugins/gog-gogdl` reads besides gogdl's own manifest and build data:
authentication, achievements, playtime, file verification and install options.
Written 2026-11-17; no GOG account was available, so every endpoint below is
from the sources named, not from a live call (see "Live checks" at the end).

## Authentication and credentials

gogdl does all of it; the plugin only runs it.

- OAuth client (gogdl's own, used by Heroic too):
  client id `46899977096215655`, client secret
  `9d85c43b1482497dbbce61f6e4aa173a433796eeae2ca8c5f6129f2dc4de46d9`.
- Code exchange: `GET https://auth.gog.com/token?client_id=…&client_secret=…
  &grant_type=authorization_code&redirect_uri=https%3A%2F%2Fembed.gog.com%2Fon_login_success%3Forigin%3Dclient&code=<code>`
- Refresh: the same URL with `grant_type=refresh_token&refresh_token=<token>`
  (plus `without_new_session=1` for background refreshes).
- What gogdl stores (in `--auth-config-path`): `access_token`, `refresh_token`,
  `expires_in`, `loginTime`, and `user_id` — the account id the gameplay API
  needs. `gogdl auth` refreshes an expired token before printing this JSON, so
  the plugin never talks to `auth.gog.com` itself.

Sources: `gogdl/auth.py` (heroic-gogdl `6c6ef53`, 2026-10-28).

## Achievements (protocol `achievements.list`)

```
GET https://gameplay.gog.com/clients/<product id>/users/<user id>/achievements
Authorization: Bearer <access_token>
X-Gog-Lc: en-US          (the language names and descriptions come in)
```

`clients/<id>` here is the game's product id (called the Galaxy client id), not
the OAuth client id. Response: `{ total_count, limit, page_token, items: [...] }`
with one item per achievement:

| Field | Use |
|---|---|
| `achievement_key` | the stable id the game's SDK unlocks with; protocol `id` |
| `achievement_id` | GOG's internal numeric id |
| `name`, `description` | display text in the `X-Gog-Lc` language |
| `visible` | `false` = hidden until unlocked (protocol `hidden`) |
| `image_url_locked`, `image_url_unlocked` | icons on `images.gog.com` |
| `date_unlocked` | ISO-8601 string, `null` while locked (protocol `unlockedAt`) |
| `rarity` | share of players, 0–100 (protocol `globalPercent`) |
| `rarity_level_slug`, `rarity_level_description` | GOG's rarity buckets |

A game without achievements (or not owned) answers 404, which the plugin reports
as an empty list; 401/403 means the token is stale.

Sources: Heroic `src/backend/storeManagers/gog/games.ts` (`getAchievements`),
comet `src/api/gog/achievements.rs`, gogapidocs `galaxy.html` (§gameplay.gog.com).

## Playtime (protocol `playtime.list`)

```
GET https://gameplay.gog.com/games/<product id>/users/<user id>/sessions
Authorization: Bearer <access_token>
-> { "time_sum": <minutes> }
```

`time_sum` is minutes (Heroic divides by 60 for hours). There is no
library-wide call: Heroic reads one game at a time, and the plugin does the same
for every owned game after one account listing. Asking for the whole library on
every launcher start is hundreds of calls, so the plugin keeps each game's time
in `playtime.json` under its data folder: an installed game is asked again
after 10 minutes, the rest after 6 hours, and the refresh stops after 25 s with
whatever finished (a slow or 429-answering game keeps its cached time instead of
showing 0). The `clients/<id>/users/<user id>/sessions` form also exists (comet
uses it for the running game) but is undocumented and was not tested; the
`games/…` form is the one Heroic ships.

Language matching for depots mirrors gogdl's table: a depot's language counts
when it is `"*"`, the installed language's code, its English name, or one of its
legacy codes (`en`/`English` for `en-US`, `br` for `pt-BR`, `cn`/`zh` for
`zh-Hans`, and so on) — case-insensitive, with no prefix matching. The same
table maps a bare UI locale to the `X-Gog-Lc` code (`fr` → `fr-FR`, unknown →
`en-US`).

Important limit: GOG only learns playtime from **posted sessions**. Heroic posts
them on game exit (`POST .../sessions` with `{ session_date, time }`, time in
minutes); the plugin does not post, so time played in Platypus only shows up in
GOG's number if another client (Galaxy/Heroic) has posted it. Core shows
`max(local, store)` anyway.

Sources: Heroic `games.ts` (`getGOGPlaytime`, `updateGOGPlaytime`),
`library.ts` (`postPlaytimeSession`), comet `src/api/gog/sessions.rs`.

## Verifying files (protocol `game.verify`)

`gogdl repair` is `gogdl download` with verification: in `gogdl/dl/managers/*`
the `repair` command sets `is_verifying`, re-hashes every file against the
manifest stored during the install, and re-downloads only the broken chunks. It
always repairs, so a check (repair off) cannot be `gogdl repair`; the plugin
hashes the same file lists itself.

- The manifest gogdl writes after an install lives in
  `$GOGDL_CONFIG_PATH/heroic_gogdl/manifests/<product id>` (v2 games) and holds
  `HGLInstallLanguage`, `HGLdlcs`, `baseProductId` and `depots[]`, where each
  depot names a manifest hash — the **file list is not in it**.
- The file list is `GET https://cdn.gog.com/content-system/v2/meta/<ab/cd/hash>`,
  zlib-compressed JSON (`Content-System V2 responses are zlib encoded, window
  size 15`): `{ depot: { items: [ { type: "DepotFile", path, chunks: [ {md5, size, …} ], flags? } ] } }`.
  `flags: ["support"]` files live in the game's `gog-support` folder, not in
  the game folder, so verify skips them; files with no chunks are skipped like
  gogdl does.
- Repair needs the stored manifest (`ManifestUnavailable: No manifest stored
  locally, unable to verify`); `download` (a fresh install) does not.
- Linux installers use `<game dir>/.gogdl-linux-manifest` instead:
  `{ files: [ { file_name: "data/noarch/…", crc32, size, is_symlink } ] }`, and
  gogdl's Linux repair opens
  `$GOGDL_CONFIG_PATH/heroic_gogdl/exclude/<product id>` unconditionally — the
  plugin creates it empty before a repair. gogdl's Linux manager rebuilds only
  the install it is told about, so a repair passes the recorded `--lang` and
  `--with-dlcs --dlcs`; the plugin stores them in its install record because a
  Linux install has no gogdl manifest to read them from.
- Generation-1 manifests (`version: 1`) point at
  `content-system/v1/manifests/...` and use a different layout; the plugin
  refuses to check those and asks for an update first.

Sources: heroic-gogdl `gogdl/cli.py`, `gogdl/args.py`, `gogdl/constants.py`,
`gogdl/dl/managers/{manager,v2,linux}.py`, `gogdl/dl/objects/v2.py`,
gogapidocs `galaxy.html` (§cdn.gog.com, v1 and v2 metadata).

## Install options (protocol `game.options`)

gogdl's `info` already reports both lists, and `download` takes both choices:

- `--lang <code>` — one installer language (v2 default `en-US`, v1 default
  English); `info` answers `languages: ["en-US", …]`.
- `--with-dlcs` + `--dlcs <id,id,...>` — take exactly these owned add-ons;
  `--skip-dlcs` takes none. (`--dlcs` alone is not enough: it only filters what
  `--with-dlcs` allows.) `info` answers `dlcs: [{id, title, size: {lang: {download_size, disk_size}}}]`
  for the add-ons the account owns, so unowned DLC never appears here.
- Sizes are keyed by language plus `"*"` (content every language installs):
  the size of an install in language L is `size["*"] + size[L]`, and each DLC
  reports the same shape.

Sources: heroic-gogdl `gogdl/args.py`, `gogdl/dl/managers/v2.py`
(`calculate_download_size`), `gogdl/dl/objects/v2.py`
(`calculate_download_size`), `gogdl/dl/managers/linux.py`.

## Live checks still needed

No account was available, so these are unverified on the wire:

1. `X-Gog-Lc` accepts the mapped code (a bare `de` becomes `de-DE`); if GOG
   rejects the mapping, achievements fall back to a fixed `en-US`.
2. The achievements list's paging (`limit` 1000): games with more than one page
   are assumed not to exist.
3. `games/<id>/users/<user>/sessions` returns `time_sum` in minutes for a real
   account, and how many requests a large library tolerates (one per game, at
   most 25 s of them per call).
4. Whether `clients/<id>/users/<user>/sessions` is equivalent (comet's form).
5. A repair on a corrupted Windows install re-downloads only the broken chunks
   and the post-check comes back clean; and the same for a Linux installer,
   including that the language and add-ons stay installed.
6. Depot language codes really are the ones gogdl's table names (`en`, `br`,
   `cn`, English names) for real multi-language games.
7. `game.options` on a movie/extra (no installers) answers empty groups rather
   than an error, and a language GOG dropped from a game is refused when asked
   for.

## Live results (arco, 2026-11-17)

Both stores logged in. Library: 29 owned games, none installed. One small
owned game (Forager, 0.12 GB download / 0.16 GB disk, Windows-only, single
language) installed for the verify/repair/move checks, then uninstalled;
records clean afterwards. No other user files touched.

1. `X-Gog-Lc` mapping: **pass**. `achievements.list` with locale `de`
   returns German names/descriptions on a 57-achievement game, so the
   `de` → `de-DE` mapping is accepted; no `en-US` fallback needed.
2. Paging: **pass (single page)**. Raw achievements answer:
   `total_count` 57, `limit` 1000, `page_token` 0, 57 items. No owned game
   exceeds one page (largest seen: 57); the multi-page path is still
   untested.
3. Sessions timing: **pass with a caveat**. Raw
   `games/<id>/users/<user>/sessions` answers `{"time_sum": <minutes>}`
   (int) in ~0.13 s for one game; at that rate a full 29-game sweep is
   ~4 s, well under the plugin's 25 s cap. `playtime.list` returned 3
   games with nonzero time in 1.65 s, but the plugin cache was fresh that
   day, so the per-game live path was timed only via the direct request.
4. `clients/.../sessions`: **not equivalent**. The raw `clients/<id>/`
   form answers 404 where `games/<id>/` answers 200 with `time_sum`, so
   the plugin's choice of the `games/` form is the correct one.
5. Repair: **pass (Windows); Linux installer half not checked**.
   Forager: check-only clean (30 checked, 0 damaged); a flipped byte
   inside a data file reports damaged 1; repair reports repaired 1,
   damaged 0, and the file is byte-identical to a pre-test backup
   (phases `checking` → `repairing`, seconds, so only broken chunks
   re-downloaded). Appending one byte past end-of-file is NOT flagged
   (chunk ranges are hashed, file size is not compared) — observation only.
   Linux exclude-file / `--lang` / `--with-dlcs` repair path and the
   language/add-on preservation claim are still unverified on the wire.
6. Depot codes: **partial**. Forager's stored manifest lists depots with
   languages `en-US` and `*`, matching gogdl's table. Legacy depot codes
   (`en`, `br`, `cn`) on a real multi-language game not observed live
   (would need a second, larger install).
7. Extras and dropped languages: **pass after one fix**. One goodies
   collection answers `game.options` → `{"groups": []}`; the other
   (Witcher goodies) errored with -32002 because the catalog lists
   installers gogdl's content system cannot install — fixed on
   `live/11-gog-epic` (treat content-system-unsupported as an extra;
   regression test `TestGogExtraWithInstallersListsNoOptions`), now `[]`
   on both. `installSize` on extras errors clearly (-32000, names the
   content-system failure). A dropped language is refused at install
   time: `game.install` with `xx-YY` fails fast with -32602 before any
   download; `installSize` with an unknown language just estimates the
   default (no refusal, by design).
