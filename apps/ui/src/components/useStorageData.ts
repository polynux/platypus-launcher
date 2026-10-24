// Storage data (issue #102): the view's core reads and row flows, owned by
// the view. Blocks take the returned state and callbacks as props only, so a
// layout can move them without touching the core.
import { useEffect, useMemo, useState } from 'react';
import { Effect } from 'effect';
import type { DiskUsage, Game, RemoveOptions } from '../types';
import { LauncherApi, onDiskUsage } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';

interface UseStorageDataProps {
  /** The in-memory library, joined to the snapshot's rows by game id. */
  games: Game[];
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Removes a sideloaded game (#43): the row's panel picks the options. */
  onRemove: (game: Game, opts: RemoveOptions) => void;
  /** Queues an uninstall for an installed store game (App's own terms gate). */
  onUninstall: (game: Game) => void;
}

export interface StorageData extends ReturnType<typeof useStorageData> {}

export function useStorageData({ games: library, onNotify, onRemove, onUninstall }: UseStorageDataProps) {
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
  return {
    usage, measuring, moveId, setMoveId, removeId, setRemoveId, uninstallId, setUninstallId,
    rescan, scanning, sorted, byId, drives, scan, openFolder, onNotify, onRemove, onUninstall, library,
  };
}
