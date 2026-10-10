import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { Loader2, Play, RefreshCw, X } from 'lucide-react';
import type { CloudInfo, CloudMode, Game, StorePlugin } from '../types';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { CapabilitySurface } from '../lib/capabilities';
import { LauncherApi, onCloud, onSession } from '../effect/runtime';
import { MODE_HELP, STATE_CHIP, agoLabel, describeCounts } from '../lib/cloud';
import { CloudConflictDialog } from './CloudSavesConflict';
import { ErrorNotice, RawDetails } from './ErrorNotice';
import { MonoLabel, Panel, SegRow, cx } from './ui';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** Games already checked on their own this session: opening a game page checks
 *  once, then "Check now" is the user's. */
const autoChecked = new Set<string>();

interface Progress { phase?: string; done: number; total: number }

/** The "Cloud saves" panel of a game page: state, counts, the plugin's reason
 *  when it can't sync, Check now, the conflict dialog and the sync mode. It
 *  renders only for a game whose plugin offers the cloud-saves capability. */
export function CloudSavesPanel({ game, plugin, framed = true }: { game: Game; plugin: StorePlugin | undefined; framed?: boolean }) {
  return (
    <CapabilitySurface plugin={plugin} capability="cloud-saves" game={game}>
      <CloudSavesRow game={game} framed={framed} />
    </CapabilitySurface>
  );
}

function CloudSavesRow({ game, framed }: { game: Game; framed: boolean }) {
  const [info, setInfo] = useState<CloudInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [resolving, setResolving] = useState(false);
  const gameRef = useRef(game);
  gameRef.current = game;

  const refresh = useCallback(() => {
    return Effect.runPromise(LauncherApi.getCloudInfo(gameRef.current)).then(setInfo).catch((e) => setError(errMsg(e)));
  }, []);

  const check = useCallback(() => {
    setChecking(true);
    setError(null);
    return Effect.runPromise(LauncherApi.checkCloudSaves(gameRef.current))
      .then(setInfo)
      .catch((e) => setError(errMsg(e)))
      .finally(() => {
        setChecking(false);
        setProgress(null);
      });
  }, []);

  // Load the last known state; look once per session when there is none yet.
  useEffect(() => {
    let live = true;
    setInfo(null);
    setError(null);
    setProgress(null);
    Effect.runPromise(LauncherApi.getCloudInfo(game))
      .then((i) => {
        if (!live) return;
        setInfo(i);
        if (!i.status && i.mode !== 'off' && !autoChecked.has(game.id)) {
          autoChecked.add(game.id);
          void check();
        }
      })
      .catch((e) => live && setError(errMsg(e)));
    return () => { live = false; };
    // Reload for another game, not on every settings edit of this one.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  useEffect(
    () =>
      onCloud((e) => {
        if (e.game_id !== game.id) return;
        if (e.kind === 'progress') {
          setProgress({ phase: e.phase, done: e.done ?? 0, total: e.total ?? 0 });
          return;
        }
        setProgress(null);
        void refresh();
      }),
    [game.id, refresh],
  );

  // Starting and stopping the game change whether its saves can be settled.
  useEffect(
    () =>
      onSession((e) => {
        if (e.game_id === game.id) void refresh();
      }),
    [game.id, refresh],
  );

  const setMode = (v: CloudMode | 'default') => {
    Effect.runPromise(LauncherApi.setCloudGameMode(game.id, v === 'default' ? '' : v))
      .then(refresh)
      .catch((e) => setError(errMsg(e)));
  };

  const viaClient = () => {
    setError(null);
    Effect.runPromise(LauncherApi.launchGameViaClient(game)).catch((e) => setError(errMsg(e)));
  };

  const cancel = () => {
    Effect.runPromise(LauncherApi.cancelCloudSync(game.id)).catch((e) => setError(errMsg(e)));
  };

  const st = info?.status ?? null;
  const busy = checking || !!progress || !!info?.busy;
  const chip = st ? STATE_CHIP[st.state] : null;
  const client = st?.client?.running ? st.client : null;
  const when = st?.checkedAt ? agoLabel(st.checkedAt) : '';
  const running = !!info?.running;

  const Frame = framed ? CloudFrame : React.Fragment;
  return (
    <Frame>
      <div className="mb-2 flex items-center justify-between gap-3">
        <h2 className="text-[15px] font-bold">Cloud saves</h2>
        {chip ? (
          <Badge data-testid="cloud-state" variant={chip.variant}>{chip.label}</Badge>
        ) : (
          <Badge data-testid="cloud-state" variant="neutral">{info?.mode === 'off' ? 'off' : checking ? 'checking' : 'not checked'}</Badge>
        )}
      </div>

      {st && st.message && st.state !== 'synced' && (
        <>
          <p className="text-[12.5px] leading-relaxed text-white/65" data-testid="cloud-message">{st.message}</p>
          {st.detail && <div className="mt-1"><RawDetails text={st.detail} /></div>}
        </>
      )}
      {st && describeCounts(st.counts) && (
        <p className="mt-1.5 font-mono text-[11.5px] text-white/55" data-testid="cloud-counts">{describeCounts(st.counts)}</p>
      )}
      {when && <p className="mt-1 font-mono text-[11px] text-white/40">checked {when}</p>}

      {client && (
        <div role="note" className="mt-3 rounded-xl bg-amber-300/[0.07] px-3.5 py-2.5 text-[12px] leading-relaxed text-white/75 ring-1 ring-amber-300/25">
          <p>
            <span className="font-bold text-amber-200">{client.name} is running.</span>{' '}
            Platypus doesn't sync this game while it is open, because two programs writing the same saves cause conflicts.
            {client.canLaunch ? ` Launch it through ${client.name} instead, or close ${client.name} to sync.` : ` Close ${client.name} to sync.`}
          </p>
          {client.canLaunch && (
            <Button variant="neutral" size="sm" className="mt-2" onClick={viaClient}>
              <Play className="h-3.5 w-3.5" /> Launch through {client.name}
            </Button>
          )}
        </div>
      )}

      {busy && (
        <div className="mt-3" role="status">
          <div className="flex items-center justify-between gap-3 font-mono text-[11.5px] uppercase tracking-wider text-amber-200">
            <span className="flex items-center gap-2">
              <Loader2 className="h-3 w-3 animate-spin" />
              {progress?.phase ?? (info?.busy ? info.busy : 'checking')}
              {progress && progress.total > 0 ? ` · ${progress.done}/${progress.total}` : ''}
            </span>
            {!checking && (
              <Button variant="ghost" size="none" className="rounded p-0.5" aria-label="Cancel cloud sync" onClick={cancel}>
                <X className="h-3.5 w-3.5" />
              </Button>
            )}
          </div>
          <div className="mt-2 h-1.5 rounded-full bg-white/15" role="progressbar" aria-valuemin={0} aria-valuemax={progress?.total || undefined} aria-valuenow={progress?.done}>
            <div
              className={cx('h-full rounded-full bg-amber-300 transition-[width]', !progress?.total && 'animate-pulse')}
              style={{ width: progress?.total ? `${Math.round((progress.done / progress.total) * 100)}%` : '100%' }}
            />
          </div>
        </div>
      )}

      {error && (
        <ErrorNotice className="mt-2" message={error} fallback="Platypus couldn't reach the cloud saves just now." onRetry={() => void check()} />
      )}

      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="neutral" size="sm" onClick={() => void check()} disabled={checking || !!progress}>
          <RefreshCw className={cx('h-3.5 w-3.5', checking && 'animate-spin')} /> Check now
        </Button>
        {st?.state === 'conflict' && (st.conflicts?.length ?? 0) > 0 && (
          <Button variant="primary" size="sm" disabled={running} onClick={() => setResolving(true)}>Choose which saves to keep…</Button>
        )}
      </div>
      {running && st?.state === 'conflict' && (
        <p className="mt-2 text-[11.5px] text-white/45">{game.title} is running. Choose its saves after it exits.</p>
      )}

      {info && (
        <div className="mt-4 border-t border-white/[0.05] pt-3">
          <MonoLabel className="mb-1.5">sync for this game</MonoLabel>
          <SegRow<CloudMode | 'default'>
            label="Sync mode for this game"
            value={info.game_mode || 'default'}
            onChange={setMode}
            options={[
              { value: 'default', label: `Default · ${info.plugin_mode}`, title: 'Follow the store plugin\'s setting' },
              { value: 'off', label: 'Off' },
              { value: 'ask', label: 'Ask' },
              { value: 'auto', label: 'Auto' },
            ]}
          />
          <p className="mt-2 text-[11.5px] leading-snug text-white/45">{MODE_HELP[info.mode]}</p>
        </div>
      )}

      {resolving && st && (
        <CloudConflictDialog
          game={game}
          status={st}
          onClose={() => setResolving(false)}
          onResolved={(next) => {
            setInfo(next);
            if (next.status?.state !== 'conflict') setResolving(false);
          }}
        />
      )}
    </Frame>
  );
}

const CloudFrame = ({ children }: { children: React.ReactNode }) => <Panel className="bg-white/[0.05]">{children}</Panel>;
