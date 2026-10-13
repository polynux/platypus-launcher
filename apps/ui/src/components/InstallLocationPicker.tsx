// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { FolderOpen } from 'lucide-react';
import type { Game, InstallLocation } from '../types';
import { Button } from '@/components/ds/button';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { freeLabel, isShort, kindLabel } from '../lib/installLocation';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

type Row = Omit<InstallLocation, 'kind'> & { kind: InstallLocation['kind'] | 'picked' };

export interface InstallLocationPickerProps {
  game: Game;
  /** Bytes the install needs (null when unknown): places it doesn't fit in say so. */
  needBytes: number | null;
  /** The chosen install folder; '' is the store's own. */
  value: string;
  /** The choice changed, with the room at the chosen place (-1 when unreadable). */
  onChange: (folder: string, freeBytes: number) => void;
  disabled?: boolean;
}

/** Where a game installs: the store's own folder, the user's game folders, folders
 *  they installed into before, and "Choose folder…". A folder picked by hand is
 *  checked by core before it can be chosen; a refusal is shown and nothing changes. */
export function InstallLocationPicker({ game, needBytes, value, onChange, disabled }: InstallLocationPickerProps) {
  const { t } = useTranslation('gameOps');
  const [rows, setRows] = useState<Row[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    setRows(null);
    setError(null);
    Effect.runPromise(LauncherApi.getInstallLocations(game)).then(
      (list) => {
        if (!live) return;
        setRows(list);
        const def = list.find((l) => l.kind === 'default');
        if (def) onChange('', def.free_bytes);
      },
      (e) => live && setError(errMsg(e)),
    );
    return () => { live = false; };
    // The places depend on the game's store, not on what the parent does with the choice.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  const select = (r: Row) => onChange(r.kind === 'default' ? '' : r.folder, r.free_bytes);

  const choose = async () => {
    setBusy(true);
    setError(null);
    try {
      const start = (value || rows?.find((l) => l.kind === 'default')?.folder) ?? '';
      const folder = await Effect.runPromise(LauncherApi.pickInstallFolder(start));
      if (!folder) return;
      // Core's guards (system folders, writable, not a file…) decide whether the folder can be chosen.
      const check = await Effect.runPromise(LauncherApi.checkInstallFolder(folder));
      setRows((list) => {
        const rest = (list ?? []).filter((r) => r.kind !== 'picked');
        const known = rest.find((r) => r.folder === check.folder);
        if (known) return rest;
        return [...rest, { folder: check.folder, kind: 'picked', free_bytes: check.free_bytes, exists: check.exists }];
      });
      onChange(check.folder, check.free_bytes);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section>
      <MonoLabel className="mb-1.5">{t('gameOps:location.heading')}</MonoLabel>
      {!rows && !error && <p className="font-mono text-micro uppercase tracking-widest text-white/55">{t('gameOps:location.loading')}</p>}
      {rows && (
        <div role="radiogroup" aria-label={t('gameOps:location.group')} className="space-y-1.5">
          {rows.map((r) => {
            const chosen = (r.kind === 'default' ? '' : r.folder) === value;
            const short = isShort(needBytes, r.free_bytes);
            return (
              <button
                key={r.kind + r.folder}
                type="button"
                role="radio"
                aria-checked={chosen}
                disabled={disabled}
                onClick={() => select(r)}
                className={cx(
                  'flex w-full items-center justify-between gap-3 rounded-xl px-3.5 py-2.5 text-left ring-1 transition disabled:opacity-60',
                  chosen ? 'bg-amber-300/[0.08] ring-amber-300/35' : 'bg-white/[0.03] ring-white/[0.06] hover:bg-white/[0.06]',
                )}
              >
                <span className="min-w-0">
                  <span className="block font-mono text-micro uppercase tracking-wider text-white/55">{kindLabel(r.kind, t)}</span>
                  <span className="block truncate font-mono text-[12px] text-white/85" title={r.folder}>{r.folder}</span>
                </span>
                <span className={cx('flex-none font-mono text-hint', short ? 'text-red-300' : 'text-white/55')}>
                  {freeLabel(r.free_bytes)}
                </span>
              </button>
            );
          })}
        </div>
      )}
      <Button variant="neutral" size="sm" onClick={() => void choose()} disabled={disabled || busy || !rows} className="mt-2">
        <FolderOpen className="h-3.5 w-3.5" /> {t('gameOps:location.choose')}
      </Button>
      {error && <ErrorNotice className="mt-2" message={error} fallback={t('gameOps:location.error')} />}
    </section>
  );
}
