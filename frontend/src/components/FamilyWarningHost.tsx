import React, { useEffect, useState } from 'react';
import type { FamilyWarning, Game } from '../types';
import { onFamily } from '../effect/runtime';
import { countdownLabel } from '../lib/family';
import { Button } from '@/components/ds/button';

/** How long a warning without a countdown stays up, before it leaves by itself. */
const UNCOUNTED_MS = 15000;

interface Shown {
  warning: FamilyWarning;
  /** Seconds left on the countdown; a warning without a time has none. */
  left: number;
}

/** Listens for family warnings app-wide and shows the latest as a toast with a
 *  countdown. The game keeps running: the store disconnects the borrower when
 *  the owner takes the game back. A warning about a game that is not in the
 *  library is not shown, and one without a time leaves after a while. */
export function FamilyWarningHost({ games }: { games: Game[] }) {
  const [shown, setShown] = useState<Shown | null>(null);

  useEffect(() => onFamily((w) => setShown({ warning: w, left: w.seconds_left })), []);

  // Count down once a second; a counting toast leaves when its time is up.
  useEffect(() => {
    if (!shown || shown.left <= 0) return;
    const t = setTimeout(() => setShown((s) => (s && s.left > 0 ? { ...s, left: s.left - 1 } : s)), 1000);
    return () => clearTimeout(t);
  }, [shown]);

  // A warning without a countdown has no time of its own: it leaves by itself.
  useEffect(() => {
    if (!shown || shown.warning.seconds_left > 0) return;
    const t = setTimeout(() => setShown(null), UNCOUNTED_MS);
    return () => clearTimeout(t);
  }, [shown]);

  if (!shown) return null;
  const { warning, left } = shown;
  const title = games.find((g) => g.id === warning.game_id)?.title;
  if (!title) return null;
  const counting = warning.seconds_left > 0;
  if (counting && left <= 0) return null;
  return (
    <div
      role="alert"
      className="fixed bottom-4 left-4 z-[60] flex max-w-md items-start gap-3 rounded-[3px] border border-amber-300/40 bg-[#191410] px-3.5 py-2.5 font-mono text-[12px]"
      style={{ boxShadow: '0 10px 30px rgba(0,0,0,.5)' }}
    >
      <span className="h-1.5 w-1.5 mt-1.5 flex-none rounded-full bg-amber-300" />
      <span className="min-w-0 flex-1">
        <span className="block font-bold text-amber-200">{title}</span>
        <span className="block break-words text-white/75">{warning.message}</span>
        {counting && <span className="tabular-nums text-white/55">{countdownLabel(left)}</span>}
      </span>
      <Button variant="ghost" size="sm" onClick={() => setShown(null)}>Dismiss</Button>
    </div>
  );
}
