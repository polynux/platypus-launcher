// i18n: converted
import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { ActivityEvent, CloudPrompt, CloudStatus, Game } from '../types';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { CloudConflictDialog } from './CloudSavesConflict';
import { LauncherApi, onCloud } from '../effect/runtime';
import { cloudNotice } from '../lib/cloud';
import { coreErrMsg } from '../lib/coreError';

type Notify = (msg: string, kind?: 'ok' | 'err' | 'busy') => void;

interface Pending {
  gameId: string;
  title: string;
  kind: CloudPrompt;
  message: string;
  status?: CloudStatus;
  client?: string;
}

/** Listens for cloud events app-wide: toasts and Activity lines for finished
 *  syncs, and the dialogs for a launch that waits on the user (a conflict, an
 *  unreachable store, the store's client running). It renders nothing else. */
export function CloudSavesHost({
  games, onNotify, onLog,
}: {
  games: Game[];
  onNotify: Notify;
  onLog: (e: Omit<ActivityEvent, 'id' | 'ts'>) => void;
}) {
  const [pending, setPending] = useState<Pending | null>(null);
  const { t } = useTranslation('cloud');
  const gamesRef = useRef(games);
  gamesRef.current = games;
  const notifyRef = useRef(onNotify);
  notifyRef.current = onNotify;
  const logRef = useRef(onLog);
  logRef.current = onLog;

  useEffect(
    () =>
      onCloud((e) => {
        const title = gamesRef.current.find((g) => g.id === e.game_id)?.title ?? e.title ?? e.game_id;
        const n = cloudNotice(e, title);
        if (n?.toast) notifyRef.current(n.toast.msg, n.toast.kind);
        if (n?.activity) logRef.current({ type: 'cloud', game: title, ...n.activity });
        // Keyed by id: a prompt is never dropped because the library lacks its game.
        if (e.kind === 'prompt' && e.prompt) {
          setPending({ gameId: e.game_id, title, kind: e.prompt, message: e.message ?? '', status: e.status, client: e.client });
        }
      }),
    [],
  );

  if (!pending) return null;
  const close = () => setPending(null);
  const game = games.find((g) => g.id === pending.gameId);
  const { title } = pending;  const run = (op: Effect.Effect<string, Error>) => {
    close();
    Effect.runPromise(op).then((m) => m && notifyRef.current(m, 'busy')).catch((e) => notifyRef.current(coreErrMsg(e), 'err'));
  };

  // Every action needs the game itself. One that left the library gets a
  // dialog that only says what is pending.
  if (!game) {
    return (
      <Modal label={t('cloud:host.decisionLabel')} onClose={close}>
        <h2 className="text-[19px] font-extrabold tracking-tight">{title}</h2>
        <p className="mt-2 text-[12.5px] leading-relaxed text-text-muted">{pending.message}</p>
        <p className="mt-2 text-[12.5px] leading-relaxed text-text-muted">{t('cloud:host.openGame')}</p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={close}>{t('common:close')}</Button>
        </div>
      </Modal>
    );
  }

  if (pending.kind === 'conflict' && pending.status) {
    return (
      <CloudConflictDialog
        game={game}
        status={pending.status}
        onClose={close}
        onLaunchAnyway={() => run(LauncherApi.launchGameAnyway(game, 'conflict'))}
        onResolved={(info) => {
          if (info.status?.state === 'conflict') {
            setPending({ ...pending, status: info.status });
            return;
          }
          notifyRef.current(t('cloud:host.settled'), 'ok');
          run(LauncherApi.launchGame(game));
        }}
      />
    );
  }

  if (pending.kind === 'client-running') {
    const name = pending.client || 'the store client';
    return (
      <Modal label={t('cloud:host.clientLabel')} onClose={close}>
        <h2 className="text-[19px] font-extrabold tracking-tight">{t('cloud:host.clientRunning', { name })}</h2>
        <p className="mt-2 text-[12.5px] leading-relaxed text-text-muted">{pending.message}</p>
        <div className="mt-5 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" onClick={close}>{t('common:cancel')}</Button>
          <Button variant="neutral" onClick={() => run(LauncherApi.launchGameAnyway(game, 'client-running'))}>{t('cloud:host.launchWithoutSync')}</Button>
          <Button variant="primary" onClick={() => run(LauncherApi.launchGameViaClient(game))}>{t('cloud:panel.launchVia', { name })}</Button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal label={t('cloud:host.offlineLabel')} onClose={close}>
      <h2 className="text-[19px] font-extrabold tracking-tight">{t('cloud:host.offlineLabel')}</h2>
      <p className="mt-2 text-[12.5px] leading-relaxed text-text-muted">{pending.message}</p>
      <p className="mt-2 text-[12.5px] leading-relaxed text-text-muted">
        {t('cloud:host.offlineWhy', { title: game.title })}
      </p>
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="ghost" onClick={close}>{t('common:cancel')}</Button>
        <Button variant="primary" onClick={() => run(LauncherApi.launchGameAnyway(game, 'offline'))}>{t('cloud:host.launchAnyway')}</Button>
      </div>
    </Modal>
  );
}
