import React, { useState } from 'react';
import { Effect } from 'effect';
import type { RedeemResult, StorePlugin } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from './ds/input';
import { MonoLabel } from './ui';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** Matches the core's cap (stats.MaxCodeLen): the input can't type past it. */
export const MAX_CODE_LENGTH = 128;

/** Redeem a code with one store, picked from the stores that offer redeem
 *  now (`stores`, the caller filters them). The code goes to that store only;
 *  the result lists what it granted. A second submit waits for the first. */
export function RedeemDialog({ stores, onClose }: { stores: StorePlugin[]; onClose: () => void }) {
  const [storeId, setStoreId] = useState(stores[0]?.id ?? '');
  const plugin = stores.find((s) => s.id === storeId) ?? stores[0];
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<RedeemResult | null>(null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const typed = code.trim();
    if (!plugin || !typed || busy) return;
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

  return (
    <Modal label="Redeem a code" onClose={onClose}>
      <div className="p-6">
        <MonoLabel className="mb-2">redeem</MonoLabel>
        <h2 className="mb-4 text-[18px] font-bold">Redeem a code</h2>
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
              Store
              <select
                value={plugin?.id ?? ''}
                onChange={(e) => { setStoreId(e.target.value); setError(null); }}
                disabled={busy || stores.length < 2}
                className="mt-1.5 block w-full rounded-xl border border-white/10 bg-white/[0.04] px-3 py-2 text-[14px] text-white/90 outline-none focus-visible:ring-2 focus-visible:ring-amber-300/50 disabled:opacity-60"
              >
                {stores.map((s) => (
                  <option key={s.id} value={s.id}>{s.name}</option>
                ))}
              </select>
            </label>
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
            {error && <ErrorNotice className="text-[13px]" message={error} fallback="That code didn't go through." />}
            <div className="mt-6 flex gap-2">
              <Button type="button" onClick={onClose} className="flex-1 text-white/70">Cancel</Button>
              <Button type="submit" disabled={!plugin || !code.trim() || busy} className="flex-1 text-sky-100">
                {busy ? 'Redeeming…' : 'Redeem'}
              </Button>
            </div>
          </form>
        )}
      </div>
    </Modal>
  );
}
