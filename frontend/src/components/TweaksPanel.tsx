import React from 'react';
import type { Game, GameSettings, Tweak } from '../types';
import { Button } from '@/components/ds/button';
import { MonoLabel, Panel, TogglePill, cx } from './ui';
import { setTweak, tweakApplied } from '../lib/settings';

/** The tweaks Platypus suggests for this machine, one switch each. `framed` wraps
 *  them in a panel of their own; the settings dialog's tab doesn't need one. */
export function TweaksPanel({
  game,
  tweaks,
  viaSteam,
  onChange,
  onOpenSettings,
  framed = true,
}: {
  game: Game;
  tweaks: Tweak[];
  viaSteam: boolean;
  onChange: (s: GameSettings, what: string) => void;
  /** Takes the user to the general settings, where Steam's launch options are. */
  onOpenSettings: () => void;
  framed?: boolean;
}) {
  const groups: [string, Tweak[]][] = [
    ['recommended', tweaks.filter((t) => t.kind === 'recommended')],
    ['if something breaks', tweaks.filter((t) => t.kind === 'troubleshoot')],
  ];
  const body = (
    <>
      <p className="text-[12px] leading-relaxed text-white/55">
        Based on this machine, not the game's rating: a platinum game can still need them on your GPU.
        {viaSteam && (
          <>
            {' '}Steam applies these only through the game's{' '}
            <Button variant="ghost" size="none" onClick={onOpenSettings} className="inline-flex rounded-none px-0 text-[12px] font-normal text-inherit underline decoration-white/30 underline-offset-2 hover:bg-transparent hover:text-white/80">launch options</Button>.
          </>
        )}
      </p>
      {groups.map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label} className="mt-3">
            <MonoLabel className="mb-1.5">{label}</MonoLabel>
            <div className="divide-y divide-white/[0.07]">
              {list.map((t) => {
                const on = tweakApplied(game.settings, t);
                return (
                  <button
                    key={t.id}
                    role="switch"
                    aria-checked={on}
                    onClick={() => onChange(setTweak(game.settings, t, !on), `${on ? 'Removed' : 'Applied'} "${t.title}"`)}
                    className="flex w-full items-start gap-3 py-2.5 text-left transition hover:text-white"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-[12.5px] font-semibold text-white/90">{t.title}</span>
                      <span className="block text-[12px] leading-snug text-white/50">{t.reason}</span>
                      {t.env && Object.keys(t.env).length > 0 && (
                        <span className="mt-1 block truncate font-mono text-[11px] text-sky-200/60">
                          {Object.entries(t.env).map(([k, v]) => `${k}=${v}`).join(' ')}
                        </span>
                      )}
                    </span>
                    <span className="flex flex-none pt-0.5"><TogglePill on={on} /></span>
                  </button>
                );
              })}
            </div>
          </div>
        )
      )}
    </>
  );
  return framed ? (
    <Panel className="bg-white/[0.05]">
      <h2 className="mb-1 text-[15px] font-bold">Suggested tweaks</h2>
      {body}
    </Panel>
  ) : body;
}
