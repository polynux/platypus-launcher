# Epic Games Store: verify, move, versions, playtime and achievements

Research for the Epic plugin (`plugins/epic-legendary`), which wraps
[legendary](https://github.com/legendary-gl/legendary). Sources: legendary
`170964e75affc0c04be467ec2b12879376738ef5` (2026-11-04), Heroic's current
`main` (checked 2026-11-17) and `gaming-stats-api`
`5fde37fb798f96a50d602b63dfd778dd157241fe` (2026-11-08). No live Epic account
was used; everything below is read from the source, and the plugin's checks ran
against fakes.

## Verify and repair (`game.verify`)

- `legendary verify <app>` (`cli.py:verify_game`) loads the **installed**
  manifest, so it needs no login or network. It prints one progress line per
  second:

  ```
  Verification progress: 123/456 (27.0%) [12.3 MiB/s]
  ```

  followed by `Verification finished successfully.` or
  `Verification failed, N file(s) corrupted, M file(s) are missing.`
  There is no `--json`; the exit code is 0 whether the check passed or not, so
  the plugin parses those lines. Missing manifests and paths are reported on
  stderr and also exit 0, so a run without a summary is a failure. A repair
  needs the downloader's `Finished installation process` (or
  `Download size is 0`): legendary logs `Installation failed` and still exits 0
  when the download fails, so only a finish line counts as a repaired game.
- `legendary repair <app>` (`repair` is an alias of `install` with
  `--repair`) verifies first when `<config>/tmp/<app>.repair` is absent, then
  re-downloads the files it listed. With a stale repair file it trusts that
  file, so the plugin deletes it before a repair to force a fresh check and get
  counts. Repair needs the login (it downloads).
- The plugin reports `phase: "checking"` while verifying and
  `phase: "repairing"` while downloading, and `job.done.verify` with
  `{checked, damaged, missing, repaired}`.

## Move (`game.relocated`)

- `legendary move <app> <new base path> --skip-move` (`cli.py:move`) does not
  touch the folder; it rewrites the record in `<config>/installed.json` and
  takes legendary's installed-data lock. The new path is
  `<new base path>/<basename of the old install_path>`, so the plugin only
  follows a move into a new parent folder, which is exactly what core's staged
  move produces (`internal/app/app_move.go`: `to = parent/basename(from)`).
- Exit code stays 0 on its own errors (no record, rename failure), so the
  plugin reads `list-installed --json` again and fails `game.relocated` unless
  the record now points at `to`.
- The plugin does not offer `move`: core's staged move is used, and
  `game.relocated` rebases legendary's record afterwards, as Steam's plugin
  does.

## Playtime (`playtime.list`)

- Endpoint: `GET https://library-service.live.use1a.on.epicgames.com/library/api/public/playtime/account/<account_id>/all`
  with `Authorization: bearer <user access token>` (Heroic's library host,
  `legendary/api/egs.py`).
- Response is a JSON array of `{accountId, artifactId, totalTime}`;
  `artifactId` is the store's app name and `totalTime` is **seconds**
  (`gaming-stats-api` `src/providers/epic/normalize.js:playtimes`). The plugin
  keeps the largest value per app and reports floor(seconds / 60) minutes.
- Credentials come from `legendary get-token --json --bearer` (hidden command,
  `cli.py:get_token`), which prints `access_token` and `account_id` from
  legendary's stored session. The token lives only in the request header and is
  never logged. A 401/403 becomes the protocol's auth-required error.
- Logged out, upstream raises `ValueError('No saved credentials')` (a traceback,
  exit 1) or prints `Login failed` / `Stored credentials are no longer valid`.
  The plugin maps all of those to the auth-required error for both get-token
  and achievements. Timeouts stay under core's 30 s calls: 15 s for the token,
  12 s for the HTTP read, 25 s for achievements and move.
- No timestamp for the last session is in the response, so `lastPlayed` is
  omitted.

## Achievements (`achievements.list`)

- Requires **legendary 0.21.0 or later** (the `achievements` subcommand was
  added on 2026-08-04). The plugin checks `legendary --version` once at startup
  and leaves `achievements` out of its runtime capabilities on older builds;
  argparse's `invalid choice: 'achievements'` is also reported as an update
  hint. `plugin.json`'s requirement names 0.21.
- `legendary achievements <app> --json` (`cli.py:achievements`, `core.py:
  get_achievements`) combines the catalogue's achievement definitions with the
  user's progress from Epic's launcher GraphQL endpoint
  `POST https://launcher.store.epicgames.com/graphql`, query
  `PlayerAchievement.playerAchievementGameRecordsBySandbox(epicAccountId,
  sandboxId)` (`legendary/models/gql.py:egl_game_achievements_user_query`,
  `legendary/api/egs.py:get_game_achievements_user`). The definitions query is
  `Achievement.productAchievementsRecordBySandbox`.
- The JSON answer groups entries into `completed`, `in_progress`,
  `uninitiated` and `hidden`, each with `name`, `display_name`, `description`,
  `icon_link`, `hidden`, `unlocked`, `progress`, `unlock_date` (a stringified
  datetime) and `rarity.percent` (global unlock percentage), plus
  `total_achievements` and `user_unlocked`.
- Heroic does **not** ship Epic achievements today: its current
  `storeManagers/legendary` has no `getAchievements`, and the request is still
  open ([issue #2382](https://github.com/Heroic-Games-Launcher/HeroicGamesLauncher/issues/2382)).
  The endpoint above is what that issue proposed and what legendary implements.
- A game without achievements prints `No achievements found` and no JSON; an
  app the account doesn't know prints `No game found` and no JSON. The plugin
  maps the former to an empty list and the latter to an RPC error.
- Unlocking is never written: this path is read-only.

## Previous versions (`versions`, protocol 1.3)

Epic has no public list of old builds for an app, and its store API only ever
names the current one (`asset_infos.<platform>.build_version`). The plugin
therefore offers only builds the user actually had:

- legendary saves every installed build's manifest at
  `<config>/manifests/<app>_<platform>_<build version>.manifest`
  (`lfs/lgndry.py:_get_manifest_filename`, `core.py:prepare_download` calls
  `save_manifest`). The plugin copies that file into its data folder before
  every update and after every install, as `archive/<app>/<build>.manifest`
  with a small `archive/<app>/<build>.json` (version name, save date, platform,
  the install record's CDN base URLs, download/disk size), keeping the
  installed build plus the three most recently saved older ones per game.
  Uninstalling a game keeps its saved builds; a prune drops the oldest older
  build (and any manifest without a sidecar) once there are more than three.
- `versions.list` answers the installed build and each saved manifest. The
  installed build is `source: "store"` while it is the store's current build
  and `source: "archive"` once the store has moved past it; with no install the
  store's current build is listed as `store`. History entries with no saved
  manifest stay listed with `available: false`; Epic never lists channels, so
  `channels` is empty and there is no `versions.unlock`. `publishedAt` on an
  archive build is the date the plugin saved the manifest, not a store
  publication date (Epic's manifests carry none).
- A target naming an archived build (or Platypus's `archiveRef`, the saved
  manifest's file name) runs `legendary install <app> --manifest <file>`
  (`cli.py:install_parser`: "Manifest URL or path to use instead of the CDN one
  (e.g. for downgrading)"), with `--update-only` for a switch and
  `--base-path` for a fresh install. A local manifest brings no base URLs of
  its own, and the metadata refetch in `install_game` rebuilds the app's entry
  without them (`Game(...)` drops `base_urls`), so `prepare_download` raises
  `No base URLs found` (`core.py:1614`). The sidecar therefore keeps the
  installed record's `base_urls`, and the plugin passes the first one as
  `--base-url` whenever it uses a saved manifest. The plugin reads sizes for
  `game.installSize` from the saved metadata rather than the store's current
  build.
- `job.done.installed` carries the build version legendary recorded and the
  saved manifest's name as `archiveRef`; Platypus's ledger keeps it.
  `updateAvailable` compares the installed build with the store's current one,
  or with the pin's build when Platypus passes one.
- A build installed with `--manifest` records its manifest path
  (`InstalledGame.manifest_path`, set to the argument and cleared by any
  normal download), so `game.launchSpec` adds `--skip-version-check`: the
  store's version check would otherwise refuse the older build as out of date.
  A pin set on an already-installed build without running a job does not leave
  a manifest path, so it still cannot launch an outdated build; fixing that
  needs a `pinned` field on `launchSpec` (none today).

## Install options (1.5): not offered

legendary exposes only `--install-tag <tag>` (selective download) and
`--with-dlcs` / `--skip-dlcs` at install time:

- Install tags come from the downloaded manifest (`list-files --json` exposes
  `install_tags` per file). They are opaque chunk groups (`chunk10`,
  `voice_de_de`, …) with no titles or sizes through the CLI; the friendly
  grouping legendary's interactive prompt shows lives in
  `legendary/utils/selective_dl.py` and a remote SDL config, and covers only
  Fortnite and Cyberpunk 2077. Presenting raw tags would not be a usable
  install dialog.
- DLC installs are separate apps (`list --json` carries `dlcs`) and legendary
  installs all owned DLC automatically because the plugin passes `-y`
  (`cli.py:1105-1124`); removing an add-on later is a separate uninstall.
  Modelling per-DLC selection would mean a multi-app job, not a flag.
- Language is a **launch** option (`launch --language`), not an install one.

So `install-options` stays off; an empty or tag-only dialog would promise more
than legendary's CLI can do.

## Live checks still needed (real account, read-only first)

1. `legendary verify` output line formats and the summary wording on a real
   install, including a large game; a check of a clean install and of one
   deliberately corrupted file.
2. `legendary repair` on that corrupted file: fresh check, repair completes,
   the next check is clean.
3. `playtime.list`: the endpoint answers the bearer from
   `legendary get-token --json --bearer`, the response shape is the array
   above, and minutes match EGL for a few games.
4. `achievements.list` on a game with known progress (e.g. a completed and a
   hidden achievement): keys, unlock timestamps and global percentages.
5. `game.relocated` after a real core move: legendary's record follows it, the
   game launches from its new folder, and no update is offered.
6. Previous versions (real account, after an update is available): install a
   game, let Epic publish/keep an update, then
   - update it, and check
     `~/.local/share/platypus/plugin-data/epic-legendary/archive/<app>/`
     holds the old build's `.manifest` and `.json` (sizes from
     `legendary info <saved manifest> --json`);
   - `versions.list` (`{"id":"<app>"}`) lists the old build as `archive`;
   - `legendary install <app> --update-only --skip-sdl --manifest
     <saved manifest> --base-url <baseUrls[0] from its sidecar>` really
     downgrades (the plugin sends exactly that), `legendary list-installed
     --json` reports the old `version` and a non-empty `manifest_path`;
   - uninstall, then install the saved build again through Platypus, and check
     the `--base-url` in the plugin log: without it legendary fails with
     `No base URLs found`;
   - launch the downgraded game: `manifest_path` makes the plugin pass
     `--skip-version-check`, so it should launch even though the store has a
     newer build. Also pin the already-installed build without running a job
     and launch: no manifest path exists then, so record whether legendary
     refuses it (needs the `pinned` field noted above).

## Live results (arco, 2026-11-17)

Logged in. Library: 256 owned games, none installed. One small owned game
(Fez, 0.40 GB download / 0.45 GB disk) installed for the verify/repair/move
checks, then uninstalled; legendary's record clean afterwards (the version
archive is kept by design). No other user files touched.

1. Verify formats: **pass**. legendary 0.21.1 prints one
   `Verification progress: N/M (P%) [X MiB/s]` line per file (25 for Fez)
   then `Verification finished successfully.`, exit 0. Plugin: clean
   check-only reports `{checked: 25, damaged: 0, missing: 0}` with phase
   `checking`.
2. Repair: **pass**. A flipped byte in a small text file reports
   `{checked: 25, damaged: 1, missing: 0}`; repair reports
   `{checked: 25, damaged: 0, missing: 0, repaired: 1}` (phases
   `checking` → `repairing`, seconds), and the file is byte-identical to
   a pre-test backup. (Check-only omits the `repaired` key; core's Go
   struct defaults it to 0, so no contract issue.)
3. Playtime: **pass**. Raw endpoint answers an array of
   `{accountId, artifactId, totalTime}` with `totalTime` an int of
   seconds (~0.3 s for the call); `floor(seconds / 60)` matches the
   plugin's minutes exactly on the top entries. 55 entries total,
   including two whose artifact id is not in the owned library (dev
   builds) — the plugin reports those too instead of dropping them.
4. Achievements: **pass**. Two games with known progress verified:
   40 total / 35 unlocked (2 hidden) and 33 total / 20 unlocked
   (5 hidden). Keys are id, name, description, hidden, icon, unlockedAt
   (unix) and globalPercent; unearned hidden entries carry the id as the
   name with an empty description. A game with no achievements answers
   total 0 / unlocked 0 / empty list. Names/descriptions come back in the
   account's language regardless of the request locale.
5. Relocated: **pass**. After a core-style move (directory moved, then
   `game.relocated`), legendary's record points at the new path (checked
   via `list-installed`) and `game.launchSpec` in preview mode builds the
   exe/cwd under the new folder with the Epic online-service args
   (`AUTH_PASSWORD` redacted); the game was not started. Note: a
   `launchSpec` sent concurrently with `game.relocated` can read the stale
   record — serialize them (relocate, wait for `{}`, then spec), as core
   does; not a plugin bug.
6. Previous versions: **skipped** — no update pending (installed build
   was the store's current one, `updateAvailable` false). Partly
   observed: install writes `archive/<app>/<build>.manifest` plus a
   sizes sidecar, `job.done.installed` carries that build as `archiveRef`,
   and the archive survives uninstall. The downgrade cycle is still
   untested on the wire.
