// i18n: converted
// The store-client row of a plugin card (protocol 1.25): for a wine-client
// store whose plugin names its standalone client, Set up while the client
// prefix is missing and Open once it is there, so the user can sign into
// the client with no game to launch from. Both the classic card and the
// Atlas detail render this.
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { Loader2, Play } from 'lucide-react';
import type { ClientState, Job, StorePlugin } from '../types';
import { LauncherApi, onJobs, onSession } from '../effect/runtime';
import { offers } from '../lib/capabilities';
import { coreErrMsg } from '../lib/coreError';
import { Button } from '@/components/ds/button';
import { ClientSetupAction } from './ClientSetupAction';

/** The plugin card's client row: Set up / Open for a wine-client store. */
export function ClientCardActions({ plugin: p, onNotify }: {
  plugin: StorePlugin;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}) {
  const { t } = useTranslation(['plugins', 'game', 'errors']);
  const [state, setState] = useState<ClientState | null>(null);
  const [busy, setBusy] = useState(false);
  // onJobs fires on every progress tick; only a state change of this
  // client's own rows refetches (the setup job appeared, moved or left, or
  // a clientRun hold flipped).
  const keys = useRef<string>('');

  const refresh = useCallback(() => {
    if (p.status !== 'ready' || !offers(p, 'launch')) {
      setState(null);
      return;
    }
    Effect.runPromise(LauncherApi.clientState(p.id))
      .then(setState)
      .catch(() => {
        // Keep the last known state: the card's other rows still work when
        // the client check can't reach the plugin right now.
      });
  }, [p]);

  useEffect(() => {
    refresh();
  }, [refresh]);
  useEffect(() => {
    const clientId = `p.${p.id}.client`;
    const keyOf = (jobs: Job[]) => {
      const setup = jobs.find((j) => j.game_id === clientId);
      const setupKey = setup ? setup.state : 'none';
      const holds = jobs
        .filter((j) => j.source === p.id && (j.state === 'queued' || j.state === 'running' || j.state === 'paused') &&
          (j.kind === 'install' || j.kind === 'update' || j.kind === 'uninstall'))
        .map((j) => j.id)
        .sort()
        .join(',');
      return `${setupKey}|${holds}`;
    };
    const offJobs = onJobs((jobs) => {
      const key = keyOf(jobs);
      if (key !== keys.current) {
        keys.current = key;
        refresh();
      }
    });
    // A game launch or stop through this client's prefix flips the running
    // note; other sources' sessions don't touch the card.
    const offSession = onSession((ev) => {
      if (ev.game_id.startsWith(`p.${p.id}.`)) refresh();
    });
    return () => {
      offJobs();
      offSession();
    };
  }, [p, refresh]);

  const open = () => {
    setBusy(true);
    Effect.runPromise(LauncherApi.openClient(p.id))
      .then((msg) => onNotify?.(msg))
      .catch((err) => onNotify?.(coreErrMsg(err), 'err'))
      .finally(() => {
        setBusy(false);
        refresh();
      });
  };

  if (state == null || !state.supported) return null;
  if (!state.setUp) {
    return (
      <ClientSetupAction
        client={p.name}
        source={p.id}
        message={null}
        job={state.setupJob ?? undefined}
        onNotify={onNotify}
        showButton
      />
    );
  }
  const held = busy || state.running || state.clientRunBusy;
  return (
    <div className="flex flex-col items-start gap-2">
      <Button variant="primary" size="none" onClick={open} disabled={held} className="inline-flex items-center gap-2">
        {busy ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <Play className="h-4 w-4" aria-hidden />}
        <span>{busy ? t('plugins:client.opening', { client: p.name }) : t('plugins:client.open', { client: p.name })}</span>
      </Button>
      <p className="text-[12px] leading-relaxed text-text-muted">{t('plugins:client.hint', { client: p.name })}</p>
      {state.running && (
        <p role="status" className="text-[12px] text-text-muted">{t('errors:client.running', { client: p.name })}</p>
      )}
      {state.clientRunBusy && !state.running && (
        <p role="status" className="text-[12px] text-text-muted">{t('plugins:client.busyNote', { client: p.name })}</p>
      )}
    </div>
  );
}
