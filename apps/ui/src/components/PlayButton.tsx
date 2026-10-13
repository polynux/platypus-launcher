// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, Play, Square } from 'lucide-react';
import type { SessionEvent } from '../types';
import { Button } from '@/components/ds/button';
import { cx } from './ui';

/** A graceful stop that takes longer than this offers a forced one. */
export const FORCE_STOP_AFTER_MS = 3000;

interface PlayButtonProps {
  session?: SessionEvent;
  launching: boolean;
  disabled: boolean;
  onPlay: () => void;
  onStop: () => void;
  onForceStop: () => void;
}

const stopBtn = 'gap-2 rounded-2xl px-6 py-3.5 text-[15px] font-extrabold disabled:pointer-events-auto disabled:cursor-default disabled:opacity-100';

/** The hero's main action: Play, then Stop once the game is up. */
export function PlayButton(p: PlayButtonProps) {
  const { t } = useTranslation('game');
  const state = p.session?.state;
  const stopping = state === 'stopping';
  const [canForce, setCanForce] = useState(false);
  useEffect(() => {
    setCanForce(false);
    if (!stopping) return;
    const id = setTimeout(() => setCanForce(true), FORCE_STOP_AFTER_MS);
    return () => clearTimeout(id);
  }, [stopping]);

  if (state === 'running' || state === 'starting') {
    const running = state === 'running';
    return (
      <div className="flex items-center gap-2.5">
        {running ? (
          <span className="flex items-center gap-2 rounded-2xl bg-emerald-400 px-6 py-3.5 text-[15px] font-extrabold text-[#0b1a12] shadow-[0_12px_30px_rgba(52,211,153,.25)]">
            <span className="h-2.5 w-2.5 rounded-full bg-[#0b1a12]" />
            {t('game:play.playing')}
          </span>
        ) : (
          <Button variant="primary" size="none" disabled title={p.session?.message} className={cx(stopBtn, 'disabled:opacity-80')}>
            <Loader2 className="h-4 w-4 animate-spin" />
            {t('game:play.starting')}
          </Button>
        )}
        <Button variant="neutral" size="none" onClick={p.onStop} className={cx(stopBtn, 'bg-white/[0.08] text-white/85 ring-1 ring-white/[0.12] hover:bg-red-400/20 hover:text-red-100 hover:ring-red-300/40')}>
          <Square className="h-3.5 w-3.5 fill-current" />
          {t('game:play.stop')}
        </Button>
      </div>
    );
  }

  if (stopping) {
    return (
      <div className="flex items-center gap-2.5">
        <Button variant="neutral" size="none" disabled className={cx(stopBtn, 'bg-white/[0.08] text-white/60 ring-1 ring-white/[0.12]')}>
          <Loader2 className="h-4 w-4 animate-spin" />
          {t('game:play.stopping')}
        </Button>
        {canForce && (
          <Button variant="danger" size="none" onClick={p.onForceStop} className={cx(stopBtn, 'bg-red-400/20 text-red-100 ring-1 ring-red-300/40 hover:bg-red-400/30')}>
            {t('game:play.forceStop')}
          </Button>
        )}
      </div>
    );
  }

  return (
    <Button
      variant="primary"
      size="none"
      onClick={p.onPlay}
      disabled={p.launching || p.disabled}
      className="gap-2 rounded-2xl px-8 py-3.5 text-[15px] shadow-[0_12px_30px_rgba(252,211,77,.25)] disabled:pointer-events-auto disabled:cursor-default disabled:opacity-80"
    >
      {p.launching ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4 fill-current" />}
      {p.launching ? t('game:play.launching') : t('game:play.play')}
    </Button>
  );
}
