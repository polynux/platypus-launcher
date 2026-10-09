import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Effect } from 'effect';
import { Copy, X } from 'lucide-react';
import type { Game, GameSettings, ProtonVersion } from '../types';
import { Modal } from './Modal';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Button } from '@/components/ds/button';
import { Switch } from '@/components/ds/switch';
import { Input } from '@/components/ds/input';
import { NumberField } from '@/components/ds/number-field';
import { MonoLabel, SegRow, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import {
  DX_LAYERS, GS_FILTERS, GS_RATES, GS_RESOLUTIONS, GS_TOGGLES, HUD_PRESETS, PROTON_FLAGS, SYNC_MODES, UPSCALERS, WINETRICKS_VERBS,
  composeGamescopeArgs, envKeyError, envToRows, rowsToEnv, type EnvRow, normalizeSettings, parseGamescopeArgs, rateChoice, resolutionChoice, steamLaunchOptions, toBackendGame,
} from '../lib/settings';

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
}

/** Physical pixels of the display the window is on. */
function displayResolution(): { w: number; h: number } | undefined {
  if (typeof window === 'undefined' || !window.screen?.width) return undefined;
  const r = window.devicePixelRatio || 1;
  return { w: Math.round(window.screen.width * r), h: Math.round(window.screen.height * r) };
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

/** On/off setting: label (+ optional mono hint) on the left, ds Switch on the right; the whole row toggles. */
function SwitchRow({ label, hint, title, checked, onChange }: { label: string; hint?: string; title?: string; checked: boolean; onChange: (v: boolean) => void }) {
  const id = React.useId();
  return (
    <label htmlFor={id} title={title} className={cx('flex cursor-pointer items-center justify-between gap-3 rounded-xl px-3.5 py-2.5 transition', checked ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}>
      <span className="min-w-0">
        <span className="block font-mono text-[11.5px] font-bold text-white/85">{label}</span>
        {hint && <span className="block truncate font-mono text-[11px] text-white/40">{hint}</span>}
      </span>
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
    </label>
  );
}

function runnerKind(r: ProtonVersion): string {
  if (r.is_system) return 'system';
  if (r.is_custom) return 'custom · compatibilitytools.d';
  return 'steam';
}

/** The launcher's own settings for one game (runner, wrappers, env…): the form
 *  of the "General" tab, and of the standalone settings dialog. */
export function GeneralSettings({ game, runners, onClose, onSave, onCopied, onDirty, embedded }: GeneralProps) {
  const [draft, setDraft] = useState<GameSettings>(() => normalizeSettings(game.settings));
  const [envRows, setEnvRows] = useState<EnvRow[]>(() => envToRows(normalizeSettings(game.settings).env_vars));
  const nextRowId = useRef(envRows.length);
  // Dirty = draft or env rows differ from what was loaded: Esc / click-outside then stay inert.
  const envSig = JSON.stringify(envRows.map((r) => [r.key, r.value]));
  const [initial, setInitial] = useState({ draft: JSON.stringify(draft), env: envSig });
  const dirty = JSON.stringify(draft) !== initial.draft || envSig !== initial.env;
  useEffect(() => { onDirty?.(dirty); }, [dirty]); // eslint-disable-line react-hooks/exhaustive-deps
  const [preview, setPreview] = useState<{ cmd?: string; err?: string }>({});
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));

  const isSteamClient = game.store === 'steam' && !game.executable;
  // A native Linux build runs without Proton: no runner and no Proton-only
  // settings to pick.
  const native = !!game.native;
  const validEnv = useMemo(() => rowsToEnv(envRows), [envRows]);
  const current = { ...draft, env_vars: validEnv };
  const backend = useMemo(() => toBackendGame({ ...game, settings: current }), [game, draft, validEnv]); // eslint-disable-line react-hooks/exhaustive-deps

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

  const envLines = Object.entries(backend.settings.env_vars).sort(([a], [b]) => a.localeCompare(b));
  const flags = draft.proton_flags ?? [];
  const tricks = draft.winetricks ?? [];
  const selectedRunner = draft.runner_id;
  const autoPick = runners.find((r) => r.is_default);
  const isAuto = !selectedRunner || selectedRunner === 'auto';
  const chosenRunner = isAuto ? undefined : runners.find((r) => r.id === selectedRunner);
  const runnerMissing = !isAuto && !chosenRunner;
  const runnerValue = isAuto ? 'auto' : selectedRunner;
  const runnerGroups = (['steam', 'custom', 'system'] as const)
    .map((k) => [k, runners.filter((r) => (r.is_system ? 'system' : r.is_custom ? 'custom' : 'steam') === k)] as const)
    .filter(([, l]) => l.length > 0);

  const display = useMemo(displayResolution, []);
  const [resCustom, setResCustom] = useState(false);
  const [rateCustom, setRateCustom] = useState(false);
  const resChoice = resCustom ? 'custom' : resolutionChoice(draft.gamescope_w, draft.gamescope_h, display);
  const rateSel = rateCustom ? 'custom' : rateChoice(draft.gamescope_r);
  const gs = parseGamescopeArgs(draft.gamescope_args);
  const [gsExtra, setGsExtra] = useState(() => parseGamescopeArgs(draft.gamescope_args).extra);
  // Toggling re-derives from the saved string, so the free-text field never keeps a flag a toggle now owns.
  const setGs = (patch: Partial<typeof gs>) => {
    const next = { ...gs, ...patch };
    setGsExtra(next.extra);
    set('gamescope_args', composeGamescopeArgs(next));
  };

  const copy = (text: string) => navigator.clipboard.writeText(text).then(onCopied).catch(() => {});

  return (
    <div className="@container">
      {!embedded && (
        <div className="mb-6 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-[20px] font-extrabold tracking-tight">Launcher settings</h2>
            <div className="font-mono text-[11.5px] text-white/45">{game.title}</div>
          </div>
          <Button variant="neutral" size="icon" onClick={onClose} aria-label="Close" className="size-8">
            <X className="size-4" />
          </Button>
        </div>
      )}

      {isSteamClient && (
        <div className="mb-6 rounded-2xl bg-sky-300/[0.07] p-4 ring-1 ring-sky-300/25">
          <p className="text-[12.5px] leading-relaxed text-sky-100/85">
            Steam games start through the Steam client, which picks the Proton version and ignores Hydra's env vars.
            To apply these settings, paste this into Steam → Properties → Launch options:
          </p>
          <div className="mt-2.5 flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-lg bg-black/35 px-3 py-2 font-mono text-[11.5px] text-sky-100/90 select-text">{steamLaunchOptions(backend)}</code>
            <Button variant="neutral" size="sm" onClick={() => copy(steamLaunchOptions(backend))}>
              <Copy className="size-3.5" /> Copy
            </Button>
          </div>
        </div>
      )}

      <div className="grid gap-6 @3xl:grid-cols-[1.5fr_1fr]">
        <div className="min-w-0 space-y-6">
          {native && (
            <div className="rounded-2xl bg-emerald-300/[0.06] p-4 ring-1 ring-emerald-300/25">
              <p className="text-[12.5px] leading-relaxed text-emerald-100/85">
                This game ships a native Linux build from the store. It runs directly, without Proton —
                runner, DX layer and Proton flags don't apply.
              </p>
            </div>
          )}
          {!native && (
          <div>
            <MonoLabel className="mb-2">runner</MonoLabel>
            <SelectField label="Runner" value={runnerValue} onChange={(v) => set('runner_id', v)}>
              <SelectItem value="auto">{autoPick ? `Automatic (uses ${autoPick.name})` : 'Automatic (no runner installed)'}</SelectItem>
              {runnerMissing && <SelectItem value={selectedRunner}>{selectedRunner} (missing)</SelectItem>}
              {runnerGroups.map(([kind, list]) => (
                <SelectGroup key={kind}>
                  <SelectLabel>{kind}</SelectLabel>
                  {list.map((r) => <SelectItem key={r.id} value={r.id}>{r.name}{r.is_default ? ' (default)' : ''}</SelectItem>)}
                </SelectGroup>
              ))}
            </SelectField>
            <div className={cx('mt-1.5 truncate font-mono text-[11px]', runnerMissing ? 'text-orange-300' : 'text-white/45')}>
              {runnerMissing
                ? 'Saved runner is not installed. Pick another or the launch falls back to an error.'
                : chosenRunner
                  ? `${runnerKind(chosenRunner)} · ${chosenRunner.path}`
                  : autoPick ? 'newest GE-Proton, then Experimental' : 'no runner installed'}
            </div>
            {runners.length === 0 && <p className="mt-1 text-[12px] text-white/50">No Proton or Wine runners detected.</p>}
          </div>
          )}

          {!native && (
          <div className="grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <div><MonoLabel className="mb-1.5">dx layer</MonoLabel><SegRow label="DX layer" options={DX_LAYERS} value={draft.dx_layer!} onChange={(v) => set('dx_layer', v)} /></div>
            <div><MonoLabel className="mb-1.5">sync</MonoLabel><SegRow label="Sync" options={SYNC_MODES} value={draft.sync_mode === 'default' ? 'fsync' : draft.sync_mode!} onChange={(v) => set('sync_mode', v)} /></div>
            <div><MonoLabel className="mb-1.5">upscaler</MonoLabel><SegRow label="Upscaler" options={UPSCALERS} value={draft.upscaler!} onChange={(v) => set('upscaler', v)} /></div>
            <div><MonoLabel className="mb-1.5">mangohud</MonoLabel><SegRow label="MangoHud" options={HUD_PRESETS} value={draft.mangohud_preset!} onChange={(v) => set('mangohud_preset', v)} /></div>
            <div><MonoLabel className="mb-1.5">dxvk shader cache</MonoLabel><SegRow label="Shader cache" options={[{ value: 'on', label: 'on' }, { value: 'off', label: 'off', title: 'DXVK_STATE_CACHE=0' }]} value={draft.shader_cache!} onChange={(v) => set('shader_cache', v)} /></div>
          </div>
          )}

          <div>
            <MonoLabel className="mb-2">pipeline</MonoLabel>
            <div className="grid gap-1.5 sm:grid-cols-2">
              {([
                ['use_gamemode', 'gamemode', 'Wrap with gamemoderun'],
                ['use_gamescope', 'gamescope', 'Run inside a gamescope session'],
                ...(!native
                  ? ([
                      ['eac_runtime', 'eac runtime', 'Bind the Proton EasyAntiCheat runtime'],
                      ['battleye_runtime', 'battleye runtime', 'Bind the Proton BattlEye runtime'],
                    ] as const)
                  : []),
              ] as const).map(([k, label, title]) => (
                <SwitchRow key={k} label={label} title={title} checked={!!draft[k]} onChange={(v) => set(k, v)} />
              ))}
            </div>
            {draft.use_gamescope && (
              <div className="mt-2.5 space-y-3 rounded-xl bg-white/[0.03] p-3.5 ring-1 ring-white/[0.07]">
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <MonoLabel className="mb-1.5">resolution</MonoLabel>
                    <SelectField
                      label="Gamescope resolution"
                      value={resChoice || 'default'}
                      onChange={(v) => {
                        if (v === 'custom') return setResCustom(true);
                        setResCustom(false);
                        const [w, h] = v !== 'default' ? v.split('x').map(Number) : [0, 0];
                        setDraft((d) => ({ ...d, gamescope_w: w, gamescope_h: h }));
                      }}
                    >
                      <SelectItem value="default">Game default</SelectItem>
                      {display && <SelectItem value={`${display.w}x${display.h}`}>{display.w}×{display.h} (this display)</SelectItem>}
                      {GS_RESOLUTIONS.filter((r) => !(display && r.w === display.w && r.h === display.h)).map((r) => (
                        <SelectItem key={`${r.w}x${r.h}`} value={`${r.w}x${r.h}`}>{r.w}×{r.h}</SelectItem>
                      ))}
                      <SelectItem value="custom">{resChoice === 'custom' && draft.gamescope_w + draft.gamescope_h > 0 ? `Custom… (${draft.gamescope_w || '?'}×${draft.gamescope_h || '?'})` : 'Custom…'}</SelectItem>
                    </SelectField>
                    {resChoice === 'custom' && (
                      <div className="mt-2 flex items-center gap-2">
                        <NumberField aria-label="gamescope width" min={0} step={10} hideSteppers value={draft.gamescope_w || undefined} onChange={(v) => set('gamescope_w', v ?? 0)} placeholder="width" className="min-w-0 flex-1" />
                        <span className="text-white/40">×</span>
                        <NumberField aria-label="gamescope height" min={0} step={10} hideSteppers value={draft.gamescope_h || undefined} onChange={(v) => set('gamescope_h', v ?? 0)} placeholder="height" className="min-w-0 flex-1" />
                      </div>
                    )}
                  </div>
                  <div>
                    <MonoLabel className="mb-1.5">frame rate</MonoLabel>
                    <SelectField
                      label="Gamescope frame rate"
                      value={rateSel || 'default'}
                      onChange={(v) => {
                        if (v === 'custom') return setRateCustom(true);
                        setRateCustom(false);
                        set('gamescope_r', v !== 'default' ? Number(v) : 0);
                      }}
                    >
                      <SelectItem value="default">Game default</SelectItem>
                      {GS_RATES.map((r) => <SelectItem key={r} value={String(r)}>{r} Hz</SelectItem>)}
                      <SelectItem value="custom">{rateSel === 'custom' && draft.gamescope_r ? `Custom… (${draft.gamescope_r} Hz)` : 'Custom…'}</SelectItem>
                    </SelectField>
                    {rateSel === 'custom' && (
                      <NumberField aria-label="gamescope refresh" min={0} hideSteppers value={draft.gamescope_r || undefined} onChange={(v) => set('gamescope_r', v ?? 0)} placeholder="Hz" className="mt-2" />
                    )}
                  </div>
                </div>
                <div className="grid gap-1.5 sm:grid-cols-2">
                  {GS_TOGGLES.map((t) => (
                    <SwitchRow key={t.key} label={t.label} hint={t.flag} title={t.title} checked={gs.toggles[t.key]} onChange={(v) => setGs({ toggles: { ...gs.toggles, [t.key]: v } })} />
                  ))}
                </div>
                <div>
                  <MonoLabel className="mb-1.5">upscale filter</MonoLabel>
                  <SegRow label="Gamescope upscale filter" options={GS_FILTERS} value={gs.filter} onChange={(v) => setGs({ filter: v })} />
                </div>
                <label className="block">
                  <MonoLabel className="mb-1.5">extra gamescope args</MonoLabel>
                  <Input aria-label="gamescope args" value={gsExtra} onChange={(e) => { setGsExtra(e.target.value); set('gamescope_args', composeGamescopeArgs({ ...gs, extra: e.target.value })); }} placeholder="--mangoapp" />
                </label>
              </div>
            )}
          </div>

          <label className="block">
            <MonoLabel className="mb-1.5">launch arguments</MonoLabel>
            <Input value={draft.launch_arguments} onChange={(e) => set('launch_arguments', e.target.value)} placeholder="-novid -fullscreen" />
          </label>

          <div>
            <MonoLabel className="mb-1.5">extra env vars</MonoLabel>
            <div className="space-y-1.5">
              {envRows.map((r, i) => {
                const err = envKeyError(envRows, i);
                const upd = (patch: Partial<EnvRow>) => setEnvRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, ...patch } : x)));
                return (
                  <div key={r.id}>
                    <div className="flex items-center gap-1.5">
                      <Input aria-label="Variable name" aria-invalid={!!err} value={r.key} onChange={(e) => upd({ key: e.target.value })} placeholder="KEY" spellCheck={false} className={cx('w-[40%] flex-none', err && 'ring-1 ring-orange-300/70')} />
                      <Input aria-label={`Value of ${r.key || 'variable'}`} value={r.value} onChange={(e) => upd({ value: e.target.value })} placeholder="value" spellCheck={false} className="min-w-0 flex-1" />
                      <Button variant="ghost" size="icon" aria-label={`Remove ${r.key || 'variable'}`} onClick={() => setEnvRows((rs) => rs.filter((x) => x.id !== r.id))} className="flex-none">
                        <X className="size-4" />
                      </Button>
                    </div>
                    {err && <p role="status" className="mt-1 font-mono text-[11.5px] text-orange-300">{err}{err.startsWith('Duplicate') ? '' : ' (not saved)'}</p>}
                  </div>
                );
              })}
            </div>
            <Button variant="ghost" size="sm" onClick={() => setEnvRows((rs) => [...rs, { id: nextRowId.current++, key: '', value: '' }])} className="mt-2">+ Add variable</Button>
          </div>
        </div>

        <div className="min-w-0 space-y-5">
          {!native && (
          <div>
            <MonoLabel className="mb-2">proton flags</MonoLabel>
            <div className="space-y-1.5">
              {PROTON_FLAGS.map((f) => {
                const on = flags.includes(f.key);
                return (
                  <SwitchRow
                    key={f.key}
                    label={f.key.replace('PROTON_', '').replace(/_/g, ' ').toLowerCase()}
                    hint={f.key}
                    title={`${f.key}=1 — ${f.desc}`}
                    checked={on}
                    onChange={(v) => set('proton_flags', v ? [...flags, f.key] : flags.filter((x) => x !== f.key))}
                  />
                );
              })}
            </div>
          </div>
          )}

          <div>
            <MonoLabel className="mb-2">winetricks <span className="normal-case tracking-normal text-white/35">· saved, not applied yet</span></MonoLabel>
            <div className="flex flex-wrap gap-1.5">
              {[...new Set([...WINETRICKS_VERBS, ...(game.fix_info?.winetricks ?? [])])].map((t) => {
                const on = tricks.includes(t);
                return (
                  <button key={t} aria-pressed={on} onClick={() => set('winetricks', on ? tricks.filter((x) => x !== t) : [...tricks, t])} className={cx('rounded-lg px-2.5 py-1.5 font-mono text-[11.5px] transition', on ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]')}>
                    {on ? '✓ ' : '+ '}{t}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="rounded-xl bg-black/35 p-3.5 font-mono text-[11.5px] leading-relaxed ring-1 ring-white/[0.07]">
            <div className="mb-1 flex items-center justify-between text-white/40">
              <span>$ preview</span>
              {preview.cmd && (
                <Button variant="ghost" size="none" onClick={() => copy([...envLines.map(([k, v]) => `${k}=${v}`), preview.cmd].join(' '))} className="rounded px-1 py-0.5" aria-label="Copy command">
                  <Copy className="size-3" />
                </Button>
              )}
            </div>
            <div className="max-h-56 overflow-y-auto break-all select-text">
              {!isSteamClient && envLines.map(([k, v]) => <div key={k} className="text-amber-200/80">{k}={v} \</div>)}
              {preview.err ? <div className="text-red-300">{preview.err}</div> : <div className="text-emerald-200/90">{preview.cmd ?? '…'}</div>}
            </div>
          </div>

          <div className="flex gap-2.5">
            <Button variant="neutral" onClick={onClose} className="flex-1">Cancel</Button>
            <Button variant="primary" onClick={() => { onSave(current); setInitial({ draft: JSON.stringify(draft), env: envSig }); }} className="flex-1">Save config</Button>
          </div>
          {dirty && <div role="status" className="font-mono text-[11.5px] text-amber-300/80">Unsaved changes. Save or Cancel.</div>}
          <div className="font-mono text-[12px] leading-relaxed text-white/55">
            saved for next launch · prefix: {draft.wine_prefix || `~/.local/share/hydra/prefixes/${game.id}`}
          </div>
        </div>
      </div>
    </div>
  );
}

/** The standalone dialog around the general settings (the game page's own tab
 *  shell uses GeneralSettings directly). */
export function SettingsModal(p: Props) {
  const [dirty, setDirty] = useState(false);
  return (
    <Modal label={`Launcher settings for ${p.game.title}`} onClose={p.onClose} dismissable={!dirty} className="max-w-[920px]">
      <GeneralSettings {...p} onDirty={setDirty} />
    </Modal>
  );
}
