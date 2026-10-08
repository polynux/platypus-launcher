# Frontend review — `frontend/src`

## Summary

Reviewed `frontend/src` (App, all components, effect runtime, lib, types, style) against PRODUCT.md, DESIGN.md and `internal/types/models.go` / `frontend/wailsjs/go/models.ts`. `bunx tsc --noEmit` is clean (after `bun install`; before that it failed only on missing `node_modules`). The UI is honest and well-structured, but session/job state lives only in memory, the startup scan fails as a unit, and several effects fire non-idempotent backend RPCs (the `auth.begin` StrictMode problem is real and has siblings). The three known issues are confirmed with root causes below (FE-1, FE-2, FE-4).

## Findings (most severe first)

### FE-1 — AuthModal fires `auth.begin` twice under StrictMode (and twice on every remount)
- severity: high; category: logic bug / side effects in effects
- file:line: `frontend/src/components/PluginsView.tsx:400-404`
- What's wrong: `useEffect(() => { Effect.runPromise(LauncherApi.authBegin(plugin.id))… }, [plugin.id])` runs a **login-starting** RPC as a passive effect with no guard. In dev StrictMode mounts effects twice, so one dialog open = two `PluginAuthBegin` calls (spawns two login flows / QR codes / store-tool processes). It also refires if the modal remounts for any reason (parent re-render unmount/remount, `loggingIn` object identity change).
- Concrete scenario: open Plugins → Log in on Epic in `bun run dev` (StrictMode): backend sees two `auth.begin` for the same plugin; user gets a QR that is immediately superseded, or two legendary login attempts racing.
- Proposed fix: guard with a `started` ref per plugin id (same pattern App uses for startup scan), or lift `authBegin` into the click handler (`setLoggingIn` → explicit begin) so opening the dialog never implicitly starts login.
- Test that would catch it: render `AuthModal` under `<React.StrictMode>` with a mocked `PluginAuthBegin` counter; assert exactly 1 call.
- Effort: S

### FE-2 — A window opened after a launch never shows the running game (sessions are memory-only)
- severity: high; category: stale state / multi-window
- file:line: `frontend/src/App.tsx:54,171-202`; `frontend/src/components/Sidebar.tsx:114-132`
- What's wrong: `sessions` lives in per-webview React state and is populated only by the live `onSession` event stream. There is no hydration (no `GetSessions`/active-session query on mount), and events are not replayed. Any fresh webview (second window, reload, or navigating to a game page in a new window) starts with `{}` and shows Play instead of Playing, and the sidebar footer shows nothing.
- Concrete scenario: launch game A in window 1 (sidebar shows "playing A"); open a second app window (or reload): sidebar shows no session, game A's page shows amber Play, clicking it calls `launch` again (blocked only if `sessions[game.id]` exists in *that* window, which it doesn't).
- Proposed fix: add a backend `ActiveSessions()` query and call it in `loadData('startup')` (and on window focus); keep event stream for updates.
- Test that would catch it: mount App with pre-seeded backend running session but no prior events; assert game page shows "Playing".
- Effort: M (needs small Go binding)

### FE-3 — `loadData` fails the whole library when any one source fails (`Effect.all`)
- severity: high; category: error handling hides failures
- file:line: `frontend/src/App.tsx:80-86,114-122`
- What's wrong: startup/rescan runs `Effect.all([getLibrary, getProtonVersions, listPlugins, getGameSettings])`. One rejection (e.g. `ListStorePlugins` throws, Steam scan throws) discards the other three successes: `games` is not updated, `loadError` is set, and on first load the user gets "backend unreachable" even though the library payload arrived.
- Concrete scenario: Steam VDF unreadable → `GetLibrary` rejects → `catch` sets `loadError`; runners/plugins that were fetched fine are thrown away; dashboard shows backend-unreachable instead of library-plus-warning. Same fragility powers FE-4: the post-job rescan can fail on an unrelated call and leave the game page stale.
- Proposed fix: partition the batch — `Effect.orElseSucceed` per call (as already done for `getGameSettings`) or `Effect.all` with `{ concurrency: "unbounded", mode: "validate" }` / `Effect.partition`; always apply whatever succeeded and surface per-source warnings.
- Test that would catch it: mock `listPlugins` to reject while `getLibrary` resolves; assert games still render and plugins section shows its own error.
- Effort: S

### FE-4 — Game page still shows "installed" after uninstall until manual refresh (confirmed)
- severity: high; category: stale state after backend events
- file:line: `frontend/src/App.tsx:222-247`; `frontend/src/components/GamePage.tsx:149-172,274-276`
- What's wrong: uninstall has **no optimistic update**. The UI waits for the `jobs` done-event → `loadRef.current('rescan')` → full `loadData` round-trip before `game.is_installed` flips. Any hiccup (FE-3 failure, missed/slow event, slow plugin `Library()`) leaves the page showing Play/Manage while the game is gone. `jobFor` also hides terminal `done` jobs (returns active-or-last-failed only), so after completion there is zero trace of what just happened — no "uninstalled, refreshing…" state.
- Concrete scenario: on a plugin game page click Uninstall → confirm → "Uninstall queued" toast → job runs → done event → rescan in flight for ~1-2 s (or fails per FE-3): page still shows Play + Installation panel; user clicks Play on removed files (backend then errors) or concludes uninstall failed.
- Proposed fix: on `done` for uninstall, optimistically `setGames(prev => prev.map(g => g.id===job.game_id ? {...g, is_installed:false} : g))` immediately (and `true` on install done), keep the rescan as reconciliation; show a transient "refreshing library…" indicator on the game page while `scanning`.
- Test that would catch it: mock jobs stream emitting uninstall `done` with `GetLibrary` delayed 2 s; assert game page shows Install within a tick, before rescan resolves.
- Effort: S

### FE-5 — Rescan wipes `fix_info` and never refetches it (Waiting-on empties after any rescan)
- severity: medium; category: stale/inconsistent state
- file:line: `frontend/src/App.tsx:93-99,141-158`
- What's wrong: `loadData` replaces `games` with the raw backend list (which carries no `fix_info`; enrichment is frontend-side), but `fixFetched` already contains every id, so the enrichment effect finds `todo = []` and returns. Every rescan (including the FE-4 post-job rescan, plugin approve/disable, folder allow) silently drops all compatibility data: Dashboard hero, Waiting-on-you, Fixes view and GamePage tiers go blank for the rest of the session.
- Concrete scenario: load app (3 games get curated fixes, hero shows "X has a community fix") → install/uninstall anything or approve a plugin → rescan → hero falls back to welcome banner, Fixes shows "no fix data yet" despite data existing minutes ago.
- Proposed fix: preserve `fix_info` across rescans like settings are preserved (carry a `fixById` map in `setGames`), or delete ids from `fixFetched` when their `fix_info` is missing after a scan.
- Test that would catch it: seed games with `fix_info`, run `loadData('rescan')` with backend games lacking it, assert `fix_info` survives (or is refetched).
- Effort: S

### FE-6 — `useDetails` double-fetches `GetGameDetails` under StrictMode; cache never invalidates
- severity: medium; category: races / StrictMode / stale cache
- file:line: `frontend/src/components/GamePage.tsx:45-69`
- What's wrong: two problems in one hook. (1) No in-flight guard: StrictMode's double effect sees an empty `detailsCache` twice and fires two concurrent `GetGameDetails` (Steam/store network fetch) per game page open in dev. (2) `detailsCache` is a module-level `Map` with no invalidation: after rescan, login/logout, or reinstall, screenshots/summary/achievements/hero stay frozen for the session.
- Concrete scenario: open a Steam game page in dev → two `GetGameDetails` RPCs; then log in to a plugin (its games gain metadata) → revisit the page → old "No store description" persists because the `null`-miss isn't even cached but the stale hit is.
- Proposed fix: add an `inflight: Map<string, Promise>` guard (or `started` ref), and clear/evict `detailsCache` entries on `loadData('rescan')` (or TTL them); key on `game.id + game.version`.
- Test that would catch it: StrictMode render of `GamePage` asserting one `getGameDetails` call; then change details payload + rescan and assert the page updates.
- Effort: S

### FE-7 — Plugin `authStatus` polling effect starts plugins as a side effect, twice in dev
- severity: medium; category: StrictMode / side effects
- file:line: `frontend/src/components/PluginsView.tsx:52-59`
- What's wrong: the comment admits "Asking starts the plugin", yet `authStatus(id)` is fired from a passive `useEffect` over `readyIds`, with no StrictMode guard. Dev double-mount starts every approved auth-capable plugin twice; any remount (parent rescan → new `plugins` array → same `readyIds` string, no refire — but any id change refires all) restarts the storm.
- Concrete scenario: enable Epic plugin → Plugins view mounts → two `PluginAuthStatus('epic')` calls, two plugin process starts; logs show duplicate starts; slow machines pay it on every login-state refresh.
- Proposed fix: same `started`-per-id ref set as FE-1, or move status fetch into `refresh()` (explicit, after user action) rather than an effect.
- Test that would catch it: mount `PluginsView` under StrictMode with two ready plugins; assert one `authStatus` per id.
- Effort: S

### FE-8 — QR login polls forever and swallows all errors
- severity: medium; category: polling loop / swallowed errors
- file:line: `frontend/src/components/PluginsView.tsx:412-434`
- What's wrong: the QR `useEffect` chains `setTimeout(poll, 1500)` indefinitely while the dialog is open, and the failure branch is `.catch(() => !stop && setTimeout(poll, 3000))` — backend errors are never shown. Cleanup only clears the *first* timeout id; a poll already scheduled when the modal closes still fires once (harmless, checks `stop`, but still one stray RPC). No backoff, no attempt cap, no pause when the tab is hidden.
- Concrete scenario: store tool crashes during QR login → dialog sits at "scan…" forever, polling every 1.5–3 s, user never told the backend is failing; closing the dialog still costs one extra `authStatus` RPC.
- Proposed fix: surface consecutive failures in the dialog (`setError` after N failures), back off (1.5 s → 5 s → 15 s, cap attempts), track each scheduled id and clear it on cleanup.
- Test that would catch it: mock `authStatus` to reject 5×; assert an error appears and polling stops/escalates instead of looping silently.
- Effort: S

### FE-9 — "Clear finished" has no UI feedback (jobs list goes stale)
- severity: medium; category: stale state
- file:line: `frontend/src/App.tsx:392`; `frontend/src/components/DownloadsView.tsx:84-86`
- What's wrong: `onClear` fires `ClearFinishedJobs()` and waits for the next `jobs` event to update `jobs` state. If the backend doesn't emit on clear (or the event is slow), the finished section stays put after clicking — the button appears broken. No optimistic clear, no refetch, no busy state.
- Concrete scenario: 5 finished uninstalls → Downloads → "Clear finished" → nothing happens for seconds (or until the next unrelated job event); user clicks repeatedly.
- Proposed fix: optimistically `setJobs(prev => prev.filter(active))` on click (reconcile on next event), or `listJobs()` after clear resolves.
- Test that would catch it: mock `clearFinishedJobs` resolving with no subsequent event; assert finished rows disappear.
- Effort: S

### FE-10 — Achievements assume `list` is always an array (backend nil → null → crash)
- severity: medium; category: types out of sync / crash
- file:line: `frontend/src/types.ts:66-70`; `frontend/src/components/GamePage.tsx:509-513`; `internal/types/models.go:176-180`
- What's wrong: Go `Achievements.List []Achievement` marshals nil as `null`, but the frontend type claims `list: Achievement[]` and calls `ach.list.map` unguarded (`AchievementsSection`, `AchievementCard`). One game with `achievements: {total: 12, unlocked: -1, list: null}` crashes the whole game page. `GameDetails.screenshots/genres/developers/publishers` have the same null-vs-array mismatch (those are guarded with `?.`/`?? []`; achievements is not).
- Concrete scenario: store returns achievement counts but no cached list (documented "may be a subset") as `null` → opening that game page throws `Cannot read properties of null`.
- Proposed fix: type lists as `Achievement[] | null` (or normalize at the edge in `runtime.ts`: `list ?? []`), and guard `ach.list?.map ?? []` plus the `ach.list.length < ach.total` check.
- Test that would catch it: render `GamePage` with `achievements: {total: 5, unlocked: -1, list: null}`; assert it renders instead of throwing.
- Effort: S

### FE-11 — Modal traps nothing: Tab escapes to the background
- severity: medium; category: keyboard / accessibility
- file:line: `frontend/src/components/Modal.tsx:12-52`
- What's wrong: `Modal` moves focus in and restores it on close and handles Esc/click-outside, but never traps Tab. Keyboard users tab out of Settings/Auth/Add-game modals into the inert background (sidebar nav, game cards) while the modal stays open; focus can sit behind the overlay with no visible indicator.
- Concrete scenario: open Launcher settings → press Tab ~10× → focus leaves the modal into Library cards; screen-reader context is lost; Esc still closes but the user doesn't know where they are.
- Proposed fix: minimal focus trap (keep Tab/Shift+Tab cycling within `panel.current`), or use a trap primitive; add `aria-modal` already present — keep it honest with `inert` on background if available.
- Test that would catch it: open modal, send Tab from last focusable element, assert focus wraps to first element inside the dialog.
- Effort: S

### FE-12 — Sidebar search hijacks navigation (typing on a game page kicks you to Library)
- severity: medium; category: UI/UX flow
- file:line: `frontend/src/components/Sidebar.tsx:60-64`
- What's wrong: every keystroke in the global search forces `setView({name:'library'})` when not already there. Reading a game page (or mid-tweak) and touching search yanks the view away, losing scroll position and context; the game-page back-label chain (`from`) is also discarded.
- Concrete scenario: on a game page, click sidebar search, type "had" → view snaps to Library on "h"; pressing Back expectations break; accidental keypress while the field is focused navigates away.
- Proposed fix: don't navigate on change — navigate on Enter/Escape or only when the user explicitly picks a result; or preserve `view` and show a search-scoped library overlay.
- Test that would catch it: render game view, fire `change` on search input, assert view stays `game` until submit.
- Effort: S

### FE-13 — Toast truncates errors into uselessness (`slice(0,90)` + CSS `truncate`)
- severity: low; category: error states shown
- file:line: `frontend/src/App.tsx:114-117,239,435-447`
- What's wrong: long backend errors are sliced to 90/120 chars *and* rendered in a `max-w-md truncate` span with no `title`/expand/copy. The user sees "Library scan failed: plugin epic-legendary…" with the actionable tail cut off; Activity log keeps only an 80-char `meta` slice too.
- Concrete scenario: plugin `list` fails with "…needs login: run `legendary auth --code …`" → toast shows the prefix, the command the user needs is cut off and unselectable.
- Proposed fix: allow wrapping (`break-words`, max 3 lines) and put the full text in `title`; keep the slice for the toast but log the full error to Activity with full text.
- Test that would catch it: notify with a 200-char message; assert the full text is in the DOM (title or expanded).
- Effort: S

### FE-14 — Design drift: `backdrop-blur`, toast shape, `ach-grid` sizing, dead assets
- severity: low; category: docs-vs-code inconsistency / dead code
- file:line: `frontend/src/components/GamePage.tsx:309`; `frontend/src/components/Dashboard.tsx:63`; `frontend/src/App.tsx:435-447`; `frontend/src/style.css:118-122`; `frontend/src/assets/fonts/nunito-v16-latin-regular.woff2`
- What's wrong: (a) DESIGN.md §1.1 bans `backdrop-filter` except hero stat chips; `JobProgress` (GamePage:309) uses `backdrop-blur` over arbitrary backgrounds. (b) Toast is `rounded-xl z-[60] max-w-md` vs spec `rounded-[3px] z-50` instrument readout (§6.6). (c) `.ach-grid` is `minmax(250px,1fr)` vs spec `repeat(auto-fill,minmax(190px,220px))` (§6.3) — fewer, wider tiles than designed. (d) `src/assets/fonts/nunito-*.woff2` is unreferenced (fonts come from `@fontsource`); `size_on_disk` (types.ts:119) has no backend counterpart (`models.go` has only `SizeBytes`) yet is rendered in two views.
- Concrete scenario: none is user-breaking; collectively the build drifts from the contract a future agent will implement against.
- Proposed fix: drop `backdrop-blur` from `JobProgress` (solid `bg-black/55` already opaque enough); align toast/`ach-grid` with spec or amend DESIGN.md; delete the nunito asset; remove `size_on_disk` or document it as legacy.
- Test that would catch it: grep CI check (`backdrop-blur` allowlist, `size_on_disk` absence) + a DESIGN acceptance snapshot at 1440/1920.
- Effort: S

## Improvements (not bugs, ranked by value/effort)

1. **Partition loadData + per-section skeletons.** Value: kills FE-3/FE-4 classes at the root; library renders even when plugins/runners fail. Effort: S.
2. **Optimistic job states + "refreshing…" affordance on game/library pages.** Value: uninstall/install feels instant and honest. Effort: S.
3. **Hoist `authBegin` into the Log-in click and add explicit `ActiveSessions()` hydration.** Value: ends the whole StrictMode-login family and the second-window blindness. Effort: M.
4. **Normalize backend payloads at the edge (`runtime.ts`).** Coerce `null` lists to `[]`, default `warnings`/`jobs`/`plugins`, strip/derive frontend-only settings fields in one place instead of every consumer. Value: removes FE-10-class crashes permanently. Effort: S.
5. **Virtualize or paginate Library table/cards past ~200 rows + `useMemo` the filters.** Value: plugin libraries will outgrow the current full-render; cheap insurance now. Effort: M.
6. **Unify wording: "Launcher settings" (per-game modal) vs "Settings" (global view) vs "Suggested tweaks" vs "Fixes".** Rename global view to "Hydra settings" and keep "Fixes" for curated/ProtonDB only. Value: ends the most common confusion in the IA. Effort: S.

## Checked and fine

- Startup scan guard (`started` ref) correctly prevents double `loadData` under StrictMode — App.tsx:127-137.
- Settings preview debounce (300 ms) with cancel flag and cleanup — SettingsModal.tsx:53-62.
- `toBackendGame`/`normalizeSettings`/`derivedEnv` round-trip (managed keys stripped and re-derived; `use_mangohud` derived from preset) — lib/settings.ts:50-120.
- Steam-client guard (env never reaches game; launch-options string + copy) and native-build guard (runner/Proton UI hidden) — SettingsModal.tsx:46-49,85-98; GamePage.tsx:94-104,281-282.
- Outside-folder allow flow with rescan — App.tsx:260-267; GamePage.tsx:177-190.
- Honest empty states everywhere (no fabricated friends/activity/runners) + pinned fixed footer — Sidebar.tsx:113-136; Dashboard.tsx:27-36,104-107.
- Keyboard roles for radiogroups/switches/progressbars, `aria-pressed`/`aria-checked`, amber `:focus-visible` ring, `prefers-reduced-motion` — ui.tsx, Modal.tsx, style.css:84-98.
- Cover fallback chain (`hero_url → banner_url → cover_url`, per-list failure index, cached-404 check) — ui.tsx:149-171; lib/games.ts:22-28.
- `tsc --noEmit` clean; Effect `call()` wrapper fails loudly outside Wails instead of faking data — effect/runtime.ts:39-54.
- Wording of destructive uninstall (two-step "Delete game files?") and honest "Can't install here" state — GamePage.tsx:329-347,391-402.

