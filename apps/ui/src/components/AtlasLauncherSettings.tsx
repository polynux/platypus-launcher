// i18n: converted
// The /22 launcher settings page (issue #156; layout blocks in #117): the
// category navigation over the settings layout's blocks. Moved out of
// LauncherSettingsView.tsx so each file holds one presentation.
import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ATLAS_PAGE } from './ui';
import { ErrorNotice } from './ErrorNotice';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import type { ThemeLayout as SettingsLayoutData, ThemeLayoutNode as SettingsLayoutNode } from '../types';

const ATLAS_CATS = ['general', 'appearance', 'folders', 'saves', 'data', 'social', 'stores'] as const;
export type AtlasCat = (typeof ATLAS_CATS)[number];

/** Which category a settings block belongs to: the Atlas tabs in drawer
 *  order. The navigation lists categories in layout order (first appearance
 *  of one of their blocks), so a theme that reorders blocks reorders the
 *  navigation with them. */
export const SETTINGS_BLOCK_CATS: Record<string, AtlasCat> = {
  language: 'general',
  tray: 'general',
  theme: 'appearance',
  appearance: 'appearance',
  folders: 'folders',
  prefixes: 'folders',
  cleanup: 'folders',
  backups: 'saves',
  cloud: 'saves',
  ratings: 'data',
  artwork: 'data',
  gameinfo: 'data',
  chat: 'social',
  notices: 'stores',
};

/** The layout's block names in render order, rows flattened, first placement
 *  wins. Unknown names stay in the order: the view renders nothing for them,
 *  like any block the core didn't validate. */
function layoutBlockOrder(layout: SettingsLayoutData): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  const collect = (node: SettingsLayoutNode): void => {
    if (node.row) {
      for (const child of node.row) collect(child);
      return;
    }
    if (node.block && !seen.has(node.block)) {
      seen.add(node.block);
      out.push(node.block);
    }
  };
  for (const region of layout.regions ?? []) {
    for (const node of region.nodes ?? []) collect(node);
  }
  return out;
}

export function AtlasSettingsPage({ blocks, loadError }: {
  blocks: Record<string, React.ReactNode>;
  loadError: string | null;
}) {
  const { t } = useTranslation('settings');
  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's Atlas page before the core answers over RPC.
  const layout = useThemeLayout('settings') ?? BUILTIN_LAYOUTS['settings'];
  const navigation = useVariant('settings.navigation');
  const order = layoutBlockOrder(layout);
  // The category navigation follows the layout's order: categories list in
  // first-appearance order of their blocks (filter would keep the constant
  // order instead), and each tab shows its blocks in layout order. Every
  // section is required, so every category is present.
  const cats: AtlasCat[] = [];
  for (const b of order) {
    const c = SETTINGS_BLOCK_CATS[b];
    if (c && !cats.includes(c)) cats.push(c);
  }
  const [cat, setCat] = useState<AtlasCat | null>(null);
  const activeCat = cat ?? cats[0] ?? 'general';
  const goCat = (c: AtlasCat) => {
    if (navigation === 'tabs') {
      setCat(c);
      return;
    }
    // Scroll navigation stacks every section in layout order; the nav jumps
    // to the category's first section.
    const first = order.find((b) => SETTINGS_BLOCK_CATS[b] === c);
    if (first) document.getElementById(`settings-block-${first}`)?.scrollIntoView({ block: 'start' });
  };
  const listed = navigation === 'tabs'
    ? order.filter((b) => SETTINGS_BLOCK_CATS[b] === activeCat)
    : order;
  const visible: Record<string, React.ReactNode> = Object.fromEntries(listed.map((b) => {
    const el = blocks[b] ?? null;
    // Scroll stacks every section in layout order, each wrapped with an
    // anchor the navigation jumps to. The wrappers only exist in scroll
    // mode, so the tabbed page keeps today's structure.
    return navigation === 'tabs' ? [b, el] : [b, <div key={b} id={`settings-block-${b}`}>{el}</div>];
  }));
  return (
    <div className={ATLAS_PAGE}>
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{t('settings:title')}</h1>
      </header>
      {loadError && <ErrorNotice message={loadError} fallback={t('loadError')} />}
      <div className="grid items-start gap-5 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav aria-label={t('sections.label')} className="grid gap-0.5">
          {cats.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => goCat(c)}
              aria-current={navigation === 'tabs' && activeCat === c ? 'page' : undefined}
              className={navigation === 'tabs' && activeCat === c
                ? 'rounded-lg bg-[color-mix(in_srgb,var(--color-accent)_15%,var(--color-surface))] px-2.5 py-[7px] text-left text-[13px] font-semibold text-text'
                : 'rounded-lg px-2.5 py-[7px] text-left text-[13px] font-semibold text-text-muted transition hover:bg-surface hover:text-text'}
            >
              {t(`atlas.cats.${c}`)}
            </button>
          ))}
        </nav>
        <section aria-label={navigation === 'tabs' ? t(`atlas.cats.${activeCat}`) : t('settings:title')} className="min-w-0 px-[26px] pt-[18px]">
          {navigation === 'tabs' ? (
            <>
              <h2 className="mb-1 text-[18px] font-extrabold tracking-tight">{t(`atlas.cats.${activeCat}`)}</h2>
              <div>
                <ThemeLayout
                  layout={layout}
                  blocks={visible}
                  regionClassName={() => undefined}
                  rowClassName={() => 'grid gap-5 md:grid-cols-2'}
                />
              </div>
            </>
          ) : (
            <ThemeLayout
              layout={layout}
              blocks={visible}
              regionClassName={() => undefined}
              rowClassName={() => 'grid gap-5 md:grid-cols-2'}
            />
          )}
        </section>
      </div>
    </div>
  );
}
