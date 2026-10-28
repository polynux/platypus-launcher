// i18n: converted
// The /22 launcher settings page (issue #156): category tabs.
// Moved out of LauncherSettingsView.tsx so each file holds one presentation.
import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ATLAS_PAGE } from './ui';
import { ErrorNotice } from './ErrorNotice';

const ATLAS_CATS = ['general', 'appearance', 'folders', 'saves', 'data', 'social', 'stores'] as const;
type AtlasCat = (typeof ATLAS_CATS)[number];
const ATLAS_SECTIONS: Record<AtlasCat, number[]> = {
  // Indexes into the sections array above: language, tray / theme, appearance
  // / folders, prefixes, cleanup / backups, cloud / ratings (with protection),
  // artwork, gameInfo / chat / store client notices.
  general: [0, 11],
  appearance: [1, 2],
  folders: [3, 4, 5],
  saves: [12, 13],
  data: [6, 7, 8],
  social: [9],
  stores: [10],
};

export function AtlasSettingsPage({ sections, loadError }: {
  sections: React.ReactNode[];
  loadError: string | null;
}) {
  const { t } = useTranslation('settings');
  const [cat, setCat] = useState<AtlasCat>('general');
  return (
    <div className={ATLAS_PAGE}>
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{t('settings:title')}</h1>
      </header>
      {loadError && <ErrorNotice message={loadError} fallback={t('loadError')} />}
      <div className="grid items-start gap-5 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav aria-label={t('sections.label')} className="grid gap-0.5">
          {ATLAS_CATS.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => setCat(c)}
              aria-current={cat === c ? 'page' : undefined}
              className={cat === c
                ? 'rounded-lg bg-[color-mix(in_srgb,var(--color-accent)_15%,var(--color-surface))] px-2.5 py-[7px] text-left text-[13px] font-semibold text-text'
                : 'rounded-lg px-2.5 py-[7px] text-left text-[13px] font-semibold text-text-muted transition hover:bg-surface hover:text-text'}
            >
              {t(`atlas.cats.${c}`)}
            </button>
          ))}
        </nav>
        <section aria-label={t(`atlas.cats.${cat}`)} className="min-w-0 px-[26px] pt-[18px]">
          <h2 className="mb-1 text-[18px] font-extrabold tracking-tight">{t(`atlas.cats.${cat}`)}</h2>
          <div>{ATLAS_SECTIONS[cat].map((i) => <React.Fragment key={i}>{sections[i]}</React.Fragment>)}</div>
        </section>
      </div>
    </div>
  );
}
