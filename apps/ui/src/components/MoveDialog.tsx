// i18n: converted
import React, { useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { Game, Job, MoveCheck } from '../types';
import { Modal, ModalFoot, ModalHead } from './Modal';
import { Button } from '@/components/ds/button';
import { LauncherApi } from '../effect/runtime';
import { formatBytes } from '../lib/games';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';

export interface MoveDialogProps {
  game: Game;
  /** Where the folder picker opens: the game's parent folder. */
  startFolder: string;
  onClose: () => void;
  /** The move was queued; it runs in Activity like an install. */
  onQueued: (job: Job) => void;
}

/** Moves an installed game into a folder the user picks. Platypus checks the room
 *  and the folder before the move is queued; the game ends up in a folder with
 *  its own name inside the one picked. */
export function MoveDialog({ game, startFolder, onClose, onQueued }: MoveDialogProps) {
  const { t } = useTranslation(['gameOps', 'common']);
  const [parent, setParent] = useState<string | null>(null);
  const [check, setCheck] = useState<MoveCheck | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const inspect = (folder: string) => {
    setParent(folder);
    setBusy(true);
    Effect.runPromise(LauncherApi.checkMove(game, folder)).then(
      (c) => {
        setCheck(c);
        setError(null);
      },
      (e) => {
        setCheck(null);
        setError(coreErrMsg(e));
      },
    ).finally(() => setBusy(false));
  };

  const pick = () => {
    Effect.runPromise(LauncherApi.pickMoveFolder(parent ?? startFolder)).then(
      (folder) => {
        if (folder) inspect(folder);
      },
      (e) => setError(coreErrMsg(e)),
    );
  };

  const move = () => {
    if (!parent || !check) return;
    setBusy(true);
    Effect.runPromise(LauncherApi.moveGame(game, parent)).then(
      (job) => onQueued(job),
      (e) => setError(coreErrMsg(e)),
    ).finally(() => setBusy(false));
  };

  return (
    <Modal label={t('gameOps:move.title', { title: game.title })} onClose={onClose} size="md">
      <div className="-m-7">
        <ModalHead title={t('gameOps:move.title', { title: game.title })} onClose={onClose} />
        <div className="p-5">
        <p className="mb-4 text-[13px] text-text-muted">
          {t('gameOps:move.description')}
        </p>
        <div className="space-y-2 font-mono text-hint">
          <div className="flex justify-between gap-4 border-t border-hairline pt-2">
            <span className="text-text-muted">{t('gameOps:move.from')}</span>
            <div className="flex min-w-0 items-center gap-2">
              <span className="truncate text-right text-text" title={game.install_dir}>{game.install_dir}</span>
              <Button variant="neutral" size="sm" onClick={pick} disabled={busy} className="flex-none font-sans">
                {parent ? t('gameOps:move.pickAnother') : t('gameOps:move.pick')}
              </Button>
            </div>
          </div>
          {check && (
            <>
              <div className="flex justify-between gap-4 border-t border-hairline pt-2">
                <span className="text-text-muted">{t('gameOps:move.to')}</span>
                <span className="truncate text-right text-text" title={check.to}>{check.to}</span>
              </div>
              <div className="flex justify-between gap-4 border-t border-hairline pt-2">
                <span className="text-text-muted">{t('gameOps:move.needs')}</span>
                <span className="text-text">{formatBytes(check.need_bytes)}</span>
              </div>
              <div className="flex justify-between gap-4 border-t border-hairline pt-2">
                <span className="text-text-muted">{t('gameOps:move.freeThere')}</span>
                <span className="text-text">{formatBytes(check.free_bytes)}</span>
              </div>
            </>
          )}
        </div>
        {error && <ErrorNotice className="mt-3 text-[13px]" message={error} fallback={t('gameOps:move.error')} />}
        </div>
        <ModalFoot>
          <Button variant="ghost" onClick={onClose}>{t('common:cancel')}</Button>
          <Button variant="primary" onClick={move} disabled={busy || !check}>{t('gameOps:move.action')}</Button>
        </ModalFoot>
      </div>
    </Modal>
  );
}
