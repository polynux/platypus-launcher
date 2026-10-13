// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { CloudMode, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { offers } from '../lib/capabilities';
import { asCloudMode, modeHelp } from '../lib/cloud';
import { MonoLabel, Panel, SegRow } from './ui';
import { ErrorNotice } from './ErrorNotice';

/** The cloud-saves default of every ready store that offers cloud saves, one
 *  row each, in Settings. Games can override their store's default on their
 *  own page. Nothing renders when no store offers cloud saves. */
export function CloudSavesDefaults({ plugins }: { plugins: StorePlugin[] }) {
  const { t } = useTranslation('cloud');
  const stores = plugins.filter((p) => p.status === 'ready' && offers(p, 'cloud-saves'));
  if (stores.length === 0) return null;
  return (
    <Panel as="section" className="mt-4 max-w-2xl">
      <h2 className="mb-1 text-[14px] font-bold">{t('cloud:panel.title')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
        {t('cloud:setting.description')}
      </p>
      <div className="space-y-4">
        {stores.map((p) => (
          <Row key={p.id} plugin={p} />
        ))}
      </div>
    </Panel>
  );
}

function Row({ plugin }: { plugin: StorePlugin }) {
  const { t } = useTranslation('cloud');
  const [mode, setMode] = useState<CloudMode>('ask');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getCloudSettings())
      .then((s) => live && setMode(asCloudMode(s.plugins[plugin.id] ?? 'ask')))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)));
    return () => { live = false; };
  }, [plugin.id]);

  const change = (next: CloudMode) => {
    const prev = mode;
    setMode(next);
    setError(null);
    Effect.runPromise(LauncherApi.setCloudPluginMode(plugin.id, next)).catch((e) => {
      setMode(prev);
      setError(e instanceof Error ? e.message : String(e));
    });
  };

  return (
    <div className="space-y-1.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <MonoLabel>{plugin.name}</MonoLabel>
        <SegRow<CloudMode>
          label={t('cloud:setting.rowLabel', { name: plugin.name })}
          value={mode}
          onChange={change}
          options={[{ value: 'off', label: t('cloud:mode.off') }, { value: 'ask', label: t('cloud:mode.ask') }, { value: 'auto', label: t('cloud:mode.auto') }]}
        />
      </div>
      <p className="text-hint leading-snug text-white/55">{modeHelp(mode)}</p>
      {error && <ErrorNotice message={error} fallback={t('cloud:setting.error')} />}
    </div>
  );
}
