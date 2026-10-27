// i18n: converted
import React, { useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { RedeemResult, StorePlugin } from '../types';
import { Modal, ModalFoot, ModalHead } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from './ds/input';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { useVariant } from '../lib/theme';

/** Matches the core's cap (stats.MaxCodeLen): the input can't type past it. */
export const MAX_CODE_LENGTH = 128;

/** Redeem a code with one store, picked from the stores that offer redeem
 *  now (`stores`, the caller filters them). The code goes to that store only;
 *  the result lists what it granted. A second submit waits for the first. */
export function RedeemDialog({ stores, onClose }: { stores: StorePlugin[]; onClose: () => void }) {
  const { t } = useTranslation('common');
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
      (err) => setError(coreErrMsg(err)),
    ).finally(() => setBusy(false));
  };

  // Atlas dialog chrome (issue #139): glass, 14px radius, the shared head
  // with the X, and the right-aligned footer. Other themes keep the dialog.
  const atlas = useVariant('shell.navigation') === 'sourcebar';

  return (
    <Modal
      label={t('common:redeem.label')}
      onClose={onClose}
      size="md"
      contentClassName={atlas ? 'bg-glass backdrop-blur-[22px] backdrop-saturate-[1.4] rounded-[14px] ring-hairline-strong' : undefined}
    >
      <div className="-m-7">
        <ModalHead title={t('common:redeem.title')} sub={t('common:redeem.sub')} onClose={onClose} />
        {result ? (
          <>
            <div className="grid gap-3 px-5 py-4 text-[14px]">
              <p className="text-text-body">{result.message}</p>
              {result.titles.length > 0 && (
                <div>
                  <div className="mb-1 text-[12px] font-semibold text-text-muted">{t('common:redeem.granted')}</div>
                  <ul className="space-y-1">
                    {result.titles.map((title) => (
                      <li key={title} className="text-accent-strong">{title}</li>
                    ))}
                  </ul>
                </div>
              )}
              {error && <ErrorNotice className="text-[13px]" message={error} fallback={t('common:redeem.error')} />}
            </div>
            <ModalFoot>
              <Button variant="ghost" onClick={() => setResult(null)}>{t('common:redeem.redeemAnother')}</Button>
              <Button variant="primary" onClick={onClose} className="h-[34px] rounded-lg">{t('common:done')}</Button>
            </ModalFoot>
          </>
        ) : (
          <form onSubmit={submit}>
            <div className="grid gap-4 px-5 py-4">
              <div className="flex items-center justify-between gap-[18px]">
                <span className="text-[13px] font-semibold text-text">{t('common:redeem.store')}</span>
                <select
                  value={plugin?.id ?? ''}
                  onChange={(e) => { setStoreId(e.target.value); setError(null); }}
                  disabled={busy || stores.length < 2}
                  aria-label={t('common:redeem.store')}
                  className="h-8 w-[220px] flex-none rounded-lg bg-surface px-2.5 text-[13px] text-text outline-none ring-1 ring-hairline focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-60"
                >
                  {stores.map((s) => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
              </div>
              <div className="grid gap-1.5">
                <label htmlFor="redeem-code" className="text-[13px] font-semibold text-text">
                  {t('common:redeem.code')}
                </label>
                <Input
                  id="redeem-code"
                  value={code}
                  maxLength={MAX_CODE_LENGTH}
                  onChange={(e) => setCode(e.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                  className="h-11 text-center font-mono text-[15px] tracking-[0.12em]"
                  placeholder={t('common:redeem.codePlaceholder')}
                  disabled={busy}
                />
                <p className="text-left text-[12px] text-text-faint">{t('common:redeem.codeHint')}</p>
              </div>
              {error && <ErrorNotice className="text-[13px]" message={error} fallback={t('common:redeem.error')} />}
            </div>
            <ModalFoot>
              <Button type="button" variant="ghost" onClick={onClose}>{t('common:cancel')}</Button>
              <Button type="submit" variant="primary" disabled={!plugin || !code.trim() || busy} className="h-[34px] rounded-lg">
                {busy ? t('common:redeem.redeeming') : t('common:redeem.redeem')}
              </Button>
            </ModalFoot>
          </form>
        )}
      </div>
    </Modal>
  );
}
