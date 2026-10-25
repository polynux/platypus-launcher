// i18n: converted
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { Copy, Trash2, X } from 'lucide-react';
import type { Game, GameSettings, PrefixPreset, PrefixShare, ProtonVersion, RunnerEnv, SettingGroup as SchemaGroup } from '../types';
import { Modal } from './Modal';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel, cx } from './ui';
import { SteamAppIdField } from './SteamAppIdField';
import { LauncherApi } from '../effect/runtime';
import { WINETRICKS_VERBS, envKeyError, envToRows, rowsToEnv, steamLaunchOptions, type EnvRow } from '../lib/settings';
import { RunnerChoices } from './RunnerChoices';

interface Props {
  game: Game;
  runners: ProtonVersion[];
  onClose: () => void;
  onSave: (s: GameSettings) => void;
  onCopied: () => void;
}

interface GeneralProps extends Props {
  /** Reports whether there are unsaved edits, so the owner can keep Esc and click-outside inert. */
  onDirty?: (dirty: boolean) => void;
  /** Inside a tab of the game settings: no title row or close button of its own. */
  embedded?: boolean;
  /** The game's executable changed (or its bit did): the library should reload. */
  onChanged?: () => void;
}

/** Sentinel value of the prefix picker: the game's own prefix (""), since Radix forbids ''. */
const PREFIX_OWN = '__prefix_own__';

/** Sentinel value of the preset picker: no preset chosen (Radix forbids ''). */
const PRESET_NONE = '__preset_none__';

/** The WINEDLLOVERRIDES value a preset composes: the game's own entries first
 *  (kept verbatim, any case), then the DLLs it doesn't name yet — the same
 *  rule the core applies, so the preview matches what Apply writes. */
function composeOverrides(existing: string, dlls: Record<string, string>): string {
  const parts: string[] = [];
  const seen = new Set<string>();
  for (const part of existing.split(';')) {
    const s = part.trim();
    if (!s) continue;
    parts.push(s);
    seen.add(s.split('=')[0].trim().toLowerCase());
  }
  for (const k of Object.keys(dlls).sort()) {
    if (seen.has(k.toLowerCase())) continue;
    parts.push(`${k}=${dlls[k]}`);
    seen.add(k.toLowerCase());
  }
  return parts.join(';');
}

/** A WINEDLLOVERRIDES value parsed back into a preset's dll_overrides map. */
function parseOverrides(value: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const part of value.split(';')) {
    const s = part.trim();
    const i = s.indexOf('=');
    if (i <= 0) continue;
    out[s.slice(0, i).trim()] = s.slice(i + 1).trim();
  }
  return out;
}

/** ds Select with an aria-label on the trigger. Radix forbids '' as an item value, so callers use sentinels. */
function SelectField({ label, value, onChange, children, className }: { label: string; value: string; onChange: (v: string) => void; children: React.ReactNode; className?: string }) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger aria-label={label} className={className}><SelectValue /></SelectTrigger>
      <SelectContent>{children}</SelectContent>
    </Select>
  );
}

/** A named group of the General form: a mono label over its controls. */
function SettingGroup({ label, heading = true, children }: { label: string; heading?: boolean; children: React.ReactNode }) {
  return (
    <section aria-label={label} className="space-y-3.5">
      {heading && <MonoLabel>{label}</MonoLabel>}
      {children}
    </section>
  );
}

/** One shared draft of a game's settings (issue #59): Suggested tweaks and
 *  General's Advanced section edit the same object, so a save from either
 *  tab writes the merged current values and never a stale copy over the
 *  other tab's changes. The dialog owns it; GeneralSettings uses it when
 *  given, else falls back to its own (the standalone dialog). */
export interface GameSettingsDraft {
  draft: GameSettings;
  setDraft: React.Dispatch<React.SetStateAction<GameSettings>>;
  envRows: EnvRow[];
  setEnvRows: React.Dispatch<React.SetStateAction<EnvRow[]>>;
  nextRowId: React.MutableRefObject<number>;
  /** The draft with the valid env rows folded in: what any save writes. */
  current: GameSettings;
  /** The form differs from the last save: Esc / click-outside stay inert. */
  dirty: boolean;
  /** The general form saved: its rows stay, the baseline catches up. */
  markClean: () => void;
  /** A save from the other tab landed: adopt it wholesale and rebase. */
  applySaved: (s: GameSettings) => void;
  /** Drop unsaved edits and return to the last save (atlas footer Discard). */
  discard: () => void;
}

function rowsSig(rows: EnvRow[]): string {
  return JSON.stringify(rows.map((r) => [r.key, r.value]));
}

/** Owns a shared draft, initialized from the game's settings. */
export function useGameSettingsDraft(game: Game): GameSettingsDraft {
  const [draft, setDraft] = useState<GameSettings>(() => game.settings);
  const [envRows, setEnvRows] = useState<EnvRow[]>(() => envToRows(game.settings.env_vars ?? {}));
  const nextRowId = useRef(envRows.length);
  const [initial, setInitial] = useState(() => ({ draft: JSON.stringify(draft), env: rowsSig(envRows) }));
  const validEnv = useMemo(() => rowsToEnv(envRows), [envRows]);
  const current = useMemo(() => ({ ...draft, env_vars: validEnv }), [draft, validEnv]);
  const dirty = JSON.stringify(draft) !== initial.draft || rowsSig(envRows) !== initial.env;
  const markClean = () => setInitial({ draft: JSON.stringify(draft), env: rowsSig(envRows) });
  const applySaved = (s: GameSettings) => {
    const n = s;
    const rows = envToRows(n.env_vars ?? {});
    nextRowId.current = rows.length;
    setDraft(n);
    setEnvRows(rows);
    setInitial({ draft: JSON.stringify(n), env: rowsSig(rows) });
  };
  // Drop unsaved edits: the draft returns to the last save (or the game's
  // settings when nothing was saved yet). The baseline stays, so the form
  // reads clean afterwards.
  const discard = () => {
    try {
      const d = JSON.parse(initial.draft) as GameSettings;
      setDraft(d);
    } catch {
      setDraft(game.settings);
    }
    // Rebuild rows from the baseline draft's env: ids restart, values match
    // the last save.
    try {
      const d = JSON.parse(initial.draft) as GameSettings;
      const rows = envToRows(d.env_vars ?? {});
      nextRowId.current = rows.length;
      setEnvRows(rows);
    } catch {
      const rows = envToRows(game.settings.env_vars ?? {});
      nextRowId.current = rows.length;
      setEnvRows(rows);
    }
  };
  // The game changed under the dialog (the other tab saved and the new
  // settings came back as props, or the library reloaded): a clean form
  // follows them, unsaved edits stay put so no save drops them.
  const prevId = useRef(game.id);
  const prevSig = useRef(JSON.stringify(game.settings));
  const sig = JSON.stringify(game.settings);
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;
  useEffect(() => {
    if (prevId.current !== game.id || sig !== prevSig.current) {
      const idChanged = prevId.current !== game.id;
      prevId.current = game.id;
      prevSig.current = sig;
      if (idChanged || !dirtyRef.current) applySaved(game.settings);
    }
  }, [game.id, sig]); // eslint-disable-line react-hooks/exhaustive-deps
  return { draft, setDraft, envRows, setEnvRows, nextRowId, current, dirty, markClean, applySaved, discard };
}

/** The launcher's own settings for one game (runner, wrappers, env…): the form
 *  of the "General" tab, and of the standalone settings dialog. */
export function GeneralSettings({ game, runners, onClose, onSave, onCopied, onDirty, embedded, onChanged, draftState }: GeneralProps & { draftState?: GameSettingsDraft }) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const fallback = useGameSettingsDraft(game);
  const shared = draftState ?? fallback;
  const { draft, setDraft, envRows, setEnvRows, nextRowId, current, dirty, markClean, applySaved } = shared;
  useEffect(() => { onDirty?.(dirty); }, [dirty]); // eslint-disable-line react-hooks/exhaustive-deps
  const [preview, setPreview] = useState<{ cmd?: string; err?: string }>({});
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));

  // The global Wine prefix folder (issue #37), for the effective-prefix
  // hint below. Unset until loaded; a failure keeps the built-in default.
  const [prefixesFolder, setPrefixesFolder] = useState<string | null>(null);
  const [defaultPrefixes, setDefaultPrefixes] = useState('');
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setPrefixesFolder(s.prefixes_folder ?? ''); setDefaultPrefixes(s.default_prefixes_folder ?? ''); })
      .catch(() => {});
  }, []);
  const effectivePrefix = draft.wine_prefix
    || (prefixesFolder ? `${prefixesFolder}/${game.id}`
      : defaultPrefixes ? `${defaultPrefixes}/${game.id}`
      : `~/.local/share/platypus/prefixes/${game.id}`);

  // Every prefix Platypus knows and who uses it (issue #52): the picker
  // offers them, and the effective prefix's sharers warn below.
  const [prefixUsage, setPrefixUsage] = useState<PrefixShare[]>([]);
  useEffect(() => {
    Effect.runPromise(LauncherApi.prefixUsage())
      .then(setPrefixUsage)
      .catch(() => {});
  }, []);
  const sharedWith = (prefixUsage.find((u) => u.path === effectivePrefix)?.games ?? [])
    .filter((g) => g.id !== game.id)
    .map((g) => g.title);

  const isSteamClient = game.store === 'steam' && !game.executable;
  // A game the store client launches (issue #88): Steam picks the runner and
  // owns the prefix, so the runner/sync/anti-cheat groups never arrive from
  // core, and the hardcoded winetricks block and prefix footer hide here.
  const clientLaunched = !!game.client_launch;
  // A native Linux build runs without Proton: no runner and no Proton-only
  // settings to pick.
  const native = !!game.native;
  // What the executable looks like by content (issue #47): the default of the
  // "Run with" choice. Sideloaded games resolve it again here so the choice
  // shows the detected value even before anything is saved.
  const isCustom = game.store === 'custom';
  const [detected, setDetected] = useState<{ native: boolean; kind: string } | null>(null);
  useEffect(() => {
    if (!isCustom || !game.executable) {
      setDetected(null);
      return;
    }
    let live = true;
    Effect.runPromise(LauncherApi.detectGameExecutable(game.executable)).then(
      (d) => { if (live) setDetected(d); },
      () => { if (live) setDetected(null); },
    );
    return () => { live = false; };
  }, [isCustom, game.executable, game.id]);
  const runMode = draft.run_mode ?? '';
  const effectiveNative = runMode === 'native' ? true : runMode === 'wine' ? false : (detected?.native ?? native);
  const backend = useMemo(() => ({ ...game, settings: current }), [game, current]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!isSteamClient) return;
    let cancelled = false;
    const t = setTimeout(() => {
      Effect.runPromise(LauncherApi.getRunnerEnv({ ...game, settings: current }))
        .then((r) => !cancelled && setRunnerEnv(r))
        .catch(() => {});
    }, 300);
    return () => { cancelled = true; clearTimeout(t); };
  }, [current]); // eslint-disable-line react-hooks/exhaustive-deps

  // Live preview from the real backend, debounced 300ms.
  useEffect(() => {
    let cancelled = false;
    const t = setTimeout(() => {
      Effect.runPromise(LauncherApi.previewCommand({ ...game, settings: current }))
        .then((cmd) => !cancelled && setPreview({ cmd }))
        .catch((e) => !cancelled && setPreview({ err: e instanceof Error ? e.message : String(e) }));
    }, 300);
    return () => { cancelled = true; clearTimeout(t); };
  }, [backend]); // eslint-disable-line react-hooks/exhaustive-deps

  const tricks = draft.winetricks ?? [];
  // The runner's per-game choices, from the schema the core got from runner.options.
  // A game the store client launches gets only the launch-options choices
  // (issue #88): the runner, sync and anti-cheat groups never arrive.
  const [runnerGroups, setRunnerGroups] = useState<SchemaGroup[]>([]);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getRunnerOptions(game)).then(setRunnerGroups, () => setRunnerGroups([]));
  }, []);
  const changeChoice = (key: string, value: boolean | string | number) => set(key as keyof GameSettings, value as never);
  // The env and wrappers the choices add: the Steam launch options spell them out.
  const [runnerEnv, setRunnerEnv] = useState<RunnerEnv>({ env: {}, wrappers: [] });

  const copy = (text: string) => navigator.clipboard.writeText(text).then(onCopied).catch(() => {});

  return (
    <div className="@container">
      {!embedded && (
        <div className="mb-6 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-[20px] font-extrabold tracking-tight">{t('gameSettings:modal.heading')}</h2>
            <div className="font-mono text-hint text-text-muted">{game.title}</div>
          </div>
          <Button variant="neutral" size="icon" onClick={onClose} aria-label={t('common:close')} className="size-8">
            <X className="size-4" />
          </Button>
        </div>
      )}

      {isSteamClient && (
        <div className="mb-6 rounded-2xl bg-info/[0.07] p-4 ring-1 ring-info/25">
          <p className="text-[12.5px] leading-relaxed text-sky-100/85">
            {t('gameSettings:steamNotice')}
          </p>
          <div className="mt-2.5 flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-lg bg-black/35 px-3 py-2 font-mono text-hint text-sky-100/90 select-text">{steamLaunchOptions(backend, runnerEnv)}</code>
            <Button variant="neutral" size="sm" onClick={() => copy(steamLaunchOptions(backend, runnerEnv))}>
              <Copy className="size-3.5" /> {t('gameSettings:steamCopy')}
            </Button>
          </div>
        </div>
      )}

      <div className="grid gap-6 @3xl:grid-cols-[1.5fr_1fr]">
        <div className="min-w-0 space-y-7">
          <SteamAppIdField game={game} />

          {isCustom && (
            <CustomProgram
              game={game}
              detected={detected}
              runMode={runMode}
              onRunMode={(v) => set('run_mode', v)}
              onChanged={onChanged}
            />
          )}

          {/* The runner's own groups carry their titles (runner.options); this section only names the area. */}
          <SettingGroup label={t('gameSettings:groups.runner')} heading={false}>
            {effectiveNative ? (
              <div className="rounded-2xl bg-emerald-300/[0.06] p-4 ring-1 ring-emerald-300/25">
                <p className="text-[12.5px] leading-relaxed text-emerald-100/85">
                  {t('gameSettings:runner.nativeNote')}
                </p>
              </div>
            ) : (
              <>
                <RunnerChoices groups={runnerGroups} ids={['runner']} draft={draft} onChange={changeChoice} />
                {runners.length === 0 && !clientLaunched && <p className="text-[12px] text-text-body">{t('gameSettings:runner.noneInstalled')}</p>}
                <RunnerChoices groups={runnerGroups} ids={['sync', 'proton']} draft={draft} onChange={changeChoice} />
                {!clientLaunched && (
                <div className="space-y-3">
                  <MonoLabel>{t('gameSettings:winetricks.kicker')} <span className="normal-case tracking-normal text-text-muted">{t('gameSettings:winetricks.note')}</span></MonoLabel>
                  <div className="flex flex-wrap gap-1.5">
                    {[...new Set([...WINETRICKS_VERBS, ...(game.fix_info?.winetricks ?? [])])].map((v) => {
                      const on = tricks.includes(v);
                      return (
                        <button key={v} aria-pressed={on} onClick={() => set('winetricks', on ? tricks.filter((x) => x !== v) : [...tricks, v])} className={cx('rounded-lg px-2.5 py-1.5 font-mono text-hint transition', on ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-surface-strong text-text-muted hover:bg-white/[0.1]')}>
                          {on ? '✓ ' : '+ '}{v}
                        </button>
                      );
                    })}
                  </div>
                </div>
                )}
              </>
            )}
          </SettingGroup>

          {!effectiveNative && (
            <SettingGroup label={t('gameSettings:groups.graphics')} heading={false}>
              <RunnerChoices groups={runnerGroups} ids={['graphics']} draft={draft} onChange={changeChoice} />
            </SettingGroup>
          )}

          <SettingGroup label={t('gameSettings:groups.launch')}>
            <RunnerChoices groups={runnerGroups} ids={['wrappers']} draft={draft} onChange={changeChoice} />
            {draft.use_gamescope && <RunnerChoices groups={runnerGroups} ids={['gamescope']} draft={draft} onChange={changeChoice} />}
            {!effectiveNative && <RunnerChoices groups={runnerGroups} ids={['anticheat']} draft={draft} onChange={changeChoice} />}

            <label className="block">
              <MonoLabel className="mb-1.5">{t('gameSettings:launch.argsKicker')}</MonoLabel>
              <Input value={draft.launch_arguments} onChange={(e) => set('launch_arguments', e.target.value)} placeholder={t('gameSettings:launch.argsPlaceholder')} />
            </label>

            {!isSteamClient && (
              <div>
                <MonoLabel className="mb-1.5">{t('gameSettings:launch.workdirKicker')} <span className="normal-case tracking-normal text-text-muted">{t('gameSettings:launch.optionalNote')}</span></MonoLabel>
                <div className="flex items-center gap-2">
                  <Input
                    aria-label={t('gameSettings:launch.workdirAria')}
                    value={draft.working_dir}
                    onChange={(e) => set('working_dir', e.target.value)}
                    placeholder={game.executable ? game.executable.split('/').slice(0, -1).join('/') || '/' : t('gameSettings:launch.workdirPlaceholder')}
                    spellCheck={false}
                    className="min-w-0 flex-1"
                  />
                  <Button
                    variant="neutral"
                    size="sm"
                    onClick={() => {
                      Effect.runPromise(LauncherApi.pickFolder(t('gameSettings:launch.browseWorkdirTitle'), draft.working_dir || game.executable))
                        .then((picked) => { if (picked) set('working_dir', picked); })
                        .catch(() => {});
                    }}
                    className="flex-none"
                  >
                    {t('gameSettings:launch.browse')}
                  </Button>
                </div>
                <p className="mt-1.5 text-[12px] leading-relaxed text-text-body">{t('gameSettings:launch.workdirHint')}</p>
              </div>
            )}

            {!isSteamClient && !effectiveNative && (
              <div>
                <MonoLabel className="mb-1.5">{t('gameSettings:launch.prefixKicker')} <span className="normal-case tracking-normal text-text-muted">{t('gameSettings:launch.optionalNote')}</span></MonoLabel>
                <div className="flex items-center gap-2">
                  <Input
                    aria-label={t('gameSettings:launch.prefixAria')}
                    value={draft.wine_prefix}
                    onChange={(e) => set('wine_prefix', e.target.value)}
                    placeholder={t('gameSettings:launch.prefixPlaceholder')}
                    spellCheck={false}
                    className="min-w-0 flex-1"
                  />
                  <Button
                    variant="neutral"
                    size="sm"
                    onClick={() => {
                      Effect.runPromise(LauncherApi.pickFolder(t('gameSettings:launch.browsePrefixTitle'), draft.wine_prefix || draft.working_dir || game.executable))
                        .then((picked) => { if (picked) set('wine_prefix', picked); })
                        .catch(() => {});
                    }}
                    className="flex-none"
                  >
                    {t('gameSettings:launch.browse')}
                  </Button>
                </div>
                <div className="mt-3">
                  <MonoLabel className="mb-1.5">{t('gameSettings:prefix.picker')}</MonoLabel>
                  <SelectField label={t('gameSettings:prefix.picker')} value={draft.wine_prefix || PREFIX_OWN} onChange={(v) => set('wine_prefix', v === PREFIX_OWN ? '' : v)}>
                    <SelectItem value={PREFIX_OWN}>{t('gameSettings:prefix.pickerDefault')}</SelectItem>
                    {prefixUsage.map((u) => (
                      <SelectItem key={u.path} value={u.path}>{u.path} · {t('gameSettings:prefix.games', { count: u.games.length })}</SelectItem>
                    ))}
                    {/* A typed path no other game uses lists itself, so the closed picker never lies. */}
                    {draft.wine_prefix && !prefixUsage.some((u) => u.path === draft.wine_prefix) && (
                      <SelectItem value={draft.wine_prefix}>{draft.wine_prefix}</SelectItem>
                    )}
                  </SelectField>
                </div>
                <p className="mt-1.5 text-[12px] leading-relaxed text-text-body">
                  {draft.wine_prefix
                    ? t('gameSettings:launch.prefixCustom')
                    : t('gameSettings:launch.prefixDefault')}
                </p>
                {sharedWith.length > 0 && (
                  <div className="mt-3 rounded-2xl bg-accent-strong/[0.06] p-4 ring-1 ring-accent-strong/25">
                    <p className="font-mono text-hint text-amber-200/90">{t('gameSettings:prefix.sharedWith', { games: sharedWith.join(', ') })}</p>
                    <p className="mt-1.5 text-[12.5px] leading-relaxed text-amber-100/85">{t('gameSettings:prefix.sharedWarning')}</p>
                  </div>
                )}
                <PrefixPresets
                  game={game}
                  current={current}
                  dirty={dirty}
                  onApplied={(saved) => applySaved({ ...game.settings, ...saved })}
                />
              </div>
            )}

              <div>
                <MonoLabel className="mb-1.5">{t('gameSettings:launch.envKicker')}</MonoLabel>
                <div className="space-y-1.5">
                  {envRows.map((r, i) => {
                    const err = envKeyError(envRows, i, t);
                    const varFallback = t('gameSettings:launch.varFallback');
                    const upd = (patch: Partial<EnvRow>) => setEnvRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, ...patch } : x)));
                    return (
                      <div key={r.id}>
                        <div className="flex items-center gap-1.5">
                          <Input aria-label={t('gameSettings:launch.varNameAria')} aria-invalid={!!err} value={r.key} onChange={(e) => upd({ key: e.target.value })} placeholder={t('gameSettings:launch.varNamePlaceholder')} spellCheck={false} className={cx('w-[40%] flex-none', err && 'ring-1 ring-orange-300/70')} />
                          <Input aria-label={t('gameSettings:launch.varValueAria', { key: r.key || varFallback })} value={r.value} onChange={(e) => upd({ value: e.target.value })} placeholder={t('gameSettings:launch.varValuePlaceholder')} spellCheck={false} className="min-w-0 flex-1" />
                          <Button variant="ghost" size="icon" aria-label={t('gameSettings:launch.removeVar', { key: r.key || varFallback })} onClick={() => setEnvRows((rs) => rs.filter((x) => x.id !== r.id))} className="flex-none">
                            <X className="size-4" />
                          </Button>
                        </div>
                        {err && <p role="status" className="mt-1 font-mono text-hint text-orange-300">{err}{err.startsWith(t('gameSettings:launch.duplicateKey')) ? '' : ` ${t('gameSettings:launch.notSaved')}`}</p>}
                      </div>
                    );
                  })}
                </div>
                <Button variant="ghost" size="sm" onClick={() => setEnvRows((rs) => [...rs, { id: nextRowId.current++, key: '', value: '' }])} className="mt-2">{t('gameSettings:launch.addVar')}</Button>
              </div>
          </SettingGroup>
        </div>

        <div className="min-w-0 space-y-5">
          <div className="rounded-xl bg-black/35 p-3.5 font-mono text-hint leading-relaxed ring-1 ring-hairline">
            <div className="mb-1 flex items-center justify-between text-text-muted">
              <span>{t('gameSettings:preview.label')}</span>
              {preview.cmd && (
                <Button variant="ghost" size="none" onClick={() => copy(preview.cmd)} className="rounded px-1 py-0.5" aria-label={t('gameSettings:preview.copyLabel')}>
                  <Copy className="size-3" />
                </Button>
              )}
            </div>
            <div className="max-h-56 overflow-y-auto break-all select-text">
              {preview.err ? <div className="text-red-300">{preview.err}</div> : <div className="text-emerald-200/90">{preview.cmd ?? '…'}</div>}
            </div>
          </div>

          <div className="flex gap-2.5">
            <Button variant="neutral" onClick={onClose} className="flex-1">{t('common:cancel')}</Button>
            <Button variant="primary" onClick={() => { onSave(current); markClean(); }} className="flex-1">{t('gameSettings:general.save')}</Button>
          </div>
          {dirty && <div role="status" className="font-mono text-hint text-accent-strong/80">{t('gameSettings:general.unsaved')}</div>}
          {/* The store client owns the prefix (issue #88): no footer naming one. */}
          {!clientLaunched && (
          <div className="font-mono text-[12px] leading-relaxed text-text-muted">
            {effectiveNative && isCustom
              ? t('gameSettings:general.savedNativeNote')
              : <>{t('gameSettings:general.savedNote', { prefix: effectivePrefix })}{draft.wine_prefix ? '' : t('gameSettings:general.defaultSuffix')}</>}
          </div>
          )}
        </div>
      </div>
    </div>
  );
}

/** The presets area of the Wine prefix section (issue #52): pick a preset to
 *  preview and apply it, or save the game's current settings as a user
 *  preset (which can then be deleted; built-ins are read-only). It lives
 *  inside the prefix block, which is hidden for native games and store
 *  client launches — exactly the games core refuses. */
export function PrefixPresets({ game, current, dirty, onApplied }: {
  game: Game;
  /** The form's current settings: what a preview compares the preset with. */
  current: GameSettings;
  /** The form has unsaved edits: applying would write core's answer over
   *  them (the core merges into the saved settings), so Apply waits. */
  dirty: boolean;
  /** A preset applied and saved by core: the form refreshes from it. */
  onApplied: (saved: GameSettings) => void;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [presets, setPresets] = useState<PrefixPreset[]>([]);
  const [chosen, setChosen] = useState('');
  const [saveName, setSaveName] = useState('');
  const [busy, setBusy] = useState(false);
  const [applied, setApplied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // What the game has recorded: the applied preset, its verbs and its
  // Windows version. These ride in settings_extra until a refresh folds
  // core's answer in after an apply.
  const [recorded, setRecorded] = useState<Record<string, unknown>>(() => ({ ...game.settings, ...(game.settings_extra ?? {}) }));

  const load = () => Effect.runPromise(LauncherApi.listPrefixPresets()).then(setPresets, () => setPresets([]));
  useEffect(() => { load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const preset = presets.find((p) => p.id === chosen);
  const builtins = presets.filter((p) => p.builtin);
  const users = presets.filter((p) => !p.builtin);
  const fail = (e: unknown) => { setBusy(false); setError(e instanceof Error ? e.message : String(e)); };

  // The preview, computed the way the core merges: additions are the preset
  // env keys the game doesn't set, the rest it keeps (skipped), and the
  // composed overrides keep the game's own DLL entries.
  const env = current.env_vars ?? {};
  const additions = Object.entries(preset?.env ?? {}).filter(([k]) => !(k in env)).sort(([a], [b]) => a.localeCompare(b));
  const skipped = Object.keys(preset?.env ?? {}).filter((k) => k in env).sort();
  const overrideDLLs = preset?.dll_overrides ?? {};
  const overrides = Object.keys(overrideDLLs).length > 0 ? composeOverrides(env.WINEDLLOVERRIDES ?? '', overrideDLLs) : '';
  const verbs = preset?.winetricks ?? [];
  const version = (typeof recorded.windows_version === 'string' && recorded.windows_version)
    || preset?.windows_version || 'win10';
  const alreadyApplied = !!preset && (applied || recorded.prefix_preset === preset.id);

  const applyPreset = () => {
    if (!preset || busy || dirty) return;
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.applyPrefixPreset(game.id, preset.id)).then(() => {
      setApplied(true);
      setBusy(false);
      // Refresh what the form shows from core's own answer (issue #52).
      Effect.runPromise(LauncherApi.getGameSettings()).then((all) => {
        const saved = all[game.id];
        if (!saved) return;
        setRecorded({ ...game.settings, ...(game.settings_extra ?? {}), ...saved });
        onApplied(saved);
      }, () => {});
    }, fail);
  };

  const deletePreset = () => {
    if (!preset || preset.builtin || busy) return;
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.deletePrefixPreset(preset.id)).then(() => {
      setBusy(false);
      setChosen('');
      setApplied(false);
      load();
    }, fail);
  };

  const savePreset = () => {
    const name = saveName.trim();
    if (!name || busy) return;
    const envVars = { ...env };
    const composed = envVars.WINEDLLOVERRIDES ?? '';
    delete envVars.WINEDLLOVERRIDES;
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.savePrefixPreset({
      id: '', builtin: false, name,
      env: envVars,
      dll_overrides: parseOverrides(composed),
      winetricks: [...new Set([
        ...(Array.isArray(recorded.winetricks) ? recorded.winetricks.filter((v): v is string => typeof v === 'string') : []),
        ...(current.winetricks ?? []),
      ])],
      windows_version: typeof recorded.windows_version === 'string' ? recorded.windows_version : '',
    })).then(() => {
      setBusy(false);
      setSaveName('');
      load();
    }, fail);
  };

  return (
    <div className="mt-3">
      <MonoLabel className="mb-1.5">{t('gameSettings:prefixPresets.picker')}</MonoLabel>
      <div className="flex items-center gap-2">
        <SelectField
          label={t('gameSettings:prefixPresets.picker')}
          value={chosen || PRESET_NONE}
          onChange={(v) => { setChosen(v === PRESET_NONE ? '' : v); setApplied(false); setError(null); }}
        >
          <SelectItem value={PRESET_NONE}>{t('gameSettings:prefixPresets.none')}</SelectItem>
          {builtins.length > 0 && (
            <SelectGroup>
              <SelectLabel>{t('gameSettings:prefixPresets.builtinGroup')}</SelectLabel>
              {builtins.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}
            </SelectGroup>
          )}
          {users.length > 0 && (
            <SelectGroup>
              <SelectLabel>{t('gameSettings:prefixPresets.userGroup')}</SelectLabel>
              {users.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}
            </SelectGroup>
          )}
        </SelectField>
        {preset && !preset.builtin && (
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('gameSettings:prefixPresets.delete', { name: preset.name })}
            disabled={busy}
            onClick={deletePreset}
            className="flex-none"
          >
            <Trash2 className="size-4" />
          </Button>
        )}
      </div>
      {preset && (
        <div className="mt-2.5 rounded-xl bg-black/35 p-3.5 font-mono text-hint leading-relaxed ring-1 ring-hairline">
          <div className="mb-1 text-text-muted">{t('gameSettings:prefixPresets.previewLabel')}</div>
          {additions.map(([k, v]) => <div key={k} className="break-all text-amber-200/80">{k}={v}</div>)}
          {overrides && <div className="break-all text-amber-200/80">{t('gameSettings:prefixPresets.overrides', { value: overrides })}</div>}
          {skipped.length > 0 && <div className="text-text-muted">{t('gameSettings:prefixPresets.skipped', { keys: skipped.join(', ') })}</div>}
          <div className="mt-2 text-text-muted">{t('gameSettings:prefixPresets.storedHeading')}</div>
          {verbs.length > 0 && <div className="text-text-body">{t('gameSettings:prefixPresets.storedVerbs', { verbs: verbs.join(' · ') })}</div>}
          <div className="text-text-body">{t('gameSettings:prefixPresets.storedVersion', { version })}</div>
          <div className="mt-2 flex items-center gap-2">
            <Button variant="primary" size="sm" disabled={busy || dirty} onClick={applyPreset}>{t('gameSettings:prefixPresets.apply')}</Button>
            {alreadyApplied && <span className="text-[12px] text-emerald-200/85">{t('gameSettings:prefixPresets.alreadyApplied')}</span>}
          </div>
          {/* The core merges into the saved settings: unsaved edits would be
              replaced by the refresh after applying, so save them first. */}
          {dirty && <div className="mt-1 text-[12px] text-text-muted">{t('gameSettings:prefixPresets.saveFirst')}</div>}
        </div>
      )}
      <div className="mt-2.5 flex items-center gap-2">
        <Input
          aria-label={t('gameSettings:prefixPresets.saveName')}
          value={saveName}
          onChange={(e) => setSaveName(e.target.value)}
          placeholder={t('gameSettings:prefixPresets.saveNamePlaceholder')}
          spellCheck={false}
          className="min-w-0 flex-1"
        />
        <Button variant="neutral" size="sm" disabled={!saveName.trim() || busy} onClick={savePreset} className="flex-none">
          {t('gameSettings:prefixPresets.save')}
        </Button>
      </div>
      {error && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{error}</p>}
    </div>
  );
}

/** A sideloaded game's program (issue #47): its executable, how it runs
 *  (detected by content, overridable per game), and the execute bit. */
export function CustomProgram({ game, detected, runMode, onRunMode, onChanged }: {
  game: Game;
  detected: { native: boolean; kind: string } | null;
  runMode: string;
  onRunMode: (v: string) => void;
  onChanged?: () => void;
}) {
  const { t } = useTranslation(['gameSettings', 'common']);
  const [exe, setExe] = useState(game.executable);
  const [savingExe, setSavingExe] = useState(false);
  const [exeError, setExeError] = useState<string | null>(null);
  const [execOk, setExecOk] = useState<boolean | null>(null);
  const [makingExec, setMakingExec] = useState(false);
  const [execError, setExecError] = useState<string | null>(null);
  useEffect(() => { setExe(game.executable); }, [game.id, game.executable]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    let live = true;
    setExecOk(null);
    setExecError(null);
    Effect.runPromise(LauncherApi.isGameExecutable(game.id)).then(
      (ok) => { if (live) setExecOk(ok); },
      () => { if (live) setExecOk(null); },
    );
    return () => { live = false; };
  }, [game.id]);
  const effectiveNative = runMode === 'native' ? true : runMode === 'wine' ? false : (detected?.native ?? game.native);
  const exeDirty = exe.trim() !== game.executable;
  const saveExe = () => {
    const next = exe.trim();
    if (!next || !exeDirty || savingExe) return;
    setSavingExe(true);
    setExeError(null);
    Effect.runPromise(LauncherApi.updateCustomGameExecutable(game.id, next)).then(
      () => { setSavingExe(false); onChanged?.(); },
      (e) => { setSavingExe(false); setExeError(e instanceof Error ? e.message : String(e)); },
    );
  };
  const makeExec = () => {
    if (makingExec) return;
    setMakingExec(true);
    setExecError(null);
    Effect.runPromise(LauncherApi.makeGameExecutable(game.id)).then(
      () => { setMakingExec(false); setExecOk(true); onChanged?.(); },
      (e) => { setMakingExec(false); setExecError(e instanceof Error ? e.message : String(e)); },
    );
  };
  return (
    <SettingGroup label={t('gameSettings:program.kicker')}>
      <div>
        <MonoLabel className="mb-1.5">{t('gameSettings:program.exeKicker')}</MonoLabel>
        <div className="flex items-center gap-2">
          <Input
            aria-label={t('gameSettings:program.exeAria')}
            value={exe}
            onChange={(e) => { setExe(e.target.value); setExeError(null); }}
            spellCheck={false}
            className="min-w-0 flex-1"
          />
          <Button
            variant="neutral"
            size="sm"
            onClick={() => {
              Effect.runPromise(LauncherApi.pickGameExecutable(exe.trim() || game.executable))
                .then((picked) => { if (picked) { setExe(picked); setExeError(null); } })
                .catch(() => {});
            }}
            className="flex-none"
          >
            {t('gameSettings:program.browse')}
          </Button>
          <Button variant="primary" size="sm" disabled={!exeDirty || savingExe} onClick={saveExe} className="flex-none">
            {t('gameSettings:program.saveExe')}
          </Button>
        </div>
        {detected && (
          <p className="mt-1.5 text-[12px] leading-relaxed text-text-body">
            {detected.native ? t('gameSettings:program.detectedNative') : t('gameSettings:program.detectedWine')}
          </p>
        )}
        {exeError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{exeError}</p>}
      </div>
      <div>
        <MonoLabel className="mb-1.5">{t('gameSettings:program.runKicker')}</MonoLabel>
        <SelectField label={t('gameSettings:program.runKicker')} value={runMode || 'auto'} onChange={(v) => onRunMode(v === 'auto' ? '' : v)}>
          <SelectItem value="auto">{detected == null ? t('gameSettings:program.runAutoUnknown') : detected.native ? t('gameSettings:program.runAutoNative') : t('gameSettings:program.runAutoWine')}</SelectItem>
          <SelectItem value="native">{t('gameSettings:program.runNative')}</SelectItem>
          <SelectItem value="wine">{t('gameSettings:program.runWine')}</SelectItem>
        </SelectField>
        <p className="mt-1.5 text-[12px] leading-relaxed text-text-body">{t('gameSettings:program.runHelp')}</p>
      </div>
      {effectiveNative && execOk === false && (
        <div className="rounded-2xl bg-accent-strong/[0.06] p-4 ring-1 ring-accent-strong/25">
          <p className="text-[12.5px] leading-relaxed text-amber-100/85">{t('gameSettings:program.notExecutable')}</p>
          <div className="mt-2.5">
            <Button variant="primary" size="sm" disabled={makingExec} onClick={makeExec}>
              {makingExec ? t('gameSettings:program.makingExecutable') : t('gameSettings:program.makeExecutable')}
            </Button>
          </div>
          {execError && <p role="alert" className="mt-1.5 font-mono text-hint text-orange-300">{execError}</p>}
        </div>
      )}
    </SettingGroup>
  );
}

/** The standalone dialog around the general settings (the game page's own tab
 *  shell uses GeneralSettings directly). */
export function SettingsModal(p: Props) {
  const { t } = useTranslation('gameSettings');
  const [dirty, setDirty] = useState(false);
  return (
    <Modal label={t('gameSettings:modal.title', { title: p.game.title })} onClose={p.onClose} dismissable={!dirty} size="xl">
      <GeneralSettings {...p} onDirty={setDirty} />
    </Modal>
  );
}
