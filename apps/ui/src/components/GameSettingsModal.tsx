// i18n: converted
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X, ArrowLeft, ChevronDown, Cloud, GitBranch, Palette, Plug, Puzzle, SlidersHorizontal, Wand2, Wrench } from 'lucide-react';
import type { Game, GameJobKind, GameSettings, Job, ProtonVersion, RemoveOptions, StorePlugin, Tweak } from '../types';
import { Modal, type ModalSize } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';
import { parseReleaseYear } from './AddGameModal';
import { GeneralSettings, useGameSettingsDraft, type GameSettingsDraft } from './SettingsModal';
import { CloudSavesPanel } from './CloudSavesPanel';
import { GameSettingsPanel } from './PluginSettings';
import { VersionsDialog } from './VersionsDialog';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { CustomizationTab } from './CustomizationTab';
import { MaintenanceTab } from './MaintenanceTab';
import { TweaksPanel } from './TweaksPanel';
import { LegalSection } from './LegalSection';
import { cx } from './ui';
import { tabLabel, pickTab, settingsTabs, settingsGroups, tabSummary, type GameSettingsTab } from '../lib/gameSettingsTabs';
import { useVariant } from '../lib/theme';
import { MaintenancePrefetchProvider, useMaintenancePrefetchSource } from './maintenancePrefetch';

export interface GameSettingsModalProps {
  game: Game;
  plugin?: StorePlugin;
  runners: ProtonVersion[];
  tweaks: Tweak[];
  /** Every job in the queue: the verify dialog follows its own. */
  jobs: Job[];
  /** The game's active job, or its last failed one. */
  job?: Job;
  /** The tab to open (General when the game has no such tab). */
  initialTab?: GameSettingsTab;
  onClose: () => void;
  /** The general form was saved. */
  onSaveSettings: (s: GameSettings) => void;
  /** A tweak switched on or off. */
  onChangeSettings: (s: GameSettings, what: string) => void;
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** A job was queued or a pin changed: the library should reload. */
  onChanged: () => void;
  /** The game's look changed: the library should reload. */
  onCustomized: () => void;
  onCopied: () => void;
  /** Removes a sideloaded game from the library (custom games only). */
  onRemove?: (opts: RemoveOptions) => void;
  /** The game is starting or running: destructive actions wait. */
  running?: boolean;
}

/** The icon an accordion header shows (issue #129), one per category. */
const TAB_ICONS: Record<GameSettingsTab, React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>> = {
  general: SlidersHorizontal,
  cloud: Cloud,
  versions: GitBranch,
  options: Puzzle,
  plugin: Plug,
  tweaks: Wand2,
  customization: Palette,
  maintenance: Wrench,
};

/** Everything about one game that is not playing it, in one place. A tab shows
 *  only where its content is on offer (see settingsTabs); the page keeps just
 *  Play or Install, the status and the key facts. */
export function GameSettingsModal(p: GameSettingsModalProps) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const { game, plugin } = p;
  const tabs = useMemo(() => settingsTabs({ game, plugin, tweaks: p.tweaks }), [game, plugin, p.tweaks]);
  const [tab, setTab] = useState<GameSettingsTab>(() => pickTab(tabs, p.initialTab));
  const [dirty, setDirty] = useState(false);
  // A tab that goes away (a capability switched off meanwhile) falls back to General.
  const shown = tabs.includes(tab) ? tab : 'general';
  useEffect(() => { if (shown !== tab) setTab(shown); }, [shown, tab]);
  // Every tab shares one scroll container (issue #57): switching tabs keeps
  // the element and starts at its top.
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => { if (scrollRef.current) scrollRef.current.scrollTop = 0; }, [shown]);
  // One shared draft of the game's settings (issue #59): Suggested tweaks
  // and General's Advanced section edit the same object, so a save from
  // either tab writes the merged current values, never a stale copy.
  const shared = useGameSettingsDraft(game);
  const handleTweakChange = (s: GameSettings, what: string) => {
    // The toggle was computed from the shared draft (see TabBody), so
    // adopting it keeps General's unsaved edits in the saved payload.
    shared.applySaved(s);
    p.onChangeSettings(s, what);
  };
  // The Maintenance core calls start with the dialog (issue #56), so the data
  // is usually ready before the tab is clicked. The tab awaits these same
  // promises: one fetch per dialog open, shared by the tab.
  const maintenance = useMaintenancePrefetchSource(game);
  // The theme picks the presentation (issue #99): a dialog, a slide-in sheet
  // on the app's shared Modal, or a full view with a back entry. `cool` takes
  // the sheet with horizontal tabs; the built-in (warm) is today's dialog
  // with the tab list down the side on wide content.
  const presentation = useVariant('game-settings.presentation');
  const tabsVariant = useVariant('game-settings.tabs');
  const horizontalTabs = tabsVariant === 'horizontal';
  // The accordion (issue #129): the same categories, in two groups, one open
  // at a time, each header carrying its current values. Atlas draws it in the
  // sheet presentation; the axis itself only picks the arrangement.
  const accordion = tabsVariant === 'accordion';
  const [open, setOpen] = useState<GameSettingsTab | null>(() => pickTab(tabs, p.initialTab));
  // A section that goes away (a capability switched off meanwhile) closes to General.
  useEffect(() => { if (open && !tabs.includes(open)) setOpen('general'); }, [open, tabs]);
  // Opening another section closes the last one; General keeps its unsaved
  // edits because its panel stays mounted (hidden) below.
  const showTab = (name: GameSettingsTab) => { setTab(name); setOpen(name); };
  const accordionRef = useRef<HTMLDivElement>(null);
  // The focus starts on the first header, not on the close button (issue #129).
  // Radix mounts and focuses the dialog's content after this component's first
  // effects, so the move runs on the next tick.
  useEffect(() => {
    if (!accordion) return;
    const id = window.setTimeout(() => {
      accordionRef.current?.querySelector<HTMLButtonElement>('button[data-gs-accordion-header]')?.focus();
    }, 0);
    return () => window.clearTimeout(id);
  }, [accordion]);
  // An explicit dialog size wins; auto (and the sheet, whose width is its
  // own) keeps this dialog's built-in xl.
  const dialogSize = useVariant('dialog.size');
  const size: ModalSize = presentation === 'dialog' && dialogSize !== 'auto' ? (dialogSize as ModalSize) : 'xl';
  // The drawer is 520px wide, as in the /21 design; other sheets keep theirs
  // (the width only applies to a sheet, never to a dialog).
  const sheetWidth = accordion && presentation === 'sheet' ? 'min(520px, 92vw)' : undefined;

  const tabsRootClass = horizontalTabs
    ? 'flex min-h-0 flex-1 flex-col gap-5'
    : 'flex min-h-0 flex-1 flex-col gap-5 @2xl:grid @2xl:grid-cols-[11rem_1fr] @2xl:grid-rows-[minmax(0,1fr)]';
  const tablistClass = horizontalTabs
    ? '-mx-6 flex min-w-0 flex-none flex-nowrap gap-1.5 overflow-x-auto px-6 [scrollbar-width:none] [mask-image:linear-gradient(to_right,transparent,black_1.5rem,black_calc(100%-1.5rem),transparent)]'
    : 'flex flex-none flex-wrap gap-1.5 @2xl:flex-col @2xl:flex-nowrap @2xl:self-start';
  // Horizontal tabs stay on one row (D7), with no scrollbar; the fades sit in
  // the padding, so the first and last tabs are clear at rest. The active tab
  // scrolls into view when it changes, by click, arrow keys or the initial tab.
  useEffect(() => {
    if (!horizontalTabs) return;
    document.getElementById(`gs-tab-${shown}`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [shown, horizontalTabs]);

  const summary = (name: GameSettingsTab) => tabSummary(t, name, { game, plugin, runners: p.runners, tweaks: p.tweaks, job: p.job });
  // One body per category, shared by the tabs and the accordion.
  const body = (name: GameSettingsTab) => (name === 'general' ? (
    <>
      {game.store === 'custom' && <CustomGameYear game={game} onChanged={p.onCustomized} />}
      <GeneralSettings
        embedded
        game={game}
        runners={p.runners}
        onClose={p.onClose}
        onSave={p.onSaveSettings}
        onCopied={p.onCopied}
        onDirty={setDirty}
        onChanged={p.onCustomized}
        draftState={shared}
      />
      <LegalSection game={game} plugin={plugin} />
    </>
  ) : (
    <>
      {/* The cloud panel brings its own heading. */}
      {name !== 'cloud' && <h3 className="mb-4 text-[16px] font-bold tracking-tight">{tabLabel(t, name)}</h3>}
      <TabBody tab={name} {...p} onOpenGeneral={() => showTab('general')} shared={shared} onTweakChange={handleTweakChange} />
    </>
  ));

  const tabsBody = (
    <div ref={scrollRef} data-testid="gs-tab-scroll" className="min-h-0 min-w-0 flex-1 overflow-y-auto [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
      {/* General keeps its unsaved edits while another tab shows: it stays mounted. */}
      <section role="tabpanel" id="gs-panel-general" aria-labelledby="gs-tab-general" hidden={shown !== 'general'}>
        {body('general')}
      </section>
      {shown !== 'general' && (
        <section role="tabpanel" id={`gs-panel-${shown}`} aria-labelledby={`gs-tab-${shown}`}>
          {body(shown)}
        </section>
      )}
    </div>
  );

  // The accordion (issue #129): group headings, then one collapsible per
  // category. General's panel stays mounted while closed, like its tab, so
  // unsaved edits survive opening another section.
  const accordionBody = (
    <div ref={accordionRef} data-testid="gs-accordion" className="min-h-0 min-w-0 flex-1 overflow-y-auto pb-4 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
      {settingsGroups(tabs).map((group) => (
        <div key={group.level}>
          <div className="px-2 pb-1.5 pt-4 font-mono text-micro uppercase tracking-[0.14em] text-text-faint">
            {t(`gameSettings:accordion.${group.level}`)}
          </div>
          {group.tabs.map((name) => {
            const isOpen = open === name;
            const Icon = TAB_ICONS[name];
            return (
              <section key={name} className={cx('rounded-xl', isOpen && 'bg-surface')}>
                <h3 className="font-sans">
                  <button
                    type="button"
                    id={`gs-acc-${name}`}
                    data-gs-accordion-header
                    aria-expanded={isOpen}
                    aria-controls={isOpen || name === 'general' ? `gs-acc-panel-${name}` : undefined}
                    aria-describedby={`gs-acc-sum-${name}`}
                    onClick={() => setOpen(isOpen ? null : name)}
                    className="flex min-h-[46px] w-full items-center gap-2.5 rounded-xl px-3 text-left outline-none hover:bg-surface-strong focus-visible:ring-2 focus-visible:ring-focus/60"
                  >
                    <Icon className={cx('size-4 flex-none', isOpen ? 'text-accent' : 'text-text-muted')} aria-hidden />
                    <span className="text-[13px] font-bold text-text">{tabLabel(t, name)}</span>
                    <span id={`gs-acc-sum-${name}`} className="ml-auto max-w-[46%] truncate text-hint text-text-faint">{summary(name)}</span>
                    <ChevronDown className={cx('size-4 flex-none text-text-faint transition-transform', isOpen && 'rotate-180')} aria-hidden />
                  </button>
                </h3>
                {(isOpen || name === 'general') && (
                  <div
                    id={`gs-acc-panel-${name}`}
                    role="region"
                    aria-labelledby={`gs-acc-${name}`}
                    hidden={!isOpen}
                    className="px-3.5 pb-3 pl-[38px]"
                  >
                    {body(name)}
                  </div>
                )}
              </section>
            );
          })}
        </div>
      ))}
    </div>
  );

  const content = (
    <div className="@container flex h-full min-h-0 flex-col">
      <div className="mb-5 flex flex-none items-start justify-between gap-4">
        <div className="min-w-0">
          <h2 className="text-[20px] font-extrabold tracking-tight">
            {accordion ? t('gameSettings:settingsModal.allSettings') : t('gameSettings:settingsModal.heading')}
          </h2>
          <div className="truncate font-mono text-hint text-white/55">{game.title}</div>
        </div>
        {presentation !== 'page' && (
          <Button variant="neutral" size="icon" onClick={p.onClose} aria-label={t('common:close')} className="size-8 flex-none">
            <X className="size-4" />
          </Button>
        )}
      </div>

      {/* Fixed header; the tab list and this content column below are the
          only things that fill the dialog. The content column is the one
          scrollbar: the dialog itself never scrolls (no double scrollbars).
          Narrow widths stack the tabs above the content and keep the same
          outer height, with the content scrolling under the tabs. */}
      <MaintenancePrefetchProvider pre={maintenance}>
      {accordion ? accordionBody : (
        <div className={tabsRootClass}>
          <div
            role="tablist"
            aria-label={t('gameSettings:settingsModal.sections')}
            aria-orientation={horizontalTabs ? 'horizontal' : 'vertical'}
            className={tablistClass}
          >
            {tabs.map((name) => (
              <button
                key={name}
                role="tab"
                id={`gs-tab-${name}`}
                aria-selected={name === shown}
                aria-controls={`gs-panel-${name}`}
                tabIndex={name === shown ? 0 : -1}
                onClick={() => setTab(name)}
                onKeyDown={(e) => {
                  const i = tabs.indexOf(name);
                  const next = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? tabs[(i + 1) % tabs.length]
                    : e.key === 'ArrowUp' || e.key === 'ArrowLeft' ? tabs[(i - 1 + tabs.length) % tabs.length] : null;
                  if (!next) return;
                  e.preventDefault();
                  setTab(next);
                  document.getElementById(`gs-tab-${next}`)?.focus();
                }}
                className={cx(
                  'rounded-xl px-3 py-2 text-left text-[12.5px] font-semibold transition',
                  horizontalTabs && 'shrink-0',
                  name === shown ? 'bg-amber-300/[0.1] text-amber-100 ring-1 ring-amber-300/30' : 'text-white/60 hover:bg-white/[0.06] hover:text-white/90',
                )}
              >
                {tabLabel(t, name)}
              </button>
            ))}
          </div>

          {tabsBody}
        </div>
      )}
      </MaintenancePrefetchProvider>
      </div>
  );

  // A page is the settings as a full view, with a back entry instead of the
  // dialog chrome. GamePage shows it in place of the game when it is open.
  if (presentation === 'page') {
    return (
      <div className="p-7">
        <Button variant="ghost" size="none" onClick={p.onClose} className="gap-1.5 rounded px-0 text-[12px] font-semibold text-text-muted hover:bg-transparent hover:text-white">
          <ArrowLeft className="h-3.5 w-3.5" /> {t('common:back')}
        </Button>
        <div className="mt-4">{content}</div>
      </div>
    );
  }

  return (
    // Constant outer height across tabs (issue #57): about the Main tab's
    // height (43rem ~= the Main dialog's 688px at an 800px window, measured
    // in the app), clamped to the window. Only this dialog passes a height;
    // the shared Modal is untouched, so no other dialog changes size. A
    // sheet spans the full viewport height, so the fixed height is for the
    // dialog presentation only.
    <Modal label={t('gameSettings:settingsModal.title', { title: game.title })} onClose={p.onClose} dismissable={!dirty} size={size} presentation={presentation === 'sheet' ? 'sheet' : 'dialog'} sheetWidth={sheetWidth} className={presentation === 'sheet' ? undefined : 'h-[min(43rem,calc(100dvh-6rem))]'}>
      {content}
    </Modal>
  );
}

/** A sideloaded game's release year, editable in the General tab (issue #35).
 *  Empty clears it back to unknown. Saving reloads the library entry quietly,
 *  like a customization change. */
function CustomGameYear({ game, onChanged }: { game: Game; onChanged: () => void }) {
  const { t } = useTranslation('gameSettings');
  const [raw, setRaw] = useState(game.release_year ? String(game.release_year) : '');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  useEffect(() => { setRaw(game.release_year ? String(game.release_year) : ''); }, [game.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const parsed = parseReleaseYear(raw);
  const saved = game.release_year ?? 0;
  const dirty = !parsed.error && (parsed.year ?? 0) !== saved;
  const save = () => {
    if (parsed.error || !dirty || saving) return;
    setSaving(true);
    setError(null);
    Effect.runPromise(LauncherApi.setCustomGameYear(game.id, parsed.year ?? 0)).then(
      () => { setSaving(false); onChanged(); },
      (e) => { setSaving(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };
  return (
    <div className="mb-6">
      <span className="mb-1.5 block font-mono text-[12px] uppercase tracking-[0.14em] text-white/55">{t('gameSettings:year.kicker')}</span>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); save(); }}
      >
        <Input
          aria-label={t('gameSettings:year.aria')}
          value={raw}
          onChange={(e) => { setRaw(e.target.value); setError(null); }}
          placeholder={t('gameSettings:year.placeholder')}
          inputMode="numeric"
          className="w-32"
        />
        <Button type="submit" variant="primary" size="sm" disabled={!dirty || saving}>{t('gameSettings:year.save')}</Button>
        {saved > 0 && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={saving}
            onClick={() => {
              setSaving(true);
              setError(null);
              Effect.runPromise(LauncherApi.setCustomGameYear(game.id, 0)).then(
                () => { setRaw(''); setSaving(false); onChanged(); },
                (e) => { setSaving(false); setError(e instanceof Error ? e.message : String(e)); },
              );
            }}
          >
            {t('gameSettings:year.clear')}
          </Button>
        )}
      </form>
      {parsed.error
        // The message comes from the shared release-year check (owned by the
        // library area's AddGameModal): it turns French with that conversion.
        ? <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{parsed.error}</p>
        : error && <ErrorNotice message={error} fallback={t('gameSettings:year.saveError')} />}
    </div>
  );
}

function TabBody({ tab, onOpenGeneral, shared, onTweakChange, ...p }: GameSettingsModalProps & { tab: GameSettingsTab; onOpenGeneral: () => void; shared: GameSettingsDraft; onTweakChange: (s: GameSettings, what: string) => void }) {
  const { game, plugin } = p;
  const viaSteam = game.source === 'steam' && !game.executable;
  switch (tab) {
    case 'cloud':
      return <CloudSavesPanel game={game} plugin={plugin} framed={false} />;
    case 'versions':
      return plugin ? <VersionsDialog embedded game={game} plugin={plugin} onClose={p.onClose} onChanged={p.onChanged} onNotify={p.onNotify} /> : null;
    case 'options':
      return plugin ? <InstallOptionsDialog embedded game={game} plugin={plugin} mode="update" onClose={p.onClose} onChanged={p.onChanged} onNotify={p.onNotify} /> : null;
    case 'plugin':
      return plugin ? <GameSettingsPanel game={game} plugin={plugin} onNotify={p.onNotify} framed={false} /> : null;
    case 'tweaks':
      return (
        <TweaksPanel
          framed={false}
          // The shared draft (issue #59): the toggle reads the same settings
          // General edits, so its instant save never drops those edits.
          game={{ ...game, settings: shared.current }}
          // Proton-only env tweaks do nothing for a native build.
          tweaks={game.native ? p.tweaks.filter((t) => !Object.keys(t.env ?? {}).some((k) => /^(PROTON_|WINE_)/.test(k))) : p.tweaks}
          viaSteam={viaSteam}
          onChange={onTweakChange}
          onOpenSettings={onOpenGeneral}
        />
      );
    case 'customization':
      return <CustomizationTab game={game} onChanged={p.onCustomized} />;
    case 'maintenance':
      return (
        <MaintenanceTab game={game} plugin={plugin} job={p.job} jobs={p.jobs} onJob={p.onJob} onJobAction={p.onJobAction} onNotify={p.onNotify} onClose={p.onClose} onRemove={p.onRemove} running={p.running} />
      );
    default:
      return null;
  }
}
