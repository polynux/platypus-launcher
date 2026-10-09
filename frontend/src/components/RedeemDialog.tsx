import React, { useState } from 'react';
import { Effect } from 'effect';
import type { RedeemResult, StorePlugin } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from './ds/input';
import { MonoLabel } from './ui';
import { LauncherApi } from '../effect/runtime';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** Matches the core's cap (stats.MaxCodeLen): the input can't type past it. */
export const MAX_CODE_LENGTH = 128;

/** Redeem a code with one store. The code goes to that store only; the
 *  result lists what it granted. A second submit waits for the first. */
export function RedeemDialog({ plugin, onClose }: { plugin: StorePlugin; onClose: () => void }) {
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<RedeemResult | null>(null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const typed = code.trim();
    if (!typed || busy) return;
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.redeemCode(plugin.id, typed)).then(
      (r) => {
        setResult(r);
        setCode('');
      },
      (err) => setError(errMsg(err)),
    ).finally(() => setBusy(false));
  };

  const title = `Redeem a code · ${plugin.name}`;

  return (
    <Modal label={title} onClose={onClose}>
      <div className="p-6">
        <MonoLabel className="mb-2">redeem</MonoLabel>
        <h2 className="mb-4 text-[18px] font-bold">{title}</h2>
        {result ? (
          <div className="space-y-3 text-[14px]">
            <p className="text-white/80">{result.message}</p>
            {result.titles.length > 0 && (
              <div>
                <MonoLabel className="mb-1">granted</MonoLabel>
                <ul className="space-y-1">
                  {result.titles.map((t) => (
                    <li key={t} className="text-amber-200">{t}</li>
                  ))}
                </ul>
              </div>
            )}
            <div className="mt-6 flex gap-2">
              <Button onClick={() => setResult(null)} className="flex-1 text-white/70">Redeem another</Button>
              <Button onClick={onClose} className="flex-1 text-sky-100">Done</Button>
            </div>
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-3">
            <label className="block text-[13px] text-white/70">
              Code
              <Input
                value={code}
                maxLength={MAX_CODE_LENGTH}
                onChange={(e) => setCode(e.target.value)}
                autoComplete="off"
                spellCheck={false}
                className="mt-1.5 font-mono"
                placeholder="XXXXX-XXXXX-XXXXX"
                disabled={busy}
              />
            </label>
            {error && <p role="alert" className="text-[13px] text-red-300">{error}</p>}
            <div className="mt-6 flex gap-2">
              <Button type="button" onClick={onClose} className="flex-1 text-white/70">Cancel</Button>
              <Button type="submit" disabled={!code.trim() || busy} className="flex-1 text-sky-100">
                {busy ? 'Redeeming…' : 'Redeem'}
              </Button>
            </div>
          </form>
        )}
      </div>
    </Modal>
  );
}
