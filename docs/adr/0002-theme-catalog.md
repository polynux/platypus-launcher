# ADR 0002: themes choose from a versioned catalog the app implements

- Status: accepted (2026-10-24)
- Issues: #90 (epic), #91 (inventory), #92 (this ADR), #93 (runtime tokens), #94 (semantic tokens), #95 (Settings testbed), #96 (Dashboard and Library layouts), #89 (bridge trust boundary)
- Input: `docs/themes/inventory.md`

## Context

The maintainer designs several versions of the launcher and wants them to coexist as themes. A theme should be able to change more than colours: how parts of the UI are presented and behave, and how they are laid out. A theme may also support only some of that.

Today none of it is possible without forking components:

- `style.css` declares colour tokens, but no component uses them. Components hard-code about 1,100 colour classes.
- Views are hand-written JSX. The inventory found eight hand-written view headers, four section patterns, three hero forms and four progress shapes.
- 33 of 63 components call the core themselves, so a block can't move without its data.

Two constraints shape the answer:

- **Accessibility and coherence stay with the app.** Focus, ARIA, keyboard behaviour, i18n and the product rule "never lie" (`PRODUCT.md:61`) can't be left to each theme.
- **No theme code in the page.** The page can call every core method through `window.platypus.invoke` (#89). Any script a theme ran there would have the user's full authority over the core.

Prior art points the same way. Google A2UI: "agents can only use pre-approved components from your catalog". Remote Flutter Widgets: remote UI "bottoms out" in widgets compiled into the app. Adaptive Cards: `fallback` and `requires`. Playnite: named slots. ES-DE: data-only XML themes. All of them keep the components in the host and let the remote side choose and arrange.

## Decision (proposed)

### 1. The catalog is the frontend contract

The app owns a **catalog**: named entries, each with typed props, accessibility built in, and **variant axes**. A theme never brings a component. It chooses a value on each axis, and arranges blocks in layouts.

There are four kinds of entry:

| Kind | What it is | Examples (from the inventory, §6.1) |
|---|---|---|
| **Component** | A reusable piece with variant axes | `GameTile` (aspect, compat footer, job overlay), `GameRow` (column set, compact/full), `Hero` (height, stat chips, action set), `JobProgress` (compact/row/overlay), `Badge`, `Button`, `EmptyState`, `ListRow`, `ActivityFeed` |
| **Container** | Presents a group of content | `Dialog` (size; presentation `dialog` / `inline` / `sheet`; dismissable), `Tabs` (vertical / horizontal), `Section` (framed / hairline / plain), `ViewHeader`, `Page` |
| **Block** | A named part of a view that a layout can place | `dashboard.continue`, `library.last-played`, `game.compat`, `settings.language`, … (inventory §2) |
| **Region** | A place in a view where blocks go | `dashboard.main`, `dashboard.side`, `game.hero`, `game.body`, `shell.sidebar`, … |

Behaviour belongs to the variant, and the app implements it. A `sheet` traps focus, closes on Escape and slides in from an edge; `inline` renders in place and adds no focus trap; `page` adds a back entry. A theme picks `sheet`; it never says how a sheet behaves.

The catalog is described once, in Go (`internal/themes/catalog.go`), as data: entry names, axes, allowed values, defaults, the `since` version, and whether a block is required. The core validates themes against it, and `cmd/gen-ts` exports it to the UI. The UI and the core can't disagree on names.

### 2. Levels, each optional, each with a fallback

| Level | A theme may | Fallback |
|---|---|---|
| `tokens` | Set token-set-v1 values: colours, fonts, radii, the background wash (#90) | Built-in value, per token |
| `variants` | Choose a value on any catalog axis, globally or per use site (e.g. `dialog.presentation = "sheet"` but `game-settings.presentation = "dialog"`) | Built-in value, per axis |
| `layouts` | Per view: order blocks, group them (rows, columns), put them in regions, hide optional blocks | Built-in layout, per view |

A theme declares the levels it supports (`supports = ["tokens", "variants"]`). Anything it doesn't mention falls back, so a colours-only theme and a full-layout theme use the same mechanism. The picker shows each theme's levels.

Interaction modes (controller, "big picture") are not a theme level. They are a built-in mode that themes can style.

### 3. Formats

- **`theme.toml`**: metadata, levels, tokens, variant choices. Flat key/values, which TOML handles well.
- **`layouts/<view>.xml`**: one file per view. A layout is a tree (regions, rows, blocks), which XML handles well. The core ships an XSD generated from the catalog, so editors can autocomplete and validate it.

```toml
[theme]
id = "noir"
name = "Noir"
schema = 1
catalog = "1.0"
supports = ["tokens", "variants", "layouts"]

[tokens]
accent = "#7dd3fc"
ground = "#0b0d12"

[variants]
"dialog.presentation" = "sheet"
"game-settings.presentation" = "dialog"
"game-settings.tabs" = "horizontal"
"game-tile.aspect" = "3:4"
```

```xml
<layout view="dashboard" catalog="1.0">
  <region name="main">
    <block name="hero" />
    <row>
      <block name="continue" />
      <block name="waiting" />
    </row>
  </region>
  <region name="side">
    <block name="activity" limit="6" />
  </region>
</layout>
```

Only the core parses these files. It sends the UI typed, validated structs over RPC, and the UI never reads theme files.

### 4. Data stays with the app, so blocks can move

A block receives its data as props from its view's data owner. It never calls the core itself. This is the prerequisite for layouts: a block that fetches its own data (game page hooks, Storage, Plugins, Settings sections, inventory §6.4) is split into a data hook owned by the view and a presenter that takes props. The split happens view by view, when that view joins the catalog, not as a big-bang refactor.

### 5. Required parts can't be hidden

A layout may hide only optional blocks. The catalog marks some blocks **required** (inventory §6.3):

- the Settings and Plugins entries;
- the plugin warnings in the library and the plugin login row;
- on the game page: Install/Play/Stop, Update, the back link, the progress of a running job (install, verify, move) and the outside-folder alert;
- toasts.

The rule behind the list: a block is required when it lets the user act on the game, or when it tells the truth about the game's state. Advice and decoration are optional: media, about, achievements, activity, notes, crash hints. Collections and copies are optional on the page, but their actions stay reachable from game settings.

A layout that leaves a required block out still gets it, in its built-in region, plus a warning. A capability surface (`CapabilitySurface`) that a layout hides would make a working feature invisible, so capability surfaces are required wherever their capability is offered.

### 6. Versioning

The catalog has a version, `major.minor`, following the plugin protocol's rules (`docs/plugins/protocol.md`, "Versioning and compatibility"):

- **Minor = additive.** New entries, axes, values, blocks and regions. Every entry carries `since`.
- **Major = breaking.** A removal or rename. Platypus keeps reading the previous major for a deprecation period, and maps old names to new ones where it can.
- **A theme declares the catalog version it targets** (`catalog = "1.0"`). Platypus also has a theme file `schema` version.
- **Unknown names are ignored with a warning**, never an error: an axis, value, block or region from a newer catalog. A theme that needs a newer major, or a newer `schema`, is listed as "needs a newer Platypus" and can't be selected.

Warnings show in the theme list. A theme never breaks the app; at worst it falls back to the built-in choice.

### 7. Guarantees in tests

- **Fixture render**: every component and container is rendered in every variant with fixture data. The render runs `axe-core` (through `vitest-axe`) for roles, names, labels and focus order, plus a Testing Library test per variant for behaviour: Escape closes, focus is trapped and restored, arrow keys move between tabs.
- **Contrast is checked on tokens, in the core.** jsdom can't compute colours, so the Go theme validator checks contrast on the token values: text on ground at least 4.5:1, muted and faint text at least 3:1, the focus ring on ground at least 3:1. A failing token falls back to the built-in value with a warning. This runs for every theme, not only in our tests.
- **Catalog guard**: a test fails when an entry, axis or value is removed or renamed without a major bump. It compares against a committed snapshot of the catalog.
- **Theme validation**: every built-in theme parses with zero warnings.
- **e2e pins the built-in theme**, so e2e results don't depend on the user's choice.

### 8. Trust

Themes are data only: no JS, no CSS text, no `url()`, no remote assets. Images and fonts are files inside the theme folder, with sniffed types and size caps, served through the existing token-protected art route.

- **Built-in themes** live in the core (`go:embed`) and may add CSS for their own look.
- **Third-party themes**, later, reuse data plugins: a folder without a process and with no approval needed, because there is nothing to run. Hash pinning and approval come back only if a theme level ever carries code.

A plugin-defined full UI is out of scope. If it comes, it gets an isolated view with its own narrow bridge (a data snapshot and a short list of intents), never the app page (#89).

### 9. What this keeps open

A native renderer later would implement the same catalog: entries, axes, regions and blocks, fed by the same typed data. Theme files wouldn't change. Nothing here is built for that now.

## Order of work

1. #93 runtime tokens and #94 semantic tokens (level `tokens`).
2. The catalog in Go, exported to the UI, with the guard test. It starts with the entries the Settings testbed needs.
3. #95 **Settings**: proves the variant mechanics (`Section`, `Dialog` with `inline`/`dialog`/`sheet`, `Tabs`). It is the simplest view, but its sections are independent, so it doesn't prove layouts.
4. **Game settings dialog**: tabs are already data-driven, and `ModalOrInline` already exists.
5. #96 **Dashboard**: the first layout proof (seven blocks, all from props). Then Library.
6. Then the smaller views (Activity, Fixes, Runners, Downloads), Storage, Plugins, the game page (split `GamePage.tsx` first), and the store view (native rect) last.

## Consequences

- Each view that joins the catalog costs a data/presenter split. The inventory sizes them (§7).
- Adding a feature means adding catalog entries or blocks with a `since`, and their fixtures. That's more ceremony than writing JSX, and it is the price of the guarantee.
- Variant combinations multiply the test surface. The fixture render keeps it bounded, and a variant that can't be kept accessible isn't added.
- The catalog will churn before 1.0. Until 1.0, the catalog major may move faster, and only built-in themes are supported.

## Answers (user, 2026-10-24)

1. **Required game-page blocks**: the rule in §5. A block is required when it lets the user act on the game or tells the truth about its state. The outside-folder alert is required.
2. **Heroes**: one `Hero` **frame** (art background, gradient, height, overlay and chip style), with the content as blocks inside it: `dashboard.attention`, `game.heading`, `game.actions`, `library.last-played`, … A theme styles the hero once, and a layout can move content (e.g. the game actions) out of it.
3. **Accessibility check**: `axe-core` via `vitest-axe` plus per-variant behaviour tests in the fixture render, and contrast in the Go validator on token values (§7).
4. **Regions are fixed in the catalog.** Themes fill and arrange them; a new region is a catalog change (minor bump) built and tested in the app. A different shell structure, e.g. a top bar instead of the sidebar, is a shell variant the app implements (`shell.navigation = sidebar | topbar`), each with its own regions.

## Implementation notes (2026-10-24)

How the accepted decisions landed, without changing them.

- Variants are one generic map. The core resolves every `entry.axis` into
  `ThemeTokens.Variants`, starting from each axis default. The UI reads them
  through `useVariant` against the generated `gen/catalog.ts`
  (`VariantValues`, `VARIANT_DEFAULTS`). No per-axis plumbing.
- Built-in layouts live in the core (`builtin_layout.go`). Each view renders
  one path: the theme layout when the theme ships one, else the built-in
  arrangement (`useThemeLayout(view) ?? BUILTIN_LAYOUTS[view]` through
  `ThemeLayout`). An omitted view keeps the built-in one.
- Every block takes the universal `span` (int 1..12). A row shares its width
  by span instead of splitting equally.
- Per-view XSDs live at `internal/themes/layouts/<view>.xsd`
  (`LayoutXSDForView`). `layout.xsd` stays as the union for compatibility.
- Where the open items landed: tokens (#93, #94), Settings testbed (#95),
  Dashboard layouts (#96), contrast checks (#97), Library blocks and tile
  variants (#98), game settings variants (#99), shared view chrome (#100),
  Hero frame (#101), Storage and Plugins blocks (#102), game page blocks
  (#103), shell navigation (#104), plugin theme assets (#105), authoring
  docs and example theme (#106), one render path with built-in layouts
  (#108), tile polish and row spans (#109), validator gaps (#110). Still
  open: the store view, the smaller views as layout views, and the native
  renderer (§9).

Source: `internal/themes/catalog.go`, `internal/themes/builtin_layout.go`,
`apps/ui/src/gen/catalog.ts`, `apps/ui/src/lib/theme.ts`,
`apps/ui/src/lib/layout.tsx`.
