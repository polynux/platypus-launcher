// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { FolderInput, ShieldCheck, Trash2 } from 'lucide-react';
import type { Game, GameJobKind, Job, RemoveCheck, RemoveOptions, RemoveTarget, SavePath, StorePlugin } from '../types';
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
  const [savePaths, setSavePaths] = useState<SavePath[] | null>(null);
  const [allowNoBackup, setAllowNoBackup] = useState(false);
  useEffect(() => { setConfirm(false); setVerifyOpen(false); setMoveOpen(false); }, [game.id]);
  useEffect(() => { setSavePaths(null); setAllowNoBackup(false); }, [game.id, confirm]);
  // The uninstall confirm shows the backup line: what the gate will back up,
  // or the "delete without a save backup" checkbox when no paths are known.
  useEffect(() => {
    if (!confirm || game.store === 'custom') return;
    Effect.runPromise(LauncherApi.getSavePaths(game)).then(setSavePaths).catch(() => setSavePaths([]));
  }, [confirm, game]);
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
          <RemoveConfirmPanel game={game} onCancel={() => setConfirm(false)} onRemove={(opts) => { setConfirm(false); onRemove?.(opts); onClose(); }} />
        ) : (
          <RemoveFirstRow game={game} onConfirm={() => setConfirm(true)} />
        )}
        {job?.state === 'failed' && (
          <ErrorNotice message={job.error ?? ''} fallback={t('gameSettings:maintenance.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
        )}
      </div>
    );
  }
  if (!plugin) return null;
  const size = formatBytes(game.size_bytes);
  const uninstallText = size && game.install_dir
    ? t('gameSettings:maintenance.uninstallConfirmFull', { title: game.title, size, dir: game.install_dir })
    : size
      ? t('gameSettings:maintenance.uninstallConfirmSize', { title: game.title, size })
      : game.install_dir
        ? t('gameSettings:maintenance.uninstallConfirmDir', { title: game.title, dir: game.install_dir })
        : t('gameSettings:maintenance.uninstallConfirmBare', { title: game.title });
  const noSavePaths = savePaths != null && savePaths.length === 0;
  return (
    <div className="space-y-5">
      <SaveBackupsSection game={game} onNotify={onNotify} />
      <CapabilitySurface plugin={plugin} capability="verify" game={game}>
        <Row title={t('gameSettings:maintenance.verifyTitle')} text={t('gameSettings:maintenance.verifyText', { title: game.title, plugin: plugin.name })}>
          <Button onClick={() => setVerifyOpen(true)}><ShieldCheck className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.verifyTitle')}</Button>
        </Row>
      </CapabilitySurface>

      <Row title={t('gameSettings:maintenance.moveTitle')} text={t('gameSettings:maintenance.moveText')}>
        <Button onClick={() => setMoveOpen(true)}><FolderInput className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.moveAction')}</Button>
      </Row>

      {offers(plugin, 'install', { game }) && (confirm ? (
        <Row title={t('gameSettings:maintenance.uninstallTitle')} text={uninstallText}>
          <div className="space-y-2">
            <p className="text-[12px] text-white/60">
              {noSavePaths
                ? t('gameSettings:maintenance.saveBackups.uninstallNoSaves')
                : t('gameSettings:maintenance.saveBackups.uninstallBackedUp')}
            </p>
            {noSavePaths && (
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
              {noSavePaths ? (
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
function RemoveFirstRow({ game, onConfirm }: { game: Game; onConfirm: () => void }) {
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

/** The confirm panel for removing a sideloaded game: library only by
 *  default, with unchecked boxes for deleting the Wine prefix and the game
 *  files. Each box shows its path and size once the check answers
 *  ("measuring…" until then), or is disabled with its reason. */
function RemoveConfirmPanel({ game, onCancel, onRemove }: {
  game: Game;
  onCancel: () => void;
  onRemove: (opts: RemoveOptions) => void;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [deletePrefix, setDeletePrefix] = useState(false);
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [allowNoSaves, setAllowNoSaves] = useState(false);
  const [check, setCheck] = useState<RemoveCheck | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [removing, setRemoving] = useState(false);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.checkRemove(game.id, { delete_prefix: true, delete_files: true, allow_no_saves: false })).then(
      (c) => { if (live) { setCheck(c); setError(null); } },
      (e) => { if (live) { setCheck(null); setError(coreErrMsg(e)); } },
    );
    return () => { live = false; };
  }, [game.id]);
  const targets = check?.targets ?? [];
  const prefix = targets.find((x) => x.kind === 'prefix');
  const files = targets.find((x) => x.kind === 'files');
  const deleting = deletePrefix || deleteFiles;
  const noPaths = check?.saves?.no_paths ?? false;
  // Deleting without known saves needs the explicit confirmation.
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
    <Row title={t('gameSettings:maintenance.removeTitle')} text={summary}>
      <div className="flex flex-col gap-2.5">
        <TargetBox
          label={t('gameSettings:maintenance.removePrefix')}
          target={prefix}
          measuring={t('gameSettings:maintenance.removeMeasuring')}
          checked={deletePrefix}
          onChange={setDeletePrefix}
        />
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
          ) : check.saves?.off ? (
            <p className="text-[12px] text-white/55">{t('gameSettings:maintenance.removeBackupOff')}</p>
          ) : (
            <p className="text-[12px] text-white/55">{t('gameSettings:maintenance.removeBackupFirst')}</p>
          )
        ) : (
          <p className="text-[12px] text-white/55">{t('gameSettings:maintenance.removeMeasuring')}</p>
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
    </Row>
  );
}

/** One deletion checkbox: path and size when allowed, the reason when not. */
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
    <label className="flex items-start gap-2 text-[12.5px] leading-snug text-white/70">
      <input
        type="checkbox"
        checked={checked}
        disabled={!target?.allowed}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 h-3.5 w-3.5 accent-amber-300"
      />
      <span>
        {label}
        <span className="block font-mono text-[12px] text-white/55" title={target?.path}>{detail}</span>
      </span>
    </label>
  );
}

function Row({ title, text, children }: { title: string; text: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b border-white/[0.06] pb-5 last:border-b-0">
      <div className="min-w-0 flex-1 basis-64">
        <MonoLabel className="mb-1">{title.toLowerCase()}</MonoLabel>
        <p className="text-[12.5px] leading-relaxed text-white/60">{text}</p>
      </div>
      <div className="flex-none">{children}</div>
    </section>
  );
}
