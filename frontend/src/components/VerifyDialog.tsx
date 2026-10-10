import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { Game, Job } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { MonoLabel } from './ui';
import { LauncherApi } from '../effect/runtime';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface VerifyDialogProps {
  game: Game;
  /** The queue's jobs; the dialog follows the one it started (check or repair). */
  jobs: Job[];
  onClose: () => void;
}

/** Checks a game's files with its plugin. The check only reports; when it
 *  finds damage the dialog asks before a repair, which is a second run. */
export function VerifyDialog({ game, jobs, onClose }: VerifyDialogProps) {
  const [jobId, setJobId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const start = (repair: boolean) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(repair ? LauncherApi.repairGame(game) : LauncherApi.verifyGame(game)).then(
      (job) => setJobId(job.id),
      (e) => setError(errMsg(e)),
    ).finally(() => setBusy(false));
  };

  // Start the check when the dialog opens; the user asked for it by clicking.
  useEffect(() => {
    start(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  const job = jobId ? jobs.find((j) => j.id === jobId) : undefined;
  const report = job?.verify;
  const wrong = report ? report.damaged + report.missing : 0;
  const running = job?.state === 'queued' || job?.state === 'running';
  const title = `Verify ${game.title}`;

  let body: React.ReactNode;
  let actions: React.ReactNode;
  if (error) {
    body = <p className="text-red-300">{error}</p>;
  } else if (!job) {
    body = <p className="text-white/60">Starting the check…</p>;
  } else if (running) {
    body = (
      <p className="text-white/70">
        {job.repair ? 'Repairing' : 'Checking'} the files{job.percent >= 0 ? ` · ${Math.round(job.percent)}%` : ''}
      </p>
    );
  } else if (job.state === 'failed') {
    body = <p className="text-red-300">The check failed: {job.error}</p>;
  } else if (job.state === 'canceled') {
    body = <p className="text-white/60">The check was canceled. Nothing was changed.</p>;
  } else if (job.state === 'done' && report) {
    if (job.repair) {
      body = (
        <p className="text-white/80">
          Repaired {report.repaired} {report.repaired === 1 ? 'file' : 'files'}.
          {wrong > 0 ? ` ${wrong} still ${wrong === 1 ? 'is' : 'are'} wrong; the store may need the game reinstalled.` : ''}
        </p>
      );
    } else if (wrong > 0) {
      body = (
        <>
          <p className="text-[15px] font-bold text-amber-200">
            {wrong} {wrong === 1 ? 'file' : 'files'} damaged — Repair?
          </p>
          <p className="mt-1 font-mono text-[11.5px] text-white/55">
            {report.damaged} damaged, {report.missing} missing, of {report.checked} files checked. A repair restores them from the store.
          </p>
        </>
      );
      actions = (
        <Button onClick={() => start(true)} disabled={busy} className="flex-1 text-sky-100">Repair</Button>
      );
    } else {
      body = <p className="text-white/80">All {report.checked} files check out.</p>;
    }
  }

  return (
    <Modal label={title} onClose={onClose} size="md">
      <div className="p-6">
        <MonoLabel className="mb-2">verify files</MonoLabel>
        <h2 className="mb-4 text-[18px] font-bold">{title}</h2>
        <div className="space-y-2 text-[14px]">{body}</div>
        <div className="mt-6 flex gap-2">
          {actions}
          <Button onClick={onClose} className="flex-1 text-white/70">{job?.state === 'done' && wrong > 0 && !job.repair ? 'Not now' : 'Close'}</Button>
        </div>
      </div>
    </Modal>
  );
}
