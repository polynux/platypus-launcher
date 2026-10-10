# Data plugins (fixes and tweaks)

A data plugin is a folder in `~/.local/share/platypus/plugins/<id>/` whose
`plugin.json` says `"kind": "data"` and has no executable. It is JSON only: fix
recipes and tweak packs. It can't run anything, so Platypus applies it without
asking the user to approve anything — the Plugins view lists it under
"data plugins · no code, always on".

Platypus's own curated fixes and hardware tweaks are the same format, embedded in
the launcher (`internal/dataplug/builtin`) and read through the same loader.

## Manifest

```json
{
  "id": "community-fixes",
  "name": "Community fixes",
  "version": "1.0.0",
  "author": "you",
  "description": "One sentence for the Plugins view.",
  "api": 1,
  "kind": "data",
  "data": ["fixes.json", "tweaks.json"]
}
```

- `id` must match the folder name; `steam` and `custom` are reserved.
- No `exec`, no `store`, no `capabilities`, no `requires` — those belong to
  store plugins (see [protocol.md](protocol.md)).
- `data` lists the JSON files, relative to the folder. Default: `fixes.json`
  and `tweaks.json`, missing ones skipped.

Everything is validated; a file with an unknown field or a bad value is
rejected and the plugin shows as invalid, with the reason, in the Plugins view.

## fixes.json

```json
{
  "fixes": [
    {
      "title": "Fallout: London",
      "source": "lutris",
      "tier": "gold",
      "description": "Apply missing GOG post-install finishing steps…",
      "winetricks": ["faudio", "xact"],
      "dllOverrides": { "xaudio2_7": "n,b" },
      "launchArgs": ""
    }
  ]
}
```

A fix matches a game by exact `title` or by `appId` (a Steam app id or a
plugin game's store id); one of the two is required. The first match in load
order wins — Platypus's built-in recipes come first, so a plugin can't override
one, but it can add titles the built-ins don't know. `tier` is one of
`platinum gold silver bronze borked`; `source` says where the recipe came from
and is shown as a badge on the game page.

## tweaks.json

```json
{
  "tweaks": [
    {
      "id": "nvidia-nvapi",
      "kind": "recommended",
      "title": "Enable NVAPI",
      "reason": "NVIDIA GPU detected. Exposes NVAPI so DLSS, Reflex and some vendor checks work.",
      "when": { "gpu": ["nvidia"] },
      "env": { "PROTON_ENABLE_NVAPI": "1" }
    }
  ]
}
```

A tweak is offered on the game page when every condition in `when` holds:

| Field      | Meaning                                        |
|------------|------------------------------------------------|
| `gpu`      | any of these vendors is present (`nvidia`, `amd`, `intel`) |
| `ntsync`   | `/dev/ntsync` exists                           |
| `gamemode` | `gamemoderun` is installed                     |
| `mangohud` | `mangohud` is installed                        |
| `wayland`  | the session is Wayland                         |

An empty `when` applies to every machine. `kind` is `recommended` (safe to
enable by default) or `troubleshoot` (fixes specific failures, costs
something). The tweak itself is `env` vars, `wrappers` (`gamemode`,
`mangohud`), or both — the same shapes a store plugin's launch spec may set.

`id` must be unique; an id already provided by the built-ins or another plugin
is ignored, so a plugin can't change what a built-in tweak does.

Tweaks are offered for every game, platinum ratings included: a rating says
the game works for most people, not that it works on this GPU or session.
