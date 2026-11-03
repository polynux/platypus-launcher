// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useMemo, useState } from 'react';
import { FolderInput, FolderOpen, HardDrive, HardDriveDownload, Loader2, RefreshCw, ScanLine, Trash2, TriangleAlert } from 'lucide-react';
import type { DiskUsage, Game, Job, PrefixBackup, RemoveOptions } from '../types';
import { Cover, EmptyState, MonoLabel, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { RemoveConfirmPanel, RemoveFirstRow, parentFolder, useRemoveCheck } from './MaintenanceTab';
import { MoveDialog } from './MoveDialog';
import { Modal } from './Modal';
import { formatBytes, heroImages, isPluginGame, lastPlayedAgo, storeLabel } from '../lib/games';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { useStorageData } from './useStorageData';
import type { StorageData } from './useStorageData';
import { AtlasStorage } from './AtlasStorage';

/** The suffix ResetPrefix gives a backup folder, past the game's id. */
export const BACKUP_SUFFIX = /\.bak-\d+(-\d+)?$/;

/** The figure for a byte count: formatBytes has no output for 0, and an
 *  unreadable disk (-1) shows an em dash instead of a made-up size. */
export function figure(n: number): string {
  if (n < 0) return '—';
  return formatBytes(n) || '0 B';
}

/** What the drive is (issue #148), e.g. "NVMe, ext4": the device kind the
 *  core read from sysfs, translated, and the filesystem type from
 *  mountinfo — either one alone when the other is unknown, "" when both
 *  are. An unrecognised kind shows the filesystem only. */
export function deviceFacts(d: DiskUsage['drives'][number], t: (key: string) => string): string {
  const kinds: Record<string, string> = {
    nvme: t('storage:device.nvme'),
    ssd: t('storage:device.ssd'),
    hdd: t('storage:device.hdd'),
    removable: t('storage:device.removable'),
    network: t('storage:device.network'),
  };
  return [kinds[d.device_kind] ?? '', d.fs_type].filter(Boolean).join(', ');
}

export function isWithinFolder(path: string, folder: string): boolean {
  const base = folder.endsWith('/') ? folder.slice(0, -1) : folder;
  return path === folder || path.startsWith(`${base}/`);
}

interface StorageViewProps {
  /** The in-memory library, joined to the snapshot's rows by game id: the
   *  actions need the full game the core calls take. */
  games: Game[];
  jobs?: Job[];
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
  // One branch per view root (issue #156): the storage presentation, not the shell.
  const presentation = useVariant('storage.presentation');
  if (presentation === 'cards') {
    return <AtlasStorage data={data} />;
  }
  return <ClassicStorage data={data} />;
}

function ClassicStorage({ data }: { data: ReturnType<typeof useStorageData> }) {
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
      {data.backups.length > 0 && (
        <div className="mt-6">
          <PrefixBackups backups={data.backups} data={data} />
        </div>
      )}
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

/** The prefix backups the resets left behind (issue #163): each one listed
 *  with the space it takes, and deleted from here — behind a confirm, since
 *  the old drive_c holds the game's saves. Hidden when a reset never left
 *  one, which is the usual case. */
export function PrefixBackups({ backups, data }: { backups: PrefixBackup[]; data: StorageData }) {
  const { t } = useTranslation(['storage', 'common']);
  // The backup whose delete was asked for, and that is now waiting for the
  // confirmation: deleting one is permanent, saves included.
  const [pending, setPending] = useState<PrefixBackup | null>(null);
  if (backups.length === 0) return null;
  // Who the backup belongs to: its folder carries the game's id, so the
  // library names it, and the id stands in for a game that left the library.
  const titleOf = (backup: PrefixBackup) =>
    data.library.find((g) => backup.name.startsWith(`${g.id}.bak-`))?.title ?? backup.name.replace(BACKUP_SUFFIX, '');
  const remove = (backup: PrefixBackup) => {
    void data.deleteBackup(backup).then((freed) => {
      if (freed !== null) data.onNotify(t('storage:backups.deleted', { name: backup.name, size: figure(freed) }), 'ok');
    });
  };
  return (
    <section className="rounded-xl bg-surface px-4 py-3.5">
      <h2 className="mb-2 text-[14px] font-bold">{t('storage:backups.title')}</h2>
      <p className="mb-2 text-[12.5px] text-text-muted">{t('storage:backups.intro')}</p>
      <ul className="divide-y divide-hairline">
        {backups.map((backup) => (
          <li key={backup.path} className="flex items-center gap-2 py-2 text-[12.5px]">
            <span className="min-w-0 flex-1 truncate select-text" title={backup.path}>{backup.name}</span>
            <span className="tabular-nums text-text-muted">{figure(backup.bytes)}</span>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              aria-label={t('storage:backups.delete', { name: backup.name })}
              onClick={() => setPending(backup)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </li>
        ))}
      </ul>
      {pending && (
        <Modal label={t('storage:backups.confirmTitle')} onClose={() => setPending(null)} size="sm">
          <div className="p-6">
            <h2 className="mb-2 text-[17px] font-bold">{t('storage:backups.confirmTitle')}</h2>
            <p className="text-[13px] leading-relaxed text-text-muted">
              {t('storage:backups.confirmBody', { title: titleOf(pending), name: pending.name, size: figure(pending.bytes) })}
            </p>
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="ghost" autoFocus onClick={() => setPending(null)}>{t('common:cancel')}</Button>
              <Button
                variant="danger"
                onClick={() => {
                  const backup = pending;
                  setPending(null);
                  remove(backup);
                }}
              >
                <Trash2 className="h-3.5 w-3.5" /> {t('storage:backups.confirmGo')}
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </section>
  );
}

/** The #43 removal of a sideloaded game, reused from the Maintenance tab:
 *  the first row, then the confirm panel with its fetched check. Mounted only
 *  for the row whose Trash was pressed, so its state starts fresh. */
export function RemoveFlow({ game, onCancel, onRemove }: {
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
  // Its own label and what it is (issue #148): "JEUX · NVMe, ext4" under the mount.
  const facts = [d.label, deviceFacts(d, t)].filter(Boolean).join(' · ');
  return (
    <div className="rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="flex items-baseline gap-2">
          <span className="text-[13px] font-bold">{d.mount}</span>
          {facts && <span className="text-[11.5px] text-text-muted">{facts}</span>}
        </span>
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
