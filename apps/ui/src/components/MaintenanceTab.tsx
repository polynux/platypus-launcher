// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { FolderInput, ShieldCheck, Trash2 } from 'lucide-react';
import type { Game, GameJobKind, Job, RemoveCheck, RemoveOptions, RemoveTarget, SavePath, StorePlugin, AutoBackupState } from '../types';
import { Button } from '@/components/ds/button';
import { CapabilitySurface, offers } from '../lib/capabilities';
import { JobProgress } from './JobProgress';
import { MoveDialog } from './MoveDialog';
import { SaveBackupsSection } from './SaveBackupsSection';
import { VerifyDialog } from './VerifyDialog';
import { MonoLabel } from './ui';
import { formatBytes } from '../lib/games';
import { ErrorNotice } from './ErrorNotice';
import { LauncherApi } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';
import { useMaintenancePrefetch } from './maintenancePrefetch';
import i18n from '../lib/i18n';

/** The folder a game folder sits in ("" when there is none). */
export function parentFolder(dir?: string): string {
  if (!dir) return '';
  const i = dir.search(/[/\\][^/\\]*$/);
  return i > 0 ? dir.slice(0, i) : dir;
}

/** What is done to an installed game's files: check them, move them, remove them. */
export function MaintenanceTab({ game, plugin, job, jobs, onJob, onJobAction, onNotify, onClose, onRemove, running }: {
  game: Game;
  plugin?: StorePlugin;
  job?: Job;
  jobs: Job[];
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Closes the settings: a removed game has nothing left to set. */
  onClose: () => void;
  /** Removes a sideloaded game from the library (custom games only). The
   *  options say whether its Wine prefix and/or game files go with it; both
   *  off means library only. */
  onRemove?: (opts: RemoveOptions) => void;
  /** The game is starting or running: it can't leave the library until it stops. */
  running?: boolean;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [verifyOpen, setVerifyOpen] = useState(false);
  const [moveOpen, setMoveOpen] = useState(false);
  const [confirm, setConfirm] = useState(false);
  // A sideloaded game's removal check is fetched with the tab, not on click.
  const removeCheck = useRemoveCheck(game, game.store === 'custom' && !running, confirm);
  const [savePaths, setSavePaths] = useState<SavePath[] | null>(null);
  // The auto-backup state: undefined until it lands, null when the call
  // failed, so the confirm line can tell loading apart from an answer.
  const [autoState, setAutoState] = useState<AutoBackupState | null | undefined>(undefined);
  const [allowNoBackup, setAllowNoBackup] = useState(false);
  // The dialog starts the Maintenance calls when it opens (issue #56): the
  // confirm below awaits the same promises instead of refetching.
  const pre = useMaintenancePrefetch();
  const shared = pre != null && pre.gameId === game.id ? pre : null;
  useEffect(() => { setConfirm(false); setVerifyOpen(false); setMoveOpen(false); }, [game.id]);
  useEffect(() => { setSavePaths(null); setAutoState(undefined); setAllowNoBackup(false); }, [game.id, confirm]);
  // The uninstall confirm shows the backup line: what the gate will back up,
  // the cloud line when the store syncs saves, or the "delete without a save
  // backup" checkbox when no paths are known without cloud. The dialog-open
  // fetch (shared) usually answered already; the section's own reloads write
  // their fresh answers back there too, so this never fires a second fetch
  // for the same dialog open.
  useEffect(() => {
    if (!confirm || game.store === 'custom') return;
    let live = true;
    if (shared) {
      if (shared.latest.paths !== undefined) setSavePaths(shared.latest.paths);
      else shared.paths.then((v) => { if (live) setSavePaths(v); }, () => { if (live) setSavePaths([]); });
      if (shared.latest.auto !== undefined) setAutoState(shared.latest.auto);
      else void shared.auto.then((v) => { if (live) setAutoState(v); });
      return () => { live = false; };
    }
    Effect.runPromise(LauncherApi.getSavePaths(game)).then(setSavePaths).catch(() => setSavePaths([]));
    Effect.runPromise(LauncherApi.getAutoBackupState(game)).then(setAutoState).catch(() => setAutoState(null));
  }, [confirm, game, shared]); // eslint-disable-line react-hooks/exhaustive-deps
  // A game that starts while the confirm row is open closes it: a running
  // game can't be removed, so there is nothing left to answer.
  useEffect(() => { if (running) setConfirm(false); }, [running]);
  // While the uninstall waits for an answer, Escape only backs out of it. The window's capture
  // phase runs before the dialog's own Escape handler, so the settings stay open.
  useEffect(() => {
    if (!confirm) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      e.stopPropagation();
      setConfirm(false);
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [confirm]);
  const active = job && (job.state === 'queued' || job.state === 'running');

  const uninstallWithoutBackup = () => {
    setConfirm(false);
    Effect.runPromise(LauncherApi.uninstallGameWithoutBackup(game))
      .then(() => onNotify(t('downloads:queued', { action: t('downloads:action.uninstall'), title: game.title }), 'busy'))
      .catch((err) => onNotify(coreErrMsg(err), 'err'));
    onClose();
  };

  if (active) {
    return (
      <div className="space-y-3">
        <p className="text-[12.5px] text-white/70">{t('gameSettings:maintenance.busy', { title: game.title })}</p>
        <JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} />
      </div>
    );
  }
  // Sideloaded games have no plugin actions: only forgetting them in Platypus.
  // Their files stay where they are unless the confirm panel's checkboxes say
  // otherwise. A running game can't leave the library until it stops, so its
  // Remove button waits disabled with no confirm step.
  if (game.store === 'custom') {
    if (running) {
      return (
        <div className="space-y-5">
          <Row title={t('gameSettings:maintenance.removeTitle')} text={t('gameSettings:maintenance.removeRunning', { title: game.title })}>
            <Button disabled><Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.removeTitle')}</Button>
          </Row>
        </div>
      );
    }
    return (
      <div className="space-y-5">
        <SaveBackupsSection game={game} onNotify={onNotify} />
        {confirm ? (
          <RemoveConfirmPanel game={game} check={removeCheck.check} error={removeCheck.error} onCancel={() => setConfirm(false)} onRemove={(opts) => { setConfirm(false); onRemove?.(opts); onClose(); }} />
        ) : (
          <RemoveFirstRow game={game} onConfirm={() => setConfirm(true)} />
        )}
        {job?.state === 'failed' && (
          <ErrorNotice message={job.error ?? ''} fallback={t('gameSettings:maintenance.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
        )}
      </div>
    );
  }
  if (!plugin) {
    // Built-in Steam games have no plugin: Maintenance is just Save backups.
    return (
      <div className="space-y-5">
        <SaveBackupsSection game={game} onNotify={onNotify} />
        {job?.state === 'failed' && (
          <ErrorNotice message={job.error ?? ''} fallback={t('gameSettings:maintenance.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
        )}
      </div>
    );
  }
  const installed = game.is_installed;
  const size = formatBytes(game.size_bytes);
  const uninstallText = size && game.install_dir
    ? t('gameSettings:maintenance.uninstallConfirmFull', { title: game.title, size, dir: game.install_dir })
    : size
      ? t('gameSettings:maintenance.uninstallConfirmSize', { title: game.title, size })
      : game.install_dir
        ? t('gameSettings:maintenance.uninstallConfirmDir', { title: game.title, dir: game.install_dir })
        : t('gameSettings:maintenance.uninstallConfirmBare', { title: game.title });
  const noSavePaths = savePaths != null && savePaths.length === 0;
  const cloudNoSaves = noSavePaths && autoState?.reason === 'cloud';
  const needsNoSavesCheck = noSavePaths && !cloudNoSaves;
  return (
    <div className="space-y-5">
      <SaveBackupsSection game={game} onNotify={onNotify} />
      {installed && (
        <CapabilitySurface plugin={plugin} capability="verify" game={game}>
          <Row title={t('gameSettings:maintenance.verifyTitle')} text={t('gameSettings:maintenance.verifyText', { title: game.title, plugin: plugin.name })}>
            <Button onClick={() => setVerifyOpen(true)}><ShieldCheck className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.verifyTitle')}</Button>
          </Row>
        </CapabilitySurface>
      )}

      {installed && (
        <Row title={t('gameSettings:maintenance.moveTitle')} text={t('gameSettings:maintenance.moveText')}>
          <Button onClick={() => setMoveOpen(true)}><FolderInput className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.moveAction')}</Button>
        </Row>
      )}

      {installed && offers(plugin, 'install', { game }) && (confirm ? (
        <Row title={t('gameSettings:maintenance.uninstallTitle')} text={uninstallText}>
          <div className="space-y-2">
            {/* Loading text in place until the save paths and the auto-backup
                state land: never an empty line, never the wrong line. */}
            {savePaths == null || autoState === undefined ? (
              <p role="status" className="text-[12px] text-white/60">{t('gameSettings:maintenance.saveBackups.loading')}</p>
            ) : (
              <p className="text-[12px] text-white/60">
                {cloudNoSaves
                  ? t('gameSettings:maintenance.saveBackups.uninstallCloudNoSaves')
                  : noSavePaths
                    ? t('gameSettings:maintenance.saveBackups.uninstallNoSaves')
                    : t('gameSettings:maintenance.saveBackups.uninstallBackedUp')}
              </p>
            )}
            {needsNoSavesCheck && (
              <label className="flex items-center gap-2 text-[12.5px] text-white/75">
                <input
                  type="checkbox"
                  checked={allowNoBackup}
                  onChange={(e) => setAllowNoBackup(e.target.checked)}
                  aria-label={t('gameSettings:maintenance.saveBackups.deleteWithoutBackup')}
                />
                {t('gameSettings:maintenance.saveBackups.deleteWithoutBackup')}
              </label>
            )}
            <div className="flex gap-2">
              <Button variant="ghost" autoFocus onClick={() => setConfirm(false)}>{t('common:cancel')}</Button>
              {needsNoSavesCheck ? (
                <Button variant="danger" disabled={!allowNoBackup} title={allowNoBackup ? undefined : t('gameSettings:maintenance.saveBackups.needsCheck')} onClick={uninstallWithoutBackup}>
                  <Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.deleteFiles')}
                </Button>
              ) : (
                <Button variant="danger" onClick={() => { setConfirm(false); onJob('uninstall'); onClose(); }}>
                  <Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.deleteFiles')}
                </Button>
              )}
            </div>
          </div>
        </Row>
      ) : (
        <Row title={t('gameSettings:maintenance.uninstallTitle')} text={t('gameSettings:maintenance.uninstallText')}>
          <Button onClick={() => setConfirm(true)}><Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.uninstallTitle')}</Button>
        </Row>
      ))}

      {job?.state === 'failed' && (
        <ErrorNotice message={job.error ?? ''} fallback={t('gameSettings:maintenance.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
      )}

      {verifyOpen && <VerifyDialog game={game} jobs={jobs} onClose={() => setVerifyOpen(false)} />}
      {moveOpen && (
        <MoveDialog
          game={game}
          startFolder={parentFolder(game.install_dir)}
          onClose={() => setMoveOpen(false)}
          onQueued={() => {
            setMoveOpen(false);
            onNotify(t('gameSettings:maintenance.moving', { title: game.title }), 'busy');
            onClose();
          }}
        />
      )}
    </div>
  );
}

/** The first removal row: what leaving the library means on its own. */
/** Fetches what removing a sideloaded game would delete as soon as the
 *  Maintenance tab opens, so the confirm panel shows the paths and sizes
 *  from its first frame instead of swapping them in. */
export function useRemoveCheck(game: Game, enabled: boolean, refresh: unknown) {
  const [state, setState] = useState<{ id: string; check: RemoveCheck | null; error: string | null }>({ id: game.id, check: null, error: null });
  useEffect(() => {
    if (!enabled) return;
    let live = true;
    // Opening the panel asks again (save folders may have changed) but keeps
    // what is shown until the answer lands; only another game starts empty.
    setState((s) => (s.id === game.id ? s : { id: game.id, check: null, error: null }));
    Effect.runPromise(LauncherApi.checkRemove(game.id, { delete_prefix: !game.native, delete_files: true, allow_no_saves: false })).then(
      (c) => { if (live) setState({ id: game.id, check: c, error: null }); },
      (e) => { if (live) setState({ id: game.id, check: null, error: coreErrMsg(e) }); },
    );
    return () => { live = false; };
  }, [game.id, enabled, refresh]);
  return state.id === game.id ? state : { check: null, error: null };
}

export function RemoveFirstRow({ game, onConfirm }: { game: Game; onConfirm: () => void }) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const stay = game.install_dir
    ? t('gameSettings:maintenance.stayWithDir', { dir: game.install_dir })
    : t('gameSettings:maintenance.stayBare');
  return (
    <Row title={t('gameSettings:maintenance.removeTitle')} text={t('gameSettings:maintenance.removeText', { title: game.title, stay })}>
      <Button onClick={onConfirm}><Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.removeTitle')}</Button>
    </Row>
  );
}

/** A refusal reason as display text: the catalog entry for a coded refusal,
 *  the English text for anything else. */
function reasonText(target: RemoveTarget): string {
  if (target.reason_code && i18n.exists(`errors:${target.reason_code}`)) {
    return i18n.t(`errors:${target.reason_code}`, target.reason_params ?? {});
  }
  return target.reason ?? '';
}

/** The other games sharing a prefix whose deletion is refused (remove.shared,
 *  issue #52), named from PrefixUsage. Null until it lands — and for anything
 *  but a shared-prefix refusal, which fetches nothing. */
function useSharers(path: string | null, gameId: string) {
  const [sharers, setSharers] = useState<string[] | null>(null);
  useEffect(() => {
    if (!path) {
      setSharers(null);
      return;
    }
    let live = true;
    Effect.runPromise(LauncherApi.prefixUsage()).then(
      (usage) => {
        if (live) {
          setSharers((usage ?? []).find((u) => u.path === path)?.games.filter((g) => g.id !== gameId).map((g) => g.title) ?? []);
        }
      },
      () => {
        if (live) setSharers(null);
      },
    );
    return () => {
      live = false;
    };
  }, [path, gameId]);
  return sharers;
}

/** The confirm panel for removing a sideloaded game: library only by
 *  default, with unchecked boxes for deleting the Wine prefix and the game
 *  files (native games have no prefix, so only the files box shows).
 *  Each box shows its path and size once the check answers
 *  ("measuring…" until then), or is disabled with its reason.
 *
 *  Stable from the first frame: the title and text sit on top full width
 *  (the same block as the first row, so nothing jumps sideways), then the
 *  checkboxes, the backup line and the buttons stack below full width. The
 *  detail line reserves one line while measuring, and long paths truncate
 *  with a tooltip instead of pushing the layout. */
export function RemoveConfirmPanel({ game, check, error, onCancel, onRemove }: {
  game: Game;
  /** The removal check, fetched when the tab opened (useRemoveCheck), so
   *  paths and sizes are there before the panel shows. */
  check: RemoveCheck | null;
  error: string | null;
  onCancel: () => void;
  onRemove: (opts: RemoveOptions) => void;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [deletePrefix, setDeletePrefix] = useState(false);
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [allowNoSaves, setAllowNoSaves] = useState(false);
  const [removing, setRemoving] = useState(false);
  const targets = check?.targets ?? [];
  const prefix = targets.find((x) => x.kind === 'prefix');
  const files = targets.find((x) => x.kind === 'files');
  // A prefix another game still uses may not go: name the sharers (issue #52).
  const sharedPath = prefix && !prefix.allowed && prefix.reason_code === 'remove.shared' ? prefix.path : null;
  const sharers = useSharers(sharedPath, game.id);
  const deleting = deletePrefix || deleteFiles;
  const noPaths = check?.saves?.no_paths ?? false;
  const cloudNoSaves = check?.saves?.cloud ?? false;
  // Deleting without known saves needs the explicit confirmation, unless the
  // cloud holds the saves.
  const blocked = deleting && noPaths && !allowNoSaves;
  const stay = game.install_dir
    ? t('gameSettings:maintenance.stayWithDir', { dir: game.install_dir })
    : t('gameSettings:maintenance.stayBare');
  const nouns = [
    deletePrefix ? t('gameSettings:maintenance.removePrefixNoun') : null,
    deleteFiles ? t('gameSettings:maintenance.removeFilesNoun') : null,
  ].filter((x): x is string => x != null);
  const summary = !deleting
    ? t('gameSettings:maintenance.removeConfirm', { title: game.title, stay })
    : t('gameSettings:maintenance.removeSummaryDelete', {
      title: game.title,
      what: nouns.length > 1
        ? t('gameSettings:maintenance.removeTwo', { a: nouns[0], b: nouns[1] })
        : (nouns[0] ?? ''),
    });
  return (
    <section data-testid="remove-confirm-panel" className="border-b border-white/[0.06] pb-5 last:border-b-0">
      <div className="min-w-0">
        <MonoLabel className="mb-1">{t('gameSettings:maintenance.removeTitle')}</MonoLabel>
        <p className="text-[12.5px] leading-relaxed text-white/60">{summary}</p>
      </div>
      <div className="mt-3 flex min-w-0 flex-col gap-2.5">
        {/* Native games have no Wine prefix (issue #47): no box to check. */}
        {!game.native && (
          <>
            <TargetBox
              label={t('gameSettings:maintenance.removePrefix')}
              target={prefix}
              measuring={t('gameSettings:maintenance.removeMeasuring')}
              checked={deletePrefix}
              onChange={setDeletePrefix}
            />
            {sharedPath && sharers && sharers.length > 0 && (
              <p role="status" className="text-[12px] leading-snug text-amber-200/85">
                {t('gameSettings:prefix.sharedWith', { games: sharers.join(', ') })}
              </p>
            )}
          </>
        )}
        <TargetBox
          label={t('gameSettings:maintenance.removeFiles')}
          target={files}
          measuring={t('gameSettings:maintenance.removeMeasuring')}
          checked={deleteFiles}
          onChange={setDeleteFiles}
        />
        {check ? (
          noPaths ? (
            <label className="flex items-start gap-2 text-[12.5px] leading-snug text-white/70">
              <input
                type="checkbox"
                checked={allowNoSaves}
                onChange={(e) => setAllowNoSaves(e.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 accent-amber-300"
              />
              <span>
                {t('gameSettings:maintenance.removeNoSaves')}
                <span className="block text-[12px] text-white/55">{t('gameSettings:maintenance.removeAllowNoSaves')}</span>
              </span>
            </label>
          ) : cloudNoSaves ? (
            <p className="min-h-[18px] text-[12px] text-white/55">{t('gameSettings:maintenance.removeCloudNoSaves')}</p>
          ) : check.saves?.off ? (
            <p className="min-h-[18px] text-[12px] text-white/55">{t('gameSettings:maintenance.removeBackupOff')}</p>
          ) : (
            <p className="min-h-[18px] text-[12px] text-white/55">{t('gameSettings:maintenance.removeBackupFirst')}</p>
          )
        ) : (
          <p className="min-h-[18px] text-[12px] text-white/55">{t('gameSettings:maintenance.removeMeasuring')}</p>
        )}
        {error && <ErrorNotice message={error} fallback={t('gameSettings:maintenance.removeError', { title: game.title })} />}
        <div className="flex gap-2">
          <Button variant="ghost" autoFocus onClick={onCancel}>{t('common:cancel')}</Button>
          <Button
            variant="danger"
            disabled={removing || !check || error != null || blocked}
            onClick={() => {
              setRemoving(true);
              onRemove({ delete_prefix: deletePrefix, delete_files: deleteFiles, allow_no_saves: allowNoSaves });
            }}
          >
            <Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.removeTitle')}
          </Button>
        </div>
      </div>
    </section>
  );
}

/** One deletion checkbox: path and size when allowed, the reason when not.
 *  The detail line keeps one line reserved while measuring so the check
 *  landing doesn't shift the panel; long paths truncate with a tooltip and
 *  wrap inside their own line without pushing the layout. */
function TargetBox({ label, target, measuring, checked, onChange }: {
  label: string;
  target: RemoveTarget | undefined;
  measuring: string;
  checked: boolean;
  onChange: (on: boolean) => void;
}) {
  const detail = !target
    ? measuring
    : target.allowed
      ? `${target.path} · ${formatBytes(target.bytes)}`
      : reasonText(target);
  return (
    <label className="flex min-w-0 items-start gap-2 text-[12.5px] leading-snug text-white/70">
      <input
        type="checkbox"
        checked={checked}
        disabled={!target?.allowed}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 h-3.5 w-3.5 flex-none accent-amber-300"
      />
      <span className="min-w-0 flex-1">
        {label}
        <span className="block min-h-[18px] truncate font-mono text-[12px] text-white/55" title={target?.path ?? detail}>{detail}</span>
      </span>
    </label>
  );
}

function Row({ title, text, children }: { title: string; text: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b border-white/[0.06] pb-5 last:border-b-0">
      <div className="min-w-0 flex-1 basis-64">
        <MonoLabel className="mb-1">{title}</MonoLabel>
        <p className="text-[12.5px] leading-relaxed text-white/60">{text}</p>
      </div>
      <div className="flex-none">{children}</div>
    </section>
  );
}
