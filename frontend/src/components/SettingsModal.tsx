import React, { useEffect, useMemo, useState } from 'react';
import { Effect } from 'effect';
import { Copy, X } from 'lucide-react';
import type { Game, GameSettings, ProtonVersion } from '../types';
import { Modal } from './Modal';
import { MonoLabel, SegRow, TogglePill, btnNeutral, btnPrimary, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import {
  DX_LAYERS, HUD_PRESETS, PROTON_FLAGS, SYNC_MODES, UPSCALERS, WINETRICKS_VERBS,
  normalizeSettings, steamLaunchOptions, toBackendGame,
} from '../lib/settings';

interface Props {
  game: Game;
  runners: ProtonVersion[];
  onClose: () => void;
  onSave: (s: GameSettings) => void;
  onCopied: () => void;
}

const field = 'w-full rounded-xl bg-white/[0.06] px-3 py-2 font-mono text-[11px] text-white placeholder-white/30 outline-none ring-amber-300/50 focus:ring-1';

function runnerKind(r: ProtonVersion): string {
  if (r.is_system) return 'system';
  if (r.is_custom) return 'custom · compatibilitytools.d';
  return 'steam';
}

function parseEnv(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split('\n')) {
    const m = line.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (m) out[m[1]] = m[2];
  }
  return out;
}

export function SettingsModal({ game, runners, onClose, onSave, onCopied }: Props) {
  const [draft, setDraft] = useState<GameSettings>(() => normalizeSettings(game.settings));
  const [envText, setEnvText] = useState(() =>
    Object.entries(normalizeSettings(game.settings).env_vars).map(([k, v]) => `${k}=${v}`).join('\n')
  );
  const [preview, setPreview] = useState<{ cmd?: string; err?: string }>({});
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));

  const isSteamClient = game.store === 'steam' && !game.executable;
  const current = { ...draft, env_vars: parseEnv(envText) };
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
  const runnerOptions: ProtonVersion[] = runners;
  const selectedRunner = draft.runner_id;

  const copy = (text: string) => navigator.clipboard.writeText(text).then(onCopied).catch(() => {});

  return (
    <Modal label={`Launcher settings for ${game.title}`} onClose={onClose} className="max-w-[920px]">
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
          <div>
            <MonoLabel className="mb-2">runner</MonoLabel>
            <div role="radiogroup" aria-label="Runner" className="space-y-2">
              {[{ id: 'auto', name: 'Automatic', path: 'first detected runner', is_system: false, is_custom: false, is_default: false } as ProtonVersion, ...runnerOptions].map((r) => {
                const on = selectedRunner === r.id || (r.id === 'auto' && !selectedRunner);
                return (
                  <button
                    key={r.id}
                    role="radio"
                    aria-checked={on}
                    onClick={() => set('runner_id', r.id)}
                    className={cx('flex w-full items-center justify-between gap-4 rounded-xl border px-4 py-3 text-left transition', on ? 'border-amber-300/50 bg-amber-300/[0.07]' : 'border-white/[0.08] hover:border-white/20')}
                  >
                    <div className="min-w-0">
                      <div className="text-[13px] font-bold">{r.name}</div>
                      <div className="truncate font-mono text-[9.5px] text-white/45">{r.id === 'auto' ? r.path : `${runnerKind(r)} · ${r.path}`}</div>
                    </div>
                    <span className={cx('h-4 w-4 flex-none rounded-full border-2', on ? 'border-amber-300 bg-amber-300' : 'border-white/20')} />
                  </button>
                );
              })}
              {runners.length === 0 && <p className="text-[12px] text-white/50">No Proton or Wine runners detected.</p>}
            </div>
          </div>

          <div className="grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <div><MonoLabel className="mb-1.5">dx layer</MonoLabel><SegRow label="DX layer" options={DX_LAYERS} value={draft.dx_layer!} onChange={(v) => set('dx_layer', v)} /></div>
            <div><MonoLabel className="mb-1.5">sync</MonoLabel><SegRow label="Sync" options={SYNC_MODES} value={draft.sync_mode === 'default' ? 'fsync' : draft.sync_mode!} onChange={(v) => set('sync_mode', v)} /></div>
            <div><MonoLabel className="mb-1.5">upscaler</MonoLabel><SegRow label="Upscaler" options={UPSCALERS} value={draft.upscaler === 'integer' ? 'native' : draft.upscaler!} onChange={(v) => set('upscaler', v)} /></div>
            <div><MonoLabel className="mb-1.5">mangohud</MonoLabel><SegRow label="MangoHud" options={HUD_PRESETS} value={draft.mangohud_preset!} onChange={(v) => set('mangohud_preset', v)} /></div>
            <div><MonoLabel className="mb-1.5">dxvk shader cache</MonoLabel><SegRow label="Shader cache" options={[{ value: 'on', label: 'on' }, { value: 'off', label: 'off', title: 'DXVK_STATE_CACHE=0' }]} value={draft.shader_cache!} onChange={(v) => set('shader_cache', v)} /></div>
          </div>

          <div>
            <MonoLabel className="mb-2">pipeline</MonoLabel>
            <div className="grid gap-1.5 sm:grid-cols-2">
              {([
                ['use_gamemode', 'gamemode', 'Wrap with gamemoderun'],
                ['use_gamescope', 'gamescope', 'Run inside a gamescope session'],
                ['eac_runtime', 'eac runtime', 'Bind the Proton EasyAntiCheat runtime'],
                ['battleye_runtime', 'battleye runtime', 'Bind the Proton BattlEye runtime'],
              ] as const).map(([k, label, title]) => (
                <button key={k} title={title} aria-pressed={!!draft[k]} onClick={() => set(k, !draft[k])} className={cx('flex items-center justify-between rounded-xl px-3.5 py-2.5 transition', draft[k] ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}>
                  <span className="font-mono text-[10.5px] font-bold text-white/85">{label}</span>
                  <TogglePill on={!!draft[k]} />
                </button>
              ))}
            </div>
            {draft.use_gamescope && (
              <div className="mt-2.5 grid grid-cols-[repeat(3,4.5rem)_1fr] gap-2">
                {(['gamescope_w', 'gamescope_h', 'gamescope_r'] as const).map((k) => (
                  <input key={k} aria-label={k.replace('gamescope_', 'gamescope ')} type="number" min={0} value={draft[k] || ''} onChange={(e) => set(k, parseInt(e.target.value, 10) || 0)} placeholder={k.slice(-1)} className={field} />
                ))}
                <input aria-label="gamescope args" value={draft.gamescope_args} onChange={(e) => set('gamescope_args', e.target.value)} placeholder="-f --adaptive-sync" className={field} />
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
            </label>
          </div>
        </div>

        <div className="min-w-0 space-y-5">
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
            <button onClick={() => onSave(current)} title="Applies this session — per-game settings aren't saved to disk yet" className={cx(btnPrimary, 'flex-1 py-2.5 text-[12.5px]')}>Save config</button>
          </div>
          <div className="font-mono text-[9px] leading-relaxed text-white/40">
            applies this session · prefix: {draft.wine_prefix || `~/.local/share/hydra/prefixes/${game.id}`}
          </div>
        </div>
      </div>
    </Modal>
  );
}
