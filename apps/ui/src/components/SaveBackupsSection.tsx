// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { Game, SaveBackup, SavePath, RestorePreview, AutoBackupState } from '../types';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { ErrorNotice } from './ErrorNotice';
import { MonoLabel } from './ui';
import { formatBytes } from '../lib/games';
import { coreErrMsg } from '../lib/coreError';
import { currentLocale } from '../lib/format';
import { useMaintenancePrefetch } from './maintenancePrefetch';

function formatTime(ms: number): string {
  return new Date(ms * 1000).toLocaleString(currentLocale(), { dateStyle: 'medium', timeStyle: 'short' });
}

/** Maintenance → Save backups (issue #42): versions with restore/delete, a
 *  per-game mode, and the save-paths editor with each path's origin. */
export function SaveBackupsSection({ game, onNotify }: {
  game: Game;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [backups, setBackups] = useState<SaveBackup[] | null>(null);
  const [paths, setPaths] = useState<SavePath[] | null>(null);
  const [mode, setMode] = useState('');
  const [auto, setAuto] = useState<AutoBackupState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmRestore, setConfirmRestore] = useState<string | null>(null);
  const [restorePreview, setRestorePreview] = useState<RestorePreview | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  // The dialog starts these calls when it opens (issue #56): await the same
  // promises instead of firing new ones — one fetch per dialog open, shared
  // by the tab. Outside the dialog there is nothing shared, so the section
  // fetches for itself as before.
  const pre = useMaintenancePrefetch();
  const shared = pre != null && pre.gameId === game.id ? pre : null;

  const reload = () => {
    Effect.runPromise(LauncherApi.listSaveBackups(game.id))
      .then((v) => { if (shared) shared.latest.backups = v; setBackups(v); })
      .catch((e) => setError(coreErrMsg(e)));
    Effect.runPromise(LauncherApi.getSavePaths(game))
      .then((v) => { if (shared) shared.latest.paths = v; setPaths(v); })
      .catch((e) => setError(coreErrMsg(e)));
    Effect.runPromise(LauncherApi.getGameSaveBackup(game.id))
      .then((v) => { if (shared) shared.latest.mode = v; setMode(v); })
      .catch(() => setMode(''));
    Effect.runPromise(LauncherApi.getAutoBackupState(game))
      .then((v) => { if (shared) shared.latest.auto = v; setAuto(v); })
      .catch(() => setAuto(null));
  };

  useEffect(() => {
    if (!shared) {
      reload();
      return;
    }
    let live = true;
    // Fresh answers the dialog already holds (a remount after an edit) apply
    // at once; anything still flying is awaited, never refetched. Either way
    // the first frame below already has its final structure (loading text in
    // place, never null), so the tab never flashes empty.
    if (shared.latest.backups !== undefined) setBackups(shared.latest.backups);
    else shared.backups.then((v) => { if (live) setBackups(v); }, (e) => { if (live) setError(coreErrMsg(e)); });
    if (shared.latest.paths !== undefined) setPaths(shared.latest.paths);
    else shared.paths.then((v) => { if (live) setPaths(v); }, (e) => { if (live) setError(coreErrMsg(e)); });
    if (shared.latest.mode !== undefined) setMode(shared.latest.mode);
    else shared.mode.then((v) => { if (live) setMode(v); }, () => { if (live) setMode(''); });
    if (shared.latest.auto !== undefined) setAuto(shared.latest.auto);
    else void shared.auto.then((v) => { if (live) setAuto(v); });
    return () => { live = false; };
  }, [game.id, shared]); // eslint-disable-line react-hooks/exhaustive-deps

  const backupNow = () => {
    setBusy(true);
    Effect.runPromise(LauncherApi.createSaveBackup(game))
      .then(() => {
        onNotify(t('gameSettings:maintenance.saveBackups.queued'), 'busy');
        reload();
      })
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const restore = (id: string) => {
    setConfirmRestore(null);
    setRestorePreview(null);
    Effect.runPromise(LauncherApi.restoreSaveBackup(game, id))
      .then(() => onNotify(t('gameSettings:maintenance.saveBackups.restoreQueued'), 'busy'))
      .catch((e) => setError(coreErrMsg(e)));
  };

  const askRestore = (id: string) => {
    setConfirmDelete(null);
    setConfirmRestore(id);
    setRestorePreview(null);
    Effect.runPromise(LauncherApi.previewRestoreSaveBackup(game, id))
      .then(setRestorePreview)
      .catch((e) => setError(coreErrMsg(e)));
  };

  const remove = (id: string) => {
    setConfirmDelete(null);
    Effect.runPromise(LauncherApi.deleteSaveBackup(game.id, id))
      .then(() => {
        onNotify(t('gameSettings:maintenance.saveBackups.deleted'));
        reload();
      })
      .catch((e) => setError(coreErrMsg(e)));
  };

  // The user's own save folders: added with the folder picker, removed one
  // by one. Found folders (the community list) aren't editable.
  const userPaths = (paths ?? []).filter((p) => p.origin === 'user').map((p) => p.path);
  const writePaths = (next: string[], doneKey: string) => {
    Effect.runPromise(LauncherApi.setGameSavePaths(game.id, next))
      .then(() => {
        onNotify(t(doneKey));
        reload();
      })
      .catch((e) => setError(coreErrMsg(e)));
  };
  const addFolder = () => {
    Effect.runPromise(LauncherApi.pickFolder(t('gameSettings:maintenance.saveBackups.pathsPickTitle'), userPaths[0] || game.install_dir || ''))
      .then((dir) => {
        if (!dir || userPaths.includes(dir)) return;
        writePaths([...userPaths, dir], 'gameSettings:maintenance.saveBackups.pathAdded');
      })
      .catch((e) => setError(coreErrMsg(e)));
  };
  const removePath = (dir: string) => writePaths(userPaths.filter((p) => p !== dir), 'gameSettings:maintenance.saveBackups.pathRemoved');

  const saveMode = (next: string) => {
    setMode(next);
    Effect.runPromise(LauncherApi.setGameSaveBackup(game.id, next))
      .then(reload)
      .catch((e) => setError(coreErrMsg(e)));
  };

  const autoText = auto == null
    ? null
    : auto.on
      ? t('gameSettings:maintenance.saveBackups.autoOn')
      : auto.reason === 'cloud'
        ? t('gameSettings:maintenance.saveBackups.autoOffCloud')
        : t('gameSettings:maintenance.saveBackups.autoOff');
  // Restoring writes into the game's current folders: without an install
  // (an uninstalled game) the targets aren't on disk, so Restore stays off
  // with a short reason while Delete still works.
  const canRestore = game.is_installed !== false;

  return (
    <div className="space-y-5">
      <section>
        <MonoLabel className="mb-1">{t('gameSettings:maintenance.saveBackups.heading').toLowerCase()}</MonoLabel>
        <p className="text-[12.5px] leading-relaxed text-white/60">{t('gameSettings:maintenance.saveBackups.description')}</p>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button onClick={backupNow} disabled={busy}>
            {busy ? t('gameSettings:maintenance.saveBackups.backingUp') : t('gameSettings:maintenance.saveBackups.backupNow')}
          </Button>
          <label className="flex items-center gap-2">
            <span className="text-[12.5px] text-white/70">{t('gameSettings:maintenance.saveBackups.modeLabel')}</span>
            <Select value={mode} onValueChange={saveMode}>
              <SelectTrigger aria-label={t('gameSettings:maintenance.saveBackups.modeLabel')} className="w-52">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t('gameSettings:maintenance.saveBackups.mode.follow')}</SelectItem>
                <SelectItem value="on">{t('gameSettings:maintenance.saveBackups.mode.on')}</SelectItem>
                <SelectItem value="off">{t('gameSettings:maintenance.saveBackups.mode.off')}</SelectItem>
              </SelectContent>
            </Select>
          </label>
        </div>
        {/* The auto-backup line keeps its line from the first frame: loading
            text in place until the state lands, never nothing. */}
        {auto == null ? (
          <p role="status" className="mt-2 min-h-[18px] text-[12px] text-white/55">{t('gameSettings:maintenance.saveBackups.loading')}</p>
        ) : autoText && (
          <p className="mt-2 min-h-[18px] text-[12px] text-white/55">{autoText}</p>
        )}
      </section>

      <section>
        {/* The backup list keeps its place from the first frame: loading text
            until the versions land, never nothing. */}
        {backups == null ? (
          <p role="status" data-testid="save-backups-loading" className="text-[12.5px] text-white/55">{t('gameSettings:maintenance.saveBackups.loading')}</p>
        ) : backups.length === 0 ? (
          <p className="text-[12.5px] text-white/55">{t('gameSettings:maintenance.saveBackups.empty')}</p>
        ) : (
          <ul className="space-y-2">
            {backups.map((b) => (
              <li key={b.id} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-lg bg-white/[0.03] px-3 py-2">
                <div className="min-w-0">
                  <div className="text-[12.5px] text-white/85">{formatTime(b.created_at)} · {b.trigger} · {formatBytes(b.size)}</div>
                  <div className="font-mono text-[11.5px] text-white/50">{b.files} {t('gameSettings:maintenance.saveBackups.files')}</div>
                </div>
                <div className="flex flex-none gap-2">
                  {confirmRestore === b.id ? (
                    <>
                      <span className="self-center text-[12px] text-white/60">
                        {restorePreview
                          ? t('gameSettings:maintenance.saveBackups.restorePreview', {
                            overwritten: restorePreview.overwritten,
                            added: restorePreview.added,
                            kept: restorePreview.kept,
                          })
                          : t('gameSettings:maintenance.saveBackups.restoreConfirm')}
                      </span>
                      <Button variant="ghost" size="sm" onClick={() => { setConfirmRestore(null); setRestorePreview(null); }}>{t('common:cancel')}</Button>
                      <Button variant="danger" size="sm" onClick={() => restore(b.id)}>{t('gameSettings:maintenance.saveBackups.restore')}</Button>
                    </>
                  ) : confirmDelete === b.id ? (
                    <>
                      <span className="self-center text-[12px] text-white/60">{t('gameSettings:maintenance.saveBackups.deleteConfirm')}</span>
                      <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(null)}>{t('common:cancel')}</Button>
                      <Button variant="danger" size="sm" onClick={() => remove(b.id)}>{t('gameSettings:maintenance.saveBackups.delete')}</Button>
                    </>
                  ) : (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => askRestore(b.id)}
                        disabled={!canRestore}
                        title={!canRestore ? t('gameSettings:maintenance.saveBackups.restoreUnavailable') : undefined}
                      >
                        {t('gameSettings:maintenance.saveBackups.restore')}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => { setConfirmRestore(null); setRestorePreview(null); setConfirmDelete(b.id); }}>
                        {t('gameSettings:maintenance.saveBackups.delete')}
                      </Button>
                    </>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
      {!canRestore && backups != null && backups.length > 0 && (
        <p className="text-[12px] text-white/55">{t('gameSettings:maintenance.saveBackups.restoreUnavailable')}</p>
      )}

      <section>
        <MonoLabel className="mb-1">{t('gameSettings:maintenance.saveBackups.pathsHeading').toLowerCase()}</MonoLabel>
        <p className="text-[12.5px] leading-relaxed text-white/60">{t('gameSettings:maintenance.saveBackups.pathsDescription')}</p>
        {/* The path list keeps its place from the first frame: loading text
            until the folders land, never an empty list. */}
        {paths == null ? (
          <p role="status" className="mt-2 text-[12px] text-white/55">{t('gameSettings:maintenance.saveBackups.loading')}</p>
        ) : (
          <>
            {paths.length === 0 && (
              <p className="mt-2 text-[12px] text-white/55">{t('gameSettings:maintenance.saveBackups.pathsNone')}</p>
            )}
            <ul className="mt-2 space-y-1">
              {paths.map((p) => (
            <li key={`${p.origin}:${p.path}`} className="flex items-center justify-between gap-3 font-mono text-[12px]">
              <span className="min-w-0 truncate text-white/75" title={p.path}>{p.path}</span>
              <span className="flex flex-none items-center gap-2 text-white/50">
                {p.origin === 'user'
                  ? t('gameSettings:maintenance.saveBackups.pathOrigin.user')
                  : t('gameSettings:maintenance.saveBackups.pathOrigin.manifest')}
                {!p.exists && ` · ${t('gameSettings:maintenance.saveBackups.pathMissing')}`}
                {p.origin === 'user' && (
                  <Button variant="ghost" size="sm" onClick={() => removePath(p.path)}
                    aria-label={t('gameSettings:maintenance.saveBackups.pathRemove', { path: p.path })}>
                    {t('gameSettings:maintenance.saveBackups.pathRemoveShort')}
                  </Button>
                )}
              </span>
            </li>
          ))}
        </ul>
          </>
        )}
        <div className="mt-2">
          <Button variant="ghost" size="sm" onClick={addFolder}>
            {t('gameSettings:maintenance.saveBackups.pathsAdd')}
          </Button>
        </div>
      </section>

      {error && <ErrorNotice message={error} fallback={t('gameSettings:maintenance.saveBackups.loadError')} />}
    </div>
  );
}
