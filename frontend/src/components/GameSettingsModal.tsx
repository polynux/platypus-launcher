import React, { useEffect, useMemo, useState } from 'react';
import { X } from 'lucide-react';
import type { Game, GameJobKind, GameSettings, Job, ProtonVersion, StorePlugin, Tweak } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { GeneralSettings } from './SettingsModal';
import { CloudSavesPanel } from './CloudSavesPanel';
import { GameSettingsPanel } from './PluginSettings';
import { VersionsDialog } from './VersionsDialog';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { CustomizationTab } from './CustomizationTab';
import { MaintenanceTab } from './MaintenanceTab';
import { TweaksPanel } from './TweaksPanel';
import { cx } from './ui';
import { TAB_LABELS, pickTab, settingsTabs, type GameSettingsTab } from '../lib/gameSettingsTabs';

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
}

/** Everything about one game that is not playing it, in one place. A tab shows
 *  only where its content is on offer (see settingsTabs); the page keeps just
 *  Play or Install, the status and the key facts. */
export function GameSettingsModal(p: GameSettingsModalProps) {
  const { game, plugin } = p;
  const tabs = useMemo(() => settingsTabs({ game, plugin, tweaks: p.tweaks }), [game, plugin, p.tweaks]);
  const [tab, setTab] = useState<GameSettingsTab>(() => pickTab(tabs, p.initialTab));
  const [dirty, setDirty] = useState(false);
  // A tab that goes away (a capability switched off meanwhile) falls back to General.
  const shown = tabs.includes(tab) ? tab : 'general';
  useEffect(() => { if (shown !== tab) setTab(shown); }, [shown, tab]);

  return (
    <Modal label={`Settings for ${game.title}`} onClose={p.onClose} dismissable={!dirty} size="xl">
      <div className="@container">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h2 className="text-[20px] font-extrabold tracking-tight">Game settings</h2>
            <div className="truncate font-mono text-[11.5px] text-white/45">{game.title}</div>
          </div>
          <Button variant="neutral" size="icon" onClick={p.onClose} aria-label="Close" className="size-8 flex-none">
            <X className="size-4" />
          </Button>
        </div>

        <div className="grid gap-5 @2xl:grid-cols-[11rem_1fr]">
          <div
            role="tablist"
            aria-label="Game settings sections"
            aria-orientation="vertical"
            className="flex flex-wrap gap-1.5 @2xl:sticky @2xl:top-0 @2xl:flex-col @2xl:flex-nowrap @2xl:self-start"
          >
            {tabs.map((t) => (
              <button
                key={t}
                role="tab"
                id={`gs-tab-${t}`}
                aria-selected={t === shown}
                aria-controls={`gs-panel-${t}`}
                tabIndex={t === shown ? 0 : -1}
                onClick={() => setTab(t)}
                onKeyDown={(e) => {
                  const i = tabs.indexOf(t);
                  const next = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? tabs[(i + 1) % tabs.length]
                    : e.key === 'ArrowUp' || e.key === 'ArrowLeft' ? tabs[(i - 1 + tabs.length) % tabs.length] : null;
                  if (!next) return;
                  e.preventDefault();
                  setTab(next);
                  document.getElementById(`gs-tab-${next}`)?.focus();
                }}
                className={cx(
                  'rounded-xl px-3 py-2 text-left text-[12.5px] font-semibold transition',
                  t === shown ? 'bg-amber-300/[0.1] text-amber-100 ring-1 ring-amber-300/30' : 'text-white/60 hover:bg-white/[0.06] hover:text-white/90',
                )}
              >
                {TAB_LABELS[t]}
              </button>
            ))}
          </div>

          <div className="min-w-0">
            {/* General keeps its unsaved edits while another tab shows: it stays mounted. */}
            <section role="tabpanel" id="gs-panel-general" aria-labelledby="gs-tab-general" hidden={shown !== 'general'}>
              <GeneralSettings
                embedded
                game={game}
                runners={p.runners}
                onClose={p.onClose}
                onSave={p.onSaveSettings}
                onCopied={p.onCopied}
                onDirty={setDirty}
              />
            </section>
            {shown !== 'general' && (
              <section role="tabpanel" id={`gs-panel-${shown}`} aria-labelledby={`gs-tab-${shown}`}>
                <h3 className="mb-4 text-[16px] font-bold tracking-tight">{TAB_LABELS[shown]}</h3>
                <TabBody tab={shown} {...p} onOpenGeneral={() => setTab('general')} />
              </section>
            )}
          </div>
        </div>
      </div>
    </Modal>
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
      return plugin ? (
        <MaintenanceTab game={game} plugin={plugin} job={p.job} jobs={p.jobs} onJob={p.onJob} onJobAction={p.onJobAction} onNotify={p.onNotify} onClose={p.onClose} />
      ) : null;
    default:
      return null;
  }
}
