// i18n: converted
// The Atlas game page as tabs (issue #127, D21.jsx `.stage`): the hero with
// the logo, the action strip on the dim accent, and sticky tabs with the
// store page. Only the tabs structure renders here; sections keep today's
// GamePage path exactly, so warm, cool and slate render as before.
//
// Data stays with the page: every block takes props only (the hooks live in
// GamePage), so the tabs re-arrange blocks without forking data loading.
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { AlertTriangle, ChevronDown, Cloud, CloudOff, Download, HardDrive, Loader2, Play, RefreshCw, SlidersHorizontal, Square } from 'lucide-react';
import type { ActivityEvent, ArtCandidate, CloudInfo, Game, GameDetails, GameJobKind, Job, SessionEvent, StorePlugin } from '../types';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Lightbox, type MediaItem } from './Lightbox';
import { PluginLogo } from './PluginLogo';
import { CrashHintsPanel } from './CrashHints';
import { CloudSavesPanel } from './CloudSavesPanel';
import { CloudConflictDialog } from './CloudSavesConflict';
import { SaveBackupsSection } from './SaveBackupsSection';
import { JobProgress } from './JobProgress';
import { useCrashHints } from '../lib/useCrashHints';
import { useCompat } from '../lib/useCompat';
import { LauncherApi, onCloud, onSession } from '../effect/runtime';
import { useVariant } from '../lib/theme';
import { formatBytes, hoursPlayed, storeLabel } from '../lib/games';
import { formatDecimal1 } from '../lib/gameLocale';
import { playedCopy, pickableCopies } from '../lib/family';
import { coreErrMsg } from '../lib/coreError';
import { cx } from './ui';
import {
  AboutSection,
  AchievementsSection,
  GameActivity,
  GameBackLink,
  GameCompat,
  GameInstallNote,
  GameOutsideAlert,
  SafeHtml,
} from './GamePage';
import { ProgressAchievements, type useAchievementProgress } from './ProgressAchievements';
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
          className="inline-flex h-[42px] items-center gap-2 rounded-xl bg-black/30 px-5 text-[15px] font-extrabold text-text ring-1 ring-accent/60 hover:bg-black/40"
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
        className="inline-flex h-[42px] items-center gap-2 rounded-xl bg-accent px-6 pl-5 text-[15px] font-extrabold text-on-accent hover:brightness-110 disabled:opacity-80"
      >
        {launching ? <Loader2 className="h-[15px] w-[15px] animate-spin" aria-hidden /> : <Play className="h-[15px] w-[15px] fill-current" aria-hidden />}
        <span>{launching ? t('game:play.launching') : t('game:play.play')}</span>
      </Button>
    );
  }
  if (!game.installable) {
    return <span className="inline-flex h-[42px] items-center rounded-xl bg-surface px-5 text-[13px] font-bold text-text-muted">{t('game:install.cantInstall')}</span>;
  }
  const size = d.installSize.state === 'known' && d.installSize.total ? formatBytes(d.installSize.total) : null;
  return (
    <Button
      variant="primary"
      size="none"
      onClick={d.onInstall}
      className="inline-flex h-[42px] items-center gap-2 rounded-xl bg-accent px-6 pl-5 text-[15px] font-extrabold text-on-accent hover:brightness-110"
    >
      <Download className="h-[15px] w-[15px]" aria-hidden />
      <span>{d.job?.state === 'failed' ? t('game:install.retryInstall') : t('game:install.install')}</span>
      {size && <span className="font-mono text-[12px] font-medium">{size}</span>}
    </Button>
  );
}

function StoreSwitch({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const { game, plugin } = d;
  const copies = pickableCopies(game);
  const [busy, setBusy] = useState(false);
  useEffect(() => setBusy(false), [game.id]);
  if (copies.length > 1) {
    const played = playedCopy(game);
    const pick = (ownerId: string) => {
      if (busy) return;
      setBusy(true);
      Effect.runPromise(LauncherApi.setPreferredCopy(game, ownerId)).then(
        () => { setBusy(false); d.onCopyChanged?.(); },
        (e: Error) => { setBusy(false); d.onNotify?.(e.message || String(e), 'err'); },
      );
    };
    return (
      <div role="radiogroup" aria-label={t('game:strip.launchFrom')} className="inline-flex gap-0.5 rounded-xl bg-black/30 p-[3px]">
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
    );
  }
  const storeName = game.store === 'custom' ? t('game:store.sideloaded') : storeLabel(game.store);
  return (
    <span className="inline-flex h-[42px] items-center gap-1.5 rounded-xl bg-black/30 px-3 text-[13px] font-semibold text-text">
      <PluginLogo src={d.sourceLogos?.[game.source] ?? d.sourceLogos?.[game.store]} name={storeName} size="xs" />
      {storeName}
      {plugin && game.source !== game.store && <span className="text-text-muted">· {plugin.name}</span>}
    </span>
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
  const busy = st == null && info?.mode !== 'off';
  if (st) {
    if (st.state === 'synced') return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><Cloud className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveSynced')}</span>;
    return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><RefreshCw className="h-3.5 w-3.5 animate-spin" aria-hidden />{t('game:strip.saveSyncing')}</span>;
  }
  if (info?.mode === 'off') return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><CloudOff className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveOff')}</span>;
  if (busy) return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><RefreshCw className="h-3.5 w-3.5 animate-spin" aria-hidden />{t('game:strip.saveSyncing')}</span>;
  return <span className="inline-flex h-10 items-center gap-1.5 px-2.5 text-[13px] text-text-muted"><HardDrive className="h-3.5 w-3.5" aria-hidden />{t('game:strip.saveLocal')}</span>;
}

function ActionStrip({ d }: { d: GameTabsData }) {
  const { t } = useTranslation('game');
  const [tuneOpen, setTuneOpen] = useState(false);
  return (
    <div className="flex flex-wrap items-center gap-2.5 bg-accent/[0.12] px-5 py-3">
      <div className="mr-2 flex items-center gap-1.5">
        <StripPlay d={d} />
        <span className="relative">
          <button
            type="button"
            aria-expanded={tuneOpen}
            aria-haspopup="dialog"
            onClick={() => setTuneOpen((v) => !v)}
            className={cx(
              'inline-flex h-[42px] items-center gap-1.5 rounded-xl bg-black/30 px-3 pl-3.5 text-[14px] font-bold text-text transition hover:bg-black/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              tuneOpen && 'bg-black/40',
            )}
          >
            <SlidersHorizontal className="h-4 w-4" aria-hidden />
            {t('game:strip.tune')}
            <ChevronDown className="h-3.5 w-3.5" aria-hidden />
          </button>
          {/* // #128: the Tune flyout opens here */}
          {tuneOpen && <span aria-hidden className="hidden" />}
        </span>
      </div>
      {d.game.update_available && d.game.is_installed && (
        <Button variant="neutral" size="sm" onClick={() => d.onJob('update')} className="gap-1.5">
          <RefreshCw className="h-3.5 w-3.5" /> {t('game:strip.update')}
        </Button>
      )}
      <StoreSwitch d={d} />
      <SaveState d={d} />
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
              'overflow-hidden rounded-lg bg-black/40 ring-1 ring-hairline transition hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
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
    <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline">
      {cells.map(([k, v]) => (
        <div key={k} className="bg-surface px-3 py-2.5">
          <dt className="text-[11.5px] text-text-faint">{k}</dt>
          <dd className="truncate font-semibold text-text" title={v}>{v}</dd>
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
  return (
    <section aria-label={t('game:storeTab.achievements')}>
      <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('game:storeTab.achievements')}</h2>
      <div className="rounded-xl bg-surface px-3.5 py-3 ring-1 ring-hairline">
        <div className="flex items-center justify-between text-[13px]">
          <span className="font-bold text-text"><b>{unlocked}</b> / {total}</span>
          <span className="font-mono text-[12px] tabular-nums text-text-muted">{pct}%</span>
        </div>
        <div className="mt-2 h-1.5 rounded-full bg-hairline" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={unlocked} aria-label={t('game:progress.barLabel')}>
          <div className="h-full rounded-full bg-accent" style={{ width: `${pct}%` }} />
        </div>
        <button type="button" onClick={onOpen} className="mt-2 font-mono text-[12px] text-accent-strong underline decoration-accent-strong/40 underline-offset-2 hover:text-text">
          {t('game:storeTab.seeAll', { total })}
        </button>
      </div>
    </section>
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
      <dl className="space-y-2">
        {rows.map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-hairline pt-2 font-mono text-[12px]">
            <dt className="text-text-muted">{k}</dt>
            <dd className="truncate text-right text-text" title={v}>{v}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

export function GameTabsView(d: GameTabsData) {
  const { t } = useTranslation('game');
  const [tab, setTab] = useState<TabId>('store');
  const { compat: info } = useCompat(d.game);
  const crash = useCrashHints(d.game, d.session?.state === 'running' || d.session?.state === 'starting' || d.launching);
  useEffect(() => setTab('store'), [d.game.id]);

  const hasAch = (d.progress?.total ?? d.ach?.total ?? 0) > 0;
  const achCount = hasAch ? `${d.progress?.unlocked ?? d.ach?.unlocked ?? 0}/${d.progress?.total ?? d.ach?.total ?? 0}` : undefined;
  const compatCount = (info?.notes?.length ?? 0) + (crash?.hints?.length ?? 0);

  const tabs: { id: TabId; label: string; count?: string | number }[] = [
    { id: 'store', label: t('game:tabs.store') },
    ...(hasAch ? [{ id: 'achievements' as TabId, label: t('game:tabs.achievements'), count: achCount }] : []),
    { id: 'activity', label: t('game:tabs.activity'), count: d.events.length },
    { id: 'saves', label: t('game:tabs.saves') },
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
      <GameBackLink backLabel={d.backLabel} onBack={d.onBack} />
      <div className="mt-1 overflow-hidden rounded-2xl ring-1 ring-hairline">
        <AtlasHero game={d.game} hero={d.hero} details={d.details} />
        <ActionStrip d={d} />
      </div>
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
      <div role="tablist" aria-label={t('game:tabs.label')} onKeyDown={onTabKey} className="sticky top-0 z-10 -mx-1 mt-2 flex gap-1 overflow-x-auto border-b border-hairline bg-ground px-4">
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
              'h-[42px] flex-none px-3 text-[13.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              x.id === cur ? 'text-text shadow-[inset_0_-2px_0_var(--color-accent)]' : 'text-text-muted hover:text-text',
            )}
          >
            {x.label}
            {x.count != null && <span className="ml-1.5 text-[12px] font-medium tabular-nums text-text-faint">{x.count}</span>}
          </button>
        ))}
      </div>
      <div role="tabpanel" key={`${d.game.id}-${cur}`} className="pt-5">
        {cur === 'store' && (
          <div className="grid grid-cols-1 gap-7 min-[1100px]:grid-cols-[minmax(0,1fr)_340px]">
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
            </aside>
          </div>
        )}
        {cur === 'achievements' && (
          <div className="max-w-[920px]">
            {d.progress ? <ProgressAchievements progress={d.progress} /> : d.ach && d.ach.total > 0 ? <AchievementsSection ach={d.ach} known={d.ach.unlocked >= 0} viaSteam={d.game.source === 'steam'} /> : null}
          </div>
        )}
        {cur === 'activity' && <GameActivity events={d.events} />}
        {cur === 'saves' && (
          <div className="grid max-w-[860px] gap-5">
            {d.plugin && <CloudSavesPanel game={d.game} plugin={d.plugin} />}
            <SaveBackupsSection game={d.game} onNotify={d.onNotify ?? (() => {})} />
          </div>
        )}
        {cur === 'compat' && (
          <div className="grid max-w-[860px] gap-5">
            <GameCompat compat={d.compatText} fix={d.game.fix_info} facts={d.facts} info={info} />
            {crash && <CrashHintsPanel game={d.game} hints={crash} onNotify={d.onNotify} onRunner={d.onOpenRunner} />}
          </div>
        )}
      </div>
    </div>
  );
}
