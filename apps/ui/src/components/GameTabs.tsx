// i18n: converted
// The Atlas game page as tabs (issue #127, D21.jsx `.stage`): the hero with
// the logo, the action strip on the dim accent, and sticky tabs with the
// store page. Only the tabs structure renders here; sections keep today's
// GamePage path exactly, so warm, cool and slate render as before.
//
// Data stays with the page: every block takes props only (the hooks live in
// GamePage), so the tabs re-arrange blocks without forking data loading.
import { TuneFlyout } from './TuneFlyout';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { Effect } from 'effect';
import { AlertTriangle, ChevronDown, Cloud, CloudOff, Download, EyeOff, Loader2, Lock, Play, RefreshCw, SlidersHorizontal, Square, Trophy } from 'lucide-react';
import type { ActivityEvent, ArtCandidate, CloudInfo, CloudMode, Collection, Game, GameDetails, GameJobKind, GameSettings, GameStatus, Job, SaveBackup, SessionEvent, StorePlugin } from '../types';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Lightbox, type MediaItem } from './Lightbox';
import { CopyCompareDialog } from './CopyCompareDialog';
import { PluginLogo } from './PluginLogo';
import { CrashHintsPanel } from './CrashHints';
import { CapabilitySurface } from '../lib/capabilities';
import { agoLabel, asCloudMode, modeHelp, modeName, stateChip } from '../lib/cloud';
import { CloudConflictDialog } from './CloudSavesConflict';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { JobProgress } from './JobProgress';
import { useCrashHints } from '../lib/useCrashHints';
import { useCompat } from '../lib/useCompat';
import { LauncherApi, onCloud, onSession } from '../effect/runtime';
import { useVariant } from '../lib/theme';
import { formatBytes, hoursPlayed, storeLabel } from '../lib/games';
import { currentLocale, formatClock, formatDate, formatPercentValue } from '../lib/format';
import { formatDecimal1 } from '../lib/gameLocale';
import { playedCopy, pickableCopies } from '../lib/family';
import { coreErrMsg } from '../lib/coreError';
import { cx } from './ui';
import {
  AboutSection,
  GameBackLink,
  GameInstallNote,
  GameOutsideAlert,
  TabsNoteLink,
} from './GamePage';
import type { useAchievementProgress } from './ProgressAchievements';
import type { InstallSize } from '../lib/installSize';

type TabId = 'store' | 'achievements' | 'activity' | 'saves' | 'compat';

export interface GameTabsData {
  game: Game;
  hero: string[];
  plugin?: StorePlugin;
  runners: { id: string; name: string }[];
  activity: ActivityEvent[];
  backLabel: string;
  onBack: () => void;
  onPlay: () => void;
  onStop: () => void;
  onForceStop: () => void;
  launching: boolean;
  session?: SessionEvent;
  sessionStartedAt?: number;
  refreshing?: boolean;
  job?: Job;
  sourceLogos?: Record<string, string>;
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onAllowFolder: (folder: string) => void;
  onNavigate: (v: { name: string }) => void;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onCopyChanged?: () => void;
  installing: boolean;
  installSize: InstallSize;
  canChooseOptions: boolean;
  canChooseVersion: boolean;
  onInstall: () => void;
  onInstallTo?: () => void;
  onOpenVersions: () => void;
  onOpenPlugins: () => void;
  media: MediaItem[];
  details: GameDetails | null;
  loading: boolean;
  error: string | null;
  compatUrls: { protondb?: string; wiki?: string };
  onReload: () => void;
  onSearchInfo: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLinkInfo: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  onUnlinkInfo: (id: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  onOpenStore?: () => void;
  hours: number | null;
  last?: string;
  runsWith: string;
  protondb: string;
  progress: ReturnType<typeof useAchievementProgress>;
  ach: GameDetails['achievements'];
  events: ActivityEvent[];
  compatText: string;
  facts: [string, string][];
  onOpenRunner: () => void;
  /** Opens the game settings (the drawer in atlas). */
  onOpenSettings: () => void;
  /** The staged game writes through here, so App's copy stays current. */
  onTuneSaved?: (settings: GameSettings) => void;
  /** The list-detail stage shows no back link (it would go nowhere). */
  hideBack?: boolean;
  /** Atlas parity (issue #132): the game's collections for the side section. */
  collections?: Collection[];
  /** Opens the collections dialog for this game. */
  onManageCollections?: (gameId: string) => void;
  /** Saves the user status through the page (the core call lives there). */
  onSaveStatus?: (id: string, status: GameStatus) => Promise<{ status: GameStatus }>; // i18n:allow: Promise return type, not user-facing text
  /** Saves the user notes through the page (the core call lives there). */
  onSaveNotes?: (id: string, text: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  /** The status or notes changed: the library should reload to carry it. */
  onStatusChanged?: () => void;
}

/** Ticks once a second while mounted, so the strip elapsed time stays live. */
function useNow(active: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [active]);
  return now;
}

function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
}

/** The cloud save state for the strip: the last known info, refreshed on
 *  cloud and session events (the same calls CloudSavesPanel makes). */
function useStripCloud(game: Game) {
  const [info, setInfo] = useState<CloudInfo | null>(null);
  const ref = useRef(game);
  ref.current = game;
  const refresh = useCallback(() => Effect.runPromise(LauncherApi.getCloudInfo(ref.current)).then(setInfo).catch(() => {}), []);
  useEffect(() => {
    setInfo(null);
    Effect.runPromise(LauncherApi.getCloudInfo(game)).then(setInfo).catch(() => {});
  }, [game.id]);
  useEffect(() => onCloud((e) => { if (e.game_id === game.id && e.kind !== 'progress') void refresh(); }), [game.id, refresh]);
  useEffect(() => onSession((e) => { if (e.game_id === game.id) void refresh(); }), [game.id, refresh]);
  return { info, refresh };
}

/** Hero art: the same fallback chain as today (details hero, then the
 *  library images). Many test games have only cover art. */
function heroSrc(hero: string[], details: GameDetails | null): string | null {
  if (details?.hero_url) return details.hero_url;
  return hero[0] ?? null;
}

function AtlasHero({ game, hero, details }: { game: Game; hero: string[]; details: GameDetails | null }) {
  const logoMode = useVariant('game-hero.logo');
  const src = heroSrc(hero, details);
  const [broken, setBroken] = useState<string | null>(null);
  useEffect(() => setBroken(null), [game.id]);
  const logo = logoMode === 'logo' && game.logo_url && game.logo_url !== broken ? game.logo_url : null;
  return (
    <div className="relative flex-none overflow-hidden bg-black/40" style={{ height: 'clamp(200px, 34vh, 380px)' }}>
      {src && (
        <img
          key={src}
          src={src}
          alt=""
          aria-hidden
          loading="lazy"
          className="h-full w-full object-cover"
          style={{ objectPosition: 'center 30%' }}
        />
      )}
      {logo ? (
        <img
          src={logo}
          alt={game.title}
          onError={() => setBroken(logo)}
          className="absolute bottom-[22px] left-7 max-h-[44%] object-contain object-left-bottom"
          style={{ maxWidth: 'min(340px, 38%)', filter: 'drop-shadow(0 3px 14px rgba(0, 0, 0, 0.6))' }}
        />
      ) : (
        <h1 className="absolute bottom-[22px] left-7 max-w-[min(640px,80%)] truncate text-[32px] font-extrabold tracking-tight text-text">{game.title}</h1>
      )}
    </div>
  );
}

function StripPlay({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const { game, job, session, launching } = d;
  const active = job && (job.state === 'queued' || job.state === 'running');
  const running = session?.state === 'running' || session?.state === 'starting';
  const starting = launching || session?.state === 'starting';
  const now = useNow(!!running && d.sessionStartedAt != null);
  if (active && job) return <JobProgress job={job} onCancel={(id) => d.onJobAction('cancel', id)} />;
  if (running && session) {
    const elapsed = d.sessionStartedAt != null ? formatElapsed(now - d.sessionStartedAt) : null;
    return (
      <span className="inline-flex items-center gap-2.5">
        <Button
          variant="neutral"
          size="none"
          onClick={d.onStop}
          className="inline-flex h-[42px] items-center gap-2 rounded-[9px] bg-black/30 px-5 text-[15px] font-extrabold text-text ring-1 ring-accent/60 hover:bg-black/40"
        >
          {starting ? <Loader2 className="h-[15px] w-[15px] animate-spin" aria-hidden /> : <Square className="h-3 w-3 fill-current" aria-hidden />}
          <span>{starting ? t('game:strip.starting') : t('game:play.stop')}</span>
          {elapsed && <span className="font-mono text-[12px] font-medium tabular-nums">{elapsed}</span>}
        </Button>
      </span>
    );
  }
  if (game.is_installed) {
    return (
      <Button
        variant="primary"
        size="none"
        onClick={d.onPlay}
        disabled={launching || game.outside_folder}
        className="inline-flex h-[42px] items-center gap-2 rounded-[9px] bg-accent px-6 pl-5 text-[15px] font-extrabold text-on-accent hover:brightness-110 disabled:opacity-80"
      >
        {launching ? <Loader2 className="h-[15px] w-[15px] animate-spin" aria-hidden /> : <Play className="h-[15px] w-[15px] fill-current" aria-hidden />}
        <span>{launching ? t('game:play.launching') : t('game:play.play')}</span>
      </Button>
    );
  }
  if (!game.installable) {
    return <span className="inline-flex h-[42px] items-center rounded-[9px] bg-surface px-5 text-[13px] font-bold text-text-muted">{t('game:install.cantInstall')}</span>;
  }
  const size = d.installSize.state === 'known' && d.installSize.total ? formatBytes(d.installSize.total) : null;
  return (
    <Button
      variant="primary"
      size="none"
      onClick={d.onInstall}
      className="inline-flex h-[42px] items-center gap-2 rounded-[9px] bg-accent px-6 pl-5 text-[15px] font-extrabold text-on-accent hover:brightness-110"
    >
      <Download className="h-[15px] w-[15px]" aria-hidden />
      <span>{d.job?.state === 'failed' ? t('game:install.retryInstall') : t('game:install.install')}</span>
      {size && <span className="font-mono text-[12px] font-medium">{size}</span>}
    </Button>
  );
}

function StoreSwitch({ d }: { d: GameTabsData }) {
  const { t } = useTranslation(['game', 'gameOps']);
  const { game, plugin } = d;
  const copies = pickableCopies(game);
  const [busy, setBusy] = useState(false);
  // The classic CopyPicker's compare dialog, next to the switch (issue #132).
  const [comparing, setComparing] = useState(false);
  useEffect(() => { setBusy(false); setComparing(false); }, [game.id]);
  // The same gate as the classic details: the dialog needs the comparison the
  // store reported, and a choice only exists with two or more copies.
  const compare = copies.length > 1 ? game.copy_compare : undefined;
  const played = playedCopy(game);
  const pick = (ownerId: string) => {
    if (busy) return;
    setBusy(true);
    Effect.runPromise(LauncherApi.setPreferredCopy(game, ownerId)).then(
      () => { setBusy(false); d.onCopyChanged?.(); },
      (e: Error) => { setBusy(false); d.onNotify?.(e.message || String(e), 'err'); },
    );
  };
  const storeName = game.store === 'custom' ? t('game:store.sideloaded') : storeLabel(game.store);
  return (
    <>
      {copies.length > 1 ? (
        <div role="radiogroup" aria-label={t('game:strip.launchFrom')} className="inline-flex gap-0.5 rounded-[10px] bg-black/30 p-[3px]">
          {copies.map((c) => {
            const on = c.owner_id === played;
            return (
              <button
                key={c.owner_id}
                type="button"
                role="radio"
                aria-checked={on}
                disabled={busy || !c.available}
                onClick={() => pick(c.owner_id)}
                title={c.note || c.owner_name}
                className={cx(
                  'inline-flex h-9 items-center gap-1.5 rounded-lg px-2.5 text-[13px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  on ? 'bg-accent/[0.22] text-text ring-1 ring-accent/50' : 'text-text-muted hover:text-text',
                )}
              >
                <PluginLogo src={d.sourceLogos?.[game.source] ?? d.sourceLogos?.[game.store]} name={c.owner_name} size="xs" />
                {c.owner_name}
              </button>
            );
          })}
        </div>
      ) : (
        <span className="inline-flex h-[42px] items-center gap-1.5 rounded-[10px] bg-black/30 px-3 text-[13px] font-semibold text-text">
          <PluginLogo src={d.sourceLogos?.[game.source] ?? d.sourceLogos?.[game.store]} name={storeName} size="xs" />
          {storeName}
          {plugin && game.source !== game.store && <span className="text-text-muted">· {plugin.name}</span>}
        </span>
      )}
      {compare && (
        <button
          type="button"
          onClick={() => setComparing(true)}
          className="inline-flex h-[42px] items-center rounded-[10px] px-3 font-sans text-[13px] font-semibold text-accent transition hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          {t('gameOps:copies.compare')}
        </button>
      )}
      {comparing && compare && (
        <CopyCompareDialog game={game} copies={copies} compare={compare} onClose={() => setComparing(false)} />
      )}
    </>
  );
}

function SaveState({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const { info, refresh } = useStripCloud(d.game);
  const [resolving, setResolving] = useState(false);
  const st = info?.status ?? null;
  if (st?.state === 'conflict' && (st.conflicts?.length ?? 0) > 0) {
    return (
      <span className="inline-flex items-center">
        <button
          type="button"
          onClick={() => setResolving(true)}
          className="inline-flex h-10 items-center gap-1.5 rounded-xl bg-danger/[0.16] px-2.5 font-semibold text-text transition hover:bg-danger/[0.22] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <AlertTriangle className="h-3.5 w-3.5" aria-hidden />
          {t('game:strip.saveConflict')}
        </button>
        {resolving && (
          <CloudConflictDialog
            game={d.game}
            status={st}
            onClose={() => setResolving(false)}
            onResolved={(next) => { setResolving(false); void refresh().then(() => next); }}
          />
        )}
      </span>
    );
  }
  // Only a real sync spins. No status yet is either still loading (nothing)
  // or not checked (a static cloud): a spinner here showed "Uploading save"
  // on every game, including ones that never sync.
  const syncing = !!info?.busy;
  if (st?.state === 'synced' && !syncing) {
    return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><Cloud className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveSynced')}</span>;
  }
  if (syncing) {
    return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><RefreshCw className="h-3.5 w-3.5 animate-spin" aria-hidden />{t('game:strip.saveSyncing')}</span>;
  }
  if (!info) return null;
  if (info.mode === 'off') return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><CloudOff className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveOff')}</span>;
  return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><Cloud className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveUnchecked')}</span>;
}

function ActionStrip({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const [tuneOpen, setTuneOpen] = useState(false);
  // Another game on the stage never inherits the open flyout.
  useEffect(() => setTuneOpen(false), [d.game.id]);
  const active = d.job && (d.job.state === 'queued' || d.job.state === 'running');
  // The outside-folder alert below already names the folders: repeating the
  // store note here would show the same long paths twice (a5).
  const outside = !!d.game.outside_folder;
  const showNote = !active && ((d.game.note && !outside) || d.installing);
  return (
    <div className="flex flex-wrap items-center gap-2.5 bg-accent-dim py-3 pl-6 pr-5">
      <div className="mr-2 flex items-center gap-1.5">
        <StripPlay d={d} />
        <span className="relative">
          <button
            type="button"
            data-tune-anchor
            aria-expanded={tuneOpen}
            aria-haspopup="dialog"
            onClick={() => setTuneOpen((v) => !v)}
            className={cx(
              'inline-flex h-[42px] items-center gap-1.5 rounded-[9px] bg-black/30 px-3 pl-3.5 text-[14px] font-bold text-text transition hover:bg-black/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              tuneOpen && 'bg-black/40',
            )}
          >
            <SlidersHorizontal className="h-4 w-4" aria-hidden />
            {t('game:strip.tune')}
            <ChevronDown className="h-3.5 w-3.5" aria-hidden />
          </button>
          <TuneFlyout game={d.game} open={tuneOpen} onClose={() => setTuneOpen(false)} onAllSettings={d.onOpenSettings} onSaved={d.onTuneSaved} />
        </span>
      </div>
      {d.game.update_available && d.game.is_installed && (
        <Button variant="neutral" size="sm" onClick={() => d.onJob('update')} className="gap-1.5">
          <RefreshCw className="h-3.5 w-3.5" /> {t('game:strip.update')}
        </Button>
      )}
      <StoreSwitch d={d} />
      <SaveState d={d} />
      {/* /21 has no row between the strip and the tabs: the install note
          and folder/version links sit inline at the end of the strip. Only
          the short links show here (the long folder path would truncate
          them): the full path lives in the title tooltip. */}
      {(showNote || d.refreshing) && (
        <span
          className="ml-auto min-w-0 max-w-[38%] truncate text-[12.5px] text-text-muted"
          title={d.refreshing || !d.installing || !d.plugin ? undefined : d.plugin.install_folder}
        >
          {d.refreshing ? (
            t('game:refreshing')
          ) : (
            <>
              {d.game.note && !outside && <span>{d.game.note} </span>}
              {d.installing && d.plugin && (
                <span>
                  {d.canChooseVersion ? (
                    <Trans
                      i18nKey="game:install.stripWithVersion"
                      components={{
                        v: <TabsNoteLink onClick={d.onOpenVersions} />,
                        d: <TabsNoteLink onClick={d.onOpenPlugins} />,
                      }}
                    />
                  ) : (
                    <Trans
                      i18nKey="game:install.stripPlain"
                      components={{ d: <TabsNoteLink onClick={d.onOpenPlugins} /> }}
                    />
                  )}
                </span>
              )}
            </>
          )}
        </span>
      )}
    </div>
  );
}

function GallerySheet({ media, title }: { media: MediaItem[]; title: string }) {
  const { t } = useTranslation('game');
  const [index, setIndex] = useState<number | null>(null);
  useEffect(() => setIndex(null), [title]);
  const shots = useMemo(() => media.filter((m) => m.kind !== 'video').slice(0, 7), [media]);
  if (shots.length === 0) return null;
  return (
    <section aria-label={t('game:storeTab.pictures')}>
      <h2 className="mb-2.5 flex items-baseline justify-between text-[13px] font-bold text-text-muted">
        <span>{t('game:storeTab.pictures')}</span>
        <small className="text-[12px] font-medium text-text-faint">{t('game:storeTab.picturesCount', { count: media.length })}</small>
      </h2>
      <div className="grid grid-cols-2 gap-1">
        {shots.map((m, i) => (
          <button
            key={`${m.src}-${i}`}
            type="button"
            onClick={() => setIndex(i)}
            aria-label={t('game:media.screenshotLabel', { index: i + 1 })}
            className={cx(
              'overflow-hidden rounded-md bg-black/40 ring-1 ring-hairline transition hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              i === 0 && 'col-span-2',
            )}
          >
            <img src={m.src} alt="" loading="lazy" className={cx('w-full object-cover', i === 0 ? 'aspect-video' : 'aspect-video')} />
          </button>
        ))}
      </div>
      {index != null && shots[index] && <Lightbox items={shots} index={index} alt={title} onIndex={setIndex} onClose={() => setIndex(null)} />}
    </section>
  );
}

function YouGrid({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const played = d.hours != null ? t('game:hero.hoursLogged', { hours: formatDecimal1(d.hours) }) : '—';
  const cells: [string, string][] = [
    [t('game:storeTab.played'), played],
    [t('game:storeTab.lastPlayed'), d.last ?? t('game:storeTab.neverPlayed')],
    [t('game:storeTab.runsWith'), d.runsWith || t('game:storeTab.unknownRunner')],
    [t('game:storeTab.protondb'), d.protondb || t('game:storeTab.noRating')],
  ];
  return (
    <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-[10px] bg-hairline text-[13.5px] leading-[normal]">
      {cells.map(([k, v]) => (
        <div key={k} className="bg-surface px-3 py-2.5">
          <dt className="text-[11.5px] text-text-faint">{k}</dt>
          <dd className="truncate font-[650] text-text" title={v}>{v}</dd>
        </div>
      ))}
    </dl>
  );
}

function AchSummary({ d, onOpen }: { d: GameTabsData; onOpen: () => void }) {
  const { t } = useTranslation('game');
  const total = d.progress?.total ?? d.ach?.total ?? 0;
  const unlocked = d.progress?.unlocked ?? d.ach?.unlocked ?? 0;
  if (!total) return null;
  const pct = total ? Math.round((unlocked / total) * 100) : 0;
  // The lab's .pg-achsum: no card, a trophy icon heading with "**2** of 5",
  // the bar, the last unlocked as rows, and an "Up next" icons row.
  const list = d.progress?.list ?? d.ach?.list ?? [];
  const recent = list.filter((a) => a.unlocked).slice(0, 3);
  const next = list.filter((a) => !a.unlocked && !('hidden' in a && (a as { hidden?: boolean }).hidden)).sort((x, y) => (y.global_percent ?? -1) - (x.global_percent ?? -1)).slice(0, 3);
  return (
    <section aria-label={t('game:storeTab.achievements')}>
      <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('game:storeTab.achievements')}</h2>
      <div className="grid gap-3">
        <div className="flex items-center gap-2 text-[13px]">
          <Trophy className="h-4 w-4 flex-none text-accent" aria-hidden />
          <span className="flex-1 text-text-muted"><b className="text-[15px] font-bold text-text">{unlocked}</b> {t('game:storeTab.of')} {total}</span>
          <span className="text-[12px] tabular-nums text-text-muted">{pct}%</span>
        </div>
        <div className="h-1.5 rounded-full bg-hairline" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={unlocked} aria-label={t('game:progress.barLabel')}>
          <div className="h-full rounded-full bg-accent" style={{ width: `${pct}%` }} />
        </div>
        {recent.length > 0 && (
          <ul className="grid gap-2">
            {recent.map((a) => (
              <li key={a.id || a.name} className="flex min-w-0 items-center gap-2.5">
                {a.icon ? (
                  <img src={a.icon} alt="" loading="lazy" className="h-[34px] w-[34px] flex-none rounded-md object-cover" />
                ) : (
                  <span className="grid h-[34px] w-[34px] flex-none place-items-center rounded-md bg-surface-strong"><Trophy className="h-4 w-4 text-text-muted" aria-hidden /></span>
                )}
                <span className="grid min-w-0">
                  <span className="truncate text-[13px] font-medium text-text">{a.name}</span>
                  <small className="truncate text-[12px] text-text-muted">{achRowMeta(t, a)}</small>
                </span>
              </li>
            ))}
          </ul>
        )}
        {next.length > 0 && unlocked < total && (
          <div className="flex items-center justify-between gap-2.5">
            <span className="text-[12.5px] text-text-muted">{t('game:storeTab.upNext')}</span>
            <span className="flex gap-1.5">
              {next.map((a) => (
                <span key={a.id || a.name} title={`${a.name}: ${a.description}`}>
                  {a.icon ? (
                    <img src={a.icon} alt="" loading="lazy" className="h-[30px] w-[30px] rounded-md object-cover opacity-70 grayscale" />
                  ) : (
                    <span className="grid h-[30px] w-[30px] place-items-center rounded-md bg-surface-strong"><Trophy className="h-3.5 w-3.5 text-text-muted" aria-hidden /></span>
                  )}
                </span>
              ))}
            </span>
          </div>
        )}
        <button type="button" onClick={onOpen} className="justify-self-start font-sans text-[12px] font-semibold text-accent hover:underline">
          {t('game:storeTab.seeAll', { total })}
        </button>
      </div>
    </section>
  );
}

/** When an achievement was unlocked, as a relative time (the lab's ago():
 *  minutes, hours, days, months, years — never a calendar date). Under a day
 *  it reuses the cloud agoLabel (minutes/hours, localized); beyond that it
 *  counts days and months like the /21 list rows. */
function achUnlockedWhen(t: (key: string, vars?: Record<string, unknown>) => string, atSeconds: number, nowMs = Date.now()): string {
  const s = Math.max(0, Math.round(nowMs / 1000 - atSeconds));
  if (s < 3600) {
    const when = agoLabel(atSeconds, nowMs);
    return t('game:progress.unlockedAgo', { when });
  }
  if (s < 86400) {
    const when = agoLabel(atSeconds, nowMs);
    return t('game:progress.unlockedAgo', { when });
  }
  const d = s / 86400;
  if (d < 2) return t('game:progress.unlockedAgo', { when: t('game:progress.yesterday' as never) as string });
  if (d < 30) {
    const c = Math.round(d);
    const when = t(c === 1 ? 'game:progress.daysAgo_one' : 'game:progress.daysAgo_other', { count: c }) as string;
    return t('game:progress.unlockedAgo', { when });
  }
  if (d < 365) {
    const c = Math.round(d / 30);
    const when = t(c === 1 ? 'game:progress.monthsAgo_one' : 'game:progress.monthsAgo_other', { count: c }) as string;
    return t('game:progress.unlockedAgo', { when });
  }
  const c = Math.round(d / 365);
  const when = t(c === 1 ? 'game:progress.yearsAgo_one' : 'game:progress.yearsAgo_other', { count: c }) as string;
  return t('game:progress.unlockedAgo', { when });
}

/** The second line under an achievement row: when it was unlocked, else the
 *  global share (dates and percents go through lib/format for the locale). */
function achRowMeta(t: (key: string, vars?: Record<string, unknown>) => string, a: { unlocked: boolean; unlocked_at?: number | null; global_percent?: number | null }): string {
  if (a.unlocked && a.unlocked_at) return achUnlockedWhen(t, a.unlocked_at);
  if (a.global_percent) return t('game:progress.shareOfPlayers', { share: formatPercentValue(a.global_percent) });
  return '';
}

type AchShow = 'all' | 'got' | 'locked';
type AchSort = 'recent' | 'rare';

interface AchRow {
  id: string;
  name: string;
  desc: string;
  icon: string;
  hidden: boolean;
  unlocked: boolean;
  at: number | null;
  pct: number | null;
}

/** The achievements tab as rows (the lab's AchList over .pg-achrows): two
 *  segment controls (All/Unlocked/Locked + Recent/Rarest) above the rows.
 *  Atlas-only (this file renders only for game.structure = tabs). Locked
 *  hidden achievements stay secret until unlocked, like the lab. */
/** The achievement row icon with the locked dim + lock overlay. Exported for tests. */
export function AchRowIcon({ a }: { a: AchRow }) {
  const [broken, setBroken] = useState(false);
  if (!a.icon || broken) {
    return (
      <span className="relative block h-11 w-11 flex-none overflow-hidden rounded-md bg-surface-strong">
        <span className="grid h-full w-full place-items-center"><Trophy className="h-4 w-4 text-text-muted" aria-hidden /></span>
        {!a.unlocked && <Lock className="absolute bottom-[3px] right-[3px] h-3.5 w-3.5 text-white drop-shadow-[0_1px_2px_rgba(0,0,0,1)]" aria-hidden />}
      </span>
    );
  }
  return (
    <span className="relative block h-11 w-11 flex-none overflow-hidden rounded-md bg-surface-strong">
      <img src={a.icon} alt="" loading="lazy" onError={() => setBroken(true)} className={cx('h-full w-full object-cover', !a.unlocked && 'opacity-50 grayscale')} />
      {!a.unlocked && <Lock className="absolute bottom-[3px] right-[3px] h-3.5 w-3.5 text-white drop-shadow-[0_1px_2px_rgba(0,0,0,1)]" aria-hidden />}
    </span>
  );
}

function AchRows({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const [show, setShow] = useState<AchShow>('all');
  const [sort, setSort] = useState<AchSort>('recent');
  useEffect(() => { setShow('all'); setSort('recent'); }, [d.game.id]);
  const { all, rows } = useMemo(() => {
    const src: { id: string; name: string; description: string; icon: string; hidden?: boolean; unlocked: boolean; unlocked_at?: number | null; global_percent?: number | null }[] =
      d.progress?.list ?? d.ach?.list ?? [];
    const all: AchRow[] = src.map((a) => ({
      id: a.id || a.name,
      name: a.name,
      desc: a.description,
      icon: a.icon,
      hidden: !!a.hidden,
      unlocked: a.unlocked,
      at: a.unlocked && a.unlocked_at ? a.unlocked_at : null,
      pct: a.global_percent ?? null,
    }));
    const shown = all.filter((a) => (show === 'all' ? true : show === 'got' ? a.unlocked : !a.unlocked));
    const list = [...shown];
    if (sort === 'rare') {
      list.sort((x, y) => (x.pct ?? 101) - (y.pct ?? 101));
    } else {
      list.sort((x, y) => {
        if (x.unlocked !== y.unlocked) return x.unlocked ? -1 : 1;
        if (x.unlocked) return (y.at ?? 0) - (x.at ?? 0);
        return (y.pct ?? -1) - (x.pct ?? -1);
      });
    }
    return { all, rows: list };
  }, [d.progress, d.ach, show, sort]);
  const total = d.progress?.total ?? d.ach?.total ?? all.length;
  const nGot = all.filter((a) => a.unlocked).length;
  const seg = (active: boolean) =>
    cx(
      'h-[26px] rounded-md px-2.5 text-[13px] tabular-nums transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
      active ? 'bg-surface-strong text-text ring-1 ring-hairline' : 'text-text-muted hover:text-text',
    );
  return (
    <div className="grid max-w-[920px] gap-3 px-6 pt-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div role="radiogroup" aria-label={t('game:achList.show')} className="inline-flex gap-0.5 rounded-lg bg-surface p-0.5 ring-1 ring-hairline">
          {([['all', t('game:achList.all', { count: all.length })], ['got', t('game:achList.unlocked', { count: nGot })], ['locked', t('game:achList.locked', { count: all.length - nGot })]] as [AchShow, string][]).map(([id, label]) => (
            <button key={id} type="button" role="radio" aria-checked={show === id} onClick={() => setShow(id)} className={seg(show === id)}>
              {label}
            </button>
          ))}
        </div>
        <div role="radiogroup" aria-label={t('game:achList.sort')} className="inline-flex gap-0.5 rounded-lg bg-surface p-0.5 ring-1 ring-hairline">
          {([['recent', t('game:achList.recent')], ['rare', t('game:achList.rarest')]] as [AchSort, string][]).map(([id, label]) => (
            <button key={id} type="button" role="radio" aria-checked={sort === id} onClick={() => setSort(id)} className={seg(sort === id)}>
              {label}
            </button>
          ))}
        </div>
      </div>
      {rows.length === 0 ? (
        <p className="py-4 text-[13px] text-text-muted">{t('game:achList.none')}</p>
      ) : (
        <ul>
          {rows.map((a) => {
            const secret = a.hidden && !a.unlocked;
            return (
              <li key={a.id} className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 border-b border-hairline py-[9px]">
                <AchRowIcon a={a} />
                <span className="grid min-w-0 gap-0.5">
                  <span className={cx('truncate text-[13.5px] font-semibold', !a.unlocked && 'text-text-muted')}>{secret ? t('game:achList.hiddenName') : a.name}</span>
                  <span className="flex items-center gap-1 text-[12.5px] text-text-muted">
                    {secret ? (<><EyeOff className="h-3 w-3 flex-none" aria-hidden />{t('game:achList.hiddenDesc')}</>) : a.desc}
                  </span>
                </span>
                <span className="grid justify-items-end gap-[3px] whitespace-nowrap text-[12px]">
                  {a.unlocked && a.at ? (
                    <span className="text-text">{achUnlockedWhen(t, a.at)}</span>
                  ) : (
                    <span className="text-text-faint">{t('game:achList.lockedLabel')}</span>
                  )}
                  {a.pct != null && <span className="tabular-nums text-text-muted">{t('game:progress.shareOfPlayers', { share: formatPercentValue(a.pct) })}</span>}
                </span>
              </li>
            );
          })}
        </ul>
      )}
      {total > all.length && (
        <p className="text-[12px] text-text-muted">
          {d.progress
            ? t('game:progress.showing', { shown: all.length, total })
            : t('game:achievements.showing', { shown: all.length, total, reason: t('game:achievements.reasonHighlight') })}
        </p>
      )}
    </div>
  );
}

function DetailsSide({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const det = d.details;
  const rows: [string, string][] = [
    ...(det?.developers?.length ? [[t('game:about.developer'), det.developers.join(', ')] as [string, string]] : []),
    ...(det?.publishers?.length ? [[t('game:about.publisher'), det.publishers.join(', ')] as [string, string]] : []),
    ...(d.game.release_year ? [[t('game:about.release'), String(d.game.release_year)] as [string, string]] : det?.release_date ? [[t('game:about.release'), det.release_date] as [string, string]] : []),
    ...(det?.genres?.length ? [[t('game:about.genres'), det.genres.join(' · ')] as [string, string]] : []),
  ];
  if (rows.length === 0) return null;
  return (
    <section aria-label={t('game:storeTab.details')}>
      <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('game:storeTab.details')}</h2>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3">
        {rows.map(([k, v]) => (
          <div key={k} className="min-w-0">
            <dt className="mb-0.5 text-[12px] text-text-muted">{k}</dt>
            <dd className="truncate text-[13px] font-[550] text-text" title={v}>{v}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

// Atlas parity (issue #132): the game's collections as small chips with a
// Manage entry, in the /21 side-section language (13px muted heading).
function AtlasCollections({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('collections');
  if (!d.onManageCollections && (d.collections ?? []).length === 0) return null;
  const mine = (d.collections ?? []).filter((c) => !c.smart && c.games?.includes(d.game.id));
  return (
    <section aria-label={t('collections:gameHeading')}>
      <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('collections:gameHeading')}</h2>
      {mine.length > 0 ? (
        <ul className="flex flex-wrap gap-1.5">
          {mine.map((c) => (
            <li key={c.id} className="rounded-lg bg-surface-strong px-2 py-1 text-[12.5px] font-medium text-text">
              {c.builtin ? t(`collections:builtin.${c.id}`) : c.name}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-[12.5px] text-text-faint">{t('collections:notInCollections')}</p>
      )}
      {d.onManageCollections && (
        <button
          type="button"
          onClick={() => d.onManageCollections?.(d.game.id)}
          className="mt-2 text-[12.5px] font-semibold text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          {t('collections:addToCollection')}
        </button>
      )}
    </section>
  );
}

// Atlas parity (issue #132): the classic Status & notes section has no other
// Atlas surface, so the store tab carries a small version: a status select
// that saves at once and notes that save on blur.
function AtlasStatusNotes({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const [status, setStatus] = useState(d.game.status ?? '');
  const [notes, setNotes] = useState(d.game.user_notes ?? '');
  const [state, setState] = useState<'idle' | 'saving' | 'saved' | 'failed'>('idle');
  useEffect(() => {
    setStatus(d.game.status ?? '');
    setNotes(d.game.user_notes ?? '');
    setState('idle');
  }, [d.game.id]);
  if (!d.onSaveStatus && !d.onSaveNotes) return null;
  const saveStatus = (next: GameStatus) => {
    setStatus(next);
    if (!d.onSaveStatus) return;
    d.onSaveStatus(d.game.id, next).then(
      (info) => { setStatus(info.status); d.onStatusChanged?.(); },
      () => { setStatus(d.game.status ?? ''); },
    );
  };
  const saveNotes = () => {
    const text = notes;
    if (text === (d.game.user_notes ?? '') || !d.onSaveNotes) return;
    setState('saving');
    d.onSaveNotes(d.game.id, text).then(
      () => { setState('saved'); d.onStatusChanged?.(); },
      () => { setState('failed'); },
    );
  };
  return (
    <section aria-label={t('game:status.heading')}>
      <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('game:status.heading')}</h2>
      <div className="grid gap-2">
        {d.onSaveStatus && (
          <Select value={status || 'none'} onValueChange={(v) => saveStatus((v === 'none' ? '' : v) as GameStatus)}>
            <SelectTrigger aria-label={t('game:status.aria')} className="h-8 w-full font-sans text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(['none', 'backlog', 'playing', 'finished', 'abandoned'] as const).map((v) => (
                <SelectItem key={v} value={v} className="font-sans text-[13px]">
                  {t(`game:status.${v}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {d.onSaveNotes && (
          <div className="grid gap-1">
            <label htmlFor={`atlas-notes-${d.game.id}`} className="text-[12px] text-text-muted">{t('game:status.notesLabel')}</label>
            <textarea
              id={`atlas-notes-${d.game.id}`}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              onBlur={saveNotes}
              placeholder={t('game:status.notesPlaceholder')}
              rows={2}
              className="min-h-16 w-full resize-y rounded-[7px] bg-surface px-2.5 py-2 text-[13px] text-text outline-none ring-1 ring-hairline placeholder:text-text-faint focus:ring-2 focus:ring-focus"
            />
            {state !== 'idle' && (
              <span role="status" className="text-[12px] text-text-faint">
                {state === 'saving' ? t('game:status.saving') : state === 'saved' ? t('game:status.saved') : t('game:status.failed')}
              </span>
            )}
          </div>
        )}
      </div>
    </section>
  );
}

/* Flat tables like `.tbl` in the design lab's kit.css: header cells 30px,
 * weight 500, faint, with a hairline bottom border; rows 36px with hairline
 * borders; the first column padded 24px; numbers right-aligned in mono
 * tabular. An empty tab shows one muted line, padded 16px 24px. */
const TBL_HEAD = 'h-[30px] px-3 text-left align-middle text-[13px] font-medium text-text-faint border-b border-hairline whitespace-nowrap';
const TBL_HEAD_FIRST = 'h-[30px] pl-6 pr-3 text-left align-middle text-[13px] font-medium text-text-faint border-b border-hairline whitespace-nowrap';
const TBL_CELL = 'h-9 px-3 text-left align-middle text-[13px] text-text border-b border-hairline';
const TBL_CELL_FIRST = 'h-9 pl-6 pr-3 text-left align-middle text-[13px] text-text border-b border-hairline whitespace-nowrap';
const TBL_NUM = 'h-9 px-3 text-right align-middle font-mono text-[12px] tabular-nums text-text border-b border-hairline whitespace-nowrap';

function TblEmpty({ text }: { text: string }) {
  return <p className="px-6 py-4 text-[13px] text-text-muted">{text}</p>;
}

function ActivityTab({ events }: { events: ActivityEvent[] }) {
  const { t } = useTranslation('game');
  const locale = currentLocale();
  if (events.length === 0) return <TblEmpty text={t('game:storeTab.noActivity')} />;
  return (
    <table className="w-full">
      <thead>
        <tr>
          <th className={TBL_HEAD_FIRST}>{t('game:storeTab.activityStarted')}</th>
          <th className={TBL_HEAD}>{t('game:storeTab.activityEvent')}</th>
          <th className={TBL_HEAD}>{t('game:storeTab.activityInfo')}</th>
        </tr>
      </thead>
      <tbody>
        {events.map((e) => (
          <tr key={e.id}>
            <td className={cx(TBL_CELL_FIRST, 'tabular-nums text-text-muted')}>
              {formatDate(Math.floor(e.ts / 1000), locale)} {formatClock(e.ts, locale)}
            </td>
            <td className={cx(TBL_CELL, 'max-w-[70ch]')}>
              <span className="block whitespace-normal leading-relaxed">{e.detail}</span>
            </td>
            <td className={cx(TBL_CELL, 'text-text-muted')}>{e.meta ?? ''}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function BackupTable({ game, onNotify, onCount, reloadKey }: { game: Game; onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void; onCount?: (n: number | undefined) => void; reloadKey?: number }) {
  const { t } = useTranslation('game');
  const [backups, setBackups] = useState<SaveBackup[] | null>(null);
  const [confirm, setConfirm] = useState<{ id: string; what: 'restore' | 'delete' } | null>(null);
  const locale = currentLocale();
  const countRef = useRef(onCount);
  countRef.current = onCount;
  const reload = useCallback(() => {
    Effect.runPromise(LauncherApi.listSaveBackups(game.id)).then(
      (v) => { setBackups(v ?? []); countRef.current?.((v ?? []).length); },
      () => { setBackups([]); countRef.current?.(0); },
    );
  }, [game.id]);
  useEffect(() => {
    setBackups(null);
    setConfirm(null);
    countRef.current?.(undefined);
    reload();
  }, [reload, reloadKey]);
  const act = (id: string, what: 'restore' | 'delete') => {
    Effect.runPromise(what === 'restore' ? LauncherApi.restoreSaveBackup(game, id) : LauncherApi.deleteSaveBackup(game.id, id)).then(
      () => { setConfirm(null); reload(); },
      (e) => { setConfirm(null); onNotify(coreErrMsg(e), 'err'); },
    );
  };
  if (backups == null) return <TblEmpty text={t('game:storeTab.noSaves')} />;
  return (
    <div>
      {backups.length === 0 ? (
        <TblEmpty text={t('game:storeTab.noSaves')} />
      ) : (
        <table className="w-full">
          <thead>
            <tr>
              <th className={TBL_HEAD_FIRST}>{t('game:storeTab.savesBackup')}</th>
              <th className={TBL_HEAD}>{t('game:storeTab.savesTrigger')}</th>
              <th className={cx(TBL_HEAD, 'text-right')}>{t('game:storeTab.savesFiles')}</th>
              <th className={cx(TBL_HEAD, 'text-right')}>{t('game:storeTab.savesSize')}</th>
              <th className={TBL_HEAD}><span className="sr-only">{t('game:storeTab.savesBackup')}</span></th>
            </tr>
          </thead>
          <tbody>
            {backups.map((b) => (
              <tr key={b.id}>
                <td className={cx(TBL_CELL_FIRST, 'tabular-nums')}>
                  {formatDate(b.created_at, locale)} {formatClock(b.created_at * 1000, locale)}
                </td>
                <td className={TBL_CELL}>{b.trigger}</td>
                <td className={TBL_NUM}>{b.files}</td>
                <td className={TBL_NUM}>{formatBytes(b.size)}</td>
                <td className={TBL_CELL}>
                  {confirm?.id === b.id ? (
                    <span className="flex items-center justify-end gap-2">
                      <span className="text-[12px] text-text-muted">
                        {confirm.what === 'restore' ? t('game:storeTab.restoreConfirm') : t('game:storeTab.deleteConfirm')}
                      </span>
                      <Button variant="ghost" size="sm" onClick={() => setConfirm(null)}>{t('game:storeTab.cancel')}</Button>
                      <Button variant="danger" size="sm" onClick={() => act(b.id, confirm.what)}>
                        {confirm.what === 'restore' ? t('game:storeTab.restore') : t('game:storeTab.delete')}
                      </Button>
                    </span>
                  ) : (
                    <span className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setConfirm({ id: b.id, what: 'restore' })} disabled={game.is_installed === false}>
                        {t('game:storeTab.restore')}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setConfirm({ id: b.id, what: 'delete' })}>
                        {t('game:storeTab.delete')}
                      </Button>
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

/** The cloud saves state and mode as one slim row under the backups table
 *  (the lab's .srow: plain text on the left, a 190px select on the right).
 *  Atlas-only (this file renders only for game.structure = tabs). The badge
 *  stays hidden under label.style = plain: the status reads as plain text. */
export function CloudSlimRow({ game, plain, action }: { game: Game; plain: boolean; action?: React.ReactNode }) {
  const { t } = useTranslation('cloud');
  const [info, setInfo] = useState<CloudInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resolving, setResolving] = useState(false);
  const gameRef = useRef(game);
  gameRef.current = game;

  const refresh = useCallback(() => {
    return Effect.runPromise(LauncherApi.getCloudInfo(gameRef.current)).then(setInfo).catch((e) => setError(coreErrMsg(e)));
  }, []);

  const check = useCallback(() => {
    setChecking(true);
    setError(null);
    return Effect.runPromise(LauncherApi.checkCloudSaves(gameRef.current))
      .then(setInfo)
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setChecking(false));
  }, []);

  // Load the last known state; like the panel, look once per session when
  // there is none yet. An uninstalled game has no saves on disk to check,
  // so skip the auto-check there (it only surfaces a "not installed" error).
  useEffect(() => {
    let live = true;
    setInfo(null);
    setError(null);
    Effect.runPromise(LauncherApi.getCloudInfo(game))
      .then((i) => {
        if (!live) return;
        setInfo(i);
        if (game.is_installed === false) return;
        if (!i.status && i.mode !== 'off' && !slimAutoChecked.has(game.id)) {
          slimAutoChecked.add(game.id);
          void check();
        }
      })
      .catch((e) => live && setError(coreErrMsg(e)));
    return () => { live = false; };
    // Reload for another game, not on every settings edit of this one.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  useEffect(
    () =>
      onCloud((e) => {
        if (e.game_id !== game.id || e.kind === 'progress') return;
        void refresh();
      }),
    [game.id, refresh],
  );

  useEffect(
    () =>
      onSession((e) => {
        if (e.game_id === game.id) void refresh();
      }),
    [game.id, refresh],
  );

  const setMode = (v: CloudMode | 'default') => {
    setError(null);
    Effect.runPromise(LauncherApi.setCloudGameMode(game.id, v === 'default' ? '' : v))
      .then(refresh)
      .catch((e) => setError(coreErrMsg(e)));
  };

  const st = info?.status ?? null;
  const chip = st ? stateChip(st.state) : null;
  const busy = checking || !!info?.busy;
  const when = st?.checkedAt ? agoLabel(st.checkedAt) : '';
  const conflicted = st?.state === 'conflict' && (st.conflicts?.length ?? 0) > 0;
  const mode: CloudMode | 'default' = info ? (info.game_mode ? asCloudMode(info.game_mode) : 'default') : 'default';

  return (
    <div className="flex items-center gap-[18px] border-t border-hairline px-6 py-3.5">
      <div className="grid min-w-0 flex-1 gap-0.5 text-[13px]">
        {conflicted ? (
          <span>
            <button type="button" onClick={() => setResolving(true)} className="font-sans font-semibold text-accent hover:underline">
              {t('cloud:panel.choose')}
            </button>
          </span>
        ) : busy ? (
          <span className="text-text-muted">{t('cloud:state.checking')}…</span>
        ) : st && chip ? (
          <span className="truncate text-text-muted">
            {!plain && <Badge data-testid="cloud-state" variant={chip.variant}>{chip.label}</Badge>}{' '}
            {chip.label}
            {when && <span> · {t('cloud:panel.checked', { when })}</span>}
          </span>
        ) : (
          <span className="truncate text-text-muted">
            {!plain && <Badge data-testid="cloud-state" variant="neutral">{t('cloud:state.unchecked')}</Badge>}{' '}
            {info?.mode === 'off' ? t('cloud:state.off') : t('cloud:state.unchecked')}
            {info && info.mode !== 'off' && (
              <>{' · '}<button type="button" onClick={() => void check()} disabled={checking} className="font-sans font-semibold text-accent hover:underline disabled:opacity-60">
                {t('cloud:panel.check')}
              </button></>
            )}
          </span>
        )}
        {error && <span className="truncate text-[12px] text-danger">{error}</span>}
        {info && !conflicted && <span className="sr-only">{modeHelp(info.mode)}</span>}
      </div>
      {action}
      <Select value={mode} onValueChange={(v) => setMode(v as CloudMode | 'default')} disabled={!info}>
        <SelectTrigger aria-label={t('cloud:panel.modeLabel')} className="h-8 w-[190px] flex-none font-sans text-[13px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="default" className="font-sans text-[13px]">{t('cloud:panel.defaultOption', { mode: modeName(asCloudMode(info?.plugin_mode ?? 'ask')) })}</SelectItem>
          <SelectItem value="off" className="font-sans text-[13px]">{t('cloud:mode.off')}</SelectItem>
          <SelectItem value="ask" className="font-sans text-[13px]">{t('cloud:mode.ask')}</SelectItem>
          <SelectItem value="auto" className="font-sans text-[13px]">{t('cloud:mode.auto')}</SelectItem>
        </SelectContent>
      </Select>
      {resolving && st && (
        <CloudConflictDialog
          game={game}
          status={st}
          onClose={() => setResolving(false)}
          onResolved={(next) => {
            setInfo(next);
            if (next.status?.state !== 'conflict') setResolving(false);
          }}
        />
      )}
    </div>
  );
}

/** Games already checked on their own this session (the slim row checks once,
 *  like the panel does; the two never mount together). */
const slimAutoChecked = new Set<string>();

function SavesTab({ d, onCount }: { d: GameTabsData; onCount?: (n: number | undefined) => void }) {
  // The badge stays hidden under label.style = plain: the slim row reads as
  // plain text (atlas sets plain; other themes never render these tabs).
  const plain = useVariant('label.style') === 'plain';
  const { t } = useTranslation('game');
  const [reloadKey, setReloadKey] = useState(0);
  const [backingUp, setBackingUp] = useState(false);
  const backupNow = () => {
    if (backingUp) return;
    setBackingUp(true);
    Effect.runPromise(LauncherApi.createSaveBackup(d.game))
      .then(() => {
        (d.onNotify ?? (() => {}))(t('game:storeTab.backupQueued'), 'busy');
        setReloadKey((k) => k + 1);
      })
      .catch((e) => (d.onNotify ?? (() => {}))(coreErrMsg(e), 'err'))
      .finally(() => setBackingUp(false));
  };
  // The lab's SavesTable has no button row: "Back up now" lives in the slim
  // row with the cloud status and the 190px mode select.
  const backupButton = (
    <Button variant="neutral" size="sm" onClick={backupNow} disabled={backingUp} className="flex-none">
      {backingUp ? t('game:storeTab.backingUp') : t('game:storeTab.backupNow')}
    </Button>
  );
  return (
    <div>
      {/* The backups table first, then the cloud status, backup action and
          mode as one slim row (the lab's SavesTable over .tbl, with .srow
          controls). */}
      <BackupTable game={d.game} onNotify={d.onNotify ?? (() => {})} onCount={onCount} reloadKey={reloadKey} />
      {d.plugin ? (
        <CapabilitySurface plugin={d.plugin} capability="cloud-saves" game={d.game}>
          <CloudSlimRow game={d.game} plain={plain} action={backupButton} />
        </CapabilitySurface>
      ) : (
        <div className="flex items-center gap-[18px] border-t border-hairline px-6 py-3.5">
          <span className="min-w-0 flex-1" />
          {backupButton}
        </div>
      )}
    </div>
  );
}

/** The drawer's Saves category reuses the tab's pieces (the table plus the
 *  one slim row), so the drawer and the tab never drift apart. */
export function AtlasSavesBlock({ game, plugin, onNotify }: { game: Game; plugin?: StorePlugin; onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void }) {
  const { t } = useTranslation('game');
  const [reloadKey, setReloadKey] = useState(0);
  const [backingUp, setBackingUp] = useState(false);
  const backupNow = () => {
    if (backingUp) return;
    setBackingUp(true);
    Effect.runPromise(LauncherApi.createSaveBackup(game))
      .then(() => {
        onNotify(t('game:storeTab.backupQueued'), 'busy');
        setReloadKey((k) => k + 1);
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBackingUp(false));
  };
  const backupButton = (
    <Button variant="neutral" size="sm" onClick={backupNow} disabled={backingUp} className="flex-none">
      {backingUp ? t('game:storeTab.backingUp') : t('game:storeTab.backupNow')}
    </Button>
  );
  return (
    <div>
      <BackupTable game={game} onNotify={onNotify} reloadKey={reloadKey} />
      {plugin ? (
        <CapabilitySurface plugin={plugin} capability="cloud-saves" game={game}>
          <CloudSlimRow game={game} plain action={backupButton} />
        </CapabilitySurface>
      ) : (
        <div className="flex items-center gap-[18px] border-t border-hairline px-6 py-3.5">
          <span className="min-w-0 flex-1" />
          {backupButton}
        </div>
      )}
    </div>
  );
}

/** The Compatibility tab count: the fix-info row plus the ProtonDB notes and
 *  the crash hints (the tab shows all three, so the count includes all). */
export function compatTabCount(game: Game, notesLen: number, hintsLen: number): number {
  return (game.fix_info?.description ? 1 : 0) + notesLen + hintsLen;
}

function CompatTab({ d, info }: { d: GameTabsData; info: ReturnType<typeof useCompat>['compat'] }) {
  const { t } = useTranslation('game');
  const rows: { source: string; note: string }[] = [];
  if (d.game.fix_info?.description) rows.push({ source: d.game.fix_info.source, note: d.game.fix_info.description });
  const rating = info?.badges?.find((b) => b.kind === 'rating');
  for (const n of info?.notes ?? []) {
    rows.push({ source: rating?.source ?? d.game.fix_info?.source ?? 'Platypus', note: n });
  }
  if (d.facts.length === 0 && rows.length === 0) return <TblEmpty text={t('game:storeTab.noCompat')} />;
  return (
    <div>
      {/* The classic facts (DirectX, wrappers, launch-via, …) as a flat
          two-column table above the notes (issue #132). */}
      {d.facts.length > 0 && (
        <table className="w-full" aria-label={t('game:storeTab.details')}>
          <tbody>
            {d.facts.map(([k, v]) => (
              <tr key={k}>
                <td className={cx(TBL_CELL_FIRST, 'w-44 text-text-muted')}>{k}</td>
                <td className={cx(TBL_CELL, 'max-w-[70ch]')}>
                  <span className="block whitespace-normal leading-relaxed">{v}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {rows.length > 0 && (
        <table className="w-full">
          <thead>
            <tr>
              <th className={TBL_HEAD_FIRST}>{t('game:storeTab.compatSource')}</th>
              <th className={TBL_HEAD}>{t('game:storeTab.compatNote')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                <td className={cx(TBL_CELL_FIRST, 'text-text-muted')}>{r.source}</td>
                <td className={cx(TBL_CELL, 'max-w-[70ch]')}>
                  <span className="block whitespace-normal leading-relaxed">{r.note}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export function GameTabsView(d: GameTabsData) {
  const { t } = useTranslation('game');
  const [tab, setTab] = useState<TabId>('store');
  const [savesCount, setSavesCount] = useState<number | undefined>(undefined);
  const { compat: info } = useCompat(d.game);
  const crash = useCrashHints(d.game, d.session?.state === 'running' || d.session?.state === 'starting' || d.launching);
  useEffect(() => { setTab('store'); setSavesCount(undefined); }, [d.game.id]);
  // The saves tab count reads the local backups (the tab's own table reports
  // again when it loads, so a backup made on the tab updates it).
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.listSaveBackups(d.game.id)).then(
      (v) => { if (live) setSavesCount((v ?? []).length); },
      () => { if (live) setSavesCount(0); },
    );
    return () => { live = false; };
  }, [d.game.id]);

  const hasAch = (d.progress?.total ?? d.ach?.total ?? 0) > 0;
  const achCount = hasAch ? `${d.progress?.unlocked ?? d.ach?.unlocked ?? 0}/${d.progress?.total ?? d.ach?.total ?? 0}` : undefined;
  const compatCount = compatTabCount(d.game, info?.notes?.length ?? 0, crash?.hints?.length ?? 0);

  const tabs: { id: TabId; label: string; count?: string | number }[] = [
    { id: 'store', label: t('game:tabs.store') },
    ...(hasAch ? [{ id: 'achievements' as TabId, label: t('game:tabs.achievements'), count: achCount }] : []),
    { id: 'activity', label: t('game:tabs.activity'), count: d.events.length },
    { id: 'saves', label: t('game:tabs.saves'), count: savesCount },
    { id: 'compat', label: t('game:tabs.compat'), count: compatCount },
  ];
  const cur: TabId = tabs.some((x) => x.id === tab) ? tab : 'store';

  const onTabKey = (e: React.KeyboardEvent) => {
    const i = tabs.findIndex((x) => x.id === cur);
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault();
      const n = tabs[(i + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
      setTab(n.id);
      (e.currentTarget.querySelector(`[data-tab="${n.id}"]`) as HTMLElement | null)?.focus();
    }
  };

  return (
    <div>
      {!d.hideBack && (
        <div className="px-7 pt-7">
          <GameBackLink backLabel={d.backLabel} onBack={d.onBack} />
        </div>
      )}
      {/* Full-bleed like /21 `.stage`: the hero art at the top with the
          title bottom-left, and the action strip edge to edge under it. No
          rounded inset card, so the Tune flyout is never clipped. */}
      <AtlasHero game={d.game} hero={d.hero} details={d.details} />
      <ActionStrip d={d} />
      {/* Notices sit flush below the strip: each carries its own slim-row
          padding, with no outer card and no page padding around them. */}
      <div className="empty:hidden">
        {d.plugin && <GameOutsideAlert game={d.game} plugin={d.plugin} onAllowFolder={d.onAllowFolder} />}
        <GameInstallNote
          game={d.game}
          plugin={d.plugin}
          installing={d.installing}
          installSize={d.installSize}
          job={d.job}
          canChooseVersion={d.canChooseVersion}
          refreshing={d.refreshing}
          onOpenVersions={d.onOpenVersions}
          onOpenPlugins={d.onOpenPlugins}
        />
      </div>
      <div role="tablist" aria-label={t('game:tabs.label')} onKeyDown={onTabKey} className="sticky top-0 z-10 flex gap-0.5 overflow-x-auto border-b border-hairline bg-raised px-4">
        {tabs.map((x) => (
          <button
            key={x.id}
            data-tab={x.id}
            type="button"
            role="tab"
            aria-selected={x.id === cur}
            tabIndex={x.id === cur ? 0 : -1}
            onClick={() => setTab(x.id)}
            className={cx(
              'h-[42px] flex-none px-2.5 text-[13.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              x.id === cur ? 'text-text shadow-[inset_0_-2px_0_var(--color-accent)]' : 'text-text-muted hover:text-text',
            )}
          >
            {x.label}
            {x.count != null && <span className="ml-3 text-[12px] font-medium tabular-nums text-text-faint">{x.count}</span>}
          </button>
        ))}
      </div>
      <div role="tabpanel" key={`${d.game.id}-${cur}`} className="pb-9">
        {cur === 'store' && (
          <div className="grid grid-cols-1 gap-7 px-6 pt-5 min-[1100px]:grid-cols-[minmax(0,1fr)_340px]">
            <div className="min-w-0">
              <AboutSection
                game={d.game}
                details={d.details}
                loading={d.loading}
                error={d.error}
                compatUrls={d.compatUrls}
                onOpenStore={d.onOpenStore}
                onReload={d.onReload}
                onSearchInfo={d.onSearchInfo}
                onLinkInfo={d.onLinkInfo}
                onUnlinkInfo={d.onUnlinkInfo}
                hideDetails
              />
            </div>
            <aside className="grid min-w-0 content-start gap-5">
              <GallerySheet media={d.media} title={d.game.title} />
              <YouGrid d={d} />
              {hasAch && <AchSummary d={d} onOpen={() => setTab('achievements')} />}
              <DetailsSide d={d} />
              <AtlasCollections d={d} />
              <AtlasStatusNotes d={d} />
            </aside>
          </div>
        )}
        {cur === 'achievements' && <AchRows d={d} />}
        {cur === 'activity' && <ActivityTab events={d.events} />}
        {cur === 'saves' && <SavesTab d={d} onCount={setSavesCount} />}
        {cur === 'compat' && (
          <div>
            {crash && (
              <div className="border-b border-hairline bg-raised px-6 py-3">
                <CrashHintsPanel game={d.game} hints={crash} onNotify={d.onNotify} onRunner={d.onOpenRunner} />
              </div>
            )}
            <CompatTab d={d} info={info} />
          </div>
        )}
      </div>
    </div>
  );
}
