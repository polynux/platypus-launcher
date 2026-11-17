// i18n: converted
// The store-client setup action (protocol 1.20): after a launch fails with
// client.notSetUp, the game page shows the error with a "Set up {client}"
// button; while the setup job runs it shows "Setting up {client}…" instead
// of Play. Shared by the tabs strip and the hero actions so both game page
// arrangements behave the same.
import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { Download, Loader2, X } from 'lucide-react';
import type { Job } from '../types';
import { LauncherApi } from '../effect/runtime';
import { coreErrorDataOf, coreErrMsg } from '../lib/coreError';
import { Button } from '@/components/ds/button';
import { cx } from './ui';

export function ClientSetupAction({
  client,
  source,
  message,
  job,
  onNotify,
  buttonClassName,
  showButton,
}: {
  /** The client to name ("Ubisoft Connect"): the launch error's client. */
  client: string;
  /** The plugin id whose prefix to set up. */
  source: string;
  /** The launch failure to show next to the button; null means no setup needed. */
  message: string | null;
  /** The client's setup job while one holds it (queued, running or paused). */
  job?: Job;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Button sizing for the surrounding surface (strip vs hero). */
  buttonClassName?: string;
  /** Show the Set up button even without a failure message (the plugin card). */
  showButton?: boolean;
}) {
  const { t } = useTranslation('game');
  const [confirm, setConfirm] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const running = job != null && (job.state === 'queued' || job.state === 'running' || job.state === 'paused');

  const runSetup = (confirmed: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setupClient(source, confirmed))
      .then(() => setConfirm(null))
      .catch((err) => {
        const data = coreErrorDataOf(err);
        const prompt = !confirmed && data?.code === 'client.confirmDownload' && typeof data.params?.message === 'string'
          ? data.params.message
          : null;
        if (prompt) {
          setConfirm(prompt);
        } else {
          setConfirm(null);
          onNotify?.(coreErrMsg(err), 'err');
        }
      })
      .finally(() => setBusy(false));
  };

  if (running) {
    return (
      <Button variant="primary" size="none" disabled className={cx('inline-flex items-center gap-2', buttonClassName)}>
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
        <span>{t('game:play.clientSetup.settingUp', { client })}</span>
      </Button>
    );
  }
  if (message == null && !showButton) return null;
  return (
    <div className="flex flex-col items-start gap-2">
      {message != null && <p role="status" className="max-w-md text-sm text-text-muted">{message}</p>}
      <Button
        variant="primary"
        size="none"
        onClick={() => runSetup(false)}
        disabled={busy}
        className={cx('inline-flex items-center gap-2', buttonClassName)}
      >
        {busy ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <Download className="h-4 w-4" aria-hidden />}
        <span>{t('game:play.clientSetup.setUp', { client })}</span>
      </Button>
      {confirm != null && (
        <div role="alertdialog" aria-label={t('game:play.clientSetup.setUp', { client })} className="max-w-md rounded-xl bg-surface-strong p-4 ring-1 ring-hairline">
          <p className="text-sm text-text">{confirm}</p>
          <div className="mt-3 flex gap-2">
            <Button variant="primary" size="sm" onClick={() => runSetup(true)} disabled={busy}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden /> : <Download className="h-3.5 w-3.5" aria-hidden />}
              {t('game:play.clientSetup.download')}
            </Button>
            <Button variant="neutral" size="sm" onClick={() => setConfirm(null)}>
              <X className="h-3.5 w-3.5" aria-hidden />
              {t('game:play.clientSetup.cancel')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
