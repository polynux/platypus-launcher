import React, { useEffect, useState } from 'react';
import { FolderInput, ShieldCheck, Trash2 } from 'lucide-react';
import type { Game, GameJobKind, Job, StorePlugin } from '../types';
import { Button } from '@/components/ds/button';
import { CapabilitySurface, offers } from '../lib/capabilities';
import { JobProgress } from './JobProgress';
import { MoveDialog } from './MoveDialog';
import { VerifyDialog } from './VerifyDialog';
import { MonoLabel, cx } from './ui';
import { ErrorNotice } from './ErrorNotice';

/** The folder a game folder sits in ("" when there is none). */
export function parentFolder(dir?: string): string {
  if (!dir) return '';
  const i = dir.search(/[/\\][^/\\]*$/);
  return i > 0 ? dir.slice(0, i) : dir;
}

/** What is done to an installed game's files: check them, move them, remove them. */
export function MaintenanceTab({ game, plugin, job, jobs, onJob, onJobAction, onNotify, onClose }: {
  game: Game;
  plugin: StorePlugin;
  job?: Job;
  jobs: Job[];
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Closes the settings: a removed game has nothing left to set. */
  onClose: () => void;
}) {
  const [verifyOpen, setVerifyOpen] = useState(false);
  const [moveOpen, setMoveOpen] = useState(false);
  const [confirm, setConfirm] = useState(false);
  useEffect(() => { setConfirm(false); setVerifyOpen(false); setMoveOpen(false); }, [game.id]);
  const active = job && (job.state === 'queued' || job.state === 'running');

  if (active) {
    return (
      <div className="space-y-3">
        <p className="text-[12.5px] text-white/55">{game.title} is busy; its files can't be changed until this finishes.</p>
        <JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} />
      </div>
    );
  }
  return (
    <div className="space-y-5">
      <CapabilitySurface plugin={plugin} capability="verify" game={game}>
        <Row title="Verify files" text={`Checks ${game.title}'s files against ${plugin.name}. A check only reports; repairing asks first.`}>
          <Button onClick={() => setVerifyOpen(true)}><ShieldCheck className="h-3.5 w-3.5" /> Verify files</Button>
        </Row>
      </CapabilitySurface>

      <Row title="Move" text="Moves the game's folder to another folder or disk. Platypus checks the room first and keeps the old copy until the new one is checked.">
        <Button onClick={() => setMoveOpen(true)}><FolderInput className="h-3.5 w-3.5" /> Move…</Button>
      </Row>

      {offers(plugin, 'install', { game }) && (
        <Row title="Uninstall" text="Removes the game's files from this computer. Your saves in the cloud stay where they are.">
          <Button
            variant={confirm ? 'danger' : 'neutral'}
            onClick={() => (confirm ? (setConfirm(false), onJob('uninstall'), onClose()) : setConfirm(true))}
            onBlur={() => setConfirm(false)}
            onKeyDown={(e) => { if (e.key === 'Escape') setConfirm(false); }}
            className={cx(confirm && 'bg-red-500/80 text-white hover:bg-red-500')}
          >
            <Trash2 className="h-3.5 w-3.5" /> {confirm ? 'Delete game files?' : 'Uninstall'}
          </Button>
        </Row>
      )}

      {job?.state === 'failed' && (
        <ErrorNotice message={job.error ?? ''} fallback={`The ${job.kind} didn't finish.`} onRetry={() => onJobAction('retry', job.id)} />
      )}

      {verifyOpen && <VerifyDialog game={game} jobs={jobs} onClose={() => setVerifyOpen(false)} />}
      {moveOpen && (
        <MoveDialog
          game={game}
          startFolder={parentFolder(game.install_dir)}
          onClose={() => setMoveOpen(false)}
          onQueued={() => {
            setMoveOpen(false);
            onNotify(`Moving ${game.title} — see Downloads`, 'busy');
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
