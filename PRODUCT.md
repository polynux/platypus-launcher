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

## Operating Context

Linux desktop (WebKitGTK webview inside Wails), dark rooms, desktop sessions (Wayland/X11). Usersoften have Steam running simultaneously. Gaming sessions happen at night; the launcher is glanced at briefly between sessions and stared at during troubleshooting.

## Capabilities and Constraints

- Steam library scan via VDF parsing (libraryfolders.vdf + appmanifest_*.acf) — works today
- Proton/GE-Proton/Wine runner discovery and per-game selection
- Launch pipeline: gamemoderun → gamescope → mangohud → proton/wine → executable
- Fixes browser: curated fixes + ProtonDB tier data (curated data real; ProtonDB fetch exists)
- Plugin registry: in-memory defaults; toggling has no runtime effect yet (known placeholder)
- Custom game addition is currently local-state only (no backend persistence binding yet)
- No persistence of per-game settings beyond session yet (backend stores nothing)

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