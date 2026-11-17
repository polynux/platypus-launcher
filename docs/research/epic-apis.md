# Epic Games Store: verify, move, playtime and achievements

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
  stderr and also exit 0, so a run without a summary is a failure.
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
- No timestamp for the last session is in the response, so `lastPlayed` is
  omitted.

## Achievements (`achievements.list`)

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
