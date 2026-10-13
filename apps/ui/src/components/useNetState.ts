import { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import { onEvent } from '../effect/bridge';
import type { types as gen } from '../gen/models';

/** Whether the machine looks online (issue #44). Reads GetNetState once,
 *  then follows the "netstate" event. Defaults to online until the first
 *  answer, so server-side renders never flash the offline indicator.
 *
 *  The bridge and the API call are both guarded: component tests mock the
 *  runtime module without the netstate surface, and outside the shell there
 *  is no backend at all. Either way the hook reads as online. */
export function useNetState(): { online: boolean } {
  const [online, setOnline] = useState(true);
  useEffect(() => {
    let live = true;
    // The event wins over the initial read: a transition that lands while
    // the read is in flight must not be overwritten by the stale answer.
    let seenEvent = false;
    const off = onEvent<gen.NetState>('netstate', (s) => {
      seenEvent = true;
      if (live) setOnline(s?.online ?? true);
    });
    const getState = (LauncherApi as Partial<typeof LauncherApi>).getNetState;
    const read = getState ? getState() : Effect.succeed({ online: true, since: 0 });
    Effect.runPromise(read as Effect.Effect<gen.NetState, Error>).then(
      (s) => {
        if (live && !seenEvent) setOnline(s?.online ?? true);
      },
      () => {},
    );
    return () => {
      live = false;
      off();
    };
  }, []);
  return { online };
}
