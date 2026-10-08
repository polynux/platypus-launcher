import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Effect } from 'effect';
import { ChevronDown, Copy, X } from 'lucide-react';
import type { Game, GameSettings, ProtonVersion } from '../types';
import { Modal } from './Modal';
import { MonoLabel, SegRow, TogglePill, btnNeutral, btnPrimary, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import {
  DX_LAYERS, GS_FILTERS, GS_RATES, GS_RESOLUTIONS, GS_TOGGLES, HUD_PRESETS, PROTON_FLAGS, SYNC_MODES, UPSCALERS, WINETRICKS_VERBS,
  composeGamescopeArgs, normalizeSettings, parseGamescopeArgs, rateChoice, resolutionChoice, steamLaunchOptions, toBackendGame,
} from '../lib/settings';

interface Props {
  game: Game;
  runners: ProtonVersion[];
  onClose: () => void;
  onSave: (s: GameSettings) => void;
  onCopied: () => void;
}

const field = 'w-full rounded-xl bg-white/[0.06] px-3 py-2 font-mono text-[11px] text-white placeholder-white/30 outline-none ring-amber-300/50 focus:ring-1';

/** Physical pixels of the display the window is on. */
function displayResolution(): { w: number; h: number } | undefined {
  if (typeof window === 'undefined' || !window.screen?.width) return undefined;
  const r = window.devicePixelRatio || 1;
  return { w: Math.round(window.screen.width * r), h: Math.round(window.screen.height * r) };
}

function SelectField({ label, value, onChange, children, className }: { label: string; value: string; onChange: (v: string) => void; children: React.ReactNode; className?: string }) {
  return (
    <div className={cx('relative', className)}>
      <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} className={cx(field, 'appearance-none py-2 pr-8 [color-scheme:dark]')}>
        {children}
      </select>
      <ChevronDown aria-hidden className="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-white/45" />
    </div>
  );
}

function runnerKind(r: ProtonVersion): string {
  if (r.is_system) return 'system';
  if (r.is_custom) return 'custom · compatibilitytools.d';
  return 'steam';
}

function parseEnv(text: string): { env: Record<string, string>; ignored: { n: number; line: string }[] } {
  const env: Record<string, string> = {};
  const ignored: { n: number; line: string }[] = [];
  text.split('\n').forEach((raw, i) => {
    const line = raw.trim();
    if (!line) return;
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (m) env[m[1]] = m[2];
    else ignored.push({ n: i + 1, line });
  });
  return { env, ignored };
}

export function SettingsModal({ game, runners, onClose, onSave, onCopied }: Props) {
  const [draft, setDraft] = useState<GameSettings>(() => normalizeSettings(game.settings));
  const [envText, setEnvText] = useState(() =>
    Object.entries(normalizeSettings(game.settings).env_vars).map(([k, v]) => `${k}=${v}`).join('\n')
  );
  // Dirty = draft or env text differs from what was loaded: Esc / click-outside then stay inert.
  const initial = useRef({ draft: JSON.stringify(draft), env: envText });
  const dirty = JSON.stringify(draft) !== initial.current.draft || envText !== initial.current.env;
  const [preview, setPreview] = useState<{ cmd?: string; err?: string }>({});
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));

  const isSteamClient = game.store === 'steam' && !game.executable;
  // A native Linux build runs without Proton: no runner and no Proton-only
  // settings to pick.
  const native = !!game.native;
  const parsedEnv = useMemo(() => parseEnv(envText), [envText]);
  const current = { ...draft, env_vars: parsedEnv.env };
  const backend = useMemo(() => toBackendGame({ ...game, settings: current }), [game, draft, envText]); // eslint-disable-line react-hooks/exhaustive-deps

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
    <Modal label={`Launcher settings for ${game.title}`} onClose={onClose} dismissable={!dirty} className="max-w-[920px]">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h2 className="text-[20px] font-extrabold tracking-tight">Launcher settings</h2>
          <div className="font-mono text-[10px] text-white/45">{game.title}</div>
        </div>
        <button onClick={onClose} aria-label="Close" className="grid h-8 w-8 place-items-center rounded-lg bg-white/[0.07] text-white/70 transition hover:bg-white/[0.12] hover:text-white">
          <X className="h-4 w-4" />
        </button>
      </div>

      {isSteamClient && (
        <div className="mb-6 rounded-2xl bg-sky-300/[0.07] p-4 ring-1 ring-sky-300/25">
          <p className="text-[12.5px] leading-relaxed text-sky-100/85">
            Steam games start through the Steam client, which picks the Proton version and ignores Hydra's env vars.
            To apply these settings, paste this into Steam → Properties → Launch options:
          </p>
          <div className="mt-2.5 flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-lg bg-black/35 px-3 py-2 font-mono text-[10.5px] text-sky-100/90 select-text">{steamLaunchOptions(backend)}</code>
            <button onClick={() => copy(steamLaunchOptions(backend))} className={cx(btnNeutral, 'flex items-center gap-1.5 px-3 py-2 text-[11.5px]')}>
              <Copy className="h-3.5 w-3.5" /> Copy
            </button>
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[1.5fr_1fr]">
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
              <option value="auto">{autoPick ? `Automatic (uses ${autoPick.name})` : 'Automatic (no runner installed)'}</option>
              {runnerMissing && <option value={selectedRunner}>{selectedRunner} (missing)</option>}
              {runnerGroups.map(([kind, list]) => (
                <optgroup key={kind} label={kind}>
                  {list.map((r) => <option key={r.id} value={r.id}>{r.name}{r.is_default ? ' (default)' : ''}</option>)}
                </optgroup>
              ))}
            </SelectField>
            <div className={cx('mt-1.5 truncate font-mono text-[9.5px]', runnerMissing ? 'text-orange-300' : 'text-white/45')}>
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
                <button key={k} title={title} aria-pressed={!!draft[k]} onClick={() => set(k, !draft[k])} className={cx('flex items-center justify-between rounded-xl px-3.5 py-2.5 transition', draft[k] ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}>
                  <span className="font-mono text-[10.5px] font-bold text-white/85">{label}</span>
                  <TogglePill on={!!draft[k]} />
                </button>
              ))}
            </div>
            {draft.use_gamescope && (
              <div className="mt-2.5 space-y-3 rounded-xl bg-white/[0.03] p-3.5 ring-1 ring-white/[0.07]">
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <MonoLabel className="mb-1.5">resolution</MonoLabel>
                    <SelectField
                      label="Gamescope resolution"
                      value={resChoice}
                      onChange={(v) => {
                        if (v === 'custom') return setResCustom(true);
                        setResCustom(false);
                        const [w, h] = v ? v.split('x').map(Number) : [0, 0];
                        setDraft((d) => ({ ...d, gamescope_w: w, gamescope_h: h }));
                      }}
                    >
                      <option value="">Game default</option>
                      {display && <option value={`${display.w}x${display.h}`}>{display.w}×{display.h} (this display)</option>}
                      {GS_RESOLUTIONS.filter((r) => !(display && r.w === display.w && r.h === display.h)).map((r) => (
                        <option key={`${r.w}x${r.h}`} value={`${r.w}x${r.h}`}>{r.w}×{r.h}</option>
                      ))}
                      <option value="custom">{resChoice === 'custom' && draft.gamescope_w + draft.gamescope_h > 0 ? `Custom… (${draft.gamescope_w || '?'}×${draft.gamescope_h || '?'})` : 'Custom…'}</option>
                    </SelectField>
                    {resChoice === 'custom' && (
                      <div className="mt-2 flex items-center gap-2">
                        <input aria-label="gamescope width" type="number" min={0} value={draft.gamescope_w || ''} onChange={(e) => set('gamescope_w', parseInt(e.target.value, 10) || 0)} placeholder="width" className={field} />
                        <span className="text-white/40">×</span>
                        <input aria-label="gamescope height" type="number" min={0} value={draft.gamescope_h || ''} onChange={(e) => set('gamescope_h', parseInt(e.target.value, 10) || 0)} placeholder="height" className={field} />
                      </div>
                    )}
                  </div>
                  <div>
                    <MonoLabel className="mb-1.5">frame rate</MonoLabel>
                    <SelectField
                      label="Gamescope frame rate"
                      value={rateSel}
                      onChange={(v) => {
                        if (v === 'custom') return setRateCustom(true);
                        setRateCustom(false);
                        set('gamescope_r', v ? Number(v) : 0);
                      }}
                    >
                      <option value="">Game default</option>
                      {GS_RATES.map((r) => <option key={r} value={r}>{r} Hz</option>)}
                      <option value="custom">{rateSel === 'custom' && draft.gamescope_r ? `Custom… (${draft.gamescope_r} Hz)` : 'Custom…'}</option>
                    </SelectField>
                    {rateSel === 'custom' && (
                      <input aria-label="gamescope refresh" type="number" min={0} value={draft.gamescope_r || ''} onChange={(e) => set('gamescope_r', parseInt(e.target.value, 10) || 0)} placeholder="Hz" className={cx(field, 'mt-2')} />
                    )}
                  </div>
                </div>
                <div className="grid gap-1.5 sm:grid-cols-2">
                  {GS_TOGGLES.map((t) => (
                    <button key={t.key} title={t.title} aria-pressed={gs.toggles[t.key]} onClick={() => setGs({ toggles: { ...gs.toggles, [t.key]: !gs.toggles[t.key] } })} className={cx('flex items-center justify-between rounded-xl px-3.5 py-2.5 transition', gs.toggles[t.key] ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}>
                      <span className="font-mono text-[10.5px] font-bold text-white/85">{t.label}</span>
                      <TogglePill on={gs.toggles[t.key]} />
                    </button>
                  ))}
                </div>
                <div>
                  <MonoLabel className="mb-1.5">upscale filter</MonoLabel>
                  <SegRow label="Gamescope upscale filter" options={GS_FILTERS} value={gs.filter} onChange={(v) => setGs({ filter: v })} />
                </div>
                <label className="block">
                  <MonoLabel className="mb-1.5">extra gamescope args</MonoLabel>
                  <input aria-label="gamescope args" value={gsExtra} onChange={(e) => { setGsExtra(e.target.value); set('gamescope_args', composeGamescopeArgs({ ...gs, extra: e.target.value })); }} placeholder="--mangoapp" className={field} />
                </label>
              </div>
            )}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block">
              <MonoLabel className="mb-1.5">launch arguments</MonoLabel>
              <input value={draft.launch_arguments} onChange={(e) => set('launch_arguments', e.target.value)} placeholder="-novid -fullscreen" className={field} />
            </label>
            <label className="block">
              <MonoLabel className="mb-1.5">extra env vars</MonoLabel>
              <textarea rows={2} value={envText} onChange={(e) => setEnvText(e.target.value)} placeholder="KEY=value (one per line)" className={cx(field, 'resize-y')} />
              {parsedEnv.ignored.length > 0 && (
                <p role="status" className="mt-1.5 break-all font-mono text-[10px] text-orange-300">
                  {parsedEnv.ignored.map((l) => `Ignored line ${l.n}: ${l.line}`).join(' · ')}
                </p>
              )}
            </label>
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
                  <button
                    key={f.key}
                    title={`${f.key}=1 — ${f.desc}`}
                    aria-pressed={on}
                    onClick={() => set('proton_flags', on ? flags.filter((x) => x !== f.key) : [...flags, f.key])}
                    className={cx('flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 transition', on ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}
                  >
                    <span className="text-left font-mono text-[10.5px] font-bold text-white/85">{f.key.replace('PROTON_', '').replace(/_/g, ' ').toLowerCase()}</span>
                    <TogglePill on={on} />
                  </button>
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
                  <button key={t} aria-pressed={on} onClick={() => set('winetricks', on ? tricks.filter((x) => x !== t) : [...tricks, t])} className={cx('rounded-lg px-2.5 py-1.5 font-mono text-[10.5px] transition', on ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]')}>
                    {on ? '✓ ' : '+ '}{t}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="rounded-xl bg-black/35 p-3.5 font-mono text-[10.5px] leading-relaxed ring-1 ring-white/[0.07]">
            <div className="mb-1 flex items-center justify-between text-white/40">
              <span>$ preview</span>
              {preview.cmd && (
                <button onClick={() => copy([...envLines.map(([k, v]) => `${k}=${v}`), preview.cmd].join(' '))} className="rounded px-1 text-white/40 hover:text-white/80" aria-label="Copy command">
                  <Copy className="h-3 w-3" />
                </button>
              )}
            </div>
            <div className="max-h-56 overflow-y-auto break-all select-text">
              {!isSteamClient && envLines.map(([k, v]) => <div key={k} className="text-amber-200/80">{k}={v} \</div>)}
              {preview.err ? <div className="text-red-300">{preview.err}</div> : <div className="text-emerald-200/90">{preview.cmd ?? '…'}</div>}
            </div>
          </div>

          <div className="flex gap-2.5">
            <button onClick={onClose} className={cx(btnNeutral, 'flex-1 py-2.5 text-[12.5px] text-white/70')}>Cancel</button>
            <button onClick={() => onSave(current)} className={cx(btnPrimary, 'flex-1 py-2.5 text-[12.5px]')}>Save config</button>
          </div>
          {dirty && <div role="status" className="font-mono text-[10px] text-amber-300/80">Unsaved changes. Save or Cancel.</div>}
          <div className="font-mono text-[9px] leading-relaxed text-white/40">
            saved for next launch · prefix: {draft.wine_prefix || `~/.local/share/hydra/prefixes/${game.id}`}
          </div>
        </div>
      </div>
    </Modal>
  );
}
