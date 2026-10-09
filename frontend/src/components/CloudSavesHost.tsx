import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import type { ActivityEvent, CloudPrompt, CloudStatus, Game } from '../types';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { CloudConflictDialog } from './CloudSavesConflict';
import { LauncherApi, onCloud } from '../effect/runtime';
import { cloudNotice } from '../lib/cloud';

type Notify = (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface Pending {
  game: Game;
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
  const gamesRef = useRef(games);
  gamesRef.current = games;
  const notifyRef = useRef(onNotify);
  notifyRef.current = onNotify;
  const logRef = useRef(onLog);
  logRef.current = onLog;

  useEffect(
    () =>
      onCloud((e) => {
        const game = gamesRef.current.find((g) => g.id === e.game_id);
        const title = game?.title ?? e.game_id;
        const n = cloudNotice(e, title);
        if (n?.toast) notifyRef.current(n.toast.msg, n.toast.kind);
        if (n?.activity) logRef.current({ type: 'cloud', game: title, ...n.activity });
        if (e.kind === 'prompt' && e.prompt && game) {
          setPending({ game, kind: e.prompt, message: e.message ?? '', status: e.status, client: e.client });
        }
      }),
    [],
  );

  if (!pending) return null;
  const { game } = pending;
  const close = () => setPending(null);
  const run = (op: Effect.Effect<string, Error>) => {
    close();
    Effect.runPromise(op).then((m) => m && notifyRef.current(m, 'busy')).catch((e) => notifyRef.current(errMsg(e), 'err'));
  };

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
          notifyRef.current('Saves settled', 'ok');
          run(LauncherApi.launchGame(game));
        }}
      />
    );
  }

  if (pending.kind === 'client-running') {
    const name = pending.client || 'the store client';
    return (
      <Modal label="Store client running" onClose={close} className="max-w-md">
        <h2 className="text-[19px] font-extrabold tracking-tight">{name} is running</h2>
        <p className="mt-2 text-[12.5px] leading-relaxed text-white/60">{pending.message}</p>
        <div className="mt-5 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" onClick={close}>Cancel</Button>
          <Button variant="neutral" onClick={() => run(LauncherApi.launchGameAnyway(game, 'client-running'))}>Launch without cloud sync</Button>
          <Button variant="primary" onClick={() => run(LauncherApi.launchGameViaClient(game))}>Launch through {name}</Button>
        </div>
      </Modal>
    );
  }

  return (
    <Modal label="Cloud saves not synced" onClose={close} className="max-w-md">
      <h2 className="text-[19px] font-extrabold tracking-tight">Cloud saves aren't synced</h2>
      <p className="mt-2 text-[12.5px] leading-relaxed text-white/60">{pending.message}</p>
      <p className="mt-2 text-[12.5px] leading-relaxed text-white/50">
        You can launch {game.title} anyway. Its saves on this device may be older or newer than the cloud's.
      </p>
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="ghost" onClick={close}>Cancel</Button>
        <Button variant="primary" onClick={() => run(LauncherApi.launchGameAnyway(game, 'offline'))}>Launch anyway</Button>
      </div>
    </Modal>
  );
}
