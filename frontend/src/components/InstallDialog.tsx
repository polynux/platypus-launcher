import React, { useState } from 'react';
import { Effect } from 'effect';
import { TriangleAlert } from 'lucide-react';
import type { Game, StorePlugin } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { InstallLocationPicker } from './InstallLocationPicker';
import { LauncherApi } from '../effect/runtime';
import { formatBytes } from '../lib/games';
import { isShort } from '../lib/installLocation';
import { ensureTerms } from '../lib/termsRequest';
import { useInstallSize } from '../lib/installSize';
import { cx } from './ui';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface InstallDialogProps {
  game: Game;
  plugin: StorePlugin;
  onClose: () => void;
  /** An install was queued: the library should reload. */
  onChanged: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

/** "Install to…": the size of the install and where it goes, before it is queued.
 *  A game with install options picks the place in the options dialog instead. */
export function InstallDialog({ game, plugin, onClose, onChanged, onNotify }: InstallDialogProps) {
  const size = useInstallSize(game, plugin, true);
  const [folder, setFolder] = useState('');
  const [free, setFree] = useState(-1);
  const [busy, setBusy] = useState(false);
  const total = size.state === 'known' ? size.total : null;
  const short = isShort(total, free);

  const install = async () => {
    setBusy(true);
    try {
      if (!(await ensureTerms(game))) return;
      await Effect.runPromise(LauncherApi.installGameTo(game, folder));
      onNotify(`Install of ${game.title} queued`, 'busy');
      onChanged();
      onClose();
    } catch (e) {
      onNotify(errMsg(e), 'err');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label={`Install ${game.title}`} onClose={onClose} size="md" dismissable={!busy}>
      <h2 className="text-[20px] font-extrabold tracking-tight">Install {game.title}</h2>
      <p className="mt-1 text-[12.5px] text-white/50">{plugin.name}</p>

      <div className="mt-5 rounded-2xl bg-white/[0.04] px-4 py-3 font-mono text-[11.5px]">
        <div className="flex justify-between gap-4">
          <span className="text-white/45">Install size</span>
          <span className={cx(size.state === 'loading' ? 'text-white/40' : 'text-white/85')} data-testid="install-size">
            {size.state === 'loading' ? 'checking…' : size.state === 'unknown' ? 'Size unknown' : formatBytes(size.total)}
          </span>
        </div>
        {size.state === 'known' && size.added > 0 && (
          <div className="mt-1.5 flex justify-between gap-4 text-white/45">
            <span>Including add-ons</span><span>+{formatBytes(size.added)}</span>
          </div>
        )}
      </div>

      <div className="mt-5">
        <InstallLocationPicker game={game} needBytes={total} value={folder} onChange={(f, fr) => { setFolder(f); setFree(fr); }} disabled={busy} />
      </div>
      {short && (
        <p role="alert" className="mt-3 flex items-center gap-1.5 font-mono text-[11.5px] text-red-300">
          <TriangleAlert className="h-3.5 w-3.5" /> Not enough space there for this game.
        </p>
      )}

      <div className="mt-6 flex justify-end gap-2.5">
        <Button variant="neutral" size="sm" disabled={busy} onClick={onClose}>Cancel</Button>
        <Button variant="primary" size="sm" disabled={busy || short} onClick={() => void install()}>Install</Button>
      </div>
    </Modal>
  );
}
