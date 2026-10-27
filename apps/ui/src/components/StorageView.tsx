// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useMemo, useState } from 'react';
import { FolderInput, FolderOpen, HardDrive, HardDriveDownload, Loader2, RefreshCw, ScanLine, Trash2, TriangleAlert } from 'lucide-react';
import type { DiskUsage, Game, Job, RemoveOptions } from '../types';
import { ATLAS_PAGE, Cover, EmptyState, MonoLabel, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { RemoveConfirmPanel, RemoveFirstRow, parentFolder, useRemoveCheck } from './MaintenanceTab';
import { MoveDialog } from './MoveDialog';
import { formatBytes, heroImages, isPluginGame, lastPlayedAgo, storeLabel } from '../lib/games';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { useStorageData, type StorageData } from './useStorageData';

/** The figure for a byte count: formatBytes has no output for 0, and an
 *  unreadable disk (-1) shows an em dash instead of a made-up size. */
function figure(n: number): string {
  if (n < 0) return '—';
  return formatBytes(n) || '0 B';
}

function isWithinFolder(path: string, folder: string): boolean {
  const base = folder.endsWith('/') ? folder.slice(0, -1) : folder;
  return path === folder || path.startsWith(`${base}/`);
}

interface StorageViewProps {
  /** The in-memory library, joined to the snapshot's rows by game id: the
   *  actions need the full game the core calls take. */
  games: Game[];
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Removes a sideloaded game (#43): the row's panel picks the options. */
  onRemove: (game: Game, opts: RemoveOptions) => void;
  /** Queues an uninstall for an installed store game (App's own terms gate). */
  onUninstall: (game: Game) => void;
}

/** Storage (issue #50): one bar per drive (Platypus games / other / free)
 *  and every installed game by size, with the row actions: open the folder,
 *  move, re-measure, remove or uninstall. The core measures in the
 *  background and emits a snapshot when a pass ends; this view only reads. */
export function StorageView(props: StorageViewProps) {
  const data = useStorageData(props);
  // Atlas renders the /22 Storage page (issue #138); other themes keep the layout blocks below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasStorage data={data} />;
  }
  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's storage before the core answers over RPC.
  const layout = useThemeLayout('storage') ?? BUILTIN_LAYOUTS['storage'];
  return (
    <div className="p-7">
      <ThemeLayout
        layout={layout}
        blocks={storageBlocks(data)}
        // A row holding the drives (the cool layout's games + drives row)
        // gives the list room and the drives a narrower side column. The
        // built-in layout has no rows, so today's spacing is untouched.
        rowClassName={(row) =>
          row.some((n) => n.block === 'drives')
            ? 'grid gap-5 xl:grid-cols-[1.75fr_1fr]'
            : undefined}
      />
    </div>
  );
}

// The storage's layout blocks (issue #102): each takes its data as props
// only and renders a single root (or null when it has nothing to show), so
// a layout orders blocks without conditionals of its own.
function storageBlocks(data: ReturnType<typeof useStorageData>): Record<string, React.ReactNode> {
  return {
    header: <StorageHeader scanning={data.scanning} onRescan={data.rescan} />,
    drives: <StorageDrives drives={data.drives} />,
    games: (
      <StorageGames
        sorted={data.sorted}
        scanning={data.scanning}
        byId={data.byId}
        measuring={data.measuring}
        moveId={data.moveId}
        setMoveId={data.setMoveId}
        removeId={data.removeId}
        setRemoveId={data.setRemoveId}
        uninstallId={data.uninstallId}
        setUninstallId={data.setUninstallId}
        scan={data.scan}
        openFolder={data.openFolder}
        onNotify={data.onNotify}
        onRemove={data.onRemove}
        onUninstall={data.onUninstall}
      />
    ),
  };
}

export function StorageHeader({ scanning, onRescan }: { scanning: boolean; onRescan: () => void }) {
  const { t } = useTranslation(['nav', 'storage', 'common']);
  return (
    <ViewHeader
      title={t('nav:title.storage')}
      subtitle={<><p className="mt-0.5 text-[12.5px] text-text-muted">{t('storage:intro')}</p>
      <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-text-muted">{t('storage:about')}</p></>}
      actions={<Button variant="neutral" onClick={onRescan} disabled={scanning}>
        {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
        {scanning ? t('storage:scanning') : t('storage:rescan')}
      </Button>}
    />
  );
}

export function StorageDrives({ drives }: { drives: DiskUsage['drives'] }) {
  const { t } = useTranslation('storage');
  if (drives.length === 0) return null;
  return (
    <section className="mb-6">
      <MonoLabel className="mb-2">{t('storage:drivesTitle')}</MonoLabel>
      <div className="space-y-2">
        {drives.map((d) => (
          <DriveCard key={d.id} drive={d} />
        ))}
      </div>
    </section>
  );
}

export function StorageGames({ sorted, scanning, byId, measuring, moveId, setMoveId, removeId, setRemoveId, uninstallId, setUninstallId, scan, openFolder, onNotify, onRemove, onUninstall }: {
  sorted: DiskUsage['games'];
  scanning: boolean;
  byId: Map<string, Game>;
  measuring: ReadonlySet<string>;
  moveId: string | null;
  setMoveId: (id: string | null) => void;
  removeId: string | null;
  setRemoveId: (id: string | null) => void;
  uninstallId: string | null;
  setUninstallId: (id: string | null) => void;
  scan: (game: Game) => void;
  openFolder: (game: Game) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onRemove: (game: Game, opts: RemoveOptions) => void;
  onUninstall: (game: Game) => void;
}) {
  const { t } = useTranslation(['storage', 'gameSettings', 'common']);
  return (
    <section>
      <MonoLabel className="mb-2">{t('storage:gamesTitle')}</MonoLabel>
      {sorted.length === 0 ? (
        scanning ? (
          <div className="space-y-2" role="status">
            <span className="sr-only">{t('storage:scanning')}</span>
            <div className="h-16 animate-pulse rounded-2xl bg-surface ring-1 ring-hairline" />
            <div className="h-16 animate-pulse rounded-2xl bg-surface ring-1 ring-hairline" />
          </div>
        ) : (
          <EmptyState label={t('storage:emptyTitle')}>{t('storage:emptyBody')}</EmptyState>
        )
      ) : (
        <ul className="space-y-2">
          {sorted.map((row) => {
            const game = byId.get(row.game_id);
            return (
              <li
                key={row.game_id}
                className="rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[13px] font-bold">{row.title}</span>
                      <Badge variant="neutral">{storeLabel(row.store)}</Badge>
                      <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-text-muted">
                        {row.from_plugin ? t('storage:size.fromStore') : t('storage:size.scanned')}
                      </span>
                    </div>
                    <div className="mt-0.5 truncate font-mono text-hint text-text-muted select-text">
                      {row.install_dir}
                    </div>
                    {row.prefix_bytes > 0 && (
                      <div className="mt-1 text-[12px] text-text-muted">
                        {t('storage:size.prefix')}{' '}
                        <span className="tabular-nums">{figure(row.prefix_bytes)}</span>
                      </div>
                    )}
                  </div>
                  <div className="flex flex-none flex-col items-end gap-1.5">
                    <span className="text-[13px] font-bold tabular-nums">{figure(row.size_bytes)}</span>
                    {game && (
                      <div className="flex gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7"
                          aria-label={t('storage:action.open', { title: game.title })}
                          onClick={() => openFolder(game)}
                        >
                          <FolderOpen className="h-4 w-4" />
                        </Button>
                        {/* Core's move goes through the store plugin
                            (planMove): sideloaded and built-in Steam games
                            have none, so no Move for them (same gate the
                            Maintenance tab shows). */}
                        {game.is_installed && isPluginGame(game) && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={t('storage:action.move', { title: game.title })}
                            onClick={() => setMoveId(row.game_id)}
                          >
                            <FolderInput className="h-4 w-4" />
                          </Button>
                        )}
                        {game.is_installed && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={t('storage:action.scan', { title: game.title })}
                            onClick={() => scan(game)}
                          >
                            {measuring.has(game.id) ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <ScanLine className="h-4 w-4" />
                            )}
                          </Button>
                        )}
                        {(game.store === 'custom' || game.is_installed) && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={
                              game.store === 'custom'
                                ? t('storage:action.remove', { title: game.title })
                                : t('storage:action.uninstall', { title: game.title })
                            }
                            onClick={() => {
                              setRemoveId(null);
                              setMoveId(null);
                              setUninstallId(game.store === 'custom' ? null : row.game_id);
                              setRemoveId(game.store === 'custom' ? row.game_id : null);
                            }}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    )}
                  </div>
                </div>

                {moveId === row.game_id && game && (
                  <MoveDialog
                    game={game}
                    startFolder={parentFolder(game.install_dir)}
                    onClose={() => setMoveId(null)}
                    onQueued={() => {
                      setMoveId(null);
                      onNotify(t('gameSettings:maintenance.moving', { title: game.title }), 'busy');
                    }}
                  />
                )}
                {removeId === row.game_id && game && (
                  <div className="mt-3 border-t border-hairline pt-3">
                    <RemoveFlow
                      key={game.id}
                      game={game}
                      onCancel={() => setRemoveId(null)}
                      onRemove={(opts) => {
                        setRemoveId(null);
                        onRemove(game, opts);
                      }}
                    />
                  </div>
                )}
                {uninstallId === row.game_id && game && (
                  <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-hairline pt-3">
                    <p className="min-w-0 flex-1 basis-64 text-[12.5px] text-text-muted">
                      {t('storage:uninstallAsk', { title: game.title })}
                    </p>
                    <Button variant="ghost" size="sm" onClick={() => setUninstallId(null)}>
                      {t('common:cancel')}
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      autoFocus
                      onClick={() => {
                        setUninstallId(null);
                        onUninstall(game);
                      }}
                    >
                      <Trash2 className="h-3.5 w-3.5" /> {t('storage:uninstallGo')}
                    </Button>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

/** The #43 removal of a sideloaded game, reused from the Maintenance tab:
 *  the first row, then the confirm panel with its fetched check. Mounted only
 *  for the row whose Trash was pressed, so its state starts fresh. */
function RemoveFlow({ game, onCancel, onRemove }: {
  game: Game;
  onCancel: () => void;
  onRemove: (opts: RemoveOptions) => void;
}) {
  const [confirm, setConfirm] = useState(false);
  const check = useRemoveCheck(game, true, confirm);
  if (!confirm) return <RemoveFirstRow game={game} onConfirm={() => setConfirm(true)} />;
  return (
    <RemoveConfirmPanel
      game={game}
      check={check.check}
      error={check.error}
      onCancel={onCancel}
      onRemove={onRemove}
    />
  );
}

/** One drive's card: mount, its folders, and the stacked share bar with a
 *  legend. A drive whose totals can't be read (-1) shows an indeterminate
 *  pulse instead of proportions. */
function DriveCard({ drive: d }: { drive: DiskUsage['drives'][number] }) {
  const { t } = useTranslation('storage');
  const known = d.total_bytes > 0;
  const gamesPct = known ? Math.min(100, (d.games_bytes / d.total_bytes) * 100) : 0;
  const otherPct = known ? Math.min(100 - gamesPct, (d.other_bytes / d.total_bytes) * 100) : 0;
  return (
    <div className="rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="text-[13px] font-bold">{d.mount}</span>
        <span className="min-w-0 truncate font-mono text-hint text-text-muted select-text">
          {d.folders.join(' · ')}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={d.mount}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(gamesPct)}
        className={known ? 'mt-2.5 flex h-1.5 overflow-hidden rounded-full bg-surface-strong' : 'mt-2.5 h-1.5 overflow-hidden rounded-full bg-surface-strong animate-pulse'}
      >
        {known && (
          <>
            <div className="bg-dl transition-[width]" style={{ width: `${gamesPct}%` }} />
            <div className="bg-surface-strong transition-[width]" style={{ width: `${otherPct}%` }} />
          </>
        )}
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-text-muted">
        <Legend dot="bg-dl" label={t('storage:legend.games')} value={figure(d.games_bytes)} />
        <Legend dot="bg-surface-strong" label={t('storage:legend.other')} value={figure(d.other_bytes)} />
        <Legend dot="bg-hairline-strong" label={t('storage:legend.free')} value={figure(d.free_bytes)} />
      </div>
    </div>
  );
}

/** One legend entry: swatch, name and tabular-nums figure. */
function Legend({ dot, label, value }: { dot: string; label: string; value: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={`inline-block h-1.5 w-1.5 rounded-full ${dot}`} />
      {label}
      <span className="tabular-nums text-text">{value}</span>
    </span>
  );
}

/** The /22 Storage page (issue #138, Atlas only): one card per drive with
 *  its share bar and game rows (art, last played, size, actions), then the
 *  same rescan/move/remove/uninstall flows. The lab's per-game bar segments,
 *  outside-folder panel and leftovers cleaner have no backing in the usage
 *  snapshot, so the card keeps the aggregate bar and the full row actions. */
function AtlasStorage({ data }: { data: StorageData }) {
  const { t } = useTranslation(['nav', 'storage', 'common']);
  // One card per drive with its games inside (the lab's StoragePage): a row
  // belongs to the drive whose folder holds its install dir, falling back to
  // the mount prefix and then the first card, so no measured game vanishes.
  const gamesByDrive = useMemo(() => {
    const rest = [...data.sorted];
    const groups = data.drives.map((d) => {
      const mine = rest.filter((row) => {
        const dir = data.byId.get(row.game_id)?.install_dir || row.install_dir;
        if (d.folders.some((f) => dir.startsWith(f))) return true;
        return d.mount !== '/' && dir.startsWith(d.mount) && !data.drives.some((o) => o !== d && o.folders.some((f) => dir.startsWith(f)));
      });
      for (const m of mine) rest.splice(rest.indexOf(m), 1);
      return { drive: d, rows: mine };
    });
    if (rest.length > 0 && groups.length > 0) groups[0].rows.push(...rest);
    return groups;
  }, [data.drives, data.sorted, data.byId]);
  const defaultFolder = data.folders?.find((f) => f.is_default)?.path;
  const outside = data.folders ? data.library.filter((g) => g.is_installed && !!g.install_dir && !data.folders.some((f) => isWithinFolder(g.install_dir, f.path))).map((game) => {
    const row = data.sorted.find((r) => r.game_id === game.id);
    return { game, size: row ? row.size_bytes + row.prefix_bytes : game.size_bytes ?? 0 };
  }) : [];
  const leftovers = data.jobs.filter((j) => j.leftover && !j.cleaned_at);
  return (
    <div className={ATLAS_PAGE}>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.storage')}</h1>
          <p className="mt-[3px] text-[13px] text-text-muted">{t('storage:intro')}</p>
        </div>
        <Button variant="neutral" onClick={data.rescan} disabled={data.scanning}>
          {data.scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {data.scanning ? t('storage:scanning') : t('storage:atlas.rescan')}
        </Button>
      </header>
      {data.drives.length === 0 && data.sorted.length === 0 ? (
        data.scanning ? (
          <div className="space-y-2" role="status">
            <span className="sr-only">{t('storage:scanning')}</span>
            <div className="h-16 animate-pulse rounded-2xl bg-surface ring-1 ring-hairline" />
            <div className="h-16 animate-pulse rounded-2xl bg-surface ring-1 ring-hairline" />
          </div>
        ) : (
          <EmptyState label={t('storage:emptyTitle')}>{t('storage:emptyBody')}</EmptyState>
        )
      ) : (
        gamesByDrive.map(({ drive, rows }) => (
          <AtlasDriveCard key={drive.id} drive={drive} rows={rows} data={data} defaultFolder={defaultFolder} />
        ))
      )}
      <StoragePanels outside={outside} leftovers={leftovers} defaultFolder={defaultFolder} data={data} />
    </div>
  );
}

function AtlasDriveCard({ drive: d, rows, data, defaultFolder }: { drive: DiskUsage['drives'][number]; rows: DiskUsage['games'][number][]; data: StorageData; defaultFolder?: string }) {
  const { t } = useTranslation(['storage', 'gameSettings', 'common']);
  const known = d.total_bytes > 0;
  const shown = rows;
  const total = rows.reduce((a, r) => a + r.size_bytes + r.prefix_bytes, 0);
  const isDefaultDrive = !!defaultFolder && d.folders.some((f) => isWithinFolder(defaultFolder, f));
  return (
    <section aria-label={d.mount} className="grid gap-2.5 rounded-2xl bg-surface p-4 ring-1 ring-hairline">
      <header className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="inline-flex items-center gap-2 text-[14px] font-bold">
          <HardDrive className="h-[18px] w-[18px] text-text-faint" aria-hidden />
          {d.mount === '/' ? t('storage:atlas.systemDrive') : d.mount.split('/').filter(Boolean).at(-1) || d.mount}
          {isDefaultDrive && <span className="rounded-md bg-surface-strong px-[7px] py-0.5 text-[11.5px] font-semibold text-text-muted">{t('storage:atlas.default')}</span>}
        </span>
        <span className="text-[12.5px] text-text-muted">
          <strong className={d.free_bytes < 20e9 ? 'font-bold text-danger' : 'font-bold text-text'}>{figure(d.free_bytes)}</strong> {t('storage:atlas.freeOf', { total: figure(d.total_bytes) })}
        </span>
      </header>
      <div className="min-w-0 truncate text-[12px] text-text-faint select-text">{d.folders.join(' · ')}</div>
      <div
        role="progressbar"
        aria-label={d.mount}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={known ? Math.round((d.games_bytes / d.total_bytes) * 100) : undefined}
        className={known ? 'flex h-3.5 gap-0.5 overflow-hidden rounded-[5px] bg-surface-strong' : 'h-3.5 overflow-hidden rounded-[5px] bg-surface-strong animate-pulse'}
      >
        {known && total > 0 && rows.map((r, i) => (
          <div
            key={r.game_id}
            className="shrink-0 bg-[color-mix(in_srgb,var(--color-accent)_75%,var(--color-surface-strong))] transition-[width]"
            style={{ width: `${Math.max(0, ((r.size_bytes + r.prefix_bytes) / d.total_bytes) * 100)}%`, backgroundColor: `color-mix(in srgb, var(--color-accent) ${i % 2 ? 55 : 75}%, var(--color-surface-strong))` }}
            title={`${r.title} · ${figure(r.size_bytes + r.prefix_bytes)}`}
          />
        ))}
        {known && d.other_bytes > 0 && <div className="shrink-0 bg-hairline-strong" style={{ width: `${(d.other_bytes / d.total_bytes) * 100}%` }} />}
        {known && <div className="min-w-0 flex-1 bg-surface-strong" />}
      </div>
      {d.free_bytes >= 0 && d.free_bytes < 20e9 && <p className="flex items-center gap-1.5 text-[12.5px] text-attention"><TriangleAlert className="size-[13px]" />{t('storage:atlas.almostFull')}</p>}
      {shown.length > 0 && (
        <ul className="divide-y divide-hairline">
          {shown.map((row) => (
            <AtlasGameRow key={row.game_id} row={row} data={data} />
          ))}
        </ul>
      )}
    </section>
  );
}

function AtlasGameRow({ row, data }: { row: DiskUsage['games'][number]; data: StorageData }) {
  const { t } = useTranslation(['storage', 'gameSettings', 'common']);
  const game = data.byId.get(row.game_id);
  return (
    <li className="group py-1.5">
      <div className="grid h-10 grid-cols-[56px_minmax(0,1fr)_200px_90px_auto] items-center gap-3">
        <button
          type="button"
          onClick={() => game && data.openFolder(game)}
          className="rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          aria-label={game ? t('storage:action.open', { title: game.title }) : row.title}
          disabled={!game}
        >
          <Cover src={game ? heroImages(game) : []} title={row.title} className="h-[26px] w-14 rounded" />
        </button>
        <div className="truncate text-[13px] font-semibold hover:underline">{row.title}</div>
        <div className="truncate text-[12.5px] text-text-muted">{game?.last_played_at ? t('storage:atlas.played', { ago: lastPlayedAgo(game) ?? '' }) : t('storage:atlas.neverPlayed')}</div>
        <span className="text-[13px] tabular-nums text-text-muted">{figure(row.size_bytes + row.prefix_bytes)}</span>
        {game ? (
          <div className="flex items-center gap-1">
            <span className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
              <Button variant="ghost" size="icon" className="size-7" aria-label={t('storage:action.open', { title: game.title })} onClick={() => data.openFolder(game)}>
                <FolderOpen className="h-4 w-4" />
              </Button>
              {game.is_installed && isPluginGame(game) && (
                <Button variant="ghost" size="icon" className="size-7" aria-label={t('storage:action.move', { title: game.title })} onClick={() => data.setMoveId(row.game_id)}>
                  <FolderInput className="h-4 w-4" />
                </Button>
              )}
              {game.is_installed && (
                <Button variant="ghost" size="icon" className="size-7" aria-label={t('storage:action.scan', { title: game.title })} onClick={() => data.scan(game)}>
                  {data.measuring.has(game.id) ? <Loader2 className="h-4 w-4 animate-spin" /> : <ScanLine className="h-4 w-4" />}
                </Button>
              )}
            </span>
            {(game.store === 'custom' || game.is_installed) && (
              <Button
                variant="ghost"
                size="md"
                className="font-medium text-[13.5px] text-text-muted hover:text-text"
                aria-label={game.store === 'custom' ? t('storage:action.remove', { title: game.title }) : t('storage:action.uninstall', { title: game.title })}
                onClick={() => {
                  data.setRemoveId(null);
                  data.setMoveId(null);
                  data.setUninstallId(game.store === 'custom' ? null : row.game_id);
                  data.setRemoveId(game.store === 'custom' ? row.game_id : null);
                }}
              >
                {t('storage:uninstallGo')}
              </Button>
            )}
          </div>
        ) : (
          <span className="w-[76px]" />
        )}
      </div>
      {data.moveId === row.game_id && game && (
        <MoveDialog
          game={game}
          startFolder={parentFolder(game.install_dir)}
          onClose={() => data.setMoveId(null)}
          onQueued={() => {
            data.setMoveId(null);
            data.onNotify(t('gameSettings:maintenance.moving', { title: game.title }), 'busy');
          }}
        />
      )}
      {data.removeId === row.game_id && game && (
        <div className="mt-2 border-t border-hairline pt-2">
          <RemoveFlow
            key={game.id}
            game={game}
            onCancel={() => data.setRemoveId(null)}
            onRemove={(opts) => {
              data.setRemoveId(null);
              data.onRemove(game, opts);
            }}
          />
        </div>
      )}
      {data.uninstallId === row.game_id && game && (
        <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-hairline pt-2">
          <p className="min-w-0 flex-1 basis-64 text-[12.5px] text-text-muted">
            {t('storage:uninstallAsk', { title: game.title })}
          </p>
          <Button variant="ghost" size="sm" onClick={() => data.setUninstallId(null)}>
            {t('common:cancel')}
          </Button>
          <Button
            variant="danger"
            size="sm"
            autoFocus
            onClick={() => {
              data.setUninstallId(null);
              data.onUninstall(game);
            }}
          >
            <Trash2 className="h-3.5 w-3.5" /> {t('storage:uninstallGo')}
          </Button>
        </div>
      )}
    </li>
  );
}

function StoragePanels({ outside, leftovers, defaultFolder, data }: { outside: { game: Game; size: number }[]; leftovers: Job[]; defaultFolder?: string; data: StorageData }) {
  const { t } = useTranslation(['storage', 'common']);
  const [picked, setPicked] = useState<Set<string> | null>(null);
  const pickedIds = picked ?? new Set(leftovers.map((j) => j.id));
  const defaultName = defaultFolder?.split('/').filter(Boolean).at(-1) || defaultFolder || t('storage:atlas.defaultFolder');
  const clean = () => { void Promise.all([...pickedIds].map((id) => data.discardJob(id))).then(data.refreshJobs).then(() => setPicked(null)); };
  return <div className="grid gap-3 md:grid-cols-2">
    <section className="rounded-xl bg-surface px-4 py-3.5">
      <h2 className="mb-2 text-[14px] font-bold">{t('storage:atlas.outsideTitle')}</h2>
      {outside.length ? <ul className="divide-y divide-hairline">{outside.map(({ game, size }) => {
        return <li key={game.id} className="flex items-center gap-3 py-2">
          <span className="min-w-0 flex-1"><span className="block truncate text-[13px] font-semibold">{game.title}</span><span className="block truncate text-[12px] text-text-faint" title={game.install_dir}>{game.install_dir}, {figure(size)}</span></span>
          <Button variant="neutral" size="sm" onClick={() => data.setMoveId(game.id)}>{t('storage:atlas.moveTo', { folder: defaultName })}</Button>
          {data.moveId === game.id && <MoveDialog game={game} startFolder={parentFolder(game.install_dir)} onClose={() => data.setMoveId(null)} onQueued={() => { data.setMoveId(null); data.onNotify(t('gameSettings:maintenance.moving', { title: game.title }), 'busy'); }} />}
        </li>;
      })}</ul> : <p className="text-[12.5px] text-text-muted">{t('storage:atlas.outsideEmpty')}</p>}
    </section>
    <section className="rounded-xl bg-surface px-4 py-3.5">
      <h2 className="mb-2 text-[14px] font-bold">{t('storage:atlas.leftoversTitle')}</h2>
      {leftovers.length ? <><ul className="divide-y divide-hairline">{leftovers.map((j) => <li key={j.id} className="flex items-center gap-2 py-2 text-[12.5px]">
        <input type="checkbox" checked={pickedIds.has(j.id)} onChange={(e) => setPicked((prev) => { const next = new Set(prev ?? leftovers.map((job) => job.id)); e.target.checked ? next.add(j.id) : next.delete(j.id); return next; })} aria-label={j.title} className="accent-accent" />
        <span className="min-w-0 flex-1 truncate">{j.title}, {t('storage:atlas.canceledInstall')}</span><span className="tabular-nums text-text-muted">{figure(j.bytes)}</span>
      </li>)}</ul><Button variant="primary" size="sm" disabled={!pickedIds.size} onClick={clean}>{t('storage:atlas.clean', { size: figure(leftovers.filter((j) => pickedIds.has(j.id)).reduce((n, j) => n + j.bytes, 0)) })}</Button></> : <p className="text-[12.5px] text-text-muted">{t('storage:atlas.nothingLeft')}</p>}
    </section>
  </div>;
}
