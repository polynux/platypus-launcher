// i18n: converted
// Game tiles and rows (issue #98, #109): the catalog components the Library
// and the Dashboard share. Warm keeps today's dashboard tile (3:4, compat and
// overlay on, comfortable) and today's library card (2:3, compact footer)
// through library-tile following game-tile; cool picks the compact values.
import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Download } from 'lucide-react';
import type { Game, Job, ProtonVersion } from '../types';
import { Cover, TIER_DOT, TIER_TEXT, TweakDot, cardClass, cx } from './ui';
import { GameCardBody } from './CompatBadges';
import { coverImages, formatBytes, hoursPlayed, needsAttention } from '../lib/games';
import { formatCount } from '../lib/format';
import { tierFilterValue } from '../lib/compat';
import { runnerShort } from '../lib/settings';
import { useVariant } from '../lib/theme';
import { Badge } from './ds/badge';

type T = (key: string, vars?: Record<string, unknown>) => string;

/** The status badge label of a game card: null when it has no status. */
function statusBadgeLabel(t: T, status: string | undefined): string | null {
  switch (status) {
    case 'backlog':
      return t('library:statusFilter.backlog');
    case 'playing':
      return t('library:statusFilter.playing');
    case 'finished':
      return t('library:statusFilter.finished');
    case 'abandoned':
      return t('library:statusFilter.abandoned');
    default:
      return null;
  }
}

function jobLabel(t: T, j: Job): string {
  if (j.state === 'queued') return t('library:job.queued', { kind: j.kind });
  if (j.percent >= 0) {
    const percent = formatCount(Math.round(j.percent));
    return j.kind === 'uninstall' ? t('library:job.removing', { percent }) : t('library:job.installing', { percent });
  }
  return j.phase ?? j.kind;
}

const ASPECT_CLASS: Record<string, string> = {
  '2:3': 'aspect-[2/3]',
  '3:4': 'aspect-[3/4]',
  '16:9': 'aspect-[16/9]',
};

interface GameTileProps {
  game: Game;
  job?: Job;
  /** One-line summary under the title; the caller builds it like today. */
  meta: string;
  /** Hide the whole chip for native builds (dashboard). */
  hideNative?: boolean;
  /** Show the library's cover badges (status, not installed, shared by). */
  coverBadges?: boolean;
  /** Which use site: the dashboard reads game-tile, the library reads
   *  library-tile (which follows game-tile unless the theme names a library
   *  value, like game-hero follows hero). */
  site?: 'dashboard' | 'library';
  onOpen: (id: string) => void;
  onManage?: (id: string) => void;
}

/** One game tile: the Dashboard continue grid and the Library cards share it.
 *  A single root (a button), so a layout row counts one column per tile. */
export const GameTile = memo(function GameTile({ game: g, job, meta, hideNative, coverBadges, site = 'dashboard', onOpen, onManage }: GameTileProps) {
  const { t } = useTranslation('library');
  const aspect = useVariant(site === 'library' ? 'library-tile.aspect' : 'game-tile.aspect');
  const compat = useVariant(site === 'library' ? 'library-tile.compat' : 'game-tile.compat');
  const overlay = useVariant(site === 'library' ? 'library-tile.overlay' : 'game-tile.overlay');
  const density = useVariant(site === 'library' ? 'library-tile.density' : 'game-tile.density');
  const statusLabel = statusBadgeLabel(t, g.status);
  const showOverlay = overlay === 'on';
  const bodyClass = density === 'compact' ? 'px-2.5 py-2' : 'px-3 py-2.5';
  return (
    <button
      onClick={() => onOpen(g.id)}
      onContextMenu={onManage ? (e) => { e.preventDefault(); onManage(g.id); } : undefined}
      data-aspect={aspect}
      className={cx(cardClass, 'card-compat flex h-full min-w-0 flex-col')}
    >
      <div className="relative flex-none">
        <Cover src={coverImages(g)} title={g.title} className={cx(ASPECT_CLASS[aspect] ?? 'aspect-[3/4]', 'w-full', !g.is_installed && !job && 'opacity-45 grayscale-[.6]')} />
        {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
        {g.shared_by && coverBadges && (
          <Badge variant="neutral" className="absolute left-2 top-2 max-w-[85%] truncate bg-black/65 text-white/80">{t('library:sharedBy', { name: g.shared_by.name })}</Badge>
        )}
        {job && showOverlay ? (
          <div className="absolute inset-x-2 bottom-2 rounded-lg bg-black/70 px-2 py-1.5">
            <div className="font-mono text-micro uppercase tracking-wider text-amber-200">{jobLabel(t, job)}</div>
            <div className="mt-1 h-1 rounded-full bg-white/15">
              <div className="h-full rounded-full bg-accent-strong" style={{ width: `${Math.max(job.percent, 2)}%` }} />
            </div>
          </div>
        ) : (
          coverBadges && showOverlay && (statusLabel || !g.is_installed) && (
            <div className="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] flex-wrap gap-1">
              {statusLabel && (
                <Badge variant="neutral" className="bg-black/65 text-text-body" data-testid="status-badge">
                  {statusLabel}
                </Badge>
              )}
              {!g.is_installed && (
                <Badge variant="neutral" className="gap-1 bg-black/65 text-text-body">
                  {g.installable && <Download className="h-2.5 w-2.5" />} {g.installable ? t('library:cardNotInstalled') : t('library:cardOwned')}
                </Badge>
              )}
            </div>
          )
        )}
      </div>
      {compat === 'on' ? (
        <GameCardBody title={g.title} meta={meta} game={g} hideNative={hideNative} className={bodyClass} />
      ) : (
        <div className={cx('flex min-w-0 flex-1 flex-col justify-start', bodyClass)}>
          <div className="truncate text-hint font-bold" title={g.title}>{g.title}</div>
          <div className="truncate font-mono text-micro uppercase tracking-wider text-white/55" title={meta || undefined}>{meta || ' '}</div>
        </div>
      )}
    </button>
  );
});

const TABLE_COLS_FULL = 'grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_6rem_4rem_7rem]';
const TABLE_COLS_MINIMAL = 'grid-cols-[16px_minmax(180px,2fr)_5rem_7rem]';

interface GameRowProps {
  game: Game;
  index: number;
  runners: ProtonVersion[];
  recency?: string;
  onOpen: (id: string) => void;
  onManage?: (id: string) => void;
}

/** One game row in the Library table: full columns today, a minimal set and a
 *  compact height when the theme picks them. A single root with row roles. */
export const GameRow = memo(function GameRow({ game: g, index, runners, recency, onOpen, onManage }: GameRowProps) {
  const { t } = useTranslation('library');
  const columns = useVariant('game-row.columns');
  const mode = useVariant('game-row.mode');
  const tier = tierFilterValue(g);
  const minimal = columns === 'minimal';
  const compact = mode === 'compact';
  return (
    <div
      role="row"
      aria-rowindex={index + 2}
      onContextMenu={onManage ? (e) => { e.preventDefault(); onManage(g.id); } : undefined}
      className={cx('grid items-center gap-3 px-4 transition hover:bg-accent-strong/[0.05]', minimal ? TABLE_COLS_MINIMAL : TABLE_COLS_FULL, compact ? 'h-8' : 'h-11', index % 2 && 'bg-white/[0.015]')}
    >
      <span role="cell" className={cx('h-1.5 w-1.5 rounded-full', TIER_DOT[tier ?? ''] ?? 'bg-white/15')} />
      <span role="cell" className="min-w-0">
        <button onClick={() => onOpen(g.id)} className="group flex w-full min-w-0 items-center gap-2.5 text-left">
          <Cover src={coverImages(g)} title="" className="h-7 w-7 flex-none rounded ring-1 ring-white/10" />
          <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
          {g.shared_by && !minimal && <Badge variant="neutral" className="flex-none">{t('library:sharedBy', { name: g.shared_by.name })}</Badge>}
        </button>
      </span>
      <span role="cell" className={cx('font-mono text-hint', TIER_TEXT[tier ?? ''] ?? 'text-text-muted')}>{tier ?? '—'}</span>
      {!minimal && <span role="cell" className="truncate font-mono text-hint text-text-muted">{runnerShort(g.settings.runner_id, runners)}</span>}
      {!minimal && <span role="cell" className="text-right font-mono text-hint tabular-nums text-text-muted">{hoursPlayed(g) ?? '—'}</span>}
      {!minimal && <span role="cell" className="text-right font-mono text-hint tabular-nums text-text-muted">{g.release_year ?? '—'}</span>}
      {!minimal && <span role="cell" className="text-right font-mono text-hint text-text-muted">{formatBytes(g.size_bytes) || '—'}</span>}
      <span role="cell" className="text-right font-mono text-hint text-text-muted">{recency ?? '—'}</span>
    </div>
  );
});

