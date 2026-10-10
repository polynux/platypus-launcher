# UI Critique — Hydra Launcher frontend (2026-10-08)

> **Handoff document.** Written for an AI agent (or human) that will implement the fixes.
> It is self-contained: context, per-surface findings with exact file/line anchors, suggested
> fixes, acceptance criteria, and a verification checklist.
>
> **Scope:** Dashboard, Library, Plugins, Game page, Settings (per-game modal + launcher view).
> **Mode:** Operate (task-completion UI; scanability, state honesty, native expectations).
> **Basis:** code review of `frontend/src/**` against `DESIGN.md` (the "warm control room"
> contract) and `PRODUCT.md`. The Wails app could not be run headless during this critique,
> so no screenshot pass was made — see §7.

---

## 1. Context you need before editing

Read both of these first; they override anything here:

- `PRODUCT.md` — product truth. Three principles drive most findings below:
  1. Zero-config by default, deep control on demand (progressive disclosure).
  2. **Never lie**: the UI shows real state — no placeholder pretending, no redundant fake stats.
  3. The launch pipeline is the product: visible and editable, not magic.
- `DESIGN.md` — the design contract. Key rules the fixes must respect:
  - Warm dark ground `#130f0a`, radial wash, translucent panels (`bg-white/[0.045]` +
    `ring-1 ring-white/[0.07]`), **no `backdrop-filter`** except the dashboard hero stat chips.
  - Single amber accent (`#fbbf24`); emerald = success, orange = attention, sky = fix info.
  - Primitives live in `frontend/src/components/ui.tsx` (`Panel`, `cardClass`, `SegRow`,
    `TogglePill`, `TierBadge`, `EmptyState`, `btnPrimary`, `btnNeutral`, `Cover`).
  - Modals share `frontend/src/components/Modal.tsx` (Esc + click-outside close, focus-in on
    open, focus restore on close).
  - Global `:focus-visible` amber ring and `prefers-reduced-motion` kill-switch exist in
    `frontend/src/style.css` (lines ~84–95). Do not regress these.

**Stack:** Wails v2 (Go backend + WebKitGTK webview), React 18, TypeScript, Tailwind 4,
Effect-TS for binding calls (`LauncherApi.*` via `frontend/src/effect/runtime.ts`).
Verify with `tsc` in `frontend/` and, for Go changes, `go build -tags "desktop,production,webkit2_41"`.

---

## 2. Global floor — currently passing, do not regress

- Amber-only accent; event-type colors match DESIGN.md §3.2 (`ui.tsx` `EVENT_TEXT`/`EVENT_DOT`).
- No fabricated data anywhere; every empty state tells the truth and usually offers a next step.
- Keyboard focus ring (amber, `:focus-visible`) global; reduced-motion respected.
- Cover art has a typographic fallback when the CDN has no art (`Cover` in `ui.tsx`).
- Modals restore focus to the opener on close.

---

## 3. Findings by surface

Severity: **P1** = fix before shipping (costs users real trust or blocks a core flow).
**P2** = quality/polish, fix in the same area when convenient.
IDs are stable (`D#` dashboard, `L#` library, `P#` plugins, `G#` game page, `S#` settings).

### 3.1 Dashboard — `frontend/src/components/Dashboard.tsx` — 7.5/10

**D1 · P1 · "ready" stat is always redundant (displayed state ≠ meaning).**
`App.tsx` passes only `installedGames` to `<Dashboard games={...}>`, and line 25 computes
`ready = games.filter((g) => g.is_installed).length` from that already-filtered array —
so `ready === games.length` unconditionally. The welcome banner (line 157) always reads
e.g. "43 titles · 43 ready", and the hero stat chips (lines ~55–70) show the same number twice
(`titles` vs `ready 43/43`). Dead data dressed as insight; violates "never lie" in spirit.

*Fix:* pass library-wide counts from `App.tsx` (it has `games`, not just `installedGames`) —
e.g. add props `totalOwned` and `installedCount`, or compute the chips in App. Only claim
"ready" when it can differ from "titles". Alternatively drop the `ready` chip entirely.

*Accept:* with a mixed installed/owned library, the banner never shows "N titles · N ready".

**D2 · P1 · "Recently added" panel is a permanent apology.**
Line 137: the panel only ever lists sideloaded (`store === 'custom'`) games; for virtually
every user it renders the confession "Store install dates aren't tracked yet. Games you
sideload appear here." An empty panel with an excuse is negative hierarchy on the landing page.

*Fix:* when `recentlyAdded.length === 0`, hide the panel (conditional render), not apologize.
Optionally back it with real data later (scan discovery order) — but hiding is the correct
minimal fix and fully honest.

*Accept:* a user with no sideloaded games sees no "Recently added" section at all.

**D2b · P2 · same empty-apology panel exists in `LibraryView.tsx` line 210** — apply the same
hide-when-empty treatment there.

**D3 · P2 · "Later" dismiss nag-loops through waiting games.**
`dismissedHero` holds one game id; dismissing the hero reveals the next `needsAttention`
game. With several pending fixes the hero never goes away. Add a session-wide dismissal
option ("Ignore all") or frame it as "1 of N" so the loop is legible.

**D4 · P2 · "Continue playing" vs "Your games" order.**
When no game has `last_played_at`, the grid is relabeled "Your games" (line ~85) but the
first 8 are still `[...games].sort(byRecent)` — effectively arbitrary. Acceptable, but a
stable sort key (title) would make the default view deterministic across rescans.

### 3.2 Library — `frontend/src/components/LibraryView.tsx` — 8/10

**Strengths (keep):** four differentiated empty-state messages (lines ~110–120); ownership
segmented control only rendered when `owned > installedCount` (progressive disclosure); job
progress overlaid on uninstalled cards; warnings banner with a direct "Log in"/"Plugins" CTA;
honest uninstalled-card treatment (`opacity-45 grayscale` + badge).

**L1 · P1 · Zero-count source chips produce one-click empty states.**
Line 35: `SOURCES = ['all', 'steam', 'gog', 'epic']` is hardcoded, so a store with zero
games still gets a chip that filters the library to nothing.

*Fix:* build the chip list from `p.games` — start with `'all'`, include a store only if it
has ≥1 game (keep the dynamic extension logic on line ~54 that already appends unknown
stores). Preserve chip order: all, steam, gog, epic, then extras.

*Accept:* no chip exists that yields an empty library while games are shown under "all".

**L2 · P2 · Table mode has no sorting.** Columns hrs/size/last played invite sorting; it's
currently a fixed snapshot. Minimum: clickable title + hrs sort. Also note the table is a
`div` grid without header-row semantics — add `role="table"/"row"/"columnheader"` (or a real
`<table>`) when touching it.

**L3 · P2 · Filter input has no clear affordance,** yet the empty state instructs "Clear the
search". Add an ✕ button inside the input (the input is at lines ~75–82).

**L4 · P2 · Three amber primaries compete** in one screen: `＋ Add game` (header), the
warnings-banner CTA, and amber hover states on cards. Demote `Add game` to `btnNeutral`, or
accept and move on — flagging for hierarchy awareness.

### 3.3 Plugins — `frontend/src/components/PluginsView.tsx` — 8.5/10

**Strengths (keep — this is the product's best moment):** `ApproveModal` (lines ~490–560)
shows `runs` path, `sha-256`, capabilities, network hosts, read/write paths, missing tools,
plus the honest "Hydra doesn't enforce them yet" disclosure — exactly PRODUCT.md's
positioning. QR login redraws when the code expires (`QrArt` + polling effect). Missing-
requirements boxes give real install hints. Data-plugin "no code, always on" framing.

**P1 · P1 · No focus trap in modals** — root cause is shared `Modal.tsx` (lines 19–31): it
moves focus in and handles Esc, but Tab escapes to background controls. Worst case is
`ApproveModal` (a security decision) where a keyboard user can activate invisible background
buttons.

*Fix (one place covers all modals):* in `Modal.tsx`, on open, collect focusable elements
inside the panel and wrap Tab at both ends (a `keydown` handler checking
`e.key === 'Tab'`); or render a first/last sentinel and loop. Keep Esc + click-outside +
focus-restore behavior unchanged.

*Accept:* with a modal open, Tab cycles only within the modal on every element.

**P2 · P2 · QR submit copy.** Line ~448: `canSubmit = (prompt?.qr && !filled) || filled` —
with a QR prompt the button labeled "Log in" actually means "I scanned it — confirm".
When `prompt.qr && !filled`, label the submit "I scanned it" (keep "Log in" for field flows).

**P2 · P2 · Stray duplicate JSDoc.** Line 361: `/** auth.begin → … */` sits directly above
the `QrArt` doc comment; delete the stray one.

### 3.4 Game page — `frontend/src/components/GamePage.tsx` — 8/10

**Strengths (keep):** strongest state honesty in the codebase — outside-folder alert with a
one-tap "Allow this folder" fix (lines ~180–195); runner shows "(missing)" when the saved
runner isn't installed; achievements explain *why* progress isn't cached; native-build
awareness strips Proton-only tweaks; install progress replaces the hero CTA.

**G1 · P1 · No way to stop a running game.**
Lines 111/154: when `state === 'running'` the Play button becomes a *disabled* emerald
"Playing". The launcher's core action dead-ends — the user must alt-tab to kill the game.

*Fix:* check the backend first — if there is no stop/kill binding, this needs a small Go
addition (a `StopGame(gameId)` that terminates the tracked session/pid; the session watcher
in `internal/` already knows the pid). Frontend: when `state === 'running'`, render a second,
neutral "Stop" button beside the emerald "Playing" indicator (or make the emerald button
itself the stop control with a confirm-less stop — a launcher convention). Show toast +
activity event on stop, exactly like the existing session events.

*Accept:* a running game can be stopped from its game page; the session event appears in
Activity.

**G2 · P2 · Padding breaks the world rhythm.** Line 121: `px-10 py-6` — every other view
uses `p-7`. Change to `p-7`.

**G3 · P2 · `backdrop-blur` violates the world rule.** Line 309 (`JobProgress`): DESIGN.md
§1.1 bans `backdrop-filter` everywhere except the dashboard hero stat chips (WebKitGTK paints
it inconsistently). The chip already sits over a dark image — replace `backdrop-blur` with a
more opaque panel (`bg-black/70`).

**G4 · P2 · Uninstall confirm keyboard inconsistency.** Line ~400: the two-step confirm
resets `onBlur` but not on Escape, and the second click fires immediately with no timeout.
Add `onKeyDown` Escape → `setConfirm(false)` to match the modal convention above it.

### 3.5 Settings (per-game modal) — `frontend/src/components/SettingsModal.tsx` — 7.5/10

**Strengths (keep):** the live `$ preview` fed by the real backend (debounced 300ms, lines
~50–60) *is* PRODUCT.md's "the launch pipeline is the product" delivered. Steam launch-
options honesty box for Steam-client games. Winetricks list merged from the game's fix
recipe. Native-build sections correctly hidden.

**S1 · P1 · No dirty guard — Esc/overlay click silently discards the draft.**
The modal is the densest form in the app (runner, DX layer, sync, upscaler, flags, winetricks,
env vars, launch args) and `Modal.tsx` closes on Esc *and* any overlay mousedown with no
confirmation. One stray click loses every edit.

*Fix:* track dirty (`draft`/`envText` compared against the normalized initial state). When
dirty, intercept close: either keep it simple (Esc/click-outside ignored while dirty; the ✕
and Cancel buttons still close, Cancel explicitly labeled) or show a tiny inline
"discard edits?" confirm. Match the app's toast idiom; don't add a second modal.

*Accept:* dirty state survives Esc and overlay clicks; explicit Save/Cancel/✕ still work.

**S2 · P1 · Displayed state ≠ saved state for mapped enum values.**
Line 140: `value={draft.sync_mode === 'default' ? 'fsync' : draft.sync_mode!}` and line 141:
`upscaler === 'integer' ? 'native' : ...`. The UI *shows* fsync/native while `current` (and
thus Save, line 243) persists `default`/`integer`. The user sees one state, saves another —
a direct "never lie" violation at the exact spot power users scrutinize.

*Fix options (pick one, keep the schema stable):*
a) Normalize on open: when initializing `draft`, map `'default'→'fsync'` and
   `'integer'→'native'` so display and save always agree (confirm with the backend that
   `fsync`/`native` are valid stored values).
b) Or render the true value as an extra "default" chip in `SYNC_MODES`/`UPSCALERS`.
Either way, delete the display-only mapping.

*Accept:* what the SegRow highlights is exactly what Save persists (verify by save → reopen).

**S3 · P2 · `parseEnv` silently drops malformed lines.** Line 29: a line without `KEY=`
vanishes — the `$ preview` then disagrees with what the user typed, with no feedback.
Validate: flag malformed lines in red under the textarea (or best-effort include them in the
preview with an error badge).

**S4 · P2 · Gamescope inputs are cryptic.** Line 169: `placeholder={k.slice(-1)}` renders
"w", "h", "r". Use real placeholders ("width", "height", "refresh") — the `aria-label`s are
already correct, so this is display-only.

### 3.6 Launcher settings — `frontend/src/components/LauncherSettingsView.tsx` — 7.5/10

Clean: dirty-tracked Save, error display with `aria-invalid`, reset-to-default link.

**LS1 · P2 · Stale copy.** Line 66: "Nothing installs here until store plugins arrive" is
false the moment a plugin is installed — false exactly when the reader cares. Reword:
"Store plugins install here, each in its own subfolder." (The example path on the line above
already conveys the rest.)

---

## 4. Prioritized fix order

Grouped so related edits land together; each group is independently shippable.

1. **Settings-modal integrity** (S1, S2) — highest user cost, both cheap, both in
   `SettingsModal.tsx` (+ tiny `Modal.tsx` hook for S1 if implemented via Esc interception).
2. **Modal focus trap** (P1) — one fix in `Modal.tsx` covers Approve/ Auth / Settings / AddGame.
3. **Game page stop control** (G1) — needs a backend check first (see §6); the only finding
   that may require Go work.
4. **Dashboard truth pass** (D1, D2, D2b) — `App.tsx` prop change + two conditional renders.
5. **Library source chips** (L1) — trivial.
6. **P2 tail:** G2, G3, G4, P2(QR copy), P2(stray JSDoc), S3, S4, L2, L3, L4, D3, D4, LS1.

## 5. Do NOT touch (already exceeds the craft floor)

- `ApproveModal` disclosure design (runs/sha/capabilities/paths + honesty note).
- The live `$ preview` pipeline in SettingsModal.
- Library's differentiated empty states and the warnings banner CTA.
- Achievements' "why progress isn't cached" copy paths.
- Event-color system (`ui.tsx`), Cover fallback, toast idiom, global focus/reduced-motion CSS.
- DESIGN.md world rules (wash, translucent panels, amber-only accent) — fixes above must
  stay inside them.

## 6. Verification checklist

- `cd frontend && npx tsc --noEmit` (and the project's lint) after every group.
- For G1: inspect `app.go` / `app_jobs.go` / session watcher for an existing stop path
  **before** adding Go code; regenerate Wails bindings (`wails generate module`) if the
  binding surface changes; `go build -tags "desktop,production,webkit2_41"`.
- Behavioral checks per finding are in each *Accept* line above.
- Visual pass (not yet done — see §7): run the app, screenshot Dashboard/Library/Game page/
  Plugins/Settings modal at 1440 and 1920 widths; compare against `DESIGN.md` reference shots
  (`/tmp/opencode/shots/it21-wide.png`, `it21-game-v3.png`, `library-page.png`). One bounded
  round: fix everything the screenshots show, one confirmation pass, stop.

## 7. Limitations of this critique

- No runtime/screenshot verification: the Wails app cannot run headless in the critique
  environment. All findings are from reading code against DESIGN.md/PRODUCT.md; line numbers
  are from the current `main` and may drift slightly before implementation — the identifiers
  and quoted strings are the stable anchors.
- G1's fix shape depends on backend capability (unknown at critique time) — check §6 before
  writing frontend code.
- Scores (Dashboard 7.5, Library 8, Plugins 8.5, Game page 8, Settings 7.5) are heuristic,
  Operate-weighted (state honesty and task completion over expression). They exist to rank
  surfaces, not to grade against an external standard.
