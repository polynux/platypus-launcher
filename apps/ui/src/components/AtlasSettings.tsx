// Atlas drawer only (game-settings.tabs = accordion, atlas-only): the /21
// SettingsDrawer categories, mapped onto the app's real settings and APIs.
// Other themes keep the tab dialog/sheet in GameSettingsModal; this file never
// loads for them.
import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import {
  Braces, ChevronDown, Cloud, Cpu, Gauge, GitBranch, Layers, MonitorPlay, Palette,
  Plug, Puzzle, TerminalSquare, Wrench,
} from 'lucide-react';
import type {
  Game, GameSettings, Job, ProtonVersion, SettingField, SettingGroup,
  StorePlugin, Tweak,
} from '../types';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { LauncherApi } from '../effect/runtime';
import { AtlasSavesBlock } from './GameTabs';
import { TweaksPanel } from './TweaksPanel';
import { VersionsDialog } from './VersionsDialog';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { GameSettingsPanel } from './PluginSettings';
import { CustomizationTab } from './CustomizationTab';
import { MaintenanceTab } from './MaintenanceTab';
import { LegalSection } from './LegalSection';
import { CustomGameYear } from './CustomGameYear';
import { CustomProgram, PrefixPresets } from './SettingsModal';
import { cardCompat } from '../lib/compat';
import { envKeyError, steamLaunchOptions, tweakApplied } from '../lib/settings';
import { useCompat } from '../lib/useCompat';
import { coreErrMsg } from '../lib/coreError';
import { ErrorNotice } from './ErrorNotice';
import { ExternalLink } from './ExternalLink';
import { useTranslation as useT } from 'react-i18next';
import { Trans as TransI18n } from 'react-i18next';
import type { GameSettingsDraft } from './SettingsModal';
import type { GameSettingsTab } from '../lib/gameSettingsTabs';
import { cx } from './ui';

/** The /21 categories, in drawer order. The Game group reuses the tab ids. */
export type AtlasCategory =
  | 'compat' | 'performance' | 'display' | 'saves'
  | 'layers' | 'fixes' | 'env' | 'launch'
  | 'versions' | 'options' | 'plugin' | 'customization' | 'maintenance';

export type AtlasLevel = 'everyday' | 'advanced' | 'game';

const EVERYDAY: AtlasCategory[] = ['compat', 'performance', 'display', 'saves'];
const ADVANCED: AtlasCategory[] = ['layers', 'fixes', 'env', 'launch'];
const GAME_TABS: AtlasCategory[] = ['versions', 'options', 'plugin', 'customization', 'maintenance'];

/** The tab the drawer opens for a requested settings tab (general splits). */
export function atlasCategoryForTab(tab?: GameSettingsTab | null): AtlasCategory {
  switch (tab) {
    case 'cloud': return 'saves';
    case 'versions': return 'versions';
    case 'options': return 'options';
    case 'plugin': return 'plugin';
    case 'tweaks': return 'fixes';
    case 'customization': return 'customization';
    case 'maintenance': return 'maintenance';
    default: return 'compat';
  }
}

/** The drawer groups: Everyday, Advanced, then Game (only tabs the game has). */
export function atlasGroups(tabs: GameSettingsTab[]): { level: AtlasLevel; cats: AtlasCategory[] }[] {
  const game = GAME_TABS.filter((t) => (tabs as string[]).includes(t));
  return [
    { level: 'everyday', cats: EVERYDAY },
    { level: 'advanced', cats: ADVANCED },
    ...(game.length ? [{ level: 'game' as const, cats: game }] : []),
  ];
}

/** Where a runner.options field lives. Unknown fields fall into Fixes. */
export function atlasFieldCategory(fieldId: string): 'compat' | 'performance' | 'display' | 'layers' | 'fixes' | 'env' | 'launch' {
  switch (fieldId) {
    case 'runner_id':
      return 'compat';
    case 'use_gamemode':
    case 'use_mangohud':
    case 'mangohud_preset':
      return 'performance';
    case 'use_gamescope':
    case 'gamescope_resolution':
    case 'gamescope_rate':
    case 'gamescope_filter':
    case 'gamescope_fullscreen':
    case 'gamescope_borderless':
    case 'gamescope_grab_cursor':
    case 'gamescope_hdr':
    case 'gamescope_vrr':
    case 'gamescope_wayland':
    case 'gamescope_w':
    case 'gamescope_h':
    case 'gamescope_r':
    case 'upscaler':
      return 'display';
    case 'dx_layer':
    case 'sync_mode':
    case 'shader_cache':
    case 'proton_enable_nvapi':
    case 'proton_hide_nvidia_gpu':
    case 'proton_enable_wayland':
    case 'proton_no_d3d11':
    case 'proton_no_d3d12':
    case 'proton_force_large_address_aware':
    case 'proton_old_gl_string':
      return 'layers';
    case 'proton_log':
    case 'wine_prefix':
    case 'env_vars':
      return 'env';
    case 'launch_arguments':
    case 'working_dir':
    case 'gamescope_args':
      return 'launch';
    case 'eac_runtime':
    case 'battleye_runtime':
    case 'winetricks':
      return 'fixes';
    default:
      return 'fixes';
  }
}

/** Every runner field lands in exactly one drawer category (unknowns in Fixes). */
const ATLAS_KNOWN_FIELDS: ReadonlySet<string> = new Set([
  'runner_id',
  'use_gamemode', 'use_mangohud', 'mangohud_preset',
  'use_gamescope', 'gamescope_resolution', 'gamescope_rate', 'gamescope_filter',
  'gamescope_fullscreen', 'gamescope_borderless', 'gamescope_grab_cursor',
  'gamescope_hdr', 'gamescope_vrr', 'gamescope_wayland', 'gamescope_w',
  'gamescope_h', 'gamescope_r', 'upscaler',
  'dx_layer', 'sync_mode', 'shader_cache',
  'proton_enable_nvapi', 'proton_hide_nvidia_gpu', 'proton_enable_wayland',
  'proton_no_d3d11', 'proton_no_d3d12', 'proton_force_large_address_aware',
  'proton_old_gl_string',
  'proton_log', 'wine_prefix', 'env_vars',
  'launch_arguments', 'working_dir', 'gamescope_args',
  'eac_runtime', 'battleye_runtime', 'winetricks',
]);
/** Fields no drawer category names (future runner flags land in Fixes). */
export function atlasUnmappedFields(groups: SettingGroup[]): string[] {
  const ids = groups.flatMap((g) => g.fields.map((f) => f.id));
  return ids.filter((id) => !ATLAS_KNOWN_FIELDS.has(id));
}

export const ATLAS_ICONS: Record<AtlasCategory, React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>> = {
  compat: Cpu,
  performance: Gauge,
  display: MonitorPlay,
  saves: Cloud,
  layers: Layers,
  fixes: Wrench,
  env: Braces,
  launch: TerminalSquare,
  versions: GitBranch,
  options: Puzzle,
  plugin: Plug,
  customization: Palette,
  maintenance: Wrench,
};

function fieldOf(groups: SettingGroup[], id: string): { group: SettingGroup; field: SettingField } | null {
  for (const g of groups) for (const f of g.fields) if (f.id === id) return { group: g, field: f };
  return null;
}

function savedValue(game: Game, id: string, field?: SettingField | null): boolean | string | number {
  const raw = (game.settings as unknown as Record<string, unknown>)[id]
    ?? (game.settings_extra as Record<string, unknown> | undefined)?.[id]
    ?? '';
  if (raw === '' || raw === undefined || raw === null) return (field?.default ?? '') as string;
  return raw as boolean | string | number;
}

// ---------------------------------------------------------------------------
// Rows: grid 1fr auto, label 600 + 0.93em muted description, control right.
// Atlas only; shared ds controls are untouched (sans via className, 190px).

export function AtlasSRow({ label, desc, hint, children, wide }: {
  label: React.ReactNode; desc?: React.ReactNode; hint?: React.ReactNode;
  children?: React.ReactNode; wide?: boolean;
}) {
  if (wide) {
    return (
      <div className="w-full py-[14px] text-left">
        <div className="grid gap-[3px] min-w-0">
          <span className="font-semibold text-text text-[13.5px]">{label}</span>
          {desc && <span className="text-text-muted text-[0.93em] leading-[1.5]">{desc}</span>}
          {hint && <span className="font-mono text-[11.5px] text-text-faint">{hint}</span>}
        </div>
        {children && <div className="mt-2 min-w-0">{children}</div>}
      </div>
    );
  }
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] w-full py-[14px] border-b border-hairline text-left last:border-b-0">
      <div className="grid gap-[3px] min-w-0">
        <span className="font-semibold text-text text-[13.5px]">{label}</span>
        {desc && <span className="text-text-muted text-[0.93em] leading-[1.5]">{desc}</span>}
        {hint && <span className="font-mono text-[11.5px] text-text-faint">{hint}</span>}
      </div>
      {children && <div className="min-w-0">{children}</div>}
    </div>
  );
}

export function AtlasSSwitch({ label, desc, hint, checked, disabled, onChange, labelId }: {
  label: React.ReactNode; desc?: React.ReactNode; hint?: React.ReactNode;
  checked: boolean; disabled?: boolean; onChange: () => void; labelId?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-labelledby={labelId}
      disabled={disabled}
      onClick={onChange}
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] w-full py-[14px] border-b border-hairline text-left last:border-b-0 disabled:opacity-45 disabled:cursor-not-allowed"
    >
      <span className="grid gap-[3px] min-w-0">
        <span className="font-semibold text-text text-[13.5px]">{label}</span>
        {desc && <span className="text-text-muted text-[0.93em] leading-[1.5]">{desc}</span>}
        {hint && <span className="font-mono text-[11.5px] text-text-faint">{hint}</span>}
      </span>
      <span aria-hidden className={cx('relative h-5 w-[34px] flex-none rounded-full transition-colors', checked ? 'bg-accent' : 'bg-surface-strong ring-1 ring-inset ring-hairline-strong')}>
        <span className={cx('absolute left-0.5 top-0.5 h-4 w-4 rounded-full transition-transform', checked ? 'translate-x-[14px] bg-on-accent' : 'bg-text-muted')} />
      </span>
    </button>
  );
}

/** The Steam AppID row as an AtlasSRow (the lab's .srow): a 600 label with a
 *  muted description on the left, a 190px sans input and Save on the right.
 *  No faint mono label or mono placeholder like the old field. */
function AtlasSteamAppIdRow({ game }: { game: Game }) {
  const { t } = useT('game');
  const { compat, update } = useCompat(game);
  const userSet = compat?.steam_app_id_source === 'user' ? compat.steam_app_id ?? '' : '';
  const effective = compat?.steam_app_id ?? '';
  const [draft, setDraft] = useState(userSet);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => setDraft(userSet), [userSet]);
  const save = () => {
    const next = draft.trim();
    if (next === userSet) return;
    if (next !== '' && !/^\d+$/.test(next)) {
      setError(t('game:steamAppId.numberError'));
      return;
    }
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setGameSteamAppID(game, next))
      .then((c) => update(c))
      .catch((e) => setError(coreErrMsg(e)))
      .finally(() => setBusy(false));
  };
  const href = effective
    ? `https://store.steampowered.com/app/${effective}`
    : `https://store.steampowered.com/search/?term=${encodeURIComponent(game.title)}`;
  const source =
    compat?.steam_app_id_source === 'user' ? t('game:appIdSource.user')
    : compat?.steam_app_id_source === 'plugin' ? t('game:appIdSource.plugin')
    : compat?.steam_app_id_source === 'steam' ? t('game:appIdSource.steam')
    : t('game:appIdSource.none');
  return (
    <AtlasSRow
      label={t('game:steamAppId.heading')}
      desc={
        effective ? (
          <TransI18n
            i18nKey="game:steamAppId.current"
            values={{ id: effective, source }}
            components={{ a: <ExternalLink href={href} className="text-accent hover:underline" /> }}
          />
        ) : (
          <TransI18n
            i18nKey="game:steamAppId.none"
            components={{ a: <ExternalLink href={href} className="text-accent hover:underline" /> }}
          />
        )
      }
    >
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <Input
          aria-label={t('game:steamAppId.aria')}
          aria-invalid={!!error}
          inputMode="numeric"
          value={draft}
          placeholder={effective ? effective : t('game:steamAppId.placeholder')}
          onChange={(e) => {
            setDraft(e.target.value);
            setError(null);
          }}
          onBlur={save}
          disabled={busy}
          spellCheck={false}
          className="h-8 w-[190px] flex-none rounded-[7px] bg-raised font-sans text-[13px] ring-1 ring-hairline-strong placeholder:text-text-faint focus-visible:ring-2 focus-visible:ring-focus"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={busy || draft.trim() === userSet}>
          {t('common:save')}
        </Button>
      </form>
      {error && <ErrorNotice className="mt-2" message={error} fallback={t('game:steamAppId.failed')} />}
    </AtlasSRow>
  );
}

const DEFAULT_CHOICE = '__default__';

function AtlasChoiceSelect({ field, value, onChange, width = 190 }: {
  field: SettingField; value: string; onChange: (v: string) => void; width?: number;
}) {
  const current = value === '' ? DEFAULT_CHOICE : value;
  const known = (field.options ?? []).some((o) => (o.id === '' ? DEFAULT_CHOICE : o.id) === current);
  return (
    <Select value={current} onValueChange={(v) => onChange(v === DEFAULT_CHOICE ? '' : v)}>
      <SelectTrigger aria-label={field.label} className="font-sans text-[13px] h-8 flex-none" style={{ width }}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(field.options ?? []).map((o) => (
          <SelectItem key={o.id || DEFAULT_CHOICE} value={o.id || DEFAULT_CHOICE} className="font-sans text-[13px]">{o.label}</SelectItem>
        ))}
        {!known && current !== DEFAULT_CHOICE && <SelectItem value={current} className="font-sans text-[13px]">{current}</SelectItem>}
      </SelectContent>
    </Select>
  );
}

// ---------------------------------------------------------------------------
// Summaries follow kit.jsx catSummary, with the app's real values.

export function atlasSummary(
  t: (key: string, opts?: Record<string, unknown>) => string,
  cat: AtlasCategory,
  ctx: { game: Game; runners: ProtonVersion[]; tweaks: Tweak[]; job?: Job },
): string {
  const { game, runners, tweaks } = ctx;
  const native = !!game.native;
  const get = (id: string): string => {
    const v = (game.settings as unknown as Record<string, unknown>)[id]
      ?? (game.settings_extra as Record<string, unknown> | undefined)?.[id];
    return typeof v === 'string' ? v : v == null ? '' : String(v);
  };
  const bool = (id: string): boolean => {
    const v = (game.settings as unknown as Record<string, unknown>)[id];
    if (v !== undefined) return !!v;
    return !!((game.settings_extra as Record<string, unknown> | undefined)?.[id]);
  };
  switch (cat) {
    case 'compat': {
      if (native) return t('gameSettings:summary.native');
      const wanted = game.settings.runner_id;
      if (!wanted || wanted === 'auto') return t('gameSettings:summary.auto');
      return runners.find((r) => r.id === wanted || r.name === wanted)?.name ?? wanted;
    }
    case 'performance': {
      const parts: string[] = [];
      if (bool('use_gamemode')) parts.push('GameMode');
      const mangohud = get('mangohud_preset') ? get('mangohud_preset') !== 'off' : bool('use_mangohud');
      if (mangohud) parts.push('Overlay');
      return parts.length ? parts.join(', ') : (t('atlas:summary.default') as string);
    }
    case 'display': {
      if (!bool('use_gamescope')) return t('atlas:summary.default') as string;
      return get('upscaler') === 'fsr1' ? 'Gamescope, FSR' : 'Gamescope';
    }
    case 'saves':
      return game.cloud_sync ? t('gameSettings:summary.cloudStore') : t('gameSettings:summary.cloudPlatypus');
    case 'layers': {
      if (native) return t('atlas:summary.notUsed') as string;
      const dx = get('dx_layer') || 'dxvk';
      if (dx === 'dxvk') return t('atlas:summary.bundled') as string;
      return dx;
    }
    case 'fixes': {
      if (native) return t('atlas:summary.notNeeded') as string;
      const applied = (game.settings.winetricks ?? []).length
        + (bool('eac_runtime') ? 1 : 0) + (bool('battleye_runtime') ? 1 : 0)
        + tweaks.filter((tw) => tweakApplied(game.settings, tw)).length;
      const suggested = tweaks.filter((tw) => !tweakApplied(game.settings, tw)).length;
      const a = applied ? t('atlas:summary.applied', { count: applied }) : t('atlas:summary.noneApplied');
      return suggested ? `${a}, ${suggested} suggested` : (a as string);
    }
    case 'env': {
      const n = Object.keys(game.settings.env_vars ?? {}).length;
      return n ? (t('atlas:summary.setCount', { count: n }) as string) : (t('atlas:summary.none') as string);
    }
    case 'launch':
      return game.settings.launch_arguments?.trim() || (t('atlas:summary.noArgs') as string);
    case 'versions':
      if (game.pinned) return t('gameSettings:summary.pinned', { version: game.pinned.id }) as string;
      return (game.update_available ? t('gameSettings:summary.update') : t('gameSettings:summary.latest')) as string;
    case 'options':
      return t('gameSettings:summary.options') as string;
    case 'plugin':
      return 'plugin';
    case 'customization':
      return game.customized?.length
        ? (t('gameSettings:summary.customized', { count: game.customized.length }) as string)
        : (t('gameSettings:summary.customization') as string);
    default:
      return ctx.job
        ? (t('gameSettings:summary.job', { kind: ctx.job.kind, percent: Math.round(ctx.job.percent) }) as string)
        : (t('gameSettings:summary.maintenance') as string);
  }
}

export function atlasLabel(t: (key: string) => string, cat: AtlasCategory): string {
  switch (cat) {
    case 'compat': return t('atlas:cats.compat');
    case 'performance': return t('atlas:cats.performance');
    case 'display': return t('atlas:cats.display');
    case 'saves': return t('atlas:cats.saves');
    case 'layers': return t('atlas:cats.layers');
    case 'fixes': return t('atlas:cats.fixes');
    case 'env': return t('atlas:cats.env');
    case 'launch': return t('atlas:cats.launch');
    case 'versions': return t('gameSettings:tabs.versions');
    case 'options': return t('gameSettings:tabs.options');
    case 'plugin': return t('gameSettings:tabs.plugin');
    case 'customization': return t('gameSettings:tabs.customization');
    default: return t('gameSettings:tabs.maintenance');
  }
}

// ---------------------------------------------------------------------------
// Category bodies. Draft edits stay in the shared draft; instant panels
// (tweaks, cloud mode, backups) save at once, as before.

export interface AtlasBodyProps {
  cat: AtlasCategory;
  game: Game;
  plugin?: StorePlugin;
  runners: ProtonVersion[];
  tweaks: Tweak[];
  jobs: Job[];
  job?: Job;
  groups: SettingGroup[];
  shared: GameSettingsDraft;
  onSaveSettings: (s: GameSettings) => void | Promise<void>;
  onChangeSettings: (s: GameSettings, what: string) => void;
  onJob: (kind: 'install' | 'update' | 'uninstall') => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onChanged: () => void;
  onCustomized: () => void;
  onCopied: () => void;
  onClose: () => void;
  onOpenGeneral: () => void;
  onRemove?: (opts: { delete_prefix: boolean; delete_files: boolean; allow_no_saves: boolean }) => void;
  running?: boolean;
}

export function AtlasCategoryBody(p: AtlasBodyProps) {
  const { cat, game, groups, shared } = p;
  const { t } = useTranslation(['gameSettings', 'atlas', 'common']);
  const { draft, setDraft, envRows, setEnvRows, nextRowId, current } = shared;
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));
  const changeChoice = (key: string, value: boolean | string | number) =>
    set(key as keyof GameSettings, value as never);
  const draftRec = draft as unknown as Record<string, unknown>;
  const native = !!game.native;
  const clientLaunched = !!game.client_launch;

  const choiceValue = (field: SettingField): string => {
    const raw = draftRec[field.id];
    if (raw === undefined || raw === null || raw === '') return (field.default ?? '') as string;
    return String(raw);
  };

  if (cat === 'compat') {
    const runnerFound = fieldOf(groups, 'runner_id');
    const compat = cardCompat(game);
    const tier = compat?.kind === 'tier' ? compat.tier : compat?.kind === 'native' ? 'native' : null;
    const unrated = !native && !tier;
    return (
      <div>
        {game.store === 'custom' && (
          <>
            <CustomProgram game={game} detected={null} runMode={draft.run_mode ?? ''} onRunMode={(v) => set('run_mode', v)} onChanged={p.onCustomized} />
            <CustomGameYear game={game} onChanged={p.onCustomized} />
          </>
        )}
        {runnerFound && !native ? (
          <AtlasSRow
            label={t('atlas:compat.runner')}
            desc={runnerFound.field.description}
          >
            <AtlasChoiceSelect field={runnerFound.field} value={choiceValue(runnerFound.field)} onChange={(v) => changeChoice('runner_id', v)} />
          </AtlasSRow>
        ) : native ? (
          <AtlasSRow label={t('atlas:compat.runner')} desc={t('gameSettings:runner.nativeNote')} />
        ) : null}
        <AtlasSRow
          label={t('atlas:compat.rating')}
          desc={native ? t('gameSettings:summary.native')
            : tier ? `${t('atlas:compat.ratedAs')} ${tier}`
            : t('atlas:compat.unrated')}
        />
        {unrated && (
          <AtlasSteamAppIdRow game={game} />
        )}
        {game.fix_info?.description && (
          <AtlasSRow label={t('atlas:compat.tip')} desc={game.fix_info.description} />
        )}
        {clientLaunched && (
          <p className="py-2 text-[12.5px] leading-relaxed text-text-body">{t('gameSettings:steamNotice')}</p>
        )}
      </div>
    );
  }

  if (cat === 'performance') {
    const gamemode = fieldOf(groups, 'use_gamemode');
    const mango = fieldOf(groups, 'mangohud_preset');
    const legacyMango = (draftRec['use_mangohud'] as boolean | undefined) ?? game.settings.use_mangohud;
    const preset = typeof draftRec['mangohud_preset'] === 'string' && draftRec['mangohud_preset'] !== ''
      ? String(draftRec['mangohud_preset'])
      : legacyMango ? 'full' : 'off';
    const mangoOn = preset !== 'off';
    return (
      <div>
        <AtlasSSwitch
          label={gamemode?.field.label ?? 'GameMode'}
          desc={gamemode?.field.description ?? t('atlas:performance.gamemodeDesc')}
          checked={!!draft.use_gamemode}
          disabled={!gamemode}
          onChange={() => changeChoice('use_gamemode', !draft.use_gamemode)}
        />
        <div className="border-b border-hairline last:border-b-0">
          <AtlasSSwitch
            label={t('atlas:performance.mangohud')}
            desc={mango?.field.description ?? t('atlas:performance.mangohudDesc')}
            checked={mangoOn}
            disabled={!mango && legacyMango === undefined}
            onChange={() => {
              if (!mango) { set('use_mangohud', !legacyMango); return; }
              changeChoice('mangohud_preset', mangoOn ? 'off' : 'full');
            }}
          />
          {mango && mangoOn && (
            <div className="flex justify-end pb-3">
              <AtlasChoiceSelect field={mango.field} value={preset} onChange={(v) => changeChoice('mangohud_preset', v)} />
            </div>
          )}
        </div>
      </div>
    );
  }

  if (cat === 'display') {
    const scope = fieldOf(groups, 'use_gamescope');
    const res = fieldOf(groups, 'gamescope_resolution');
    const rate = fieldOf(groups, 'gamescope_rate');
    const up = fieldOf(groups, 'upscaler');
    const upOn = String(draftRec['upscaler'] ?? 'native') === 'fsr1';
    const extraDisplay = groups.flatMap((g) => g.fields).filter((f) =>
      ['gamescope_filter', 'gamescope_fullscreen', 'gamescope_borderless', 'gamescope_grab_cursor', 'gamescope_hdr', 'gamescope_vrr', 'gamescope_wayland', 'shader_cache'].includes(f.id));
    return (
      <div>
        <AtlasSSwitch
          label={scope?.field.label ?? 'Gamescope'}
          desc={scope?.field.description ?? t('atlas:display.gamescopeDesc')}
          checked={!!draft.use_gamescope}
          disabled={!scope}
          onChange={() => changeChoice('use_gamescope', !draft.use_gamescope)}
        />
        {(res || rate) && (
          <AtlasSRow label={t('atlas:display.resolution')} desc={res?.field.description}>
            <div className="flex gap-2">
              {res && <AtlasChoiceSelect field={res.field} value={choiceValue(res.field)} onChange={(v) => changeChoice('gamescope_resolution', v)} width={130} />}
              {rate && <AtlasChoiceSelect field={rate.field} value={choiceValue(rate.field)} onChange={(v) => changeChoice('gamescope_rate', v)} width={110} />}
            </div>
          </AtlasSRow>
        )}
        {up ? (
          up.field.options && up.field.options.length <= 2 ? (
            <AtlasSSwitch
              label={t('atlas:display.upscaler')}
              desc={`${up.field.description ?? ''} ${t('atlas:display.needsScope')}`}
              checked={upOn}
              disabled={!draft.use_gamescope}
              onChange={() => changeChoice('upscaler', upOn ? 'native' : 'fsr1')}
            />
          ) : (
            <AtlasSRow label={up.field.label} desc={up.field.description}>
              <AtlasChoiceSelect field={up.field} value={choiceValue(up.field)} onChange={(v) => changeChoice('upscaler', v)} />
            </AtlasSRow>
          )
        ) : null}
        {extraDisplay.map((f) => f.kind === 'bool' ? (
          <AtlasSSwitch
            key={f.id}
            label={f.label}
            desc={f.description}
            checked={!!(draftRec[f.id] ?? f.default)}
            onChange={() => changeChoice(f.id, !(draftRec[f.id] ?? f.default))}
          />
        ) : (
          <AtlasSRow key={f.id} label={f.label} desc={f.description}>
            <AtlasChoiceSelect field={f} value={String(draftRec[f.id] ?? f.default ?? '')} onChange={(v) => changeChoice(f.id, v)} />
          </AtlasSRow>
        ))}
      </div>
    );
  }

  if (cat === 'saves') {
    // The tab's own pieces (the lab's SavesTable over .tbl, with .srow
    // controls): the backups table, then the cloud status, backup action
    // and mode as one slim row. No uppercase badges or mono hints.
    return (
      <AtlasSavesBlock game={{ ...game, settings: current }} plugin={p.plugin} onNotify={p.onNotify} />
    );
  }

  if (cat === 'layers') {
    const dx = fieldOf(groups, 'dx_layer');
    const sync = fieldOf(groups, 'sync_mode');
    const flags = ['proton_enable_nvapi', 'proton_hide_nvidia_gpu', 'proton_enable_wayland', 'proton_no_d3d11', 'proton_no_d3d12', 'proton_force_large_address_aware', 'proton_old_gl_string']
      .map((id) => fieldOf(groups, id)).filter((x): x is NonNullable<typeof x> => !!x);
    return (
      <div>
        {native && <p className="py-2 text-[12.5px] text-text-body">{t('gameSettings:runner.nativeNote')}</p>}
        {dx && (
          <AtlasSRow label={dx.field.label} desc={dx.field.description}>
            <AtlasChoiceSelect field={dx.field} value={choiceValue(dx.field)} onChange={(v) => changeChoice('dx_layer', v)} />
          </AtlasSRow>
        )}
        {sync && (
          <AtlasSRow label={sync.field.label} desc={sync.field.description}>
            <AtlasChoiceSelect field={sync.field} value={choiceValue(sync.field)} onChange={(v) => changeChoice('sync_mode', v)} />
          </AtlasSRow>
        )}
        {flags.map(({ field }) => (
          <AtlasSSwitch
            key={field.id}
            label={field.description || field.label}
            hint={field.label}
            checked={!!(draftRec[field.id] ?? field.default)}
            disabled={native}
            onChange={() => changeChoice(field.id, !(draftRec[field.id] ?? field.default))}
          />
        ))}
      </div>
    );
  }

  if (cat === 'fixes') {
    const eac = fieldOf(groups, 'eac_runtime');
    const be = fieldOf(groups, 'battleye_runtime');
    const viaSteam = game.source === 'steam' && !game.executable;
    const known = new Set(['runner_id', 'use_gamemode', 'use_mangohud', 'mangohud_preset', 'use_gamescope', 'gamescope_resolution', 'gamescope_rate', 'gamescope_filter', 'gamescope_fullscreen', 'gamescope_borderless', 'gamescope_grab_cursor', 'gamescope_hdr', 'gamescope_vrr', 'gamescope_wayland', 'gamescope_args', 'gamescope_w', 'gamescope_h', 'gamescope_r', 'upscaler', 'dx_layer', 'sync_mode', 'shader_cache', 'proton_enable_nvapi', 'proton_hide_nvidia_gpu', 'proton_enable_wayland', 'proton_no_d3d11', 'proton_no_d3d12', 'proton_force_large_address_aware', 'proton_old_gl_string', 'proton_log', 'wine_prefix', 'env_vars', 'launch_arguments', 'working_dir', 'eac_runtime', 'battleye_runtime']);
    const leftover = groups.flatMap((g) => g.fields).filter((f) => !known.has(f.id));
    const tricks = draft.winetricks ?? [];
    return (
      <div>
        {eac && (
          <AtlasSSwitch label={eac.field.label} desc={eac.field.description} checked={!!draft.eac_runtime} disabled={native} onChange={() => set('eac_runtime', !draft.eac_runtime)} />
        )}
        {be && (
          <AtlasSSwitch label={be.field.label} desc={be.field.description} checked={!!draft.battleye_runtime} disabled={native} onChange={() => set('battleye_runtime', !draft.battleye_runtime)} />
        )}
        <div className="py-2">
          <TweaksPanel
            framed={false}
            atlas
            game={{ ...game, settings: current }}
            tweaks={native ? p.tweaks.filter((tw) => !Object.keys(tw.env ?? {}).some((k) => /^(PROTON_|WINE_)/.test(k))) : p.tweaks}
            viaSteam={viaSteam}
            onChange={p.onChangeSettings}
            onOpenSettings={p.onOpenGeneral}
          />
        </div>
        <AtlasSRow label="winetricks" desc={t('gameSettings:winetricks.note')} wide>
          <div className="flex flex-wrap gap-1.5">
            {['vcrun2022', 'vcrun2019', 'd3dx9', 'd3dcompiler_47', 'dotnet48', 'corefonts', 'xact', 'mfc140'].map((v) => {
              const on = tricks.includes(v);
              return (
                <button key={v} aria-pressed={on} onClick={() => set('winetricks', on ? tricks.filter((x) => x !== v) : [...tricks, v])}
                  className={cx('rounded-lg px-2.5 py-1.5 font-sans text-[13px] transition', on ? 'bg-accent/[0.22] text-text ring-1 ring-accent/50' : 'bg-surface-strong text-text-muted hover:bg-surface-strong')}>
                  {on ? '✓ ' : '+ '}{v}
                </button>
              );
            })}
          </div>
        </AtlasSRow>
        {leftover.map((f) => f.kind === 'bool' ? (
          <AtlasSSwitch key={f.id} label={f.description || f.label} hint={f.description ? f.label : undefined} checked={!!(draftRec[f.id] ?? f.default)} onChange={() => changeChoice(f.id, !(draftRec[f.id] ?? f.default))} />
        ) : f.kind === 'select' ? (
          <AtlasSRow key={f.id} label={f.label} desc={f.description}>
            <AtlasChoiceSelect field={f} value={String(draftRec[f.id] ?? f.default ?? '')} onChange={(v) => changeChoice(f.id, v)} />
          </AtlasSRow>
        ) : (
          <AtlasSRow key={f.id} label={f.label} desc={f.description}>
            <Input aria-label={f.label} value={String(draftRec[f.id] ?? '')} placeholder={f.placeholder} spellCheck={false}
              onChange={(e) => changeChoice(f.id, e.target.value)} className="w-[190px] font-sans text-[13px]" />
          </AtlasSRow>
        ))}
      </div>
    );
  }

  if (cat === 'env') {
    return <AtlasEnvBody p={p} groups={groups} />;
  }

  if (cat === 'launch') {
    return <AtlasLaunchBody p={p} groups={groups} />;
  }

  // Game group: the old tabs, restyled in the same accordion.
  switch (cat) {
    case 'versions':
      return p.plugin ? <VersionsDialog embedded game={game} plugin={p.plugin} onClose={p.onClose} onChanged={p.onChanged} onNotify={p.onNotify} /> : null;
    case 'options':
      return p.plugin ? <InstallOptionsDialog embedded game={game} plugin={p.plugin} mode="update" onClose={p.onClose} onChanged={p.onChanged} onNotify={p.onNotify} /> : null;
    case 'plugin':
      return p.plugin ? <GameSettingsPanel game={game} plugin={p.plugin} onNotify={p.onNotify} framed={false} /> : null;
    case 'customization':
      return (
        <>
          <CustomizationTab game={game} onChanged={p.onCustomized} />
          <LegalSection game={game} plugin={p.plugin} />
        </>
      );
    default:
      return (
        <MaintenanceTab game={game} plugin={p.plugin} job={p.job} jobs={p.jobs} onJob={p.onJob} onJobAction={p.onJobAction}
          onNotify={p.onNotify} onClose={p.onClose} onRemove={p.onRemove} running={p.running} />
      );
  }
}

// ---------------------------------------------------------------------------
// The drawer: groups, one open at a time, sticky dirty footer.

export function AtlasDrawer(props: Omit<AtlasBodyProps, 'cat' | 'onOpenGeneral' | 'groups'> & {
  tabs: GameSettingsTab[];
  initialTab?: GameSettingsTab;
  open: AtlasCategory | null;
  setOpen: (c: AtlasCategory | null) => void;
}) {
  const { t } = useTranslation(['gameSettings', 'atlas', 'common']);
  const { tabs, open, setOpen, game, runners, tweaks, job, shared } = props;
  const groups = useRunnerGroups(game);
  // The open accordion shows the lab's accent edge: focusing its header
  // paints the 1.5px accent ring (lab .acc-h:focus on open).
  useEffect(() => {
    if (!open) return;
    const id = window.requestAnimationFrame(() => {
      document.getElementById(`gs-acc-${open}`)?.focus({ preventScroll: true });
    });
    return () => window.cancelAnimationFrame(id);
  }, [open]);
  const summary = (cat: AtlasCategory) => {
    if ((['versions', 'options', 'plugin', 'customization', 'maintenance'] as AtlasCategory[]).includes(cat)) {
      if (cat === 'plugin') return props.plugin?.name ?? (t('gameSettings:summary.plugin') as string);
      if (cat === 'versions') {
        if (game.pinned) return t('gameSettings:summary.pinned', { version: game.pinned.id }) as string;
        return (game.update_available ? t('gameSettings:summary.update') : t('gameSettings:summary.latest')) as string;
      }
      if (cat === 'options') return t('gameSettings:summary.options') as string;
      if (cat === 'customization') {
        return game.customized?.length
          ? (t('gameSettings:summary.customized', { count: game.customized.length }) as string)
          : (t('gameSettings:summary.customization') as string);
      }
      return job
        ? (t('gameSettings:summary.job', { kind: job.kind, percent: Math.round(job.percent) }) as string)
        : (t('gameSettings:summary.maintenance') as string);
    }
    return atlasSummary(t as unknown as (k: string, o?: Record<string, unknown>) => string, cat, { game: { ...game, settings: shared.current } as Game, runners, tweaks, job });
  };
  const groupLabel = (level: AtlasLevel) =>
    level === 'game' ? (t('gameSettings:accordion.game') as string)
    : level === 'everyday' ? (t('gameSettings:accordion.everyday') as string)
    : (t('gameSettings:accordion.advanced') as string);

  return (
    <>
      <div data-testid="gs-accordion" className="min-h-0 min-w-0 flex-1 overflow-y-auto px-4 pt-1 pb-[30px] [scrollbar-width:thin] [scrollbar-color:var(--color-hairline-strong)_transparent]">
        {atlasGroups(tabs).map((group) => (
          <div key={group.level}>
            <div className="px-2 pb-1.5 pt-4 text-[12px] font-bold text-text-faint">{groupLabel(group.level)}</div>
            {group.cats.map((cat) => {
              const isOpen = open === cat;
              const Icon = ATLAS_ICONS[cat];
              return (
                <section key={cat} className={cx('rounded-[10px]', isOpen && 'bg-surface')}>
                  <h3 className="font-sans">
                    <button
                      type="button"
                      id={`gs-acc-${cat}`}
                      data-gs-accordion-header
                      aria-expanded={isOpen}
                      aria-controls={isOpen ? `gs-acc-panel-${cat}` : undefined}
                      aria-describedby={`gs-acc-sum-${cat}`}
                      onClick={() => setOpen(isOpen ? null : cat)}
                      className="flex h-[46px] w-full items-center gap-2.5 rounded-[10px] px-3 text-left outline-none hover:bg-surface focus:bg-surface focus:ring-[1.5px] focus:ring-accent"
                    >
                      <Icon className={cx('size-4 flex-none', isOpen ? 'text-accent' : 'text-text')} aria-hidden />
                      <span className="text-[13.5px] font-bold text-text">{atlasLabel(t as unknown as (k: string) => string, cat)}</span>
                      <span id={`gs-acc-sum-${cat}`} className="ml-auto max-w-[46%] truncate text-[12.5px] text-text-faint">{summary(cat)}</span>
                      <ChevronDown aria-hidden className={cx('size-[15px] flex-none text-text-faint transition-transform', isOpen && 'rotate-180')} />
                    </button>
                  </h3>
                  {isOpen && (
                    <div id={`gs-acc-panel-${cat}`} role="region" aria-labelledby={`gs-acc-${cat}`} className="px-3.5 pb-3 pl-[39px]">
                      <AtlasCategoryBody {...props} groups={groups} cat={cat} onOpenGeneral={() => setOpen('compat')} />
                    </div>
                  )}
                </section>
              );
            })}
          </div>
        ))}
      </div>
      {shared.dirty && (
        <div className="flex flex-none items-center justify-between gap-3 border-t border-hairline bg-raised px-4 py-2.5">
          <span className="text-[12.5px] text-text-muted">{t('atlas:footer.unsaved')}</span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => shared.discard()}>{t('atlas:footer.discard')}</Button>
            <Button variant="primary" size="sm" onClick={() => {
              // A failed save keeps its dirty state: the error toast (App)
              // explains, and the footer stays until the save lands.
              const r = props.onSaveSettings(shared.current) as unknown as Promise<void> | void;
              if (r && typeof (r as Promise<void>).then === 'function') (r as Promise<void>).then(() => shared.markClean(), () => {});
              else shared.markClean();
            }}>
              {t('atlas:footer.save')}
            </Button>
          </div>
        </div>
      )}
    </>
  );
}

/** The runner groups the drawer edits (runner.options). Shared by bodies. */
export function useRunnerGroups(game: Game): SettingGroup[] {
  const [groups, setGroups] = useState<SettingGroup[]>([]);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getRunnerOptions(game)).then(
      (g) => { if (live) setGroups((g as unknown as SettingGroup[]) ?? []); },
      () => { if (live) setGroups([]); },
    );
    return () => { live = false; };
  }, [game.id]); // eslint-disable-line react-hooks/exhaustive-deps
  return groups;
}

function AtlasEnvBody({ p, groups }: { p: AtlasBodyProps; groups: SettingGroup[] }) {
  const { game, shared } = p;
  const { t } = useTranslation(['gameSettings', 'atlas', 'common']);
  const { draft, setDraft, envRows, setEnvRows, nextRowId, current } = shared;
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));
  const log = fieldOf(groups, 'proton_log');
  return (
    <div>
      {log && (
        <AtlasSSwitch label={log.field.description || log.field.label} hint={log.field.label} checked={!!draft.proton_log} onChange={() => set('proton_log', !draft.proton_log)} />
      )}
      <AtlasSRow label={t('gameSettings:launch.prefixKicker')} desc={t('gameSettings:launch.prefixDefault')} wide>
        <div className="flex items-center gap-2">
          <Input aria-label={t('gameSettings:launch.prefixAria')} value={draft.wine_prefix} onChange={(e) => set('wine_prefix', e.target.value)}
            placeholder={t('gameSettings:launch.prefixPlaceholder')} spellCheck={false} className="min-w-0 flex-1 font-sans text-[13px]" />
          <Button variant="neutral" size="sm" className="flex-none"
            onClick={() => {
              Effect.runPromise(LauncherApi.pickFolder(t('gameSettings:launch.browsePrefixTitle'), draft.wine_prefix || draft.working_dir || game.executable))
                .then((picked) => { if (picked) set('wine_prefix', picked); }).catch(() => {});
            }}>{t('gameSettings:launch.browse')}</Button>
        </div>
        <div className="mt-2">
          <PrefixPresets game={game} current={current} dirty={shared.dirty} onApplied={(saved) => shared.applySaved({ ...game.settings, ...saved })} />
        </div>
      </AtlasSRow>
      <AtlasSRow label={t('gameSettings:launch.envKicker')} wide>
        <div className="space-y-1.5">
          {envRows.map((r, i) => {
            const err = envKeyError(envRows, i, t);
            return (
              <div key={r.id}>
                <div className="flex items-center gap-1.5">
                  <Input aria-label={t('gameSettings:launch.varNameAria')} aria-invalid={!!err} value={r.key}
                    onChange={(e) => setEnvRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, key: e.target.value } : x)))}
                    placeholder={t('gameSettings:launch.varNamePlaceholder')} spellCheck={false} className="w-[40%] flex-none font-sans text-[13px]" />
                  <Input aria-label={typeof t === 'function' ? (t('gameSettings:launch.varValueAria', { key: r.key || t('gameSettings:launch.varFallback') }) as string) : r.key} value={r.value}
                    onChange={(e) => setEnvRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, value: e.target.value } : x)))}
                    placeholder={t('gameSettings:launch.varValuePlaceholder')} spellCheck={false} className="min-w-0 flex-1 font-sans text-[13px]" />
                  <Button variant="ghost" size="icon" aria-label={t('gameSettings:launch.removeVar', { key: r.key || t('gameSettings:launch.varFallback') }) as string}
                    onClick={() => setEnvRows((rs) => rs.filter((x) => x.id !== r.id))} className="flex-none size-7">×</Button>
                </div>
                {err && <p role="status" className="mt-1 font-mono text-hint text-danger">{err}</p>}
              </div>
            );
          })}
        </div>
        <Button variant="ghost" size="sm" onClick={() => setEnvRows((rs) => [...rs, { id: nextRowId.current++, key: '', value: '' }])} className="mt-2">
          {t('gameSettings:launch.addVar')}
        </Button>
      </AtlasSRow>
    </div>
  );
}

function AtlasLaunchBody({ p, groups }: { p: AtlasBodyProps; groups: SettingGroup[] }) {
  const { game, shared } = p;
  const { t } = useTranslation(['gameSettings', 'atlas', 'common']);
  const { draft, setDraft, current } = shared;
  const draftRec = draft as unknown as Record<string, unknown>;
  const set = <K extends keyof GameSettings>(k: K, v: GameSettings[K]) => setDraft((d) => ({ ...d, [k]: v }));
  const changeChoice = (key: string, value: boolean | string | number) => set(key as keyof GameSettings, value as never);
  const clientLaunched = !!game.client_launch;
  const [preview, setPreview] = useState<{ cmd?: string; err?: string }>({});
  const [runnerEnv, setRunnerEnv] = useState<{ env: Record<string, string>; wrappers: string[] }>({ env: {}, wrappers: [] });
  const backend = useMemo(() => ({ ...game, settings: current }), [game, current]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(() => {
      Effect.runPromise(LauncherApi.previewCommand(backend)).then(
        (cmd) => !cancelled && setPreview({ cmd }),
        (e) => !cancelled && setPreview({ err: e instanceof Error ? e.message : String(e) }),
      );
    }, 300);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [backend]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!clientLaunched) return;
    let cancelled = false;
    const timer = setTimeout(() => {
      Effect.runPromise(LauncherApi.getRunnerEnv(backend)).then(
        (r) => !cancelled && setRunnerEnv(r),
        () => {},
      );
    }, 300);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [backend, clientLaunched]); // eslint-disable-line react-hooks/exhaustive-deps
  const copy = (text: string) => navigator.clipboard.writeText(text).then(p.onCopied).catch(() => {});
  const gargs = fieldOf(groups, 'gamescope_args');
  return (
    <div>
      {clientLaunched && (
        <div className="mb-2 rounded-xl bg-info/[0.07] p-3 ring-1 ring-info/25">
          <p className="text-[12.5px] leading-relaxed text-text-body">{t('gameSettings:steamNotice')}</p>
          <div className="mt-2 flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-lg bg-ground px-2.5 py-2 font-mono text-hint text-text-body select-text">
              {steamLaunchOptions(backend, runnerEnv)}
            </code>
            <Button variant="neutral" size="sm" onClick={() => copy(steamLaunchOptions(backend, runnerEnv))}>{t('gameSettings:steamCopy')}</Button>
          </div>
        </div>
      )}
      <AtlasSRow label={t('gameSettings:launch.argsKicker')} wide>
        <Input value={draft.launch_arguments} onChange={(e) => set('launch_arguments', e.target.value)}
          placeholder={t('gameSettings:launch.argsPlaceholder')} className="w-full font-sans text-[13px]" />
      </AtlasSRow>
      {!clientLaunched && (
        <AtlasSRow label={t('gameSettings:launch.workdirKicker')} desc={t('gameSettings:launch.workdirHint')} wide>
          <div className="flex items-center gap-2">
            <Input aria-label={t('gameSettings:launch.workdirAria')} value={draft.working_dir} onChange={(e) => set('working_dir', e.target.value)}
              placeholder={game.executable ? game.executable.split('/').slice(0, -1).join('/') || '/' : (t('gameSettings:launch.workdirPlaceholder') as string)}
              spellCheck={false} className="min-w-0 flex-1 font-sans text-[13px]" />
            <Button variant="neutral" size="sm" className="flex-none"
              onClick={() => {
                Effect.runPromise(LauncherApi.pickFolder(t('gameSettings:launch.browseWorkdirTitle') as string, draft.working_dir || game.executable))
                  .then((picked) => { if (picked) set('working_dir', picked); }).catch(() => {});
              }}>{t('gameSettings:launch.browse')}</Button>
          </div>
        </AtlasSRow>
      )}
      {gargs && (
        <AtlasSRow label={gargs.field.label} desc={gargs.field.description} wide>
          <Input aria-label={gargs.field.label} value={String(draftRec['gamescope_args'] ?? '')} onChange={(e) => changeChoice('gamescope_args', e.target.value)}
            spellCheck={false} className="w-full font-sans text-[13px]" />
        </AtlasSRow>
      )}
      <AtlasSRow label={t('gameSettings:preview.label')} wide>
        <div className="rounded-lg bg-ground p-2.5 font-mono text-[11.5px] leading-relaxed break-all ring-1 ring-hairline">
          <div className="mb-1 flex items-center justify-between text-text-muted">
            <span>{t('gameSettings:preview.label')}</span>
            {preview.cmd && (
              <Button variant="ghost" size="none" onClick={() => copy(preview.cmd!)} className="rounded px-1 py-0.5" aria-label={t('gameSettings:preview.copyLabel') as string}>⧉</Button>
            )}
          </div>
          <div className="max-h-56 overflow-y-auto select-text">
            {preview.err ? <div className="text-danger">{preview.err}</div> : <div className="text-text-muted">{preview.cmd ?? '…'}</div>}
          </div>
        </div>
      </AtlasSRow>
    </div>
  );
}
