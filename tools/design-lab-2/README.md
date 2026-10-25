# Design lab 2

Interactive redesign explorations for the Platypus UI (Vite + React, plain CSS).

```sh
bun install
bun run dev   # http://localhost:5176
```

The index page lists every round (`/1` to `/24`). **`/22` (Atlas, top bar) is the main design**: `/21` with every feature of the launcher placed.

- top bar: Home, Library and Store; the command palette; downloads (progress ring), needs attention, friends and a menu for Activity, Runners, Plugins, Storage, Settings, Redeem and Add a game
- library bar: All / Installed, one tab per store plus Sideloaded, collections, filters, and the list / covers / table views
- game page as in `/21`: Play (or Install, Update, Retry), Tune, the copy switch with a compare button, the save state, launch notes; tabs for the store page, achievements, activity, saves and compatibility
- friends, chats and family in a side panel
- game settings in a drawer; app settings as a page
- game colour (off, accent, whole app) and the theme (dark, black, light) live in Settings > Appearance
- the footer's States button previews loading, offline, core unreachable, empty library and the after-update notice (lab only)

`/21` is the previous reference; `/23` and `/24` are the other layouts tried for the whole app. The other routes are kept for the history of decisions.

Shared code lives in `src/core`:

- `launcher.js`: state, keys and art tone sampling
- `kit.jsx`: palette, conflict resolver, settings categories and the fixes panel
- `page.jsx`: store page, screenshots and achievements
- `app.js`: app-level mock data and state (pages, collections, filters, plugins, runners, storage, friends, chat)
- `appLibrary.jsx`, `appPages.jsx`, `app.css`: library controls, game page panels, every page, social, dialogs and chrome

Store pages are a Steam snapshot in `src/core/storePages.json`. Refresh it with:

```sh
bun scripts/fetch-store-pages.mjs
```

Library data, sessions and achievement unlocks are illustrative.
