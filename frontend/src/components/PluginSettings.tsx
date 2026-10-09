import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import type { Game, SettingField, SettingValue, SettingsView, StorePlugin } from '../types';
import { Input } from '@/components/ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { MonoLabel, Panel } from './ui';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { effectiveValue, formValues } from '../lib/pluginSettings';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** A saved setting that the plugin answers with its own words. */
export function SettingError({ message }: { message: string }) {
  return <p role="alert" className="font-mono text-[11.5px] text-red-300">{message}</p>;
}

/** One field of the form. Text and number commit when the user leaves the box
 *  or presses Enter, so a save doesn't go out with every key. */
function FieldControl({ field, value, onChange }: { field: SettingField; value: SettingValue; onChange: (v: SettingValue) => void }) {
  if (field.kind === 'bool') {
    return <Switch aria-label={field.label} checked={value === true} onCheckedChange={(v) => onChange(v)} />;
  }
  if (field.kind === 'select') {
    return (
      <Select value={String(value)} onValueChange={(v) => onChange(v)}>
        <SelectTrigger aria-label={field.label} className="w-44">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(field.options ?? []).map((o) => <SelectItem key={o.id} value={o.id}>{o.label}</SelectItem>)}
        </SelectContent>
      </Select>
    );
  }
  return <DraftInput field={field} value={value} onCommit={onChange} />;
}

function DraftInput({ field, value, onCommit }: { field: SettingField; value: SettingValue; onCommit: (v: SettingValue) => void }) {
  const [draft, setDraft] = useState(String(value));
  useEffect(() => setDraft(String(value)), [value]);
  const commit = () => {
    if (field.kind === 'number') {
      const n = Number(draft);
      if (draft.trim() === '' || !Number.isFinite(n)) return setDraft(String(value));
      if (n !== value) onCommit(n);
      return;
    }
    if (draft !== value) onCommit(draft);
  };
  return (
    <Input
      aria-label={field.label}
      type={field.kind === 'number' ? 'number' : 'text'}
      min={field.min}
      max={field.max}
      placeholder={field.placeholder}
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => { if (e.key === 'Enter') commit(); }}
      className="w-44"
    />
  );
}

/** The form of one scope: groups with their fields, the values the user chose
 *  (or the plugin's defaults). Every change saves the whole scope. */
function SettingsEditor({ scopeKey, load, save, onNotify, framed }: {
  scopeKey: string;
  load: () => Effect.Effect<SettingsView, Error>;
  save: (values: Record<string, SettingValue>) => Effect.Effect<void, Error>;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Wrap the form in a panel of its own (the game page); the plugin view's card already is one. */
  framed?: boolean;
}) {
  const [view, setView] = useState<SettingsView | null>(null);
  const [values, setValues] = useState<Record<string, SettingValue>>({});
  const [error, setError] = useState<string | null>(null);
  // Latest functions for the async callbacks: the props change identity each render.
  const loadRef = useRef(load);
  loadRef.current = load;
  const saveRef = useRef(save);
  saveRef.current = save;

  useEffect(() => {
    let live = true;
    setView(null);
    setError(null);
    Effect.runPromise(loadRef.current()).then(
      (v) => { if (live) { setView(v); setValues(formValues(v)); } },
      (e) => { if (live) setError(errMsg(e)); },
    );
    return () => { live = false; };
  }, [scopeKey]);

  // A refused value is put back: the form shows what the backend holds.
  const change = (id: string, v: SettingValue) => {
    const prev = values;
    const next = { ...values, [id]: v };
    setValues(next);
    setError(null);
    Effect.runPromise(saveRef.current(next)).then(
      () => onNotify?.('Setting saved'),
      (e) => { setValues(prev); setError(errMsg(e)); },
    );
  };

  if (error && !view) return <SettingError message={error} />;
  if (!view || view.groups.length === 0) return null;
  const body = (
    <div className="space-y-4">
      {view.groups.map((g) => (
        <div key={g.id} className="space-y-2.5">
          <div>
            <MonoLabel>{g.label}</MonoLabel>
            {g.description && <p className="mt-1 text-[12px] leading-relaxed text-white/50">{g.description}</p>}
          </div>
          {g.fields.map((f) => (
            <div key={f.id} className="flex items-center justify-between gap-4">
              <div className="min-w-0">
                <div className="text-[12.5px] text-white/85">{f.label}</div>
                {f.description && <div className="text-[11.5px] leading-relaxed text-white/45">{f.description}</div>}
              </div>
              <FieldControl field={f} value={effectiveValue(f, values)} onChange={(v) => change(f.id, v)} />
            </div>
          ))}
        </div>
      ))}
      {error && <SettingError message={error} />}
    </div>
  );
  return framed ? <Panel className="bg-white/[0.05]"><MonoLabel className="mb-3">settings</MonoLabel>{body}</Panel> : body;
}

/** The plugin-wide settings of a store plugin, in the Plugins view. Shown only
 *  while the plugin is ready and offers `settings`. */
export function PluginSettingsPanel({ plugin, onNotify }: { plugin: StorePlugin; onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void }) {
  return (
    <CapabilitySurface plugin={plugin} capability="settings">
      <SettingsEditor
        scopeKey={`plugin:${plugin.id}`}
        load={() => LauncherApi.getPluginSettings(plugin.id)}
        save={(values) => LauncherApi.savePluginSettings(plugin.id, values)}
        onNotify={onNotify}
      />
    </CapabilitySurface>
  );
}

/** The settings of one game, on its page. Only for a game whose features include
 *  `settings`; a game without any scope-game field shows nothing. */
export function GameSettingsPanel({ game, plugin, onNotify }: { game: Game; plugin: StorePlugin; onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void }) {
  return (
    <CapabilitySurface plugin={plugin} capability="settings" game={game}>
      <SettingsEditor
        scopeKey={`game:${game.id}`}
        load={() => LauncherApi.getGameSettingsFields(game)}
        save={(values) => LauncherApi.saveGameSettingsFields(game, values)}
        onNotify={onNotify}
        framed
      />
    </CapabilitySurface>
  );
}
