# DESIGN.md — Platypus (finalist design: /21)

> **Purpose:** self-contained implementation contract for an agent rebuilding the Wails
> frontend (`frontend/src/**`) to this design. Readable without the design-lab conversation.
> The living mockup lives in `design-lab/` (`npm run dev`; port 5175; key routes: `/21`,
> `/21-game`, `/library`; alternates `/17`–`/20`). Reference screenshots:
> `/tmp/opencode/shots/it21-wide.png`, `it21-game-v3.png`, `library-page.png`.

---

## 1. Identity & product context

Read `PRODUCT.md` for product truth. Design summary:

- **Product:** Linux game launcher unifying multi-store libraries (Steam/GOG/Epic/sideload)
  with per-game Proton config and community fixes. Zero-config by default, deep control on demand.
- **Design mode:** Operate. Brand: **"the warm control room"** — GOG Galaxy's information
  architecture and warm radial wash, + /10's tactile rounded cards, at /13's density discipline.
- **References:** GOG Galaxy (dashboard structure), Cirno downloader (cinematic dark),
  iterations /7, /10, /16–/21 in `design-lab/`.

### 1.1 World rules

1. **Warm dark ground with radial color wash.** Base `#130f0a` (warm near-black). Three
   fixed radial gradients: amber/rose (top-left), violet (top-right), teal (bottom).
   Panels are **translucent dark** (`bg-white/[0.04–0.07]`) with
   `ring-1 ring-white/[0.06–0.08]` — NOT glass-blur. **Never use `backdrop-filter`**
   (WebKitGTK paints it inconsistently; panel rgba values carry the effect alone). The
   hero-over-image stat chips may keep one `backdrop-blur` (it sits over a dark image and
   painted correctly everywhere).
2. **Single warm accent: amber** (`#fbbf24` family) for primary action, active nav,
   progress/highlight values, gold/platinum markers. NOT purple. Emerald = success/online;
   orange = attention/fix-needed. No gradients on text.
3. **Tactile rounded cards.** Radius scale: `rounded-xl` (chips/inputs), `rounded-2xl`
   (cards/plates), `rounded-3xl` (panels/heroes/banners). Depth = ring hairlines on
   translucent panels, not shadows (except modal + toast, see §6.4/§6.6).
4. **Typography.** One UI family: **Hanken Grotesk** — 800 headings (H1 23–27px, game
   title 32–36px), 700 card titles (12–13px), 400 body (12–13px), plus **JetBrains Mono**
   uppercase micro-labels 11–11.5px `tracking-[0.12–0.16em]` for metadata. **Size floor:** mono micro-labels 11–11.5px, body text at least 12px; hint sentences (empty states, notes under fields, warnings, footers) are at least 12px and at least `text-white/55`. All numeric
   readouts `tabular-nums`. **Fonts bundled locally** (no network fetch in the app).
5. **Event-type color system** (Activity, fixes): session=emerald, unlock=amber, fix=sky,
   update=neutral, download=violet, add=neutral, tweak=orange. Source of truth:
   `ActivityFeed` in `design-lab/src/pages/p6.jsx`.

---

## 2. App structure & navigation

### 2.1 App shell

```
┌──────────┬──────────────────────────────────────────────
│ sidebar  │  main content (full-wide, fluid to window)
│ w-56     │
│ sticky   │   NO max-width container on the shell.
└──────────┴──────────────────────────────────────────────
```

- Root: `flex min-h-screen` full-wide.
- **Sidebar** `w-56` sticky `h-screen`, `bg-black/25`, `border-r border-white/[0.06]`.
- Main scrolls independently.

### 2.2 Views

| View | Purpose | Trigger |
|---|---|---|
| **Dashboard** | Landing: hero + continue grid + waiting-on + activity | default |
| **Library** | Full collection: cards + table toggle | sidebar "Library" |
| **Game page** | Screenshots/summary/achievements by default; settings behind modal | click a game card |
| **Settings modal** | Launcher settings for one game | "⚙ Launcher settings" |

Secondary sidebar targets (Downloads, Achievements, Activity full log, Runners, Fix DB):
build as simple stub views with honest empty states (§7.5) — do not fabricate content.

---

## 3. Tokens (style.css)

Fonts: ship woff2 (Hanken Grotesk 400/600/700/800; JetBrains Mono 400/700) to
`frontend/src/assets/fonts/`, `@font-face` in `style.css`. No Google Fonts in the app.

```css
@theme {
  --font-sans: "Hanken Grotesk", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", system-ui, monospace;

  --color-ground: #130f0a;
  --color-accent: #fbbf24;          /* amber-400 */
  --color-accent-strong: #fcd34d;
  --color-on-accent: #130f0a;       /* text on amber */
  --color-ok: #34d399;
  --color-attention: #fb923c;
  --color-info: #7dd3fc;
  --color-dl: #a78bfa;
  --color-text: #ece6da;            /* warm white */
  --color-text-body: #a8a29e;
  --color-text-faint: #78716c;
}

body { background-color: #130f0a; color: var(--color-text); }

.app-wash {
  background-image:
    radial-gradient(1100px 620px at 14% -8%, rgba(150,96,58,.46), transparent 58%),
    radial-gradient(880px 520px at 90% 10%, rgba(86,62,128,.44), transparent 54%),
    radial-gradient(760px 460px at 52% 108%, rgba(30,78,96,.36), transparent 56%);
  background-attachment: fixed;
}
.app-wash-game {
  background-image:
    radial-gradient(1000px 560px at 88% -10%, rgba(86,62,128,.46), transparent 56%);
  background-attachment: fixed;
}
```

Apply `.app-wash` on the root div (not `body`). Game pages use `.app-wash-game`.

### 3.1 Primitives (build once in `frontend/src/components/ui.tsx`)

- **Panel:** `rounded-3xl bg-white/[0.045] ring-1 ring-white/[0.07] p-5` (hero panels `p-6`)
- **Card:** `rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07]` hover: `ring-amber-300/40` + `-translate-y-0.5` + `transition duration-300`
- **Seg chip:** `rounded-lg px-3 py-1.5 font-mono text-[12px]`; active `bg-amber-300 font-bold text-[#130f0a]`; inactive `bg-white/[0.06] text-white/55 hover:bg-white/[0.1]`
- **SegRow:** flex-wrap row of chips (all enum settings)
- **Toggle pill:** track `h-4 w-7` thumb `h-3 w-3 bg-[#191410]`; on = amber track, thumb translated (14px)
- **Mono label:** `font-mono text-[11px] uppercase tracking-[0.16em] text-white/35`
- **Tier badge** (mono uppercase `rounded-md px-2 py-0.5`):
  platinum = solid `bg-amber-300 text-[#130f0a]`; gold = `bg-amber-300/15 text-amber-300 ring-1 ring-amber-300/40`;
  silver = `bg-white/[0.06] text-white/55`; bronze = `bg-orange-400/10 text-orange-300`; borked = `bg-red-500/15 text-red-300`
- **Status dot (tweak pending):** `h-2 w-2 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,.8)]`
- **Presence dot:** `h-2.5 w-2.5 rounded-full` + inline `style={{ boxShadow: '0 0 0 2px #171310' }}`
- **Buttons:** primary `rounded-xl bg-amber-300 font-extrabold text-[#130f0a] hover:bg-amber-200`;
  Play CTA on game page `rounded-2xl px-8 py-3.5 text-[15px] shadow-[0_12px_30px_rgba(252,211,77,.25)]`;
  neutral `rounded-xl bg-white/[0.06] font-bold text-white/80 hover:bg-white/[0.1]`

### 3.2 Event-type colors

| type | text | dot |
|---|---|---|
| session | `text-emerald-300` | `bg-emerald-400` |
| unlock | `text-amber-300` | `bg-amber-400` |
| fix | `text-sky-300` | `bg-[#7dd3fc]` |
| update / add | `text-white/60` | `bg-white/30` |
| download | `text-violet-400` | `bg-[#a78bfa]` |
| tweak | `text-orange-300` | `bg-orange-400` |

---

### 3.3 Components (`frontend/src/components/ds/`)

`ds/` is shadcn/ui (Radix primitives, copied into the repo) restyled to this contract; it is the
source of truth for controls. Import via `@/components/ds/<name>`. Never use native `<select>`,
`<input type="number">` or hand-rolled field/toggle classes in surfaces. `cn()` lives in `@/lib/utils`.
`ui.tsx` keeps thin adapters (`SegRow`, `TogglePill`, `TierBadge`, `btnPrimary`, `btnNeutral`).

| Component | Variants / key props | Use when |
|---|---|---|
| `Button` | `variant`: primary (amber), neutral, ghost, link, danger; `size`: sm, md, lg, icon, none; `asChild` | Any action. One primary per view region; icon-only buttons use `size="icon"` + Tooltip. |
| `Input`, `Textarea` | `fieldClass` shared look, `h-9`, mono 12px, amber focus ring | Free text. `aria-invalid` shows a red ring. |
| `Select` (+ `SelectTrigger`, `SelectValue`, `SelectContent`, `SelectGroup`, `SelectLabel`, `SelectItem`) | Radix; `value` / `onValueChange`; groups get mono uppercase labels; list scrolls past 20rem | More than 4 options or any dynamic list (runners, resolutions, sort). Never a native select. |
| `NumberField` | `value: number \| undefined`, `onChange`, `min`, `max`, `step`, `hideSteppers`; clamps on blur, empty = unset, no native spinner, wheel ignored | Every numeric setting. |
| `Switch` | Radix; `checked` / `onCheckedChange` | Booleans as a standalone control (`TogglePill` is the visual-only variant inside clickable rows). |
| `ToggleGroup` / `ToggleGroupItem` | Radix single/multiple; amber when on | Backs `SegRow`; use `SegRow` for 2-5 fixed options. |
| `ScrollArea` | `orientation`: vertical, horizontal, both; `viewportClassName`, `viewportRef`; thumb widens and brightens on hover | Any horizontal strip and any bounded scrolling region. Native scrollbars (global CSS) are styled to match. |
| `Dialog` (+ `DialogContent label`) | Overlay `bg-black/65 z-50`, raised `rounded-3xl` panel, inner scroll region | Modals. `Modal` (props `label`, `onClose`, `className`, `dismissable`) wraps it. |
| `Badge` | `variant`: platinum, gold, silver, bronze, borked, neutral, accent, ok, info | Tier/store pills (`h-6`, mono uppercase). `TierBadge` wraps it. |
| `Tooltip` (`TooltipProvider`, `Tooltip`, `TooltipTrigger`, `TooltipContent`) | Radix, 300ms delay | Icon-only buttons; wrap the app once in `TooltipProvider`. |
| `Label`, `Separator` | mono micro-label; `bg-white/[0.07]` hairline | Form field captions; dividers. |

## 4. Sidebar spec (w-56; fixed, never scrolls)

1. **Brand:** 36px amber→rose gradient rounded-xl tile "P"; "Platypus" 15px extrabold;
   `GAME HUB` mono 11px `text-white/30`.
2. **Search:** styled input with lucide `Search` icon absolute-left; `rounded-xl bg-white/[0.06] px-3.5 py-2.5 text-[12px] placeholder-white/30`. Live-filters library.
3. **Nav** (12.5px semibold, `px-3 py-2 rounded-xl mb-0.5`): Dashboard, Library (count badge),
   Downloads (badge only >0), Achievements, Activity. Active: `bg-amber-300/12 text-amber-200`.
   Hover: `hover:bg-white/[0.05] hover:text-white`.
4. **Sources:** mono `SOURCES` label; rows Steam/GOG/Epic — dot `h-1.5 w-1.5 bg-amber-300/50`,
   count mono `text-[11px] text-white/25`. Click → Library pre-filtered.
5. **Friends panel** (pinned `mt-auto`, **`flex-none`, fixed content** — this fixes the
   "friends list bugged" squeeze bug; never grows with content):
   - Header: `FRIENDS` mono + `● 2 online` `text-emerald-300`.
   - Exactly 3 visible rows in `rounded-2xl bg-white/[0.05] p-2.5`: 32px avatar
     `rounded-lg`, presence dot bottom-right with boxShadow ring; name 11.5px bold;
     status mono 11px uppercase (in-game = `text-emerald-300/80` + game; away =
     `text-white/30` + "away · 3h"). "See all friends" link if more. Real presence data
     only — until friends/presence are real backend features, this panel is a **stub:
     render "Friends — coming soon" honest empty state** OR omit; do not fabricate.
     (The mockup shows fake friends; the app must not.)
   - Footer: `platypus v0.2 · scan 09:41` mono 11px `text-white/55`.

---

## 5. Data model contract (target schema)

Extend `internal/types/models.go` + `frontend/src/types.ts`; regenerate Wails bindings.

```ts
interface Game {
  id: string; title: string;
  store: 'steam'|'epic'|'gog'|'ubisoft'|'custom';
  store_app_id: string;
  cover_url: string;
  banner_url?: string;          // wide image for hero cards (Steam library_*_2x)
  install_dir: string; executable: string; is_installed: boolean;
  last_played?: string | null;  // relative label or ISO
  playtime_hours?: number;
  size_on_disk?: string;
  tier?: 'platinum'|'gold'|'silver'|'bronze'|'borked'|null;   // ProtonDB
  fix?: { source: string; description: string; winetricks?: string[];
          dll_overrides?: Record<string,string>; launch_args?: string } | null;
  achievements?: { total: number; unlockedCount: number; score: number;
                   list: Array<{ name: string; desc?: string; unlocked: boolean;
                                 date?: string; rarity?: string }> } | null;
  screenshots?: string[] | null;
  settings: GameSettings;
}

interface GameSettings {
  runner_id: string;
  wine_prefix: string;
  launch_arguments: string;
  env_vars: Record<string, string>;
  // proton-specific (new):
  proton_flags: string[];                    // e.g. ["PROTON_ENABLE_NVAPI"]
  sync_mode: 'esync'|'fsync'|'ntsync'|'off';
  dxvk_version: string;                      // 'DXVK 2.5.1' | 'WineD3D (no DXVK)'
  vkd3d_version: string;                     // 'VKD3D-Proton 2.14' | 'off'
  upscaler: 'native'|'FSR 3'|'NIS'|'Integer scaling';
  mangohud_preset: 'off'|'minimal'|'full'|'fps only';
  shader_cache: 'on'|'off';
  winetricks: string[];
  // legacy booleans kept and mapped: use_gamemode, use_mangohud, use_gamescope,
  // gamescope_w/h/r/args, eac_runtime, battleye_runtime, working_dir
}

type ActivityType = 'session'|'unlock'|'fix'|'update'|'download'|'add'|'tweak';
interface ActivityEvent { ts: string; type: ActivityType; game?: string;
                          detail: string; meta?: string }
```

### 5.1 New/changed Go bindings (app.go, then `wails generate module`)

- `GetRecentActivity(limit int) []ActivityEvent` — honest data only (from scans/launches);
  may return `[]`.
- `GetGameAchievements(gameID string) *Achievements` — `null` when source provides none → UI hides.
- `GetGameScreenshots(gameID string) []string` — may be `[]` → UI hides.
- `SetGameSettings(gameID string, settings GameSettings) error` /
  `GetGameSettings(gameID string) GameSettings` — JSON persistence in config dir.
- Existing: `GetGames`, `GetProtonVersions`, `PreviewLaunchCommand`, `LaunchGame`,
  `ListPlugins`, `TogglePlugin`. `PreviewLaunchCommand` must include the new settings in
  its output (flags → env vars) — currently it returns `(string, error)`; keep that shape.

Optionally split bindings into a `LauncherApi` group so `LaunchGame` can also accept
override settings for "launch once with X".

## 6. View specs

### 6.1 Dashboard (`p-7`, fluid; source: `design-lab/src/pages/p21.jsx`)

1. **Action hero** — when a fix/download needs attention (else fallback welcome banner):
   `relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]`; image
   `h-[290px] w-full object-cover`; overlay
   `linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)`;
   bottom-left: orange `ACTION REQUIRED` chip (mono 11px black uppercase), H1 27px extrabold
   "{Game} needs its fix re-applied", CTAs: amber "Open game" + neutral "Later".
   **Stat chips** `absolute bottom-6 right-7 hidden gap-3 lg:flex`: 3 ×
   `w-28 rounded-2xl bg-black/45 p-3.5` (this one may use `backdrop-blur`) —
   hours `text-amber-300`, pts `text-amber-200`, ready `text-emerald-300`; mono labels.
   **Fallback (nothing needs attention):** compact welcome banner panel:
   greeting "Good evening, {user}" + one-line status ("24 titles · 19 ready · 0 fixes") —
   then the same sections below.
2. **Continue playing** (left `1.75fr`) + right column (stacked:
   **Waiting on you** → up to 3 cards (`rounded-2xl bg-white/[0.045] p-4 ring-1 ring-white/[0.06] hover:ring-orange-400/40`: title 13px bold + tier mono orange + verdict 2-line clamp);
   below it **Recently added** panel (5 rows: 32px thumb, title 12px semibold,
   `STORE · WHEN` mono 11px uppercase)).
   ContinueGrid card: cover `aspect-[3/4]`, tweak dot, below title 12px bold truncate,
   meta mono 11px `{hours}h · {RUNNER-SHORT}`. Grid: `grid-cols-4 gap-3.5` (2 rows = 8 games).
   Hover: `-translate-y-0.5`, ring-amber-300/40. **No center overlay buttons on cards.**
   Header row: title 16px bold + `full library →` link (goes Library).
3. **Hero fix target routing:** "Open game" opens that game's page.
4. **Activity** full-width panel (`rounded-3xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]`):
   header "Activity" 15px + `FULL LOG →` mono right; 8 rows from `GetRecentActivity(8)`;
   row: ts mono 11.5px `text-white/30` tabular-nums · dot `h-1.5 rounded-full` ·
   detail 13px (type label mono uppercase 10px colored per §3.2 + text `text-white/85`) ·
   meta mono 11.5px `text-white/35`.

### 6.2 Library (`p-7`; source: `design-lab/src/pages/library.jsx`)

Header: "Library" 23px + subtitle `{n} of {total} titles · all sources verified {time}`;
controls right: filter input (`w-48 font-mono text-[11.5px] rounded-xl bg-white/[0.06]`),
source chips (ALL/STEAM/GOG/EPIC `rounded-xl bg-white/[0.05] p-1` container),
**cards/table 2-mode toggle** (default cards), amber `＋ Add game`.

**Cozy (cards) view:**
1. `LAST PLAYED` (mono section label): 3 large cards `h-[210px]`:
   wide image + `linear-gradient(0deg, rgba(19,15,10,.95) 8%, transparent 60%)`;
   bottom: title 16px extrabold + tier mono right; meta row
   `{hours}h · {last} · {RUNNER}` mono 11px `text-white/45`; **playtime progress bar**
   `h-1 rounded-full bg-white/[0.14]` fill `bg-amber-300/80` width = hours/maxHours;
   hover: play badge top-right (`h-9 w-9 rounded-full bg-black/45` grid center, opacity 0→100).
2. `ALL TITLES`: `grid-cols-5 gap-3` vertical cards (aspect 2/3 cover, tweak dot,
   title 11.5px bold + `{hours}h · {last}` mono 8px meta).
3. Bottom: Activity panel (left `1.5fr`) + **Recently added** list panel (right).

**Table view:** row grid `grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem]`
gap-3: tier dot, title with 28px cover thumb, tier, runner, hrs, size, last
(`p13.jsx` row anatomy; zebra odd rows `bg-white/[0.015]`, hover `bg-amber-300/[0.05]`).

### 6.3 Game page — default view shows NO settings (source: `shared16.jsx` GamePageShell)

1. **Hero banner** `rounded-3xl ring-1 ring-white/[0.08]`, image `h-[280px]`,
   overlay `linear-gradient(0deg, rgba(19,15,10,.98) 10%, rgba(19,15,10,.35) 62%, rgba(19,15,10,.25) 100%)`.
   Bottom-left: badges (tier-verified solid amber; store + size on `bg-black/50`),
   title 36px extrabold truncate. Bottom-right: mono stats (2 lines) + amber Play CTA.
2. **Grid `1.6fr / 1fr` gap-5:**
   - LEFT: screenshot viewer + thumb strip (`overflow-x-auto`, thumbs 150px). Clicking a
     thumb swaps the main shot. No lightbox needed.
   - RIGHT: **Summary panel** (verdict 13px `text-white/60` + mono fact rows:
     Runner / DX layer / Uptime) + full-width `⚙ Launcher settings` button (opens modal §6.4).
3. **Achievements** (always present when data; else hidden):
   header `{unlocked}/{total} · {score}%` mono right; progress `h-1.5 bg-white/[0.08]`
   amber fill; tiles grid **`repeat(auto-fill, minmax(190px, 220px))`** — define as a static
   CSS class in style.css (`.ach-grid`) to avoid Tailwind arbitrary-value flakiness:
   tile = star box 40px `rounded-xl` (unlocked: `bg-amber-300/15 ring-1 ring-amber-300/40`;
   locked: `bg-white/[0.05] opacity-30`), name 12px bold truncate, date/rarity mono 11px.
4. Optional per-game activity rows (only if `GetRecentActivity` has entries for this game).

### 6.4 Settings modal (source: shared16.jsx modal; 14's layout, centered)

- Overlay: `fixed inset-0 z-50 grid place-items-center bg-black/65 p-6`; click-outside closes; **Esc closes**.
- Panel: `w-[880px] max-h-[86vh] overflow-y-auto rounded-3xl bg-[#191410] p-7 ring-1 ring-white/10`, shadow `0 40px 100px rgba(0,0,0,.6)`. Enter animation: fade + scale .96→1, 150ms ease-out.
- Header: "Launcher settings" 20px extrabold + game title mono 11.5px + close button.
- Body `grid grid-cols-[1.5fr_1fr] gap-6`:
  - **Left:** Runner radio-cards (four, from `GetProtonVersions`; selected =
    `border-amber-300/50 bg-amber-300/[0.07]` + amber radio dot; sub-line
    `{kind} · updated {when}` mono 11px). Renderer & sync SegRows in 2-col grid:
    DXVK version / VKD3D-Proton / sync / upscaler / MangoHud preset / shader cache.
  - **Right:** Proton flags — 8 pill rows; label = flag name without `PROTON_` prefix,
    lowercase spaces; full var + description in `title` attr; on-row =
    `bg-amber-300/[0.08] ring-1 ring-amber-300/35`. Winetricks multi-select chips
    (`✓ name` when set, emerald). `$ preview` block (`bg-black/35 rounded-xl font-mono`)
    fed by live `PreviewLaunchCommand` with current draft settings — updates on any change
    (debounced 300ms). Footer: Cancel + **Save config** (calls `SetGameSettings`, closes,
    toasts "Settings saved").
- `prefix: {path}` mono 12px `text-white/55` at the modal bottom.

### 6.5 Stub views (Downloads / Achievements-global / Activity-log / Runners / Fix DB)

Real content where trivial (Runners = `GetProtonVersions` radio-card list; Activity log =
`GetRecentActivity(50)`; Downloads = games with update/download state). Otherwise honest
empty state: `rounded-3xl border border-dashed border-white/15 p-10 text-center` — mono
label + one-line explanation. **Never fabricate rows.**

### 6.6 Toast notifications

Keep existing toast: `fixed bottom-4 right-4 z-[60] rounded-[3px] border px-3 py-2 font-mono
text-[12px] bg-[#191410]` with event dot; kinds ok/emerald, err/red, busy/amber;
3.5s auto-dismiss. (Instrument readout style.) The z sits above the modal
overlay (`z-50`): a toast must stay readable over an open dialog, not lean on
DOM order.

---

## 7. Implementation plan for the agent

**Phase A — tokens & shell:** style.css tokens/fonts/wash/`.ach-grid`; full-wide shell;
Sidebar (§4); toast. Exit: app renders with wash + panels on the existing real library.

**Phase B — Dashboard + Library:** §6.1, §6.2 against real bindings.
Friends panel = honest stub until real friends feature exists.

**Phase C — Game page + Settings modal:** new `GamePage.tsx` + `SettingsModal.tsx`
(modal content replaces the old GameModal; old GameModal is retired). Settings save calls
`SetGameSettings` (Phase D) — until then, keep session-only persistence and mark the
button "applies this session" (honest).

**Phase D — data plumbing:** extend Go models + bindings per §5; persistence JSON;
activity feed from real events (launches, scans, fix applications). Wails tags:
`go build -tags "desktop,production,webkit2_41"`; regenerate wailsjs bindings; tsc clean.

**Phase E — polish & verify:** hover/focus states (amber focus rings), empty states,
reduced-motion, keyboard (Esc/close, tab order), run design detector, rebuild, launch,
screenshot at 1440+1920 widths, compare with `/tmp/opencode/shots/it21-wide.png` and
`it21-game-v3.png`, `library-page.png`.

### 7.1 Extractable code map (mockup → app)

| Mockup file | Extract into |
|---|---|
| `design-lab/src/pages/shared16.jsx` | `ui.tsx` (primitives), `Sidebar.tsx`, `GamePage.tsx`, `SettingsModal.tsx` |
| `design-lab/src/pages/p21.jsx` | `Dashboard.tsx` |
| `design-lab/src/pages/library.jsx` | `LibraryView.tsx` |
| `design-lab/src/pages/p13.jsx` | table-row anatomy for library table mode |
| `design-lab/src/pages/p6.jsx` | `ActivityFeed.tsx` + event colors |
| `design-lab/src/pages/p20.jsx` | AchievementsTab/RunsTab/FixesTab (future tabs) |

All mockup JSX is plain React + Tailwind 4 syntax; conversion = TypeScript types from
`frontend/src/types.ts` (§5), replace mock data with bindings, keep class strings.

### 7.2 Verification checklist (acceptance)

- [ ] Full-wide shell, sidebar sticky, fluid at 1440 and 1920
- [ ] `.app-wash` on root div; **no `backdrop-filter`** except hero stat chips
- [ ] Local fonts only; no network requests
- [ ] Amber-only accent; zero purple; event colors per §3.2
- [ ] Game page default = screenshots + summary + achievements; **no settings visible**
- [ ] Settings = centered modal; Esc + click-outside; live `$ preview`
- [ ] New schema fields present (`proton_flags`, `sync_mode`, `dxvk_version`, …)
- [ ] No mock data in app code; null-data sections hidden
- [ ] Friends panel fixed-height `flex-none`, no squeeze; honest stub content
- [ ] Real library renders (43 Steam titles) + honest source counts
- [ ] go build + tsc + detector clean; screenshots match references
- [ ] Reduced-motion + focus-visible amber; toasts preserved

---

**Precedence:** this doc wins over mockup code; `PRODUCT.md` principles (honesty,
zero-config default, pipeline transparency) win over this doc. When adding anything new,
match the world rules in §1.1 rather than inventing new patterns.