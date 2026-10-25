// i18n: converted
import React from 'react';
import { useTranslation, Trans } from 'react-i18next';
import type { Game, GameSettings, Tweak } from '../types';
import { Button } from '@/components/ds/button';
import { MonoLabel, Panel, TogglePill, cx } from './ui';
import { setTweak, tweakApplied } from '../lib/settings';

/** The tweaks Platypus suggests for this machine, one switch each. `framed` wraps
 *  them in a panel of their own; the settings dialog's tab doesn't need one.
 *  `atlas` renders the /21 `.fix` rows: sans titles and muted descriptions,
 *  with the env hint in faint sans (no sky-blue mono). */
export function TweaksPanel({
  game,
  tweaks,
  viaSteam,
  onChange,
  onOpenSettings,
  framed = true,
  atlas = false,
}: {
  game: Game;
  tweaks: Tweak[];
  viaSteam: boolean;
  onChange: (s: GameSettings, what: string) => void;
  /** Takes the user to the general settings, where Steam's launch options are. */
  onOpenSettings: () => void;
  framed?: boolean;
  atlas?: boolean;
}) {
  const { t } = useTranslation('gameSettings');
  const groups: [string, Tweak[]][] = [
    [t('gameSettings:tweaks.recommended'), tweaks.filter((tw) => tw.kind === 'recommended')],
    [t('gameSettings:tweaks.troubleshoot'), tweaks.filter((tw) => tw.kind === 'troubleshoot')],
  ];
  const body = (
    <>
      <p className={atlas ? 'text-[12.5px] leading-relaxed text-text-muted' : 'text-[12px] leading-relaxed text-white/70'}>
        {t('gameSettings:tweaks.intro')}
        {viaSteam && (
          <>
            {' '}<Trans
              i18nKey="gameSettings:tweaks.steamNote"
              components={{
                lo: <Button variant="ghost" size="none" onClick={onOpenSettings} className={atlas ? 'inline-flex rounded-none px-0 text-[12.5px] font-normal text-inherit underline decoration-white/30 underline-offset-2 hover:bg-transparent hover:text-text' : 'inline-flex rounded-none px-0 text-[12px] font-normal text-inherit underline decoration-white/30 underline-offset-2 hover:bg-transparent hover:text-white/80'} />,
              }}
            />
          </>
        )}
      </p>
      {groups.map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label} className="mt-3">
            <MonoLabel className="mb-1.5">{label}</MonoLabel>
            <div className={atlas ? 'divide-y divide-hairline' : 'divide-y divide-white/[0.07]'}>
              {list.map((tw) => {
                const on = tweakApplied(game.settings, tw);
                return (
                  <button
                    key={tw.id}
                    role="switch"
                    aria-checked={on}
                    onClick={() => onChange(setTweak(game.settings, tw, !on), on
                      ? t('gameSettings:tweaks.removed', { title: tw.title })
                      : t('gameSettings:tweaks.applied', { title: tw.title }))}
                    className={atlas ? 'flex w-full items-start gap-3 py-[11px] text-left transition' : 'flex w-full items-start gap-3 py-2.5 text-left transition hover:text-white'}
                  >
                    <span className="min-w-0 flex-1">
                      <span className={atlas ? 'block text-[13px] font-semibold text-text' : 'block text-[12.5px] font-semibold text-white/90'}>{tw.title}</span>
                      <span className={atlas ? 'block text-[12.5px] leading-[1.45] text-text-muted' : 'block text-[12px] leading-snug text-white/55'}>{tw.reason}</span>
                      {tw.env && Object.keys(tw.env).length > 0 && (
                        <span className={atlas ? 'mt-1 block truncate font-sans text-[12px] text-text-faint' : 'mt-1 block truncate font-mono text-hint text-sky-200/60'}>
                          {Object.entries(tw.env).map(([k, v]) => `${k}=${v}`).join(' ')}
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
      <h2 className="mb-1 text-[15px] font-bold">{t('gameSettings:tweaks.title')}</h2>
      {body}
    </Panel>
  ) : body;
}
