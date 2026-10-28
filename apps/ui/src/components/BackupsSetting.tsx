// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { BackupSettings } from '../types';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { NumberField } from '@/components/ds/number-field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { useVariant } from '../lib/theme';
import { AtlasSRow, AtlasSSwitch } from './AtlasSettings';

const SECTION = 'max-w-2xl border-t border-white/[0.07] py-6';

/** Settings → Backups (issue #42): folder, versions kept, mode, after-session, manifest state + update. */
export function BackupsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [saved, setSaved] = useState<BackupSettings | null>(null);
  const [folder, setFolder] = useState('');
  const [keep, setKeep] = useState<number | ''>('');
  const [mode, setMode] = useState('destructive');
  const [afterSession, setAfterSession] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getBackupSettings())
      .then((s) => {
        setSaved(s);
        setFolder(s.folder === s.default_folder ? '' : s.folder);
        setKeep(s.keep);
        setMode(s.mode);
        setAfterSession(s.after_session);
      })
      .catch((e) => setError(coreErrMsg(e)));
  }, []);

  const dirty =
    saved != null &&
    ((folder || saved.default_folder) !== saved.folder ||
      (typeof keep === 'number' && keep !== saved.keep) ||
      mode !== saved.mode ||
      afterSession !== saved.after_session);

  const save = () => {
    if (!dirty || busy) return;
    setBusy(true);
    setError(null);
    Effect.runPromise(
      LauncherApi.setBackupSettings(folder, typeof keep === 'number' ? keep : 0, mode, afterSession),
    )
      .then((s) => {
        setSaved(s);
        setFolder(s.folder === s.default_folder ? '' : s.folder);
        setKeep(s.keep);
        setMode(s.mode);
        setAfterSession(s.after_session);
        onNotify(t('backups.saved'));
      })
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };

  const refresh = () => {
    setRefreshing(true);
    Effect.runPromise(LauncherApi.refreshSaveManifest())
      .then(() => Effect.runPromise(LauncherApi.getBackupSettings()))
      .then((s) => {
        setSaved(s);
        onNotify(t('backups.saved'));
      })
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setRefreshing(false));
  };

  const manifestState = saved?.manifest === 'ready' ? t('backups.manifest.ready')
    : saved?.manifest === 'fetching' ? t('backups.manifest.fetching')
    : saved?.manifest === 'stale' ? t('backups.manifest.stale')
    : t('backups.manifest.none');

  if (useVariant('settings-row.style') === 'srow') {
    const persist = (nextFolder: string, nextKeep: number, nextMode: string, nextAfter: boolean) => {
      setBusy(true);
      Effect.runPromise(LauncherApi.setBackupSettings(nextFolder, nextKeep, nextMode, nextAfter)).then((s) => {
        setSaved(s); setFolder(s.folder === s.default_folder ? '' : s.folder); setKeep(s.keep); setMode(s.mode); setAfterSession(s.after_session);
      }).catch((e) => setError(coreErrMsg(e))).finally(() => setBusy(false));
    };
    const changeFolder = () => Effect.runPromise(LauncherApi.pickFolder(t('backups.folderLabel'), saved?.folder || saved?.default_folder || '')).then((p) => { if (p) persist(p, Number(keep) || 5, mode, afterSession); }).catch((e) => setError(coreErrMsg(e)));
    return <section>
      <AtlasSSwitch label={t('backups.atlasAutoLabel')} desc={t('backups.afterSessionHint')} checked={afterSession} disabled={!saved || busy} onChange={() => persist(folder, Number(keep) || 5, mode, !afterSession)} />
      <AtlasSRow label={t('backups.keepLabel')}><Select value={String(keep || 5)} onValueChange={(v) => persist(folder, Number(v), mode, afterSession)} disabled={!saved || busy}><SelectTrigger aria-label={t('backups.keepLabel')} className="h-8 w-[180px] font-sans text-[13px]"><SelectValue /></SelectTrigger><SelectContent>{[3, 5, 10, 20].map((n) => <SelectItem key={n} value={String(n)}>{n}</SelectItem>)}</SelectContent></Select></AtlasSRow>
      <AtlasSRow label={t('backups.mode.destructive')}><Select value={mode} onValueChange={(v) => persist(folder, Number(keep) || 5, v, afterSession)} disabled={!saved || busy}><SelectTrigger aria-label={t('backups.mode.destructive')} className="h-8 w-[210px] font-sans text-[13px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="destructive">{t('backups.mode.destructive')}</SelectItem><SelectItem value="all">{t('backups.mode.all')}</SelectItem><SelectItem value="off">{t('backups.mode.off')}</SelectItem></SelectContent></Select></AtlasSRow>
      <AtlasSRow label={t('backups.folderLabel')} desc={saved?.folder || saved?.default_folder}><Button variant="ghost" size="sm" onClick={changeFolder} disabled={!saved || busy}>{t('prefixes.change')}</Button></AtlasSRow>
      <AtlasSRow label={t('backups.manifestLabel')} desc={manifestState}><Button variant="ghost" size="sm" onClick={refresh} disabled={refreshing}>{refreshing ? t('backups.updating') : t('backups.update')}</Button></AtlasSRow>
      {error && <ErrorNotice message={error} fallback={t('backups.saveError')} />}
    </section>;
  }

  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">{t('backups.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">{t('backups.description')}</p>
      <div className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-[12.5px] text-white/70">{t('backups.folderLabel')}</span>
          <Input
            value={folder}
            onChange={(e) => setFolder(e.target.value)}
            placeholder={saved?.default_folder ?? t('backups.folderPlaceholder')}
            disabled={!saved}
            aria-label={t('backups.folderLabel')}
          />
        </label>
        <div className="flex flex-wrap items-end gap-4">
          <label className="block">
            <span className="mb-1 block text-[12.5px] text-white/70">{t('backups.keepLabel')}</span>
            <NumberField min={1} max={100} step={1} hideSteppers className="w-24"
              value={keep === '' ? undefined : keep}
              onChange={(v) => setKeep(v ?? '')}
              disabled={!saved}
              aria-label={t('backups.keepLabel')} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12.5px] text-white/70">{t('backups.modeLabel')}</span>
            <Select value={mode} onValueChange={setMode} disabled={!saved}>
              <SelectTrigger aria-label={t('backups.modeLabel')} className="w-64">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="destructive">{t('backups.mode.destructive')}</SelectItem>
                <SelectItem value="all">{t('backups.mode.all')}</SelectItem>
                <SelectItem value="off">{t('backups.mode.off')}</SelectItem>
              </SelectContent>
            </Select>
          </label>
        </div>
        <p className="text-[12px] text-white/55">
          {mode === 'all' ? t('backups.modeHint.all') : mode === 'off' ? t('backups.modeHint.off') : t('backups.modeHint.destructive')}
        </p>
        <label className="flex items-start gap-2">
          <Switch checked={afterSession} onCheckedChange={setAfterSession} disabled={!saved} aria-label={t('backups.afterSessionLabel')} />
          <span>
            <span className="block text-[12.5px]">{t('backups.afterSessionLabel')}</span>
            <span className="block text-[12px] text-white/55">{t('backups.afterSessionHint')}</span>
          </span>
        </label>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[12.5px] text-white/70">{t('backups.manifestLabel')}: {manifestState}</span>
          <Button variant="ghost" onClick={refresh} disabled={refreshing}>
            {refreshing ? t('backups.updating') : t('backups.update')}
          </Button>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="primary" onClick={save} disabled={!dirty || busy}>{t('common:save')}</Button>
        </div>
        {error && <ErrorNotice message={error} fallback={t('backups.saveError')} />}
      </div>
    </section>
  );
}
