// i18n: converted
import React, { useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { Activity, ChevronRight, Gauge, MonitorPlay, Sparkles, type LucideIcon } from 'lucide-react';
import type { Game, GameSettings, SettingGroup } from '../types';
import { LauncherApi } from '../effect/runtime';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { cx } from './ui';
import { cardCompat, TIER_LABEL } from '../lib/compat';

/** The four switches the flyout offers, and how they read off a game's settings. */
export type TuneKey = 'gamemode' | 'mangohud' | 'gamescope' | 'fsr';
export type TuneSwitches = Record<TuneKey, boolean>;

export type TunePreset = 'quality' | 'balanced' | 'performance';

/** A preset writes three of the four switches. MangoHud is left alone: the
 *  presets are about how the game runs, not what is drawn over it. */
export const TUNE_PRESETS: { id: TunePreset; set: Pick<TuneSwitches, 'gamemode' | 'gamescope' | 'fsr'> }[] = [
  { id: 'quality', set: { gamemode: true, gamescope: false, fsr: false } },
  { id: 'balanced', set: { gamemode: true, gamescope: true, fsr: false } },
  { id: 'performance', set: { gamemode: true, gamescope: true, fsr: true } },
];

/** The switches as the flyout shows them. FSR reads off while Lock resolution
 *  is off, because it needs it, even if `upscaler` still says FSR. */
export function tuneSwitches(s: GameSettings): TuneSwitches {
  const gamescope = !!s.use_gamescope;
  return {
    gamemode: !!s.use_gamemode,
    // The same rule the launch command uses (the runner's mangohudPreset):
    // a saved preset wins, and the old flag means "full".
    mangohud: s.mangohud_preset ? s.mangohud_preset !== 'off' : !!s.use_mangohud,
    gamescope,
    fsr: gamescope && (s.upscaler ?? 'native') === 'fsr1',
  };
}

/** The preset whose three switches match, or null (Custom). */
export function tunePreset(w: TuneSwitches): TunePreset | null {
  return TUNE_PRESETS.find((p) => p.set.gamemode === w.gamemode && p.set.gamescope === w.gamescope && p.set.fsr === w.fsr)?.id ?? null;
}

/** What picking a preset saves. A preset that turns gamescope or FSR off also
 *  writes `native`, so no stale FSR outlives the lock. */
export function applyTunePreset(s: GameSettings, id: TunePreset): GameSettings {
  const p = TUNE_PRESETS.find((x) => x.id === id);
  if (!p) return s;
  return { ...s, use_gamemode: p.set.gamemode, use_gamescope: p.set.gamescope, upscaler: p.set.fsr ? 'fsr1' : 'native' };
}

/** What one switch's click saves. Turning Lock resolution off clears FSR with
 *  it: upscaling through gamescope needs the lock. */
export function toggleTune(s: GameSettings, key: TuneKey): GameSettings {
  const on = !tuneSwitches(s)[key];
  switch (key) {
    case 'gamemode':
      return { ...s, use_gamemode: on };
    case 'mangohud':
      return { ...s, mangohud_preset: on ? 'full' : 'off' };
    case 'gamescope':
      return { ...s, use_gamescope: on, upscaler: on ? s.upscaler ?? 'native' : 'native' };
    case 'fsr':
      return { ...s, upscaler: on ? 'fsr1' : 'native' };
  }
}

/** Lock resolution's sub-line: the real resolution and refresh rate when the
 *  game has them saved, else what the switch does. */
export function gamescopeLine(s: GameSettings, t: (key: string, o?: Record<string, unknown>) => string): string {
  const resolution = s.gamescope_resolution
    ? s.gamescope_resolution.replace('x', '×')
    : s.gamescope_w > 0 && s.gamescope_h > 0 ? `${s.gamescope_w}×${s.gamescope_h}` : '';
  const rate = s.gamescope_rate || (s.gamescope_r > 0 ? String(s.gamescope_r) : '');
  if (resolution && rate) return t('tune:gamescope.known', { resolution, rate });
  if (resolution) return resolution;
  if (rate) return t('tune:gamescope.rateOnly', { rate });
  return t('tune:gamescope.sub');
}

/** The Tune quick settings flyout: a preset for people who don't want to think,
 *  the four switches behind it for people who do, and a way into everything
 *  else. The parent wraps it in a `relative` element; the panel opens under it.
 *
 *  Esc and a click outside close it. Mark the anchor (the Tune button) with
 *  `data-tune-anchor` so a click there toggles it instead of closing and
 *  reopening it. */
export function TuneFlyout({
  game,
  open,
  onClose,
  onAllSettings,
  onSaved,
}: {
  game: Game;
  open: boolean;
  onClose: () => void;
  /** The game page's own settings surface (the drawer in atlas). The flyout
   *  closes first so the panel never sits over it. */
  onAllSettings: () => void;
  /** The settings a write saved, for the owner's copy of the game. */
  onSaved?: (settings: GameSettings) => void;
}) {
  const { t } = useTranslation(['tune', 'game']);
  const ref = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<GameSettings>(game.settings);
  const [groups, setGroups] = useState<SettingGroup[] | null>(null);
  const [failed, setFailed] = useState(false);

  // The last settings the core confirmed, so a failed write can fall back
  // to what is persisted without discarding a newer optimistic write.
  // Every write builds from the latest optimistic draft (not the render
  // closure), and the writes run one at a time: overlapping toggles used to
  // roll back to a stale `before`.
  const confirmedRef = useRef<GameSettings>(game.settings);
  const draftRef = useRef<GameSettings>(draft);
  draftRef.current = draft;
  const writeSeq = useRef(0);
  const writeQueue = useRef(Promise.resolve());

  // Follow the game's saved settings (a save from the game page, or another
  // game): the flyout never shows a stale switch.
  const saved = JSON.stringify(game.settings);
  const last = useRef({ id: game.id, saved });
  useEffect(() => {
    if (last.current.id === game.id && last.current.saved === saved) return;
    last.current = { id: game.id, saved };
    confirmedRef.current = game.settings;
    setDraft(game.settings);
    setFailed(false);
  }, [game.id, saved]); // eslint-disable-line react-hooks/exhaustive-deps

  // The runner's own choices: which fields exist (and the runner pick's
  // options). A game the store client launches gets no `runner` group.
  useEffect(() => {
    if (!open) return;
    let live = true;
    Effect.runPromise(LauncherApi.getRunnerOptions(game)).then(
      (g) => live && setGroups(g),
      () => live && setGroups([]),
    );
    return () => { live = false; };
  }, [open, game.id]); // eslint-disable-line react-hooks/exhaustive-deps

  // Focus the first control on open (without scrolling the stage: the
  // flyout now renders outside any clipping ancestor).
  useEffect(() => {
    if (!open) return;
    ref.current?.querySelector<HTMLElement>('button:not(:disabled), [role="combobox"]')?.focus({ preventScroll: true });
  }, [open, game.id]);

  // Esc and a click outside close. Radix's own listbox handles Escape itself
  // (it preventDefaults), and the anchor toggles. The runner pick's portaled
  // listbox is not inside the flyout, so a pointer down there must not close
  // it before the selection lands.
  useEffect(() => {
    if (!open) return;
    const focusAnchor = () => {
      document.querySelector<HTMLElement>('[data-tune-anchor]')?.focus();
    };
    const down = (e: PointerEvent) => {
      const target = e.target instanceof Element ? e.target : null;
      if (!target || ref.current?.contains(target)) return;
      if (target.closest('[data-tune-anchor]')) return;
      if (target.closest('[role="listbox"], [data-radix-popper-content-wrapper]')) return;
      focusAnchor();
      onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented) return;
      if (e.target instanceof Element && e.target.closest('[role="listbox"], [data-radix-popper-content-wrapper]')) return;
      focusAnchor();
      onClose();
    };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('pointerdown', down);
      window.removeEventListener('keydown', key);
    };
  }, [open, onClose]);

  if (!open) return null;

  const write = (next: GameSettings) => {
    const mySeq = ++writeSeq.current;
    draftRef.current = next;
    setDraft(next);
    setFailed(false);
    const task = () =>
      Effect.runPromise(LauncherApi.saveGameSettings(game.id, next)).then(
        () => {
          confirmedRef.current = next;
          onSaved?.(next);
          if (mySeq === writeSeq.current) setFailed(false);
        },
        () => {
          // Only the latest write falls back; an older failure must not
          // wipe a newer optimistic draft queued behind it.
          if (mySeq === writeSeq.current) {
            draftRef.current = confirmedRef.current;
            setDraft(confirmedRef.current);
          }
          setFailed(true);
        },
      );
    writeQueue.current = writeQueue.current.then(task, task);
  };

  // A switch is offered only when the runner's schema has the field. Until the
  // schema arrives nothing is locked out.
  const offers = (group: string, field: string) =>
    !groups || !!groups.find((g) => g.id === group)?.fields.some((f) => f.id === field);
  const reason = groups && groups.length === 0 ? t('tune:unavailable.runner') : t('tune:unavailable.setting');

  const sw = tuneSwitches(draft);
  const preset = tunePreset(sw);
  const clientLaunched = !!game.client_launch;
  const runnerField = groups?.find((g) => g.id === 'runner')?.fields.find((f) => f.id === 'runner_id');

  const rows: { key: TuneKey; icon: LucideIcon; label: string; sub: string }[] = [
    { key: 'gamemode', icon: Gauge, label: t('tune:gamemode.label'), sub: t('tune:gamemode.sub') },
    { key: 'mangohud', icon: Activity, label: t('tune:mangohud.label'), sub: t('tune:mangohud.sub') },
    { key: 'gamescope', icon: MonitorPlay, label: t('tune:gamescope.label'), sub: gamescopeLine(draft, t) },
    { key: 'fsr', icon: Sparkles, label: t('tune:fsr.label'), sub: t('tune:fsr.sub') },
  ];

  // Why a switch can't be used here: the runner has no such field, the build
  // is native (no Proton, so no FSR), or FSR's lock is off.
  const why = (key: TuneKey): string | null => {
    if (key === 'gamemode' || key === 'gamescope') {
      return offers('wrappers', `use_${key}`) ? null : reason;
    }
    if (key === 'mangohud') return offers('graphics', 'mangohud_preset') ? null : reason;
    if (game.native) return t('tune:fsr.native');
    if (!offers('graphics', 'upscaler')) return reason;
    if (!sw.gamescope) return t('tune:fsr.needsLock');
    return null;
  };

  const compat = cardCompat(game);
  const tierLine = compat?.kind === 'native'
    ? t('game:compat.native')
    : compat?.kind === 'tier'
      ? t('tune:tier.rated', { tier: t(`game:tier.${compat.tier}`, { defaultValue: TIER_LABEL[compat.tier] ?? compat.tier }) })
      : t('tune:tier.none');

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label={t('tune:title', { title: game.title })}
      className="absolute left-0 top-[calc(100%+6px)] z-[60] w-[380px] animate-modal-in overflow-hidden rounded-[10px] bg-raised text-text ring-1 ring-hairline shadow-[0_18px_48px_rgba(0,0,0,.5)] motion-reduce:animate-none leading-[normal]"
    >
      <div className="border-b border-hairline px-3.5 py-3">
        <div className="mb-2 flex items-baseline justify-between gap-3">
          <span className="text-[13px] font-bold">{t('tune:preset.label')}</span>
          <small className="text-[12px] text-text-faint">{preset ? t('tune:preset.applies') : t('tune:preset.custom')}</small>
        </div>
        <div role="group" aria-label={t('tune:preset.label')} className="grid grid-cols-3 gap-1">
          {TUNE_PRESETS.map((p) => {
            const on = preset === p.id;
            return (
              <button
                key={p.id}
                type="button"
                aria-pressed={on}
                onClick={() => write(applyTunePreset(draftRef.current, p.id))}
                className={cx(
                  'h-[30px] rounded-md text-[12.5px] font-semibold ring-1 transition',
                  on ? 'bg-accent/20 text-text ring-accent/40' : 'text-text-muted ring-hairline hover:text-text',
                )}
              >
                {t(`tune:preset.${p.id}`)}
              </button>
            );
          })}
        </div>
      </div>

      <ul className="p-1">
        {rows.map(({ key, icon: Icon, label, sub }) => {
          const disabledReason = why(key);
          const on = sw[key] && !disabledReason;
          return (
            <li key={key}>
              <button
                type="button"
                role="switch"
                aria-checked={on}
                disabled={!!disabledReason}
                onClick={() => write(toggleTune(draftRef.current, key))}
                className="grid w-full grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2.5 rounded-md px-2.5 py-[7px] text-left transition hover:bg-surface-strong disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent"
              >
                <Icon aria-hidden className={cx('size-4', on ? 'text-accent' : 'text-text-faint')} />
                <span className="grid min-w-0">
                  <span className="text-[13px] font-semibold text-text">{label}</span>
                  <small className="truncate text-[12px] text-text-faint">{disabledReason ?? sub}</small>
                </span>
                <span aria-hidden className={cx('relative h-5 w-[34px] flex-none rounded-full transition-colors', on ? 'bg-accent' : 'bg-white/15')}>
                  <span className={cx('absolute left-0.5 top-0.5 h-4 w-4 rounded-full transition-transform', on ? 'translate-x-[14px] bg-on-accent' : 'bg-raised')} />
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {failed && <p role="alert" className="px-3.5 pb-2 text-[12px] text-danger">{t('tune:saveFailed')}</p>}

      <div className="mx-1 flex items-center justify-between gap-3 border-t border-hairline px-2.5 pb-3 pt-2.5">
        <span className="grid min-w-0">
          <span className="text-[13px] font-semibold text-text">{t('tune:runner.label')}</span>
          <small className="truncate text-[12px] text-text-faint">{tierLine}</small>
        </span>
        {clientLaunched ? (
          <small className="flex-none text-[12px] text-text-muted">{t('game:facts.steamRunner')}</small>
        ) : runnerField?.options?.length ? (
          <Select value={draft.runner_id || 'auto'} onValueChange={(v) => write({ ...draftRef.current, runner_id: v })}>
            <SelectTrigger aria-label={t('tune:runner.label')} className="h-8 w-[180px] flex-none font-sans text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {runnerField.options.map((o) => (
                <SelectItem key={o.id} value={o.id} className="font-sans text-[13px]">{o.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
      </div>

      <button
        type="button"
        onClick={() => { onClose(); onAllSettings(); }}
        className="grid w-full grid-cols-[1fr_auto] grid-rows-[auto_auto] items-center gap-x-2 border-t border-hairline px-3.5 py-2.5 text-left transition hover:bg-surface-strong"
      >
        <span className="font-bold">{t('tune:all')}</span>
        <small className="col-start-1 row-start-2 text-[12px] text-text-faint">{t('tune:allHint')}</small>
        <ChevronRight aria-hidden className="col-start-2 row-span-2 size-4 text-text-faint" />
      </button>
    </div>
  );
}
