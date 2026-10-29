// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { Game, GameCopy, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { diffFor, pickableCopies, playedCopy } from '../lib/family';
import { copyDiffSummaryText, copyDlcLabel, copyStatusLabel } from '../lib/copyLabels';
import { Button } from '@/components/ds/button';
import { CopyCompareDialog } from './CopyCompareDialog';
import { Panel, cx } from './ui';
import { ErrorNotice } from './ErrorNotice';

/** Which family copy a borrowed game is played from, and the choice between
 *  the lenders' copies (family, protocol 1.8). Platypus shows the copies and
 *  sends the pick; the store decides what a copy allows. Renders nothing for
 *  the user's own games. */
export function CopyPicker({ game, plugin, onChanged }: { game: Game; plugin: StorePlugin | undefined; onChanged: () => void }) {
  const { t } = useTranslation('gameOps');
  const copies = pickableCopies(game);
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [comparing, setComparing] = useState(false);
  useEffect(() => { setPending(null); setError(''); setComparing(false); }, [game.id]);
  if (copies.length === 0) return null;

  const played = playedCopy(game);
  const from = copies.find((c) => c.owner_id === played)?.owner_name ?? game.shared_by?.name ?? '';
  const compare = game.copy_compare;
  const baselineName = copies.find((c) => c.owner_id === compare?.baseline)?.owner_name ?? '';
  const pick = (copy: GameCopy) => {
    setPending(copy.owner_id);
    setError('');
    Effect.runPromise(LauncherApi.setPreferredCopy(game, copy.owner_id)).then(
      () => { setPending(null); onChanged(); },
      (e: Error) => { setPending(null); setError(e.message || t('gameOps:copies.switchError')); },
    );
  };

  return (
    <CapabilitySurface plugin={plugin} capability="family" game={game}>
      <Panel>
        <h2 className="mb-2 text-[15px] font-bold">{t('gameOps:copies.title')}</h2>
        <p className="font-mono text-hint text-text select-text">{t('gameOps:copies.from', { name: from })}</p>
        {copies.length > 1 && (
          <div role="radiogroup" aria-orientation="vertical" aria-label={t('gameOps:copies.group')} className="mt-3 space-y-2">
            {copies.map((copy) => {
              const status = copyStatusLabel(t, copy);
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
                    'block w-full rounded-xl border px-3 py-2 text-left font-mono text-hint',
                    chosen ? 'border-info/50 bg-info/10 text-text' : 'border-hairline text-text-body hover:bg-surface',
                    !copy.available && 'cursor-not-allowed opacity-40 hover:bg-transparent',
                  )}
                >
                  <span className="flex justify-between gap-3">
                    <span>{copy.owner_name}</span>
                    {status && <span className={copy.available ? 'text-attention/80' : 'text-text-muted'}>{status}</span>}
                  </span>
                  <span className="mt-1 block text-text-muted">{copyDlcLabel(t, copy)}</span>
                  {compare && copy.owner_id !== compare.baseline && diffFor(compare, copy.owner_id) && (
                    <span className="mt-1 block text-text-muted">
                      {copyDiffSummaryText(t, copy.owner_name, baselineName, diffFor(compare, copy.owner_id)!)}
                    </span>
                  )}
                  {copy.note && <span className="mt-1 block text-text-muted">{copy.note}</span>}
                  {copy.exclude && <span className="mt-1 block text-text-muted">{copy.exclude}</span>}
                </button>
              );
            })}
          </div>
        )}
        {compare && (
          <div className="mt-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
            <span className="font-mono text-hint text-text-muted">{t('gameOps:copies.vsBaseline', { name: baselineName })}</span>
            <Button variant="neutral" size="sm" onClick={() => setComparing(true)}>{t('gameOps:copies.compare')}</Button>
          </div>
        )}
        {error && <ErrorNotice className="mt-2" message={error} fallback={t('gameOps:copies.switchError')} />}
      </Panel>
      {comparing && compare && (
        <CopyCompareDialog game={game} copies={copies} compare={compare} onClose={() => setComparing(false)} />
      )}
    </CapabilitySurface>
  );
}
