# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Wails v2 (Go backend + WebKitGTK 4.1 webview) with React 18, Effect-TS, TypeScript, Tailwind CSS 4, Vite.

## Users

Linux gamers with mixed-store libraries (Steam, GOG, Epic, sideloaded). By default they want a launcher that just works with minimal involvement — open, see library, click play. The same users, when a game misbehaves, want complete control of their setup: per-game Proton runner, winetricks, DXVK/VKD3D, MangoHud/gamescope, env vars, launch flags.

## Product Purpose

Hydra Launcher unifies a Linux gamer's multi-store library in one place and gives per-game control over Proton/Wine runner configuration and community fixes. Success: a user opens it and every installed game is there with sensible defaults; power options exist but never intrude.

## Positioning

Progressive disclosure of deep control: zero-config by default, full pipeline visibility (exact launch command preview, env vars, wrapper stack) for those who go looking. A neighboring store-front launcher could copy the library view, but not the per-game runner/fixes pipeline transparency.

Launcher-free installs: Hydra installs, updates and uninstalls owned games through each store's own APIs with the user's own login, without the store's official launcher. This is the approach Heroic proved, but open to any store through out-of-process plugins instead of hard-wired backends.

## Operating Context

Linux desktop (WebKitGTK webview inside Wails), dark rooms, desktop sessions (Wayland/X11). Users often have Steam running simultaneously. Gaming sessions happen at night; the launcher is glanced at briefly between sessions and stared at during troubleshooting.

## Capabilities and Constraints

- Steam library scan via VDF parsing (libraryfolders.vdf + appmanifest_*.acf) — works today
- Proton/GE-Proton/Wine runner discovery and per-game selection
- Launch pipeline: gamemoderun → gamescope → mangohud → proton/wine → executable
- Native Linux binary detection (including binaries with no extension or exec bit; the Unity player is preferred as the native entry point); installed games show their launch mode (native vs. runner)
- Fixes browser: curated fixes + ProtonDB tier data (curated data real; ProtonDB fetch exists)
- Custom game addition is currently local-state only (no backend persistence binding yet)
- Per-game settings and the Games folder persist in ~/.local/share/hydra/settings.json (sideloaded games and their settings are still session-only)
- Plugins (docs/plugins/): two kinds, both installed under ~/.local/share/hydra/plugins/<id>/
  - Store plugins (protocol.md): out-of-process executables over JSON-RPC. The user approves each one, and approval pins the whole plugin folder. Enable/disable and log in from the Plugins view; their games merge into the library (installed/owned); install, update, uninstall through a resumable download queue; launches validated and run through Hydra's runner pipeline
  - Data plugins (data-plugins.md): JSON-only fix recipes and tweak packs, no code, so always on with no approval. Hydra's own curated fixes and hardware tweaks use the same format (internal/dataplug/builtin)
- Real store plugins: Epic (plugins/epic-legendary, via legendary), GOG (plugins/gog-gogdl, via gogdl), Amazon (plugins/amazon-nile, via nile), Steam standalone (plugins/steam-standalone, QR login, Hydra's own Go depot downloader hydra-steam-dl). Third-party CLIs (legendary, gogdl, nile) must currently be installed on the system; release builds of the plugins will bundle them
- Plugin authoring: hydra-plugin scaffold (plugins/new.sh) and a conformance suite
- Non-goal: no bypassing DRM, licence checks or ownership. A plugin downloads only what the user's account owns; games with launcher-bound DRM (Denuvo, Ubisoft Connect, EA app) still need their launcher at runtime, and the plugin says so

## Brand Commitments

- Name: Hydra Launcher (user-chosen)
- Voice: technical but welcoming; no hype

## Evidence on Hand

- Real Steam library on the dev machine (43 games detected with cover art from Steam CDN)
- Curated fixes data in internal/fixes
- Runners on the dev machine: Steam Proton, GE-Proton, system Wine

## Product Principles

1. Zero-config by default, deep control on demand — progressive disclosure.
2. Never lie: the UI shows real state (real library, real runners, real fix sources), no placeholder pretending.
3. The launch pipeline is the product: make the command pipeline visible and editable, not magic.

## Accessibility & Inclusion

Keyboard navigable; legible contrast on dark theme; no motion without reduced-motion respect.