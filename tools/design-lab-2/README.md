# Design lab 2

Interactive redesign explorations for the Platypus UI (Vite + React, plain CSS).

```sh
bun install
bun run dev   # http://localhost:5176
```

The index page lists every round (`/1` to `/21`). **`/21` (Atlas final) is the reference design**:

- top bar with the command palette and the colour mode switch (game colour as accent, or across the whole app)
- source bar for library views and stores
- game list on the left
- game page with Play, Tune (quick settings), the store switch and the save state
- store page tabs: the description on the left; pictures, play stats, achievements and details on the right
- all settings in a drawer

The other routes are kept for the history of decisions.

Shared code lives in `src/core`:

- `launcher.js`: state, keys and art tone sampling
- `kit.jsx`: palette, conflict resolver, settings categories and the fixes panel
- `page.jsx`: store page, screenshots and achievements

Store pages are a Steam snapshot in `src/core/storePages.json`. Refresh it with:

```sh
bun scripts/fetch-store-pages.mjs
```

Library data, sessions and achievement unlocks are illustrative.
