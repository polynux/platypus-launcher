// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { LauncherSettings, StorePlugin, ThemeInfo } from '../types';
import { NumberField } from '@/components/ds/number-field';
import { Input } from '@/components/ds/input';
import { Switch } from '@/components/ds/switch';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { PSEUDO_LOCALE, applyLanguage, loadSessionInfo, pseudoAvailable } from '../lib/i18n';
import { CloudSavesDefaults } from './CloudSavesSetting';
import { GameFoldersSettings } from './GameFoldersSettings';
import { BackupsSetting } from './BackupsSetting';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { DEFAULT_THEME_ID, loadTheme, useColorScheme, useThemeSchemes, useVariant, saveColorSchemeChoice, type ColorScheme } from '../lib/theme';
import { useGameToneChoice, type GameToneChoice } from '../lib/gameTone';
import { ToggleGroup, ToggleGroupItem } from './ds/toggle-group';
import { ATLAS_PAGE, ViewHeader, cx } from './ui';
import { AtlasSRow, AtlasSSwitch } from './AtlasSettings';

/** A settings section: a hairline and air, not a card (no card inside the page). */
const SECTION = 'max-w-2xl border-t border-hairline py-6';

// The active Settings section survives a Settings re-mount (issue #111): a
// theme switch can move Settings from page to sheet/dialog (App mounts a new
// LauncherSettingsView), which would otherwise drop the tab back to Language.
// The last shown section persists here; a fresh mount starts from it. The
// pending mount index carries the picker's focus across that same re-mount:
// only a Theme picker mounted after the switch takes focus, so the instance
// being replaced never steals it back.
let persistedSettingsSection = 'language';
let themePickerMountCount = 0;
let pendingThemePickerMount = 0;

interface LauncherSettingsViewProps {
  onNotify: (msg: string, kind?: 'ok' | 'err') => void;
  /** The plugins list: the cloud-saves defaults come from the stores that offer them. */
  plugins: StorePlugin[];
}

export function LauncherSettingsView({ onNotify, plugins }: LauncherSettingsViewProps) {
  const { t } = useTranslation('settings');
  const navigation = useVariant('settings.navigation');
  const chrome = useVariant('section.chrome');
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  const sectionNames = ['language', 'theme', 'appearance', 'folders', 'prefixes', 'cleanup', 'ratings', 'artwork', 'gameInfo', 'chat', 'tray', 'backups', 'cloud'];
  const [, setRenderTick] = useState(0);
  const setActiveSection = (index: number) => {
    persistedSettingsSection = sectionNames[index] ?? persistedSettingsSection;
    // Leaving Theme abandons a pending picker-focus handoff.
    if (persistedSettingsSection !== 'theme') pendingThemePickerMount = 0;
    setRenderTick((n) => n + 1);
  };
  // Derived from the persisted section so a same-instance navigation switch
  // (scroll -> tabs, no re-mount) lands on what the user was on. Tab clicks
  // update the persisted section, so this agrees with the state above.
  const activeSection = Math.max(0, sectionNames.indexOf(persistedSettingsSection));
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [cleanupDays, setCleanupDays] = useState<number | ''>('');
  const [cleanupError, setCleanupError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setCleanupDays(s.cancel_cleanup_days); })
      .catch((e) => setError(coreErrMsg(e)));
  }, []);

  const saveCleanupDays = (days: number) => {
    setBusy(true);
    setCleanupError(null);
    Effect.runPromise(LauncherApi.setCancelCleanupDays(days))
      .then((s) => {
        setSaved(s);
        setCleanupDays(s.cancel_cleanup_days);
        onNotify(t('cleanup.saved'));
      })
      .catch((e) => setCleanupError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const cleanupDirty = saved != null && typeof cleanupDays === 'number' && cleanupDays !== saved.cancel_cleanup_days;

  const sections = [
    <LanguageSetting onNotify={onNotify} />,
    <ThemeSetting onNotify={onNotify} />,
    <AppearanceSetting onNotify={onNotify} />,
    <GameFoldersSettings onNotify={onNotify} />,
    <PrefixesFolderSetting onNotify={onNotify} />,
    atlas ? (
      <section>
        <AtlasSRow label={t('cleanup.heading')} desc={t('cleanup.description')}>
          <Select value={String(cleanupDays)} onValueChange={(v) => saveCleanupDays(Number(v))} disabled={!saved || busy}>
            <SelectTrigger aria-label={t('cleanup.daysLabel')} className="h-8 w-[210px] font-sans text-[13px]"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="0">{t('cleanup.deleteRightAway')}</SelectItem><SelectItem value="7">{t('cleanup.keepWeek')}</SelectItem><SelectItem value="365">{t('cleanup.keepYear')}</SelectItem></SelectContent>
          </Select>
        </AtlasSRow>
        <div className="mt-2.5 flex min-h-4 items-center gap-4">{cleanupError ? <ErrorNotice message={cleanupError} fallback={t('cleanupError')} /> : <span />}</div>
      </section>
    ) : (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('cleanup.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('cleanup.description')}</p>
      <form className="flex flex-wrap items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (cleanupDirty && typeof cleanupDays === 'number') saveCleanupDays(cleanupDays); }}>
        <label className="flex items-center gap-2"><span className="text-[12.5px] text-text-body">{t('cleanup.keepFor')}</span>
          <NumberField min={0} max={365} step={1} hideSteppers value={cleanupDays === '' ? undefined : cleanupDays} onChange={(v) => { setCleanupDays(v ?? ''); setCleanupError(null); }} disabled={!saved} aria-label={t('cleanup.daysLabel')} className="w-24" />
          <span className="text-[12.5px] text-text-body">{t('cleanup.days')}</span>
        </label>
        <Button type="submit" variant="primary" disabled={!cleanupDirty || busy}>{t('common:save')}</Button>
      </form>
      <div className="mt-2.5 flex min-h-4 items-center gap-4">{cleanupError ? <ErrorNotice message={cleanupError} fallback={t('cleanupError')} /> : <span />}</div>
    </section>
    ),
    <RatingsSetting onNotify={onNotify} />,
    <ArtworkSetting onNotify={onNotify} />,
    <GameInfoSetting onNotify={onNotify} />,
    <SocialSettings plugins={plugins} onNotify={onNotify} />,
    <TraySetting onNotify={onNotify} />,
    <BackupsSetting onNotify={onNotify} />,
    <CloudSavesDefaults plugins={plugins} />,
  ];
  const sectionLabels = [t('sections.language'), t('sections.theme'), t('sections.appearance'), t('sections.folders'), t('sections.prefixes'), t('sections.cleanup'), t('sections.ratings'), t('sections.artwork'), t('sections.gameInfo'), t('sections.chat'), t('sections.tray'), t('sections.backups'), t('sections.cloud')];
  const chromeClass = chrome === 'framed' ? '[&>section]:rounded-2xl [&>section]:border [&>section]:border-hairline-strong [&>section]:bg-surface [&>section]:px-5' : chrome === 'plain' ? '[&>section]:border-t-0' : '';

  // Atlas renders the /22 Settings page (issue #138): the lab's six backed
  // categories in the left nav. The lab's "Store clients" row has no backing
  // setting in the core, so it has no category here.
  if (atlas) {
    return <AtlasSettingsPage sections={sections} loadError={error} />;
  }

  return (
    <div className="p-7">
      <ViewHeader
        tight
        title={t('settings:title')}
        subtitle={<p className="mt-0.5 max-w-[75ch] text-[12.5px] text-text-body">{t('subtitle')}</p>}
      />
      {error && <ErrorNotice className="mt-2" message={error} fallback={t('loadError')} />}

      {navigation === 'tabs' ? (
        <div className="grid grid-cols-[10rem_minmax(0,1fr)] gap-5">
          <div role="tablist" aria-label={t('sections.label')} aria-orientation="vertical" className="flex flex-col gap-1.5">
            {sectionNames.map((name, index) => <button key={name} role="tab" id={`settings-tab-${name}`} aria-selected={activeSection === index} aria-controls={`settings-panel-${name}`} tabIndex={activeSection === index ? 0 : -1} onClick={() => setActiveSection(index)} onKeyDown={(e) => {
              const next = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? (index + 1) % sectionNames.length : e.key === 'ArrowUp' || e.key === 'ArrowLeft' ? (index - 1 + sectionNames.length) % sectionNames.length : -1;
              if (next < 0) return;
              e.preventDefault(); setActiveSection(next); document.getElementById(`settings-tab-${sectionNames[next]}`)?.focus();
            }} className="rounded-xl px-3 py-2 text-left text-[12.5px] font-semibold aria-selected:bg-accent/15 aria-selected:text-text">{sectionLabels[index]}</button>)}
          </div>
          <div className={chromeClass} role="tabpanel" id={`settings-panel-${sectionNames[activeSection]}`} aria-labelledby={`settings-tab-${sectionNames[activeSection]}`}>
            {sections[activeSection]}
          </div>
        </div>
      ) : (
        <div className={chromeClass}>{sections.map((section, index) => <React.Fragment key={sectionNames[index]}>{section}</React.Fragment>)}</div>
      )}
    </div>
  );
}

/** The /22 Settings page (issue #138, Atlas only): the left category nav and
 *  the right body reusing the same section components as the classic view.
 *  Appearance comes from #134 and stays untouched. */
const ATLAS_CATS = ['general', 'appearance', 'folders', 'saves', 'data', 'social'] as const;
type AtlasCat = (typeof ATLAS_CATS)[number];
const ATLAS_SECTIONS: Record<AtlasCat, number[]> = {
  // Indexes into the sections array above: language, tray / theme, appearance
  // / folders, prefixes, cleanup / backups, cloud / ratings, artwork,
  // gameInfo / chat.
  general: [0, 10],
  appearance: [1, 2],
  folders: [3, 4, 5],
  saves: [11, 12],
  data: [6, 7, 8],
  social: [9],
};

function AtlasSettingsPage({ sections, loadError }: {
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

/** The UI language (issue #39): System follows the desktop language, or an
 *  explicit language. Saving applies it live, without a restart. */
export function LanguageSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [value, setValue] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dev, setDev] = useState(false);
  useEffect(() => {
    let live = true;
    try {
      Effect.runPromise(LauncherApi.getLauncherSettings())
        .then((s) => live && setValue(s?.language ?? ''))
        .catch(() => live && setValue(''));
    } catch {
      setValue('');
    }
    void loadSessionInfo().then(() => live && setDev(pseudoAvailable()));
    return () => { live = false; };
  }, []);
  const change = (next: string) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setLanguage(next))
      .then((s) => {
        const saved = s.language ?? next;
        setValue(saved);
        return applyLanguage(saved);
      })
      .then(() => onNotify(t('language.saved')))
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  // Language names stay in their own language (endonyms): translating them
  // would hide what the option means.
  // Radix Select treats "" as no value (an empty trigger), so System is
  // 'system' inside the picker and "" in the setting. Controlled from the
  // first render (issue #111): while loading we show System, never undefined.
  const SYSTEM = 'system';
  const options = [
    { value: SYSTEM, label: t('language.system') },
    { value: 'en', label: 'English' },
    { value: 'fr', label: 'Français' },
  ];
  if (dev) options.push({ value: PSEUDO_LOCALE, label: `${PSEUDO_LOCALE} (pseudo)` });
  // Atlas renders the row as a lab .srow: 600 label + muted desc left, a
  // 210px Select right; the dev-only pseudo note sits inside the desc, sans.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSRow
          label={t('language.heading')}
          desc={<>
            {t('language.atlasDescription')}
            {dev && <span className="mt-1 block text-[12px] text-text-faint">{t('language.pseudoNote')}</span>}
          </>}
        >
          <Select value={value || SYSTEM} onValueChange={(v) => change(v === SYSTEM ? '' : v)} disabled={value === null || busy}>
            <SelectTrigger aria-label={t('language.label')} className="h-8 w-[210px] flex-none font-sans text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {options.map((o) => (
                <SelectItem key={o.value} value={o.value} className="font-sans text-[13px]">{o.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </AtlasSRow>
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('language.heading')}</h2>
      <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('language.description')}</p>
      <Select value={value || SYSTEM} onValueChange={(v) => change(v === SYSTEM ? '' : v)} disabled={value === null || busy}>
        <SelectTrigger aria-label={t('language.label')} className="w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      {dev && <p className="mt-2 font-mono text-hint text-text-muted">{t('language.pseudoNote')}</p>}
    </section>
  );
}

/** The launcher's look (issue #93): a built-in theme restyles the whole app
 *  as soon as it is picked. A theme that needs a newer Platypus, or whose file
 *  is broken, stays in the list with the reason, and can't be picked. */
export function ThemeSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [themes, setThemes] = useState<ThemeInfo[] | null>(null);
  const [value, setValue] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const mountIndex = useRef(0);
  if (mountIndex.current === 0) mountIndex.current = ++themePickerMountCount;
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.listThemes())
      .then((list) => live && setThemes(list ?? []))
      .catch(() => live && setThemes([]));
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => live && setValue(s.theme || DEFAULT_THEME_ID))
      .catch(() => live && setValue(DEFAULT_THEME_ID));
    return () => { live = false; };
  }, []);
  // Across a theme switch that re-mounts Settings (page -> sheet/dialog,
  // issue #111) the fresh picker takes focus once it is enabled. Only a
  // picker mounted after the switch qualifies, so the replaced instance
  // never consumes the pending focus.
  useEffect(() => {
    if (pendingThemePickerMount === 0 || mountIndex.current < pendingThemePickerMount) return;
    if (themes === null || value === null || busy) return;
    pendingThemePickerMount = 0;
    triggerRef.current?.focus();
    document.getElementById('settings-section-theme')?.scrollIntoView({ block: 'nearest' });
  }, [themes, value, busy]);
  const change = (id: string) => {
    // Remember where the user was and that focus belongs to the picker: the
    // theme apply below can re-mount this whole view on another presentation.
    persistedSettingsSection = 'theme';
    pendingThemePickerMount = themePickerMountCount + 1;
    setBusy(true);
    Effect.runPromise(LauncherApi.setTheme(id))
      .then((s) => {
        setValue(s.theme || DEFAULT_THEME_ID);
        return loadTheme();
      })
      .then(() => onNotify(t('theme.saved')))
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  // A pick the list no longer offers shows as the default, not as an empty trigger.
  // Controlled from the first render (issue #111): never undefined.
  const shown = themes?.some((th) => th.id === value) ? (value ?? DEFAULT_THEME_ID) : DEFAULT_THEME_ID;
  const warnings = (themes ?? []).flatMap((th) => th.warnings.map((w) => ({ key: `${th.id}:${w}`, text: `${th.name}: ${w}` })));
  const label = (th: ThemeInfo): string => {
    const status = th.status === 'ok' ? '' : t(`theme.status.${th.status}`, { defaultValue: th.status });
    const from = th.source ? t('theme.source', { plugin: th.source }) : '';
    if (status && from) return `${th.name} (${status}, ${from})`;
    if (status) return `${th.name} (${status})`;
    if (from) return `${th.name} (${from})`;
    return th.name;
  };
  // Atlas renders the row as a lab .srow: the Theme select right-aligned at
  // 210px, with no inner heading (the category nav owns it).
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSRow label={t('theme.heading')}>
          <Select value={shown} onValueChange={change} disabled={themes === null || value === null || busy}>
            <SelectTrigger
              ref={triggerRef}
              aria-label={t('theme.label')}
              className="h-8 w-[210px] flex-none font-sans text-[13px]"
              onFocus={() => { persistedSettingsSection = 'theme'; }}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(themes ?? []).map((th) => (
                <SelectItem key={th.id} value={th.id} disabled={th.status !== 'ok'} className="font-sans text-[13px]">
                  {label(th)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </AtlasSRow>
        {warnings.length > 0 && (
          <ul aria-label={t('theme.warningsLabel')} className="mt-3 space-y-1 font-mono text-hint leading-relaxed text-text-muted">
            {warnings.map((w) => <li key={w.key}>{w.text}</li>)}
          </ul>
        )}
      </section>
    );
  }
  return (
    <section className={SECTION} id="settings-section-theme">
      <h2 className="text-[14px] font-bold">{t('theme.heading')}</h2>
      <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('theme.description')}</p>
      <Select value={shown} onValueChange={change} disabled={themes === null || value === null || busy}>
        <SelectTrigger
          ref={triggerRef}
          aria-label={t('theme.label')}
          className="w-64"
          onFocus={() => { persistedSettingsSection = 'theme'; }}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(themes ?? []).map((th) => (
            <SelectItem key={th.id} value={th.id} disabled={th.status !== 'ok'}>
              {label(th)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {warnings.length > 0 && (
        <ul aria-label={t('theme.warningsLabel')} className="mt-3 space-y-1 font-mono text-hint leading-relaxed text-text-muted">
          {warnings.map((w) => <li key={w.key}>{w.text}</li>)}
        </ul>
      )}
    </section>
  );
}

/** Appearance (issue #134): the colour scheme and the game colour, as segmented
 *  controls in the Atlas look. The scheme picker shows only when the theme
 *  declares schemes; the game colour picker only when the theme's
 *  color.game-tone allows it. Both save through the settings API, so the
 *  top-bar switch (which shares the game-colour pick) stays in agreement. */
export function AppearanceSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const { t: tTone } = useTranslation('gameTone');
  const schemes = useThemeSchemes();
  const scheme = useColorScheme();
  const tone = useGameToneChoice();
  const [busy, setBusy] = useState(false);
  const showScheme = schemes.length > 1;
  const showTone = tone.supported !== 'off';
  if (!showScheme && !showTone) {
    return (
      <section className={SECTION} id="settings-section-appearance">
        <h2 className="text-[14px] font-bold">{t('appearance.heading')}</h2>
        <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('appearance.none')}</p>
      </section>
    );
  }
  const segWrap = 'gap-0.5 rounded-[9px] bg-surface p-[3px]';
  const segItem =
    'gap-1.5 rounded-md bg-transparent px-2.5 py-0 font-sans text-[12.5px] font-semibold text-text-muted transition hover:bg-transparent hover:text-text ' +
    'data-[state=on]:bg-[color-mix(in_srgb,var(--color-accent)_20%,var(--color-surface-strong))] data-[state=on]:font-semibold data-[state=on]:text-text ' +
    'data-[state=on]:ring-1 data-[state=on]:ring-inset data-[state=on]:ring-accent/40';
  const pickScheme = (next: string) => {
    if (next !== 'dark' && next !== 'light' && next !== 'black') return;
    persistedSettingsSection = 'appearance';
    setBusy(true);
    void saveColorSchemeChoice(next as ColorScheme)
      .then(() => onNotify(t('appearance.schemeSaved')))
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  const pickTone = (next: string) => {
    if (next !== 'off' && next !== 'accent' && next !== 'whole') return;
    persistedSettingsSection = 'appearance';
    tone.setChoice(next as GameToneChoice);
    onNotify(t('appearance.gameSaved'));
  };
  // Atlas renders the lab's two .srows (issue #138): Theme is its own
  // section above; here Colour scheme and Game colour sit right-aligned and
  // sized to their content, with no inner heading or description sentence.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section id="settings-section-appearance">
        {showScheme && (
          <AtlasSRow label={t('appearance.schemeLabel')}>
            <ToggleGroup
              type="single"
              value={scheme}
              onValueChange={(v) => { if (v) pickScheme(v); }}
              aria-label={t('appearance.schemeLabel')}
              className={cx('inline-flex w-auto', segWrap)}
              disabled={busy}
            >
              {(['dark', 'black', 'light'] as const).filter((s) => schemes.includes(s)).map((s) => (
                <ToggleGroupItem key={s} value={s} aria-label={t(`appearance.scheme.${s}`)} className={cx('h-[26px]', segItem)}>
                  {t(`appearance.scheme.${s}`)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </AtlasSRow>
        )}
        {showTone && (
          <AtlasSRow label={t('appearance.gameLabel')} desc={t('appearance.gameDescription')}>
            <ToggleGroup
              type="single"
              value={tone.choice}
              onValueChange={(v) => { if (v === 'off' || v === 'accent' || v === 'whole') pickTone(v); }}
              aria-label={tTone('label')}
              className={cx('inline-flex w-auto', segWrap)}
            >
              <ToggleGroupItem value="off" title={tTone('offHint')} className={cx('h-[26px]', segItem)}>
                {tTone('off')}
              </ToggleGroupItem>
              <ToggleGroupItem value="accent" title={tTone('accentHint')} className={cx('h-[26px]', segItem)}>
                {tTone('accent')}
              </ToggleGroupItem>
              {tone.supported === 'whole' && (
                <ToggleGroupItem value="whole" title={tTone('wholeHint')} className={cx('h-[26px]', segItem)}>
                  {tTone('whole')}
                </ToggleGroupItem>
              )}
            </ToggleGroup>
          </AtlasSRow>
        )}
      </section>
    );
  }
  return (
    <section className={SECTION} id="settings-section-appearance">
      <h2 className="text-[14px] font-bold">{t('appearance.heading')}</h2>
      <p className="mb-4 mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('appearance.description')}</p>
      <div className="grid gap-5">
        {showScheme && (
          <div>
            <div className="mb-1.5 text-[13px] font-semibold">{t('appearance.schemeLabel')}</div>
            <p className="mb-2 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('appearance.schemeDescription')}</p>
            <ToggleGroup
              type="single"
              value={scheme}
              onValueChange={(v) => { if (v) pickScheme(v); }}
              aria-label={t('appearance.schemeLabel')}
              className={cx(segWrap)}
              disabled={busy}
            >
              {(['dark', 'light', 'black'] as const).filter((s) => schemes.includes(s)).map((s) => (
                <ToggleGroupItem key={s} value={s} aria-label={t(`appearance.scheme.${s}`)} className={cx('h-[26px]', segItem)}>
                  {t(`appearance.scheme.${s}`)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
        )}
        {showTone && (
          <div>
            <div className="mb-1.5 text-[13px] font-semibold">{t('appearance.gameLabel')}</div>
            <p className="mb-2 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('appearance.gameDescription')}</p>
            <ToggleGroup
              type="single"
              value={tone.choice}
              onValueChange={(v) => { if (v === 'off' || v === 'accent' || v === 'whole') pickTone(v); }}
              aria-label={tTone('label')}
              className={cx(segWrap)}
            >
              <ToggleGroupItem value="off" title={tTone('offHint')} className={cx('h-[26px]', segItem)}>
                {tTone('off')}
              </ToggleGroupItem>
              <ToggleGroupItem value="accent" title={tTone('accentHint')} className={cx('h-[26px]', segItem)}>
                {tTone('accent')}
              </ToggleGroupItem>
              {tone.supported === 'whole' && (
                <ToggleGroupItem value="whole" title={tTone('wholeHint')} className={cx('h-[26px]', segItem)}>
                  {tTone('whole')}
                </ToggleGroupItem>
              )}
            </ToggleGroup>
          </div>
        )}
      </div>
    </section>
  );
}

/** "Fetch game info online" and the credits for the data Platypus shows. Off, Platypus
 *  keeps what it fetched before and asks the network for nothing new. */
export function RatingsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.ratings_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setRatingsOnline(next))
      .then((s) => {
        setOn(s.ratings_online ?? next);
        onNotify(next ? t('ratings.on') : t('ratings.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSSwitch
          label={t('ratings.heading')}
          desc={t('ratings.description')}
          checked={on ?? true}
          disabled={on === null || busy}
          onChange={() => toggle(!(on ?? true))}
        />
        <p className="border-t border-hairline pt-3 text-[12px] font-sans text-text-faint">{t('ratings.creditCompat')} {t('ratings.creditWiki')} {t('ratings.creditAnticheat')}</p>
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('ratings.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('ratings.description')}
          </p>
        </div>
        <Switch aria-label={t('ratings.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-hairline pt-3 font-mono text-hint leading-relaxed text-text-muted">
        <p>{t('ratings.creditCompat')}</p>
        <p>{t('ratings.creditWiki')}</p>
        <p>{t('ratings.creditAnticheat')}</p>
      </div>
    </section>
  );
}

/** "Find artwork online" for sideloaded games (issue #36): Steam's public
 *  store search after an add, and the on-demand finder in the art panel. Off,
 *  nothing goes out: the placeholder art stays and no search runs. */
export function ArtworkSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.artwork_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setArtworkOnline(next))
      .then((s) => {
        setOn(s.artwork_online ?? next);
        onNotify(next ? t('artwork.on') : t('artwork.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSSwitch
          label={t('artwork.heading')}
          desc={t('artwork.description')}
          checked={on ?? true}
          disabled={on === null || busy}
          onChange={() => toggle(!(on ?? true))}
        />
        <p className="border-t border-hairline pt-3 text-[12px] font-sans text-text-faint">{t('artwork.creditArt')}</p>
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('artwork.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('artwork.description')}
          </p>
        </div>
        <Switch aria-label={t('artwork.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-hairline pt-3 font-mono text-hint leading-relaxed text-text-muted">
        <p>{t('artwork.creditArt')}</p>
      </div>
    </section>
  );
}
/** "Find game info online" for sideloaded games (issue #38): the on-demand
 *  description and details lookup. Off, nothing goes out: the game page keeps
 *  what it cached before and no new fetch runs. */
export function GameInfoSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.game_info_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setGameInfoOnline(next))
      .then((s) => {
        setOn(s.game_info_online ?? next);
        onNotify(next ? t('gameInfo.on') : t('gameInfo.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSSwitch
          label={t('gameInfo.heading')}
          desc={t('gameInfo.description')}
          checked={on ?? true}
          disabled={on === null || busy}
          onChange={() => toggle(!(on ?? true))}
        />
        <p className="border-t border-hairline pt-3 text-[12px] font-sans text-text-faint">{t('gameInfo.creditInfo')}</p>
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('gameInfo.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('gameInfo.description')}
          </p>
        </div>
        <Switch aria-label={t('gameInfo.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-hairline pt-3 font-mono text-hint leading-relaxed text-text-muted">
        <p>{t('gameInfo.creditInfo')}</p>
      </div>
    </section>
  );
}
/** "Desktop notifications for chat" (issue #12): a system notification for a
 *  new chat message when the window isn't focused. Off, only the in-app toasts
 *  stay. On by default, like the other switches. */
export function ChatNotificationsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.chat_desktop_notify ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setChatDesktopNotify(next))
      .then((s) => {
        setOn(s.chat_desktop_notify ?? next);
        onNotify(next ? t('chat.on') : t('chat.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSSwitch
          label={t('chat.heading')}
          desc={t('chat.description')}
          checked={on ?? true}
          disabled={on === null || busy}
          onChange={() => toggle(!(on ?? true))}
        />
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('chat.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('chat.description')}
          </p>
        </div>
        <Switch aria-label={t('chat.label')} checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
    </section>
  );
}

function SocialSettings({ plugins, onNotify }: { plugins: StorePlugin[]; onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  const [views, setViews] = useState<{ plugin: string; enabled: boolean }[]>([]);
  const stores = plugins.filter((p) => p.capabilities.includes('social') || p.declared.includes('social'));
  const refresh = () => Effect.runPromise(LauncherApi.socialViews()).then((v) => setViews(v ?? [])).catch(() => {});
  useEffect(() => { if (atlas) refresh(); }, [atlas]);
  return <section>
    {atlas && stores.map((p) => {
      const enabled = views.find((v) => v.plugin === p.id)?.enabled ?? false;
      return <AtlasSSwitch key={p.id} label={t('social.friends', { store: p.name })} checked={enabled} onChange={() => Effect.runPromise(LauncherApi.setSocialEnabled(p.id, !enabled)).then(refresh).catch((e) => onNotify(coreErrMsg(e), 'err'))} />;
    })}
    <ChatNotificationsSetting onNotify={onNotify} />
  </section>;
}
/** "Minimize to tray while a game runs" (issue #49): while a game runs the
 *  window hides to a tray icon instead of staying open, and comes back when
 *  the game exits. Off by default, unlike the online switches. */
export function TraySetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.minimize_to_tray ?? false))
      .catch(() => setOn(false));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setMinimizeToTray(next))
      .then((s) => {
        setOn(s.minimize_to_tray ?? next);
        onNotify(next ? t('tray.on') : t('tray.off'));
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  if (useVariant('shell.navigation') === 'sourcebar') {
    return (
      <section>
        <AtlasSSwitch
          label={t('tray.heading')}
          desc={<span title={t('tray.description')}>{t('tray.atlasDescription')}</span>}
          checked={on ?? false}
          disabled={on === null || busy}
          onChange={() => toggle(!(on ?? false))}
        />
      </section>
    );
  }
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">{t('tray.heading')}</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('tray.description')}
          </p>
        </div>
        <Switch aria-label={t('tray.label')} checked={on ?? false} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
    </section>
  );
}
/** Where new Wine prefixes are created (issue #37): one folder per game in
 *  this folder, unless the game has its own prefix. Changing it never moves
 *  or deletes existing prefixes; empty resets it to the default. */
export function PrefixesFolderSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [folder, setFolder] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setFolder(s.prefixes_folder ?? ''); })
      .catch((e) => setError(coreErrMsg(e)));
  }, []);

  const dirty = saved != null && folder.trim() !== (saved.prefixes_folder ?? '');
  const custom = (saved?.prefixes_folder ?? '') !== '';

  const save = (path: string) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setPrefixesFolder(path))
      .then((s) => {
        setSaved(s);
        setFolder(s.prefixes_folder ?? '');
        onNotify(path ? t('prefixes.saved') : t('prefixes.resetDone'));
      })
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const browse = () => {
    Effect.runPromise(LauncherApi.pickFolder(t('prefixes.browseTitle'), folder.trim() || saved?.prefixes_folder || saved?.default_prefixes_folder || ''))
      .then((picked) => { if (picked) { setFolder(picked); setError(null); if (atlas) save(picked); } })
      .catch((e) => setError(coreErrMsg(e)));
  };

  // Atlas: the lab's wide .srow (label + desc up top, the folder controls below).
  if (atlas) {
    return (
      <section>
        <AtlasSRow label={t('prefixes.heading')} desc={saved ? (saved.prefixes_folder || saved.default_prefixes_folder || t('prefixes.placeholder')) : t('prefixes.description')}>
          <div className="flex items-center gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={browse} disabled={!saved || busy}>{t('prefixes.change')}</Button>
            {custom && <Button type="button" variant="ghost" size="sm" onClick={() => save('')} disabled={busy}>{t('prefixes.reset')}</Button>}
          </div>
        </AtlasSRow>
        {error && <ErrorNotice message={error} fallback={t('prefixError')} />}
      </section>
    );
  }

  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('prefixes.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">
{t('prefixes.description')}
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (dirty) save(folder.trim());
        }}
      >
        <Input
          aria-label={t('prefixes.label')}
          value={folder}
          onChange={(e) => { setFolder(e.target.value); setError(null); }}
          placeholder={saved?.default_prefixes_folder || t('prefixes.placeholder')}
          spellCheck={false}
          disabled={!saved}
          className="min-w-0 max-w-xl flex-1"
        />
        <Button type="button" variant="neutral" onClick={browse} disabled={!saved || busy}>{t('prefixes.browse')}</Button>
        <Button type="submit" variant="primary" disabled={!dirty || busy}>{t('prefixes.save')}</Button>
        {custom && (
          <Button type="button" variant="ghost" onClick={() => save('')} disabled={busy}>{t('prefixes.reset')}</Button>
        )}
      </form>
      <div className="mt-2.5 flex min-h-4 flex-wrap items-center gap-4">
        {error ? <ErrorNotice message={error} fallback={t('prefixError')} /> : <span />}
        {saved && (
          <span className="font-mono text-hint text-text-muted">
{t('prefixes.effective', { folder: folder.trim() || saved.prefixes_folder || saved.default_prefixes_folder || t('prefixes.placeholder') })}
          </span>
        )}
      </div>
    </section>
  );
}
