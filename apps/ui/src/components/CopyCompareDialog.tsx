// i18n: converted
import React, { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { CopyComparison, Game, GameCopy } from '../types';
import { Modal } from './Modal';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Switch } from '@/components/ds/switch';
import { compareRows } from '../lib/family';
import { copyStatusLabel } from '../lib/copyLabels';
import { cx } from './ui';

export interface CopyCompareDialogProps {
  game: Game;
  copies: GameCopy[];
  compare: CopyComparison;
  onClose: () => void;
}

/** The family copies of a borrowed game side by side: a row per add-on, a
 *  column per copy, the baseline first. "Differences only" is on and soundtracks
 *  are hidden by default, so the table stays short at 1024×640. */
export function CopyCompareDialog({ game, copies, compare, onClose }: CopyCompareDialogProps) {
  const { t } = useTranslation(['gameOps', 'common']);
  const [differencesOnly, setDifferencesOnly] = useState(true);
  const [soundtracks, setSoundtracks] = useState(false);

  // The baseline column comes first; the other copies keep the order they came in.
  const ordered = useMemo(() => {
    const base = copies.find((c) => c.owner_id === compare.baseline);
    return base ? [base, ...copies.filter((c) => c !== base)] : copies;
  }, [copies, compare.baseline]);
  const rows = useMemo(() => compareRows(ordered, { differencesOnly, soundtracks }), [ordered, differencesOnly, soundtracks]);
  const baseline = ordered[0];
  const why = game.preferred_copy === compare.baseline ? t('gameOps:compare.whyPreferred') : t('gameOps:compare.whyMost');

  return (
    <Modal label={t('gameOps:compare.dialogLabel', { title: game.title })} onClose={onClose} size="lg">
      <h2 className="text-[20px] font-extrabold tracking-tight">{t('gameOps:compare.title')}</h2>
      <p className="mt-1 text-[12.5px] text-white/70">
        {t('gameOps:compare.subtitle', { title: game.title, name: baseline.owner_name, why })}
      </p>

      <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 font-mono text-hint text-white/80">
        <label className="flex items-center gap-2">
          <Switch aria-label={t('gameOps:compare.diffsOnly')} checked={differencesOnly} onCheckedChange={setDifferencesOnly} />
          {t('gameOps:compare.diffsOnly')}
        </label>
        <label className="flex items-center gap-2">
          <Switch aria-label={t('gameOps:compare.soundtracks')} checked={soundtracks} onCheckedChange={setSoundtracks} />
          {t('gameOps:compare.soundtracks')}
        </label>
      </div>

      <div className="mt-4 max-h-[340px] overflow-auto rounded-2xl bg-white/[0.04]">
        <table className="w-full border-collapse font-mono text-hint" aria-label={t('gameOps:compare.table')}>
          <thead className="sticky top-0 bg-[#191410]">
            <tr className="text-left text-white/55">
              <th scope="col" className="px-3 py-2 font-normal">{t('gameOps:compare.addon')}</th>
              {ordered.map((c) => (
                <th key={c.owner_id} scope="col" className="px-3 py-2 align-top font-normal">
                  <span className={cx('block', c.owner_id === baseline.owner_id ? 'text-amber-200' : 'text-white/85')}>{c.owner_name}</span>
                  <CopyFlags copy={c} />
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-white/[0.06]">
                <td className="px-3 py-2 text-white/85">
                  {row.name}
                  {row.kind !== 'dlc' && <span className="ml-2 text-white/55">{row.kind}</span>}
                </td>
                {ordered.map((c) => {
                  const has = row.owners.includes(c.owner_id);
                  return (
                    <td key={c.owner_id} className="px-3 py-2 text-center">
                      <span aria-label={has ? t('gameOps:compare.hasIt', { name: c.owner_name }) : t('gameOps:compare.lacksIt', { name: c.owner_name })} className={has ? 'text-emerald-300' : 'text-white/55'}>
                        {has ? '✓' : '—'}
                      </span>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && (
          <p className="px-4 py-6 text-center text-[12.5px] text-white/70">
            {!differencesOnly ? t('gameOps:compare.emptyBare') : soundtracks ? t('gameOps:compare.emptySame') : t('gameOps:compare.emptySoundtracks')}
          </p>
        )}
      </div>

      {ordered.some((c) => c.exclude) && (
        <ul className="mt-4 space-y-1 font-mono text-hint text-white/55">
          {ordered.filter((c) => c.exclude).map((c) => <li key={c.owner_id}>{c.owner_name}: {c.exclude}</li>)}
        </ul>
      )}

      <div className="mt-6 flex justify-end">
        <Button variant="neutral" size="sm" onClick={onClose}>{t('common:close')}</Button>
      </div>
    </Modal>
  );
}

/** A copy's state as the table's header shows it: in use or unavailable. */
function CopyFlags({ copy }: { copy: GameCopy }) {
  const { t } = useTranslation('gameOps');
  const status = copyStatusLabel(t, copy);
  if (!status) return null;
  return <Badge variant="neutral" className="mt-1">{status}</Badge>;
}
