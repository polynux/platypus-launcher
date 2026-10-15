// i18n: converted
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { Button } from '@/components/ds/button';
import { LauncherApi } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';
import type { CrashHints, Game } from '../types';

export interface CrashHintsProps {
  game: Game;
  hints: CrashHints;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** Opens the game's settings, where the runner is chosen. */
  onRunner: () => void;
}

type T = (key: string, vars?: Record<string, unknown>) => string;

// One explanation per rule id (internal/crashhints). Each key is written out so
// the catalogue check sees it used.
function ruleText(t: T, rule: string, dll: string): string {
  switch (rule) {
    case 'vcredist':
      return t('game:crash.rule.vcredist', { dll });
    case 'directx':
      return t('game:crash.rule.directx', { dll });
    case 'media':
      return t('game:crash.rule.media');
    case 'dotnet':
      return t('game:crash.rule.dotnet');
    case 'vulkan':
      return t('game:crash.rule.vulkan');
    case 'anticheat':
      return t('game:crash.rule.anticheat');
    case 'missingDll':
      return t('game:crash.rule.missingDll', { dll });
    default:
      return rule;
  }
}

/** What Platypus found in the log of a launch that ended early (issue #46).
 *  The log stays on this machine; the buttons are the only things it does. */
export function CrashHintsPanel({ game, hints, onNotify, onRunner }: CrashHintsProps) {
  const { t } = useTranslation();
  const install = (verb: string) =>
    Effect.runPromise(LauncherApi.runWinetricksVerb(game, verb))
      .then(() => onNotify?.(t('game:crash.started', { verb })))
      .catch((e) => onNotify?.(coreErrMsg(e), 'err'));
  const openLog = () => Effect.runPromise(LauncherApi.openCrashLog(game)).catch((e) => onNotify?.(coreErrMsg(e), 'err'));
  return (
    <section className="pb-6" aria-labelledby="crash-heading">
      <h2 id="crash-heading" className="mb-1 text-[15px] font-bold">
        {t('game:crash.heading')}
      </h2>
      <p className="mb-3 text-[12.5px] text-white/55">{t('game:crash.intro')}</p>
      <ul className="space-y-3">
        {hints.hints.map((h, i) => (
          <li key={`${h.rule}-${h.param ?? ''}-${h.verb ?? ''}-${i}`} className="text-[13px] leading-relaxed text-white/75">
            <p>{ruleText(t, h.rule, h.param ?? '')}</p>
            <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1">
              {h.verb && hints.winetricks && (
                <Button variant="link" size="none" onClick={() => install(h.verb ?? '')} className="font-mono text-[12px] underline underline-offset-2">
                  {t('game:crash.action.winetricks', { verb: h.verb })}
                </Button>
              )}
              {h.verb && !hints.winetricks && <span className="font-mono text-[12px] text-sky-200/80">{t('game:crash.manual', { verb: h.verb })}</span>}
              {h.action === 'runner' && (
                <Button variant="link" size="none" onClick={onRunner} className="font-mono text-[12px] underline underline-offset-2">
                  {t('game:crash.action.runner')}
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
      <Button variant="link" size="none" onClick={openLog} className="mt-3 font-mono text-[12px] underline underline-offset-2">
        {t('game:crash.action.openLog')}
      </Button>
    </section>
  );
}
