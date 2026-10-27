// i18n: converted
import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { Game, SettingField, SettingValue, SettingsSaved, SettingsView, StorePlugin } from '../types';
import { Input } from '@/components/ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { cx, MonoLabel, Panel } from './ui';
import { LauncherApi } from '../effect/runtime';
import { CapabilitySurface } from '../lib/capabilities';
import { effectiveValue } from '../lib/pluginSettings';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';
import { useVariant } from '../lib/theme';

/** A saved setting that the plugin answers with its own words. */
export function SettingError({ message }: { message: string }) {
  const { t } = useTranslation('plugins');
  return <ErrorNotice message={message} fallback={t('plugins:settings.saveFailed')} />;
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
 *  (or the plugin's defaults). A change saves just that field, and the saves of
 *  a scope run one after the other. A failed save shows the backend's values
 *  again, so the form never keeps what the backend refused. */
function SettingsEditor({ scopeKey, load, save, onNotify, framed, emptyText }: {
  scopeKey: string;
  load: () => Effect.Effect<SettingsView, Error>;
  save: (changed: Record<string, SettingValue>) => Effect.Effect<SettingsSaved, Error>;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Wrap the form in a panel of its own (the game page); the plugin view's card already is one. */
  framed?: boolean;
  /** Said where the form would be when the plugin has no fields in this scope (the game settings tab). */
  emptyText?: string;
}) {
  const [view, setView] = useState<SettingsView | null>(null);
  // The values the backend holds, as the form shows them; defaults fill the rest.
  const [values, setValues] = useState<Record<string, SettingValue>>({});
  const [error, setError] = useState<string | null>(null);
  // A save the plugin didn't take: kept, and said so, not reverted.
  const [notice, setNotice] = useState<string | null>(null);
  // Latest functions for the async callbacks: the props change identity each render.
  const loadRef = useRef(load);
  loadRef.current = load;
  const saveRef = useRef(save);
  saveRef.current = save;
  // Saves of this scope, one after the other: each waits for the one before.
  const queue = useRef<Promise<void>>(Promise.resolve());
  const { t } = useTranslation('plugins');
  const atlas = useVariant('shell.navigation') === 'sourcebar';

  // Reads the view and the values the backend holds.
  const reload = (live: () => boolean) => Effect.runPromise(loadRef.current()).then(
    (v) => { if (live()) { setView(v); setValues(v.values); } },
    (e) => { if (live()) setError(coreErrMsg(e)); },
  );

  useEffect(() => {
    let live = true;
    setView(null);
    setError(null);
    setNotice(null);
    void reload(() => live);
    return () => { live = false; };
  }, [scopeKey]);

  const change = (id: string, v: SettingValue) => {
    setValues((prev) => ({ ...prev, [id]: v }));
    setError(null);
    setNotice(null);
    queue.current = queue.current.then(() => Effect.runPromise(saveRef.current({ [id]: v })).then(
      (res) => {
        if (res.applied) onNotify?.(t('plugins:settings.saved'));
        else setNotice(res.message ?? t('plugins:settings.notApplied'));
      },
      (e) => { setError(coreErrMsg(e)); return reload(() => true); },
    ));
  };

  if (error && !view) return <SettingError message={error} />;
  // Loading text in place, never an empty first frame (issue #56): the game
  // settings' plugin tab renders this before the plugin answers.
  if (!view) return <p role="status" className="text-[12.5px] text-text-faint">{t('plugins:settings.loading')}</p>;
  if (view.groups.length === 0) return emptyText ? <p className="text-[12.5px] text-text-muted">{emptyText}</p> : null;
  const body = (
    <div className="space-y-4">
      {view.groups.map((g) => (
        <div key={g.id} className="space-y-2.5">
          <div>
            {atlas ? <h3 className="mt-3.5 text-[13px] font-bold text-text-muted">{g.label}</h3> : <MonoLabel>{g.label}</MonoLabel>}
            {g.description && <p className="mt-1 text-[12px] leading-relaxed text-text-muted">{g.description}</p>}
          </div>
          {g.fields.map((f) => (
            <div key={f.id} className="flex items-center justify-between gap-4">
              <div className="min-w-0">
                <div className={cx('text-[12.5px] text-text', atlas && 'font-semibold')}>{f.label}</div>
                {f.description && <div className={atlas ? 'text-[12px] leading-relaxed text-text-muted' : 'text-hint leading-relaxed text-text-muted'}>{f.description}</div>}
              </div>
              <FieldControl field={f} value={effectiveValue(f, values)} onChange={(v) => change(f.id, v)} />
            </div>
          ))}
        </div>
      ))}
      {notice && <p role="status" className="text-attention">{notice}</p>}
      {error && <SettingError message={error} />}
    </div>
  );
  return framed ? <Panel className={atlas ? 'bg-surface' : undefined}>{atlas ? <h3 className="mb-3 text-[13px] font-bold text-text-muted">{t('plugins:settings.sectionTitle')}</h3> : <MonoLabel className="mb-3">{t('plugins:settings.sectionTitle')}</MonoLabel>}{body}</Panel> : body;
}

/** The plugin-wide settings of a store plugin, in the Plugins view. Shown only
 *  while the plugin is ready and offers `settings`. */
export function PluginSettingsPanel({ plugin, onNotify }: { plugin: StorePlugin; onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void }) {
  return (
    <CapabilitySurface plugin={plugin} capability="settings">
      <SettingsEditor
        scopeKey={`plugin:${plugin.id}`}
        load={() => LauncherApi.getPluginSettings(plugin.id)}
        save={(changed) => LauncherApi.savePluginSettings(plugin.id, changed)}
        onNotify={onNotify}
      />
    </CapabilitySurface>
  );
}

/** The settings of one game, on its page. Only for a game whose features include
 *  `settings`; a game without any scope-game field shows nothing. */
export function GameSettingsPanel({ game, plugin, onNotify, framed = true }: { game: Game; plugin: StorePlugin; onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void; framed?: boolean }) {
  const { t } = useTranslation('plugins');
  return (
    <CapabilitySurface plugin={plugin} capability="settings" game={game}>
      <SettingsEditor
        scopeKey={`game:${game.id}`}
        load={() => LauncherApi.getGameSettingsFields(game)}
        save={(changed) => LauncherApi.saveGameSettingsFields(game, changed)}
        onNotify={onNotify}
        framed={framed}
        emptyText={framed ? undefined : t('plugins:settings.noGameSettings', { plugin: plugin.name, title: game.title })}
      />
    </CapabilitySurface>
  );
}
