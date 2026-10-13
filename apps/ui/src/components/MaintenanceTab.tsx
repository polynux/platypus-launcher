// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FolderInput, ShieldCheck, Trash2 } from 'lucide-react';
import type { Game, GameJobKind, Job, StorePlugin } from '../types';
import { Button } from '@/components/ds/button';
import { CapabilitySurface, offers } from '../lib/capabilities';
import { JobProgress } from './JobProgress';
import { MoveDialog } from './MoveDialog';
import { VerifyDialog } from './VerifyDialog';
import { MonoLabel } from './ui';
import { formatBytes } from '../lib/games';
import { ErrorNotice } from './ErrorNotice';

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
  /** Removes a sideloaded game from the library (custom games only). */
  onRemove?: () => void;
  /** The game is starting or running: it can't leave the library until it stops. */
  running?: boolean;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [verifyOpen, setVerifyOpen] = useState(false);
  const [moveOpen, setMoveOpen] = useState(false);
  const [confirm, setConfirm] = useState(false);
  useEffect(() => { setConfirm(false); setVerifyOpen(false); setMoveOpen(false); }, [game.id]);
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

  if (active) {
    return (
      <div className="space-y-3">
        <p className="text-[12.5px] text-white/70">{t('gameSettings:maintenance.busy', { title: game.title })}</p>
        <JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} />
      </div>
    );
  }
  // Sideloaded games have no plugin actions: only forgetting them in Platypus.
  // Their files stay where they are. A running game can't leave the library
  // until it stops, so its Remove button waits disabled with no confirm step.
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
    const stay = game.install_dir
      ? t('gameSettings:maintenance.stayWithDir', { dir: game.install_dir })
      : t('gameSettings:maintenance.stayBare');
    const removeText = t('gameSettings:maintenance.removeText', { title: game.title, stay });
    const removeConfirmText = t('gameSettings:maintenance.removeConfirm', { title: game.title, stay });
    return (
      <div className="space-y-5">
        {confirm ? (
          <Row title={t('gameSettings:maintenance.removeTitle')} text={removeConfirmText}>
            <div className="flex gap-2">
              <Button variant="ghost" autoFocus onClick={() => setConfirm(false)}>{t('common:cancel')}</Button>
              <Button variant="danger" onClick={() => { setConfirm(false); onRemove?.(); onClose(); }}>
                <Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.removeTitle')}
              </Button>
            </div>
          </Row>
        ) : (
          <Row title={t('gameSettings:maintenance.removeTitle')} text={removeText}>
            <Button onClick={() => setConfirm(true)}><Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.removeTitle')}</Button>
          </Row>
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
  return (
    <div className="space-y-5">
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
          <div className="flex gap-2">
            <Button variant="ghost" autoFocus onClick={() => setConfirm(false)}>{t('common:cancel')}</Button>
            <Button variant="danger" onClick={() => { setConfirm(false); onJob('uninstall'); onClose(); }}>
              <Trash2 className="h-3.5 w-3.5" /> {t('gameSettings:maintenance.deleteFiles')}
            </Button>
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
