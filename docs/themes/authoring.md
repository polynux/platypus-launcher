# Authoring a theme

A theme is a data plugin that carries a `theme.toml` file. It holds no code:
no JS, no CSS text, no `url()`, no remote assets. Only the core reads theme
files. The UI gets typed, validated values over RPC.

Start from `plugins/theme-example/`. It uses all three levels and loads with
zero warnings. The catalog reference is `catalog.md`, generated from the same
Go data the core validates against.

Source: `internal/themes/themes.go`, `internal/plugins/manifest.go`.

## Levels and `supports`

A theme declares what it changes in `[theme]` as `supports`. Each level is
optional. Anything the theme doesn't mention keeps the built-in value.

- `tokens`: colours, fonts, radii, the background wash.
- `variants`: a value on any catalog axis (see `catalog.md`).
- `layouts`: per-view block arrangements in `layouts/<view>.xml`.

All three levels are parsed always, so the picker can show their warnings.
They apply only when the theme declares them. A colours-only theme sets
`supports = ["tokens"]`. A theme without `tokens` keeps the built-in tokens,
even if its `[tokens]` table is full.
An unknown value (e.g. `supports = ["bogus"]`) warns and is ignored: only
`tokens`, `variants` and `layouts` ever apply.

Source: `internal/themes/themes.go` (`setSupports`, `resolve`, `LayoutForTheme`).

## `theme.toml` reference

```toml
[theme]
id = "my-theme"
name = "My theme"
schema = 1
catalog = "1.0"
supports = ["tokens", "variants", "layouts"]
```

- `id` is required. Lowercase letters, digits and dashes, up to 40
  characters (`^[a-z][a-z0-9-]{0,39}$`). A bad id makes the theme invalid.
- `name` defaults to the id when empty or missing.
- `schema` is the theme file version this build reads (`Schema = 1`).
  Below 1 (or missing) is invalid. Above 1 lists the theme as `newer`: it
  shows in the picker but can't be picked, and its tokens are not read.
- `catalog` is the catalog version the theme targets (`CatalogVersion =
  "1.0"`). An omitted catalog assumes `"1.0"` with no warning. A malformed
  catalog warns and assumes `"1.0"`. A newer major (e.g. `"2.0"`) lists the
  theme as `newer`. A newer minor (e.g. `"1.1"`) is usable: known names
  apply, unknown ones warn and fall back.
- Unknown keys anywhere in the file are ignored with a warning, never an
  error.

Source: `internal/themes/themes.go` (`parseThemeFS`, `catalogVersion`).

## `[tokens]`

Every token is a string. A non-string warns and keeps the built-in value.
An unknown token name warns and is ignored. One bad token costs only that
token.

Colour tokens (23): `ground`, `ground-2`, `raised`, `surface`, `surface-strong`,
`hairline`, `hairline-strong`, `popover`, `glass`, `text`, `text-body`,
`text-muted`, `text-faint`, `accent`, `accent-strong`, `accent-dim`,
`on-accent`, `ok`, `attention`, `info`, `dl`, `danger`, `focus`. `ground-2`
is the second background (the lab's `--s1`: the list and the drawer foot),
`popover` the floating panels (`--s-pop`), `hairline-strong` the stronger
line (`--k-line-strong`), `glass` the translucent controls over art
(`--k-glass`). `accent-dim` is the dim shade of the accent: the game page
action strip in a theme that takes the game's colour (the game tone repaints
it from the art; without a tone it is the theme's own value).

A colour is a hex (`#abc`, `#abcd`, `#aabbcc`, `#aabbccdd`), `transparent`,
or `rgb()` / `rgba()` / `hsl()` / `hsla()` / `oklch()` with plain numbers
carrying only `%` or `deg`, separated by commas, slashes or spaces. Anything
holding `;`, `{`, `}` or `url(` is rejected whole.

Radii (3): `radius-chip`, `radius-card`, `radius-panel`. A number in `px` or
`rem` only (`10px`, `0.75rem`). `10em`, `calc(1rem)` and uppercase units are
rejected.

Fonts (2): `font-sans`, `font-mono`. A comma-separated list. A bundled
family (`Hanken Grotesk`, `JetBrains Mono`) goes in double quotes. Anything
else is a generic family (`system-ui`, `ui-sans-serif`, `sans-serif`,
`serif`, `ui-monospace`, `monospace`). A quoted family may also be one the
theme ships in `[fonts]`.

Source: `internal/themes/themes.go` (`setToken`, `checkToken`, `fontStack`).

## `[schemes.*]`

A theme may declare light and black token overrides (issue #134): `[schemes.light]`
and `[schemes.black]` tables with token keys, validated like `[tokens]`. Dark
is the theme's own `[tokens]`; themes without schemes are dark only.

```toml
[schemes.light]
ground = "#e9ecf0"
accent = "#3b5bdb"

[schemes.black]
ground = "#000000"
accent-dim = "#0e1016"
```

An unknown scheme name warns and is ignored; only `light` and `black` ever
apply. One bad token costs only that token, like one bad `[tokens]` entry.
Each scheme is contrast-checked over its own resolved tokens: a failing
foreground token falls back to the dark value with a warning, like any other
bad token. The UI paints the stored scheme pick when the theme declares it,
else dark; native controls follow through `color-scheme: light|dark` on the
root.

Source: `internal/themes/themes.go` (`setScheme`), `internal/themes/contrast.go`
(`checkContrast`), `apps/ui/src/lib/theme.ts` (`themeVariables`, `applyTheme`).

## `[[wash]]`

The background wash is a list of up to 4 gradient layers (`maxWashLayers`).
More than 4 drops the whole wash with a warning. One bad layer drops the
whole wash too; a single token never does that.

```toml
[[wash]]
at = "10% -10%"
size = "1000px 600px"
color = "rgba(56, 189, 248, 0.22)"
fade = "56%"
image = "assets/bg.png"
```

- `at`: two positions, each `-`digits with `px`, `rem` or `%`. May be
  negative.
- `size`: two lengths in `px`, `rem` or `%`.
- `color`: any colour the `[tokens]` grammar accepts.
- `fade`: one length in `px`, `rem` or `%`.
- `image`: optional. A `png`, `webp` or `svg` inside the theme folder (see
  Assets). Checked like a font file.
- All four string fields are required. Surrounding whitespace is collapsed.
  A non-string field drops the whole wash. An unknown wash key warns and is
  ignored.

Without `[[wash]]` the theme keeps the built-in wash. An explicit empty
`wash = []` clears it (a flat stage with no glow, like Atlas).

Source: `internal/themes/themes.go` (`setWash`, `washLayer`).

## `[variants]`

```toml
[variants]
"settings.presentation" = "sheet"
"game-settings.tabs" = "horizontal"
"game-tile.aspect" = "16:9"
```

The key is `"entry.axis"` (exactly one dot). Anything else warns as an
unknown variant axis. The entry and axis must exist in `catalog.md`, and the
value must be a string from the axis's list. A bad value warns and keeps the
built-in value for that axis only.

Three use sites inherit. `game-settings.presentation` follows
`dialog.presentation`. Every `game-hero` axis follows its `hero` axis.
Every `library-tile` axis follows its `game-tile` axis. A use site follows
the broader axis only when the theme leaves the use site unset, and only
when the broader value is one the use site offers. An explicit use-site
value always wins. A use-site value with no broader value set keeps its
built-in default. Inheritance applies only when the theme declares
`variants`.

Source: `internal/themes/themes.go` (`setVariant`, `resolve`), `catalog.go`.

## `[variants]` game colour

`color.game-tone` is how much of the selected game's art colour the app
takes (issue #123). It declares the most the theme supports; the user picks
"Accent" or "Whole app" among the values it allows:

```toml
[variants]
"color.game-tone" = "whole"
```

- `off` (the default): the theme's tokens, whatever the user picked.
- `accent`: the accent and the text on it take the game's hue; surfaces,
  lines and text keep the theme's tokens.
- `whole`: the surfaces, lines and text are derived from the hue too.

The app samples the colour off the render path (cached per art URL) and
fades between games over 0.6s. Derived text is clamped to stay readable
(text 4.5:1, muted and faint 3:1); with no art, or art whose pixels can't be
read, the theme tokens stand. The switch only shows on a theme that allows
`whole`; an `accent` theme always takes the accent.

Source: `apps/ui/src/lib/gameTone.ts`, `apps/ui/src/components/GameToneSwitch.tsx`.

## `[fonts]`

```toml
[fonts]
"My Sans" = "fonts/my-sans.woff2"
```

Each entry maps a family name to its `woff2` file inside the theme folder
(see Assets). A family the app ships (bundled or generic) is refused with a
warning, as is a name holding `"`, `,`, `;`, `{`, `}` or `url(`. A second
entry under one family name is ignored with a warning. Two families may
share one file.

A shipped family becomes usable in `font-sans` / `font-mono`:

```toml
[tokens]
font-sans = '"My Sans", system-ui, sans-serif'
```

Source: `internal/themes/themes.go` (`setFonts`, `fontStackCustom`).

## Layouts

Every layout view has a built-in arrangement in the core
(`internal/themes/builtin_layout.go`). A theme layout replaces it per view:
a shipped `layouts/<view>.xml` replaces the built-in one for that view, and
an omitted view keeps the built-in one. The UI renders one path per view:
the theme layout when the theme ships one, else the built-in arrangement
(`useThemeLayout(view) ?? BUILTIN_LAYOUTS[view]`, mapped through
`ThemeLayout`).

The views that take layouts: `dashboard`, `library`, `game`, `storage`,
`plugins`, and `shell`. Shell regions follow `shell.navigation`: `sidebar`
for the sidebar variant, `topbar` for the topbar variant. Shell takes only
the variant. A `layouts/shell.xml` file validates like any other layout,
but the shell never loads a layout: `Shell.tsx` reads only
`shell.navigation` and renders its blocks directly, arranged by the variant.

Regions per view, from the catalog:

- `dashboard`: `top`, `main`, `side`, `bottom`.
- `library`: `top`, `main`, `side`, `bottom`.
- `game`: `hero`, `main`, `side`.
- `storage`: `top`, `main`, `side`.
- `plugins`: `top`, `main`, `side`.
- `shell`: `sidebar`, `topbar` (variant only, see above).

Block names are per view; see `catalog.md` for the full list. Required
blocks per view, from the catalog:

- `dashboard`: none. Every block is hideable.
- `library`: `warnings` (in `top`).
- `game`: `back`, `actions` and `outside` (all in `hero`). The running job
  progress lives inside the actions and installation panels, so it moves
  with them.
- `storage`: none. Every block is hideable.
- `plugins`: `store` (in `main`). Each store card renders its own login row
  and approve/enable actions.
- `shell`: none. Both variants keep Settings and Plugins reachable by
  construction.

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

- The root is `<layout>`. Anything else discards the file with a warning.
- `view` should match the file name. A mismatch warns and reads the file as
  the file name's view. An unknown view discards the file.
- `catalog` works like the `theme.toml` one: omitted assumes `"1.0"`,
  malformed warns, a newer major discards the file with a warning.
- Under `<layout>` only `<region>` is known. Under `<region>` only `<block>`
  and `<row>` are known, in the order written. Under `<row>` only `<block>`
  is known: a nested row is ignored with a warning. Unknown elements are
  skipped with a warning. Unknown attributes on `<layout>`, `<region>` and
  `<row>` warn and are ignored.
- A region name the catalog doesn't know for the view is skipped with a
  warning, with everything in it. A block name the catalog doesn't know for
  the view is ignored with a warning.
- A block placed twice in one view warns and keeps only the first: the
  second placement is dropped, never merged.
- Block attributes are only the ones the catalog declares, with the type the
  catalog gives them. In catalog 1.0 the typed attributes are `limit` on
  `dashboard.activity` and `library.activity`: an int from 1 to 50. Every
  block also takes the universal `span`: an int from 1 to 12, its share of
  a row's width. A row shares its width by span: blocks without `span`
  count 1, and a row with any `span` over 1 divides its 12-column grid by
  the spans instead of splitting equally. A bad value warns and is ignored;
  the block stays. An undeclared attribute warns and is ignored. Enum
  attributes take exactly one of their values.
- Files that don't end in `.xml` are ignored with a warning. A file that
  can't be read is ignored with a warning. A document that breaks mid-way
  is discarded whole: a half-read tree never reaches the UI.
- A layout may list regions and blocks in any order, and may leave optional
  blocks out. A layout that leaves a required block out still gets it,
  appended in its built-in region (see the list above), with a warning.

The editor schemas are generated from the catalog. `internal/themes/layout.xsd`
(`LayoutXSD()`) is the compatibility schema: the union of every view's
region names, block names and attribute types, for autocomplete. Per-view
placement (which block goes in which region, and which view a block belongs
to) stays with the core validator.
For precise checking use the per-view files `internal/themes/layouts/<view>.xsd`
(`LayoutXSDForView()`): each lists only its view's regions, blocks and
attribute types, with `view` fixed to that view. They are plain XSD 1.0, so
the VS Code XML extension (LemMinX) and IntelliJ validate them; a single
file switching types on the `view` attribute would need XSD 1.1
`xs:alternative`, which those editors handle poorly, so one file per view
beats one clever file. `layout.xsd` stays so existing editor setups keep
working.
Point your editor at the file for the view you edit (e.g. VS Code file
association `layouts/dashboard.xml` → `internal/themes/layouts/dashboard.xsd`),
or add an `<?xml-model href="...">` processing instruction pointing at it:
the core skips processing instructions, so it never warns. Refresh all
schemas with `go test ./internal/themes -run TestLayoutXSDCurrent -update`.

The UI maps region and block names onto the view's components
(`apps/ui/src/lib/layout.tsx`). Regions and rows are plain divs. A block the
view doesn't know renders nothing, never an error.

Source: `internal/themes/layout.go`, `internal/themes/catalog.go`,
`internal/themes/layout.xsd`, `apps/ui/src/lib/layout.tsx`.

## Fallback and warnings

A theme never breaks the app. Every problem is a warning in the theme list
(`ThemeInfo.Warnings`); the built-in choice fills the gap.

- Unknown keys, tokens, axes, values, blocks, regions, attributes and
  `supports` levels: warned and ignored.
- A duplicate block in one view: warned, the second placement dropped.
- Bad token or font value: that token keeps the built-in value.
- Bad wash layer, or more than 4 layers: the whole wash keeps the built-in
  one.
- Bad variant value: that axis keeps the built-in value.
- Bad layout file or part: that file or part is skipped; the view keeps its
  built-in arrangement where the theme ships nothing usable.
- Missing theme, invalid theme, `newer` theme, or a level the theme doesn't
  declare: the built-in values and arrangement.
- A failing contrast pair (below) costs the foreground token its value, like
  any bad token.

An invalid theme (unreadable file, bad id, missing schema) is listed as
`invalid` with the reason. A theme that needs a newer schema or catalog
major is listed as `newer` and can't be picked.

Source: `internal/themes/themes.go`, `internal/themes/layout.go`,
`internal/themes/contrast.go`, `internal/types/models.go`.

## Contrast rules

The core checks contrast on the token values every theme ships. A failing
foreground token falls back to the built-in value with a warning giving the
measured ratio.

Thresholds, from WCAG 2.2 AA: 4.5:1 for text, 3:1 for large text and
non-text indicators such as the focus ring.

| Foreground | Backgrounds | Needs |
| --- | --- | --- |
| `text` | `ground`, `raised` | 4.5:1 |
| `text-body` | `ground`, `raised` | 4.5:1 |
| `text-muted` | `ground`, `raised` | 3:1 |
| `text-faint` | `ground`, `raised` | 3:1 |
| `on-accent` | `accent`, `accent-strong` | 4.5:1 |
| `focus` | `ground` | 3:1 |

The check runs the theme's tokens over the built-in ones, so a theme that
only moves `ground` still has its text checked. The foreground is what gets
dropped. `ground` is painted over black (the window behind every theme);
every other surface is painted over `ground`, so translucent values are
checked as the user sees them. A pair with a value the grammar accepts but
the checker can't read is skipped; the grammar, not the contrast check,
decides what is a colour.

The check runs for every theme, including plugin themes. It is skipped only
when reading the built-in default itself, which supplies the fallback
values.

Source: `internal/themes/contrast.go`.

## Assets

A theme may ship images (`png`, `webp`, `svg`) and fonts (`woff2`) inside
its own folder. References are relative paths from the theme folder, e.g.
`assets/bg.png`. They must stay inside it: absolute paths, `..` escapes and
quotes, backslashes or control characters are refused with a warning, and
the token, layer or family that names them is dropped.

- The type comes from the content, never the file name. PNG needs the PNG
  signature, WebP a `RIFF....WEBP` header, SVG an `<svg>` root (optionally
  after an XML declaration; HTML is refused), woff2 the `wOF2` magic. The
  extension must match the sniffed type (`.png`, `.webp`, `.svg`,
  `.woff2`).
- One file caps at 2 MiB (`MaxAssetBytes`). Empty files and non-files are
  refused.
- Without the theme folder (a bare `theme.toml` read) asset references
  can't be checked, so they are dropped with a warning.

The stored paths stay relative. The app turns them into art URLs at serve
time, under `/platypus-art/theme/<plugin>/<path>` on the token-protected
art route. The UI draws a wash image under its gradient layer and registers
each shipped font as a `FontFace`, so a font token naming it paints at once.

Working example: `plugins/theme-example/` ships `assets/bg.png` and names it
from its wash layer, and loads with zero warnings:

```toml
[[wash]]
at = "10% -10%"
size = "1000px 600px"
color = "rgba(56, 189, 248, 0.22)"
fade = "56%"
image = "assets/bg.png"
```

Fonts follow the same rules. A shipped family needs its file in the theme
folder (`woff2` magic, `.woff2` ext, under 2 MiB) and a token naming it:

```toml
[fonts]
"My Sans" = "fonts/my-sans.woff2"

[tokens]
font-sans = '"My Sans", system-ui, sans-serif'
```

The family must not be one the app ships (bundled or generic).

Source: `internal/themes/assets.go`, `internal/plugins/theme.go`,
`apps/ui/src/lib/theme.ts`.

## Installing a theme plugin

A theme plugin is a data plugin folder:

```
~/.local/share/platypus/plugins/<id>/
  plugin.json
  theme.toml
  layouts/<view>.xml
  assets/…
  fonts/…
```

`plugin.json` says `"kind": "data"` and names the theme file. It carries no
`exec`, `store`, `capabilities` or `requires`. A theme-only plugin names no
data files: the missing `fixes.json` / `tweaks.json` defaults are not an
error when a theme is named. Only a data plugin may carry a theme; a store
plugin that names one is invalid. The `theme` path must stay inside the
folder.

```json
{
  "id": "theme-example",
  "name": "Example theme",
  "version": "1.0.0",
  "author": "you",
  "description": "One sentence for the Plugins view.",
  "api": 1,
  "kind": "data",
  "theme": "theme.toml"
}
```

Install from a checkout with:

```sh
plugins/install.sh theme-example
```

It copies the data files, the manifest, the theme file, `layouts/`,
`assets/` and `fonts/`. A data plugin needs no approval: there is nothing
to run. The Plugins view lists it under data plugins, and the theme picker
lists its theme with the plugin id as source. A broken theme never breaks
the plugin's fixes: the theme shows as invalid in the picker instead.

Source: `docs/plugins/data-plugins.md`, `internal/plugins/manifest.go`,
`internal/plugins/manager.go`, `plugins/install.sh`.

## `catalog = "x.y"` versioning

The catalog is versioned `major.minor` (`CatalogVersion = "1.0"`). Minor is
additive: new entries, axes, values, blocks and regions. Every entry and
axis carries `since`. Major is breaking: a removal or rename.

- A theme declares the catalog version it targets in `theme.toml` and in
  each layout file.
- Names from a newer catalog warn and fall back, per value. A theme that
  needs a newer major, or a newer file schema, is listed as needing a newer
  Platypus and can't be picked.
- The guard test refuses breaking changes: `go test ./internal/themes -run
  TestCatalogGuard -update` refreshes the snapshot
  (`internal/themes/testdata/catalog-1.json`) only for compatible changes.
- Until the themes epic merges the catalog is unreleased: keep
  `CatalogVersion = "1.0"` and mark additions `since = "1.0"`.
- `docs/themes/catalog.md` is generated from the catalog. Refresh it with
  `go run ./cmd/gen-ts`. The gents staleness test fails when it is out of
  date.

Source: `internal/themes/catalog.go`, `internal/themes/catalog_test.go`,
`internal/gents/gents_test.go`, `cmd/gen-ts/main.go`.
