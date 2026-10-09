import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { Game, GameCopy, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { copyStatus, dlcLabel, pickableCopies, playedCopy } from '../lib/family';
import { Panel, cx } from './ui';

/** Which family copy a borrowed game is played from, and the choice between
 *  the lenders' copies (family, protocol 1.8). Hydra shows the copies and
 *  sends the pick; the store decides what a copy allows. Renders nothing for
 *  the user's own games. */
export function CopyPicker({ game, plugin, onChanged }: { game: Game; plugin: StorePlugin | undefined; onChanged: () => void }) {
  const copies = pickableCopies(game);
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState('');
  useEffect(() => { setPending(null); setError(''); }, [game.id]);
  if (copies.length === 0) return null;

  const played = playedCopy(game);
  const from = copies.find((c) => c.owner_id === played)?.owner_name ?? game.shared_by?.name ?? '';
  const pick = (copy: GameCopy) => {
    setPending(copy.owner_id);
    setError('');
    Effect.runPromise(LauncherApi.setPreferredCopy(game, copy.owner_id)).then(
      () => { setPending(null); onChanged(); },
      (e: Error) => { setPending(null); setError(e.message || 'Could not change the copy.'); },
    );
  };

  return (
    <CapabilitySurface plugin={plugin} capability="family" game={game}>
      <Panel className="bg-white/[0.05]">
        <h2 className="mb-2 text-[15px] font-bold">Copy</h2>
        <p className="font-mono text-[11.5px] text-white/85 select-text">Copy: from {from}</p>
        {copies.length > 1 && (
          <div role="radiogroup" aria-label="Family copies" className="mt-3 space-y-2">
            {copies.map((copy) => {
              const status = copyStatus(copy);
              const chosen = copy.owner_id === played;
              return (
                <button
                  key={copy.owner_id}
                  type="button"
                  role="radio"
                  aria-checked={chosen}
                  disabled={!copy.available || pending !== null}
                  onClick={() => pick(copy)}
                  className={cx(
                    'block w-full rounded-xl border px-3 py-2 text-left font-mono text-[11.5px]',
                    chosen ? 'border-sky-300/50 bg-sky-400/10 text-white' : 'border-white/[0.08] text-white/75 hover:bg-white/[0.06]',
                    !copy.available && 'cursor-not-allowed opacity-40 hover:bg-transparent',
                  )}
                >
                  <span className="flex justify-between gap-3">
                    <span>{copy.owner_name}</span>
                    {status && <span className={copy.available ? 'text-amber-200/80' : 'text-white/50'}>{status}</span>}
                  </span>
                  <span className="mt-1 block text-white/50">{dlcLabel(copy)}</span>
                  {copy.note && <span className="mt-1 block text-white/45">{copy.note}</span>}
                </button>
              );
            })}
          </div>
        )}
        {error && <p role="alert" className="mt-2 font-mono text-[11.5px] text-red-300">{error}</p>}
      </Panel>
    </CapabilitySurface>
  );
}
