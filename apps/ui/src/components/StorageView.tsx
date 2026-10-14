// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useMemo, useState } from 'react';
import { Effect } from 'effect';
import { FolderInput, FolderOpen, Loader2, RefreshCw, ScanLine, Trash2 } from 'lucide-react';
import type { DiskUsage, Game, RemoveOptions } from '../types';
import { LauncherApi, onDiskUsage } from '../effect/runtime';
import { EmptyState, MonoLabel } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { RemoveConfirmPanel, RemoveFirstRow, parentFolder, useRemoveCheck } from './MaintenanceTab';
import { MoveDialog } from './MoveDialog';
import { formatBytes, isPluginGame, storeLabel } from '../lib/games';
import { coreErrMsg } from '../lib/coreError';

/** The figure for a byte count: formatBytes has no output for 0, and an
 *  unreadable disk (-1) shows an em dash instead of a made-up size. */
function figure(n: number): string {
  if (n < 0) return '—';
  return formatBytes(n) || '0 B';
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
export function StorageView({ games: library, onNotify, onRemove, onUninstall }: StorageViewProps) {
  const { t } = useTranslation(['nav', 'storage', 'gameSettings', 'common']);
  const [usage, setUsage] = useState<DiskUsage | null>(null);
  // Rows whose Scan was pressed, waiting for the next snapshot (local, not
  // global: only this view shows them).
  const [measuring, setMeasuring] = useState<ReadonlySet<string>>(new Set());
  const [moveId, setMoveId] = useState<string | null>(null);
  const [removeId, setRemoveId] = useState<string | null>(null);
  const [uninstallId, setUninstallId] = useState<string | null>(null);
  // The first answer can land after an event already brought a newer
  // snapshot: keep the newer of the two.
  const apply = (next: DiskUsage | null) => {
    if (!next) return;
    setUsage((prev) => (prev && next.generated_at < prev.generated_at ? prev : next));
  };
  useEffect(() => {
    Effect.runPromise(LauncherApi.getDiskUsage()).then(apply).catch(() => {});
    return onDiskUsage(apply);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // A finished pass answers every row's Scan: the spinners go with it.
  useEffect(() => {
    if (usage && !usage.scanning) setMeasuring(new Set());
  }, [usage]);
  const rescan = () => {
    Effect.runPromise(LauncherApi.refreshDiskUsage()).then(apply).catch(() => {});
  };
  const scanning = usage?.scanning ?? false;
  // Biggest first, by files + prefix: the payload comes sorted, but a later
  // snapshot or a partial event may not.
  const sorted = useMemo(
    () =>
      [...(usage?.games ?? [])].sort(
        (a, b) => b.size_bytes + b.prefix_bytes - (a.size_bytes + a.prefix_bytes),
      ),
    [usage],
  );
  const byId = useMemo(() => new Map(library.map((g) => [g.id, g])), [library]);
  const drives = usage?.drives ?? [];
  const scan = (game: Game) => {
    setMeasuring((prev) => new Set(prev).add(game.id));
    Effect.runPromise(LauncherApi.scanGameSize(game)).then(apply).catch((err) => {
      setMeasuring((prev) => {
        const next = new Set(prev);
        next.delete(game.id);
        return next;
      });
      onNotify(coreErrMsg(err), 'err');
    });
  };
  const openFolder = (game: Game) => {
    Effect.runPromise(LauncherApi.openGameFolder(game)).catch((err) => onNotify(coreErrMsg(err), 'err'));
  };
  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">{t('nav:title.storage')}</h1>
          <p className="mt-0.5 text-[12.5px] text-white/70">{t('storage:intro')}</p>
          <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-white/70">{t('storage:about')}</p>
        </div>
        <Button variant="neutral" onClick={rescan} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? t('storage:scanning') : t('storage:rescan')}
        </Button>
      </header>

      {drives.length > 0 && (
        <section className="mb-6">
          <MonoLabel className="mb-2">{t('storage:drivesTitle')}</MonoLabel>
          <div className="space-y-2">
            {drives.map((d) => (
              <DriveCard key={d.id} drive={d} />
            ))}
          </div>
        </section>
      )}

      <section>
        <MonoLabel className="mb-2">{t('storage:gamesTitle')}</MonoLabel>
        {sorted.length === 0 ? (
          scanning ? (
            <div className="space-y-2" role="status">
              <span className="sr-only">{t('storage:scanning')}</span>
              <div className="h-16 animate-pulse rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07]" />
              <div className="h-16 animate-pulse rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07]" />
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
                  className="rounded-2xl bg-white/[0.045] px-4 py-3 ring-1 ring-white/[0.07]"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-[13px] font-bold">{row.title}</span>
                        <Badge variant="neutral">{storeLabel(row.store)}</Badge>
                        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/55">
                          {row.from_plugin ? t('storage:size.fromStore') : t('storage:size.scanned')}
                        </span>
                      </div>
                      <div className="mt-0.5 truncate font-mono text-hint text-white/55 select-text">
                        {row.install_dir}
                      </div>
                      {row.prefix_bytes > 0 && (
                        <div className="mt-1 text-[12px] text-white/70">
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
                    <div className="mt-3 border-t border-white/[0.06] pt-3">
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
                    <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-white/[0.06] pt-3">
                      <p className="min-w-0 flex-1 basis-64 text-[12.5px] text-white/60">
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
    </div>
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
    <div className="rounded-2xl bg-white/[0.045] px-4 py-3 ring-1 ring-white/[0.07]">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="text-[13px] font-bold">{d.mount}</span>
        <span className="min-w-0 truncate font-mono text-hint text-white/55 select-text">
          {d.folders.join(' · ')}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={d.mount}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(gamesPct)}
        className={known ? 'mt-2.5 flex h-1.5 overflow-hidden rounded-full bg-white/15' : 'mt-2.5 h-1.5 overflow-hidden rounded-full bg-white/15 animate-pulse'}
      >
        {known && (
          <>
            <div className="bg-amber-300 transition-[width]" style={{ width: `${gamesPct}%` }} />
            <div className="bg-white/25 transition-[width]" style={{ width: `${otherPct}%` }} />
          </>
        )}
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-white/70">
        <Legend dot="bg-amber-300" label={t('storage:legend.games')} value={figure(d.games_bytes)} />
        <Legend dot="bg-white/25" label={t('storage:legend.other')} value={figure(d.other_bytes)} />
        <Legend dot="bg-white/15" label={t('storage:legend.free')} value={figure(d.free_bytes)} />
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
      <span className="tabular-nums text-white/85">{value}</span>
    </span>
  );
}
