// i18n: converted
// The /22 Storage page (issue #138): drive cards + panels.
// Moved out of StorageView.tsx (issue #156) so each file holds one presentation.
import { useTranslation } from 'react-i18next';
import React, { useMemo, useState } from 'react';
import { FolderInput, FolderOpen, HardDrive, Loader2, RefreshCw, ScanLine, Trash2, TriangleAlert } from 'lucide-react';
import type { DiskUsage, Game, Job } from '../types';
import { ATLAS_PAGE, Cover, EmptyState } from './ui';
import { Button } from './ds/button';
import { RemoveFirstRow, RemoveConfirmPanel, parentFolder, useRemoveCheck } from './MaintenanceTab';
import { MoveDialog } from './MoveDialog';
import { Modal } from './Modal';
import { formatBytes, heroImages, isPluginGame, lastPlayedAgo } from '../lib/games';
import { figure, deviceFacts, isWithinFolder, PrefixBackups, RemoveFlow } from './StorageView';
import type { StorageData } from './useStorageData';

/** The /22 Storage page (issue #138, Atlas only): one card per drive with
 *  its share bar and game rows (art, last played, size, actions), then the
 *  same rescan/move/remove/uninstall flows. The lab's per-game bar segments,
 *  outside-folder panel and leftovers cleaner have no backing in the usage
 *  snapshot, so the card keeps the aggregate bar and the full row actions. */
export function AtlasStorage({ data }: { data: StorageData }) {
  const { t } = useTranslation(['nav', 'storage', 'common']);
  // One card per drive with its games inside (the lab's StoragePage): a row
  // belongs to the drive whose folder holds its install dir, falling back to
  // the mount prefix and then the first card, so no measured game vanishes.
  const gamesByDrive = useMemo(() => {
    const rest = [...data.sorted];
    const groups = data.drives.map((d) => {
      const mine = rest.filter((row) => {
        const dir = data.byId.get(row.game_id)?.install_dir || row.install_dir;
        if (d.folders.some((f) => isWithinFolder(dir, f))) return true;
        return d.mount !== '/' && isWithinFolder(dir, d.mount) && !data.drives.some((o) => o !== d && o.folders.some((f) => isWithinFolder(dir, f)));
      });
      for (const m of mine) rest.splice(rest.indexOf(m), 1);
      return { drive: d, rows: mine };
    });
    if (rest.length > 0 && groups.length > 0) groups[0].rows.push(...rest);
    return groups;
  }, [data.drives, data.sorted, data.byId]);
  const defaultFolder = data.folders?.find((f) => f.is_default)?.path;
  const outside = data.library.filter((g) => g.is_installed && g.outside_folder).map((game) => {
    const row = data.sorted.find((r) => r.game_id === game.id);
    return { game, size: row ? row.size_bytes + row.prefix_bytes : game.size_bytes ?? 0 };
  });
  const leftovers = data.jobs.filter((j) => j.state === 'canceled' && !j.cleaned_at && (j.kind === 'install' || j.kind === 'update') && j.bytes > 0);
  const otherLeftovers = data.jobs.filter((j) => j.leftover && !j.cleaned_at && !leftovers.includes(j));
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
      <StoragePanels outside={outside} leftovers={leftovers} otherLeftovers={otherLeftovers} defaultFolder={defaultFolder} data={data} />
    </div>
  );
}

export function AtlasDriveCard({ drive: d, rows, data, defaultFolder }: { drive: DiskUsage['drives'][number]; rows: DiskUsage['games'][number][]; data: StorageData; defaultFolder?: string }) {
  const { t } = useTranslation(['storage', 'gameSettings', 'common']);
  const known = d.total_bytes > 0;
  const shown = rows;
  const total = rows.reduce((a, r) => a + r.size_bytes + r.prefix_bytes, 0);
  const isDefaultDrive = !!defaultFolder && d.folders.some((f) => isWithinFolder(defaultFolder, f));
  // The design's drive head: the label when the filesystem has one, then
  // what the drive is (issue #148) over its folders.
  const name = d.label || (d.mount === '/' ? t('storage:atlas.systemDrive') : d.mount.split('/').filter(Boolean).at(-1) || d.mount);
  const facts = deviceFacts(d, t);
  return (
    <section aria-label={d.mount} className="grid gap-2.5 rounded-2xl bg-surface p-4 ring-1 ring-hairline">
      <header className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="inline-flex items-center gap-2 text-[14px] font-bold">
          <HardDrive className="h-[18px] w-[18px] text-text-faint" aria-hidden />
          {name}
          {isDefaultDrive && <span className="rounded-md bg-surface-strong px-[7px] py-0.5 text-[11.5px] font-semibold text-text-muted">{t('storage:atlas.default')}</span>}
        </span>
        <span className="text-[12.5px] text-text-muted">
          <strong className={d.free_bytes < 20e9 ? 'font-bold text-danger' : 'font-bold text-text'}>{figure(d.free_bytes)}</strong> {t('storage:atlas.freeOf', { total: figure(d.total_bytes) })}
        </span>
      </header>
      <div className="min-w-0 truncate text-[12px] text-text-faint select-text">{d.folders.join(' · ')}{facts && <>, {facts}</>}</div>
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

export function AtlasGameRow({ row, data }: { row: DiskUsage['games'][number]; data: StorageData }) {
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
      {data.moveId === row.game_id && game && !game.outside_folder && (
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

export function StoragePanels({ outside, leftovers, otherLeftovers, defaultFolder, data }: { outside: { game: Game; size: number }[]; leftovers: Job[]; otherLeftovers: Job[]; defaultFolder?: string; data: StorageData }) {
  const { t } = useTranslation(['storage', 'common']);
  // What the user unticked: new leftovers start ticked, and a clean keeps
  // the rest of the user's choice.
  const [unpicked, setUnpicked] = useState<Set<string>>(() => new Set());
  const pickedIds = new Set(leftovers.filter((j) => !unpicked.has(j.id)).map((j) => j.id));
  const defaultName = defaultFolder?.split('/').filter(Boolean).at(-1) || defaultFolder || t('storage:atlas.defaultFolder');
  const clean = () => { void Promise.all([...pickedIds].map((id) => data.discardJob(id))).then(data.refreshJobs); };
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
        <input type="checkbox" checked={pickedIds.has(j.id)} onChange={(e) => setUnpicked((prev) => { const next = new Set(prev); e.target.checked ? next.delete(j.id) : next.add(j.id); return next; })} aria-label={j.title} className="accent-accent" />
        <span className="min-w-0 flex-1 truncate">{j.title}, {t(j.kind === 'update' ? 'storage:atlas.canceledUpdate' : 'storage:atlas.canceledInstall')}</span><span className="tabular-nums text-text-muted">{figure(j.bytes)}</span>
      </li>)}</ul><Button variant="primary" size="sm" disabled={!pickedIds.size} onClick={clean}>{t('storage:atlas.clean', { size: figure(leftovers.filter((j) => pickedIds.has(j.id)).reduce((n, j) => n + j.bytes, 0)) })}</Button></> : <p className="text-[12.5px] text-text-muted">{t('storage:atlas.nothingLeft')}</p>}
      {otherLeftovers.map((j) => {
        const game = data.library.find((g) => g.id === j.game_id);
        return (
          <div key={j.id} className="mt-2 flex items-center gap-2 text-[12px] text-text-faint">
            <span className="min-w-0 flex-1 truncate">{j.title}: {j.leftover}</span>
            {game && <Button variant="ghost" size="sm" onClick={() => data.openFolder(game)}>{t('storage:action.open', { title: j.title })}</Button>}
          </div>
        );
      })}
    </section>
    <PrefixBackups backups={data.backups} data={data} />
  </div>;
}
