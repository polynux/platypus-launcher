// i18n: converted
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';
import type { Game, GameJobKind, GameSettings, Job, ProtonVersion, RemoveOptions, StorePlugin, Tweak } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';
import { parseReleaseYear } from './AddGameModal';
import { GeneralSettings } from './SettingsModal';
import { CloudSavesPanel } from './CloudSavesPanel';
import { GameSettingsPanel } from './PluginSettings';
import { VersionsDialog } from './VersionsDialog';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { CustomizationTab } from './CustomizationTab';
import { MaintenanceTab } from './MaintenanceTab';
import { TweaksPanel } from './TweaksPanel';
import { LegalSection } from './LegalSection';
import { cx } from './ui';
import { tabLabel, pickTab, settingsTabs, type GameSettingsTab } from '../lib/gameSettingsTabs';
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
  // The Maintenance core calls start with the dialog (issue #56), so the data
  // is usually ready before the tab is clicked. The tab awaits these same
  // promises: one fetch per dialog open, shared by the tab.
  const maintenance = useMaintenancePrefetchSource(game);

  return (
    // Constant outer height across tabs (issue #57): about the Main tab's
    // height (43rem ~= the Main dialog's 688px at an 800px window, measured
    // in the app), clamped to the window. Only this dialog passes a height;
    // the shared Modal is untouched, so no other dialog changes size.
    <Modal label={t('gameSettings:settingsModal.title', { title: game.title })} onClose={p.onClose} dismissable={!dirty} size="xl" className="h-[min(43rem,calc(100dvh-6rem))]">
      <div className="@container flex h-full min-h-0 flex-col">
        <div className="mb-5 flex flex-none items-start justify-between gap-4">
          <div className="min-w-0">
            <h2 className="text-[20px] font-extrabold tracking-tight">{t('gameSettings:settingsModal.heading')}</h2>
            <div className="truncate font-mono text-hint text-white/55">{game.title}</div>
          </div>
          <Button variant="neutral" size="icon" onClick={p.onClose} aria-label={t('common:close')} className="size-8 flex-none">
            <X className="size-4" />
          </Button>
        </div>

        {/* Fixed header; the tab list and this content column below are the
            only things that fill the dialog. The content column is the one
            scrollbar: the dialog itself never scrolls (no double scrollbars).
            Narrow widths stack the tabs above the content and keep the same
            outer height, with the content scrolling under the tabs. */}
        <MaintenancePrefetchProvider pre={maintenance}>
        <div className="flex min-h-0 flex-1 flex-col gap-5 @2xl:grid @2xl:grid-cols-[11rem_1fr] @2xl:grid-rows-[minmax(0,1fr)]">
          <div
            role="tablist"
            aria-label={t('gameSettings:settingsModal.sections')}
            aria-orientation="vertical"
            className="flex flex-none flex-wrap gap-1.5 @2xl:flex-col @2xl:flex-nowrap @2xl:self-start"
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
                  name === shown ? 'bg-amber-300/[0.1] text-amber-100 ring-1 ring-amber-300/30' : 'text-white/60 hover:bg-white/[0.06] hover:text-white/90',
                )}
              >
                {tabLabel(t, name)}
              </button>
            ))}
          </div>

          <div ref={scrollRef} data-testid="gs-tab-scroll" className="min-h-0 min-w-0 flex-1 overflow-y-auto [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
            {/* General keeps its unsaved edits while another tab shows: it stays mounted. */}
            <section role="tabpanel" id="gs-panel-general" aria-labelledby="gs-tab-general" hidden={shown !== 'general'}>
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
              />
              <LegalSection game={game} plugin={plugin} />
            </section>
            {shown !== 'general' && (
              <section role="tabpanel" id={`gs-panel-${shown}`} aria-labelledby={`gs-tab-${shown}`}>
                {/* The cloud panel brings its own heading. */}
                {shown !== 'cloud' && <h3 className="mb-4 text-[16px] font-bold tracking-tight">{tabLabel(t, shown)}</h3>}
                <TabBody tab={shown} {...p} onOpenGeneral={() => setTab('general')} />
              </section>
            )}
          </div>
        </div>
        </MaintenancePrefetchProvider>
      </div>
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

function TabBody({ tab, onOpenGeneral, ...p }: GameSettingsModalProps & { tab: GameSettingsTab; onOpenGeneral: () => void }) {
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
          game={game}
          // Proton-only env tweaks do nothing for a native build.
          tweaks={game.native ? p.tweaks.filter((t) => !Object.keys(t.env ?? {}).some((k) => /^(PROTON_|WINE_)/.test(k))) : p.tweaks}
          viaSteam={viaSteam}
          onChange={p.onChangeSettings}
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
