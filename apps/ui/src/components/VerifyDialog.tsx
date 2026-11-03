// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { Game, Job } from '../types';
import { Modal, ModalFoot, ModalHead } from './Modal';
import { Button } from '@/components/ds/button';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';

export interface VerifyDialogProps {
  game: Game;
  /** The queue's jobs; the dialog follows the one it started (check or repair). */
  jobs: Job[];
  onClose: () => void;
}

/** Checks a game's files with its plugin. The check only reports; when it
 *  finds damage the dialog asks before a repair, which is a second run. */
export function VerifyDialog({ game, jobs, onClose }: VerifyDialogProps) {
  const { t } = useTranslation(['gameOps', 'common']);
  const [jobId, setJobId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const start = (repair: boolean) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(repair ? LauncherApi.repairGame(game) : LauncherApi.verifyGame(game)).then(
      (job) => setJobId(job.id),
      (e) => setError(coreErrMsg(e)),
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
  const title = t('gameOps:verify.title', { title: game.title });

  let body: React.ReactNode;
  let actions: React.ReactNode;
  if (error) {
    body = <ErrorNotice message={error} fallback={t('gameOps:verify.startError')} />;
  } else if (!job) {
    body = <p className="text-text-muted">{t('gameOps:verify.starting')}</p>;
  } else if (running) {
    const progress = job.percent >= 0 ? t('gameOps:verify.progress', { percent: Math.round(job.percent) }) : '';
    body = (
      <p className="text-text-body">
        {job.repair ? t('gameOps:verify.repairing', { progress }) : t('gameOps:verify.checking', { progress })}
      </p>
    );
  } else if (job.state === 'failed') {
    body = <ErrorNotice message={job.error ?? ''} fallback={t('gameOps:verify.failed')} onRetry={() => start(job.repair)} />;
  } else if (job.state === 'canceled') {
    body = <p className="text-text-muted">{t('gameOps:verify.canceled')}</p>;
  } else if (job.state === 'done' && report) {
    if (job.repair) {
      body = (
        <p className="text-text-body">
          {t('gameOps:verify.repaired', { count: report.repaired })}
          {wrong > 0 ? ` ${t('gameOps:verify.stillWrong', { count: wrong })}` : ''}
        </p>
      );
    } else if (wrong > 0) {
      body = (
        <>
          <p className="text-[15px] font-bold text-attention">
            {t('gameOps:verify.damagedTitle', { count: wrong })}
          </p>
          <p className="mt-1 font-mono text-hint text-text-muted">
            {t('gameOps:verify.damageDetail', { damaged: report.damaged, missing: report.missing, checked: report.checked })}
          </p>
        </>
      );
      actions = (
        <Button variant="primary" onClick={() => start(true)} disabled={busy}>{t('gameOps:verify.repair')}</Button>
      );
    } else {
      body = <p className="text-text-body">{t('gameOps:verify.allClean', { checked: report.checked })}</p>;
    }
  }

  return (
    <Modal label={title} onClose={onClose} size="md">
      <div className="-m-7">
        <ModalHead title={title} onClose={onClose} />
        <div className="space-y-2 p-5 text-[14px]">
          {body}
        </div>
        <ModalFoot>
          <Button variant="ghost" onClick={onClose}>{job?.state === 'done' && wrong > 0 && !job.repair ? t('gameOps:verify.notNow') : t('common:close')}</Button>
          {actions}
        </ModalFoot>
      </div>
    </Modal>
  );
}
