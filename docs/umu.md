# umu-launcher (issue #48)

Windows games can run through [umu-launcher](https://github.com/Open-Wine-Components/umu-launcher)
(`umu-run`) instead of a direct `proton run`.

## Survey

How Platypus launches Proton today (`internal/runner/launcher.go`): the
command is `<proton> run <exe> <args>` (plain Wine builds run as `<wine>
<exe>`), wrapped as `gamemoderun → gamescope → mangohud → runner`. The env
sets `WINEPREFIX` = `STEAM_COMPAT_DATA_PATH` = the game's prefix
(`runner.PrefixPath`: per-game pin, else `<prefixes>/<id>`), plus
`STEAM_COMPAT_CLIENT_INSTALL_PATH=~/.local/share/Steam` and the optional
EAC/BattlEye runtimes. Proton keeps the Wine prefix in `<compat>/pfx`, so
cloud saves and save backups add that `pfx` (user `steamuser`) whenever the
runner path ends in `proton`.

What umu does differently (umu 1.4.4;
[README](https://github.com/Open-Wine-Components/umu-launcher),
[`umu.1.scd`](https://github.com/Open-Wine-Components/umu-launcher/blob/main/docs/umu.1.scd),
[`umu.5.scd`](https://github.com/Open-Wine-Components/umu-launcher/blob/main/docs/umu.5.scd)):
it runs Proton inside the Steam Linux Runtime / pressure-vessel container,
exactly as Steam does, so behaviour matches Steam without Steam installed.
It applies protonfixes per game from the
[umu-database](https://github.com/Open-Wine-Components/umu-database) using
`GAMEID` (a `umu-<id>`, else `umu-default`) and `STORE` (`egs`, `gog`,
`battlenet`, `amazon`, `humble`, `itchio`, `ubisoft`). `PROTONPATH` is a
Proton folder, or a codename (`GE-Proton`, `UMU-Latest`) that auto-downloads
the build (and the runtime) into `~/.local/share/umu` on first launch.
`WINEPREFIX` is the pfx dir itself (default `$HOME/Games/umu/$GAMEID`).

What Lutris and Heroic set: Lutris finds `umu-run` on `PATH`, else under
`/app/share`, `/usr/local/share`, `/usr/share`, `/opt` or its own
`~/.local/share/lutris/runtime/umu` (`lutris/util/wine/proton.py`:
`get_umu_path`, `update_proton_env`, `get_game_id`), then sets `PROTONPATH`
(`GE-Proton` when the runner is umu itself, else the Proton folder),
`GAMEID` (the umu-database id or `umu-default`), `WINEARCH=win64` and
`PROTON_VERB=waitforexitandrun` with `WINEPREFIX` = the prefix. Heroic
likewise invokes `umu-run` with `GAMEID` (`umu-0` when unknown), `STORE`,
`PROTONPATH` and `WINEPREFIX`.

## Design (what Platypus does)

- Detection: `umu-run` on `PATH`, plus `~/.local/bin/umu-run`, the Lutris
  runtime copy (read-only) and the `/app`, `/usr/local/share`, `/usr/share`,
  `/opt` `umu` locations. When present, the runner list gains one entry
  (`umu`, shown as "Proton via umu (recommended)").
- Selection: the auto default is unchanged (still the newest GE-Proton…);
  with no explicit runner and umu installed, the game runs via umu wrapping
  that same auto Proton (`PROTONPATH` = its folder). An explicit `umu`
  choice does the same; an explicit Proton/Wine choice runs directly.
  With no Proton build at all, `PROTONPATH=UMU-Latest`: the first launch
  downloads UMU-Proton (and the runtime) — the preview shows
  `PROTONPATH=UMU-Latest`, so the download is never a surprise.
- Env: `WINEPREFIX` = the game's prefix (today's location), `GAMEID` =
  `umu-<Steam AppID>` for Steam and compat-AppID games, else `umu-0`
  (`STORE` = `gog`/`egs`/… where it applies). The umu-database is **not**
  queried at launch (no network): unknown games get `umu-0` (no fix) rather
  than a wrong one. `STEAM_COMPAT_DATA_PATH` stays the compat folder.
- Prefixes: no migration. umu's `WINEPREFIX` is the pfx dir itself, so a
  prefix that already has Proton's `<compat>/pfx` layout keeps pointing at
  it; new prefixes use the folder itself. Cloud saves, save backups and the
  wineserver stop hook use the same rule.
- Unchanged: the wrapper stack, Steam client launches (`steam://`), and the
  native-vs-direct decision.
