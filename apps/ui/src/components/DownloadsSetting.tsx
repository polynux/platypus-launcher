// i18n: converted
// Downloads settings (issues #207, #209): the global speed cap, automatic
// updates with the daily window they wait for, plus the canceled-downloads
// retention these rows sit with. The window only ever holds updates Platypus
// starts by itself; anything the user starts runs right away.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import type { LauncherSettings } from '../types';
import { NumberField } from '@/components/ds/number-field';
import { Input } from '@/components/ds/input';
import { Switch } from '@/components/ds/switch';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { LauncherApi } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';
import { useVariant } from '../lib/theme';
import { ErrorNotice } from './ErrorNotice';
import { AtlasSRow } from './AtlasSettings';

const SECTION = 'max-w-2xl border-t border-hairline py-6';

export function DownloadsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [busy, setBusy] = useState(false);
  // The speed cap is stored in bytes per second and edited in KiB/s; 0 is
  // unlimited.
  const [limitKib, setLimitKib] = useState<number | ''>('');
  const [autoOn, setAutoOn] = useState(true);
  const [windowOn, setWindowOn] = useState(false);
  const [windowStart, setWindowStart] = useState('03:00');
  const [windowEnd, setWindowEnd] = useState('06:00');
  const [cleanupDays, setCleanupDays] = useState<number | ''>('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s: LauncherSettings) => {
        setSaved(s);
        setCleanupDays(s.cancel_cleanup_days);
        setLimitKib(Math.round((s.download_limit ?? 0) / 1024));
        setAutoOn(!!s.auto_update);
        setWindowOn(!!s.update_window_enabled);
        if (s.update_window_start) setWindowStart(s.update_window_start);
        if (s.update_window_end) setWindowEnd(s.update_window_end);
      })
      .catch((e: unknown) => setError(coreErrMsg(e)));
  }, []);

  const saveLimit = (kib: number) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setDownloadLimit(Math.round(kib) * 1024))
      .then((s: LauncherSettings) => {
        setSaved(s);
        setLimitKib(Math.round(s.download_limit / 1024));
        onNotify(t('downloads.limitSaved'));
      })
      .catch((e: unknown) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };
  const limitDirty = saved != null && typeof limitKib === 'number' && Math.round(limitKib) !== Math.round((saved.download_limit ?? 0) / 1024);

  const saveAuto = (on: boolean) => {
    setAutoOn(on);
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setAutoUpdate(on))
      .then((s: LauncherSettings) => {
        setSaved(s);
        setAutoOn(!!s.auto_update);
        onNotify(t('downloads.autoSaved'));
      })
      .catch((e: unknown) => { setAutoOn(!on); setError(coreErrMsg(e)); })
      .finally(() => setBusy(false));
  };

  const saveWindow = (on: boolean, start: string, end: string) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setUpdateWindow(on, start, end))
      .then((s: LauncherSettings) => {
        setSaved(s);
        setWindowOn(!!s.update_window_enabled);
        setWindowStart(s.update_window_start);
        setWindowEnd(s.update_window_end);
        onNotify(on ? t('downloads.scheduleSaved') : t('downloads.scheduleOff'));
      })
      .catch((e: unknown) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };
  const windowDirty = saved != null && (windowOn !== !!saved.update_window_enabled
    || windowStart !== saved.update_window_start || windowEnd !== saved.update_window_end);
  const windowLabel = t('downloads.windowLabel', { start: windowStart, end: windowEnd });

  const saveCleanupDays = (days: number) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setCancelCleanupDays(days))
      .then((s: LauncherSettings) => {
        setSaved(s);
        setCleanupDays(s.cancel_cleanup_days);
        onNotify(t('cleanup.saved'));
      })
      .catch((e: unknown) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };
  const cleanupDirty = saved != null && typeof cleanupDays === 'number' && cleanupDays !== saved.cancel_cleanup_days;

  const errorLine = <div className="mt-2.5 flex min-h-4 items-center gap-4">{error ? <ErrorNotice message={error} fallback={t('downloads.saveError')} /> : <span />}</div>;

  if (useVariant('settings-row.style') === 'srow') {
    return (
      <section>
        <AtlasSRow label={t('downloads.limitLabel')} desc={t('downloads.limitDescription')}>
          <div className="flex items-center gap-2">
            <NumberField min={0} max={1048576} step={1} hideSteppers value={limitKib === '' ? undefined : limitKib} onChange={(v) => { setLimitKib(v ?? ''); setError(null); }} disabled={!saved || busy} aria-label={t('downloads.limitLabel')} className="w-24" />
            <span className="font-mono text-[11.5px] text-text-muted">{t('downloads.kib')}</span>
            <Button variant="primary" disabled={!limitDirty || busy} onClick={() => { if (typeof limitKib === 'number') saveLimit(limitKib); }}>{t('common:save')}</Button>
          </div>
        </AtlasSRow>
        <AtlasSRow label={t('downloads.autoLabel')} desc={t('downloads.autoDescription')}>
          <Switch aria-label={t('downloads.autoLabel')} checked={autoOn} disabled={!saved || busy} onCheckedChange={saveAuto} />
        </AtlasSRow>
        <AtlasSRow label={windowLabel} desc={t('downloads.windowDescription')}>
          <div className="flex flex-wrap items-center gap-2">
            <Switch aria-label={t('downloads.windowToggle')} checked={windowOn} disabled={!saved || busy} onCheckedChange={(on) => { setWindowOn(on); setError(null); }} />
            <Input type="time" value={windowStart} onChange={(e) => { setWindowStart(e.target.value); setError(null); }} disabled={!saved || busy || !windowOn} aria-label={t('downloads.start')} className="w-28" />
            <span className="text-[12px] text-text-muted">{t('downloads.to')}</span>
            <Input type="time" value={windowEnd} onChange={(e) => { setWindowEnd(e.target.value); setError(null); }} disabled={!saved || busy || !windowOn} aria-label={t('downloads.end')} className="w-28" />
            <Button variant="primary" disabled={!windowDirty || busy} onClick={() => saveWindow(windowOn, windowStart, windowEnd)}>{t('common:save')}</Button>
          </div>
        </AtlasSRow>
        <AtlasSRow label={t('cleanup.heading')} desc={t('cleanup.description')}>
          <Select value={String(cleanupDays)} onValueChange={(v) => saveCleanupDays(Number(v))} disabled={!saved || busy}>
            <SelectTrigger aria-label={t('cleanup.daysLabel')} className="h-8 w-[210px] font-sans text-[13px]"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="0">{t('cleanup.deleteRightAway')}</SelectItem><SelectItem value="7">{t('cleanup.keepWeek')}</SelectItem><SelectItem value="365">{t('cleanup.keepYear')}</SelectItem></SelectContent>
          </Select>
        </AtlasSRow>
        {errorLine}
      </section>
    );
  }

  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('downloads.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-text-body">{t('downloads.description')}</p>
      <form className="flex flex-wrap items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (limitDirty && typeof limitKib === 'number') saveLimit(limitKib); }}>
        <label className="flex items-center gap-2"><span className="text-[12.5px] text-text-body">{t('downloads.limitLabel')}</span>
          <NumberField min={0} max={1048576} step={1} hideSteppers value={limitKib === '' ? undefined : limitKib} onChange={(v) => { setLimitKib(v ?? ''); setError(null); }} disabled={!saved || busy} aria-label={t('downloads.limitLabel')} className="w-24" />
          <span className="text-[12.5px] text-text-body">{t('downloads.kib')}</span>
        </label>
        <Button type="submit" variant="primary" disabled={!limitDirty || busy}>{t('common:save')}</Button>
      </form>
      <p className="mt-1 max-w-[75ch] text-[12px] leading-relaxed text-text-muted">{t('downloads.limitDescription')}</p>
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Switch aria-label={t('downloads.autoLabel')} checked={autoOn} disabled={!saved || busy} onCheckedChange={saveAuto} />
        <span className="text-[12.5px] text-text-body">{t('downloads.autoLabel')}</span>
      </div>
      <p className="mt-1 max-w-[75ch] text-[12px] leading-relaxed text-text-muted">{t('downloads.autoDescription')}</p>
      <form className="mt-4 flex flex-wrap items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (windowDirty) saveWindow(windowOn, windowStart, windowEnd); }}>
        <label className="flex items-center gap-2">
          <Switch aria-label={t('downloads.windowToggle')} checked={windowOn} disabled={!saved || busy} onCheckedChange={(on) => { setWindowOn(on); setError(null); }} />
          <span className="text-[12.5px] text-text-body">{windowLabel}</span>
        </label>
        <Input type="time" value={windowStart} onChange={(e) => { setWindowStart(e.target.value); setError(null); }} disabled={!saved || busy || !windowOn} aria-label={t('downloads.start')} className="w-28" />
        <span className="text-[12.5px] text-text-body">{t('downloads.to')}</span>
        <Input type="time" value={windowEnd} onChange={(e) => { setWindowEnd(e.target.value); setError(null); }} disabled={!saved || busy || !windowOn} aria-label={t('downloads.end')} className="w-28" />
        <Button type="submit" variant="primary" disabled={!windowDirty || busy}>{t('common:save')}</Button>
      </form>
      <p className="mt-1 max-w-[75ch] text-[12px] leading-relaxed text-text-muted">{t('downloads.windowDescription')}</p>
      <form className="mt-4 flex flex-wrap items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (cleanupDirty && typeof cleanupDays === 'number') saveCleanupDays(cleanupDays); }}>
        <label className="flex items-center gap-2"><span className="text-[12.5px] text-text-body">{t('cleanup.keepFor')}</span>
          <NumberField min={0} max={365} step={1} hideSteppers value={cleanupDays === '' ? undefined : cleanupDays} onChange={(v) => { setCleanupDays(v ?? ''); setError(null); }} disabled={!saved || busy} aria-label={t('cleanup.daysLabel')} className="w-24" />
          <span className="text-[12.5px] text-text-body">{t('cleanup.days')}</span>
        </label>
        <Button type="submit" variant="primary" disabled={!cleanupDirty || busy}>{t('common:save')}</Button>
      </form>
      <p className="mt-1 max-w-[75ch] text-[12px] leading-relaxed text-text-muted">{t('cleanup.description')}</p>
      {errorLine}
    </section>
  );
}
