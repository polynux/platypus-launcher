import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { CloudMode, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { MODE_HELP } from '../lib/cloud';
import { MonoLabel, SegRow } from './ui';

/** A plugin's default cloud sync mode (off, ask, auto), shown on its card in
 *  the Plugins view for plugins that offer cloud saves. Games can override it
 *  on their own page. */
export function CloudSavesSetting({ plugin }: { plugin: StorePlugin }) {
  return (
    <CapabilitySurface plugin={plugin} capability="cloud-saves">
      <Row plugin={plugin} />
    </CapabilitySurface>
  );
}

function Row({ plugin }: { plugin: StorePlugin }) {
  const [mode, setMode] = useState<CloudMode>('ask');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getCloudSettings())
      .then((s) => live && setMode(s.plugins[plugin.id] ?? 'ask'))
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
      <MonoLabel>cloud saves</MonoLabel>
      <SegRow<CloudMode>
        label={`Cloud saves for ${plugin.name}`}
        value={mode}
        onChange={change}
        options={[{ value: 'off', label: 'Off' }, { value: 'ask', label: 'Ask' }, { value: 'auto', label: 'Auto' }]}
      />
      <p className="text-[11.5px] leading-snug text-white/45">{MODE_HELP[mode]} Each game can override this on its page.</p>
      {error && <p role="alert" className="font-mono text-[11.5px] text-red-300">{error}</p>}
    </div>
  );
}
