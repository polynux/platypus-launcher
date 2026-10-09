import React, { useState } from 'react';
import { Effect } from 'effect';
import { Loader2 } from 'lucide-react';
import type { CloudConflict, CloudInfo, CloudStatus, Game } from '../types';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { LauncherApi } from '../effect/runtime';
import { fileSize, whenLabel } from '../lib/cloud';
import { MonoLabel, SegRow, cx } from './ui';

type Choice = 'local' | 'remote' | 'later';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface Props {
  game: Game;
  status: CloudStatus;
  onClose: () => void;
  /** Called with the game's new cloud info after files were settled. */
  onResolved: (info: CloudInfo) => void;
  /** Set when a launch is waiting on the answer: adds "Launch without resolving". */
  onLaunchAnyway?: () => void;
}

/** Saves that changed on this device and in the cloud: per file, both sides'
 *  sizes and dates and a choice (Keep this device / Keep cloud / Decide later).
 *  Hydra never picks for the user. */
export function CloudConflictDialog({ game, status, onClose, onResolved, onLaunchAnyway }: Props) {
  const conflicts = status.conflicts ?? [];
  const [choices, setChoices] = useState<Record<string, Choice>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const choiceOf = (path: string): Choice => choices[path] ?? 'later';
  const chosen = conflicts.filter((c) => choiceOf(c.path) !== 'later');
  const setAll = (v: Choice) => setChoices(Object.fromEntries(conflicts.map((c) => [c.path, v])));

  const apply = () => {
    setBusy(true);
    setError(null);
    const payload: Record<string, 'local' | 'remote'> = {};
    for (const c of chosen) payload[c.path] = choiceOf(c.path) as 'local' | 'remote';
    Effect.runPromise(LauncherApi.resolveCloudSaves(game, payload))
      .then(onResolved)
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  return (
    <Modal label="Cloud save conflict" onClose={onClose} size="lg">
      <h2 className="text-[19px] font-extrabold tracking-tight">Saves changed on both sides</h2>
      <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">
        {game.title} has {conflicts.length === 1 ? 'a save file' : `${conflicts.length} save files`} that changed on this device and in the cloud.
        Hydra won't overwrite either without your say. Keeping the cloud copy backs up the file on this device first;
        keeping this device's copy replaces the cloud one.
      </p>

      {conflicts.length > 1 && (
        <div className="mt-3 flex items-center gap-3 font-mono text-[11.5px] text-white/45">
          all files:
          <Button variant="link" onClick={() => setAll('local')}>keep this device</Button>
          <Button variant="link" onClick={() => setAll('remote')}>keep cloud</Button>
          <Button variant="link" onClick={() => setAll('later')}>decide later</Button>
        </div>
      )}

      <ul aria-label="Conflicting files" className="mt-4 space-y-3">
        {conflicts.map((c) => (
          <li key={c.path} className="rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.07]">
            <div className="truncate font-mono text-[12px] text-white/85 select-text" title={c.path}>{c.path}</div>
            <div className="mt-2.5 grid gap-2 sm:grid-cols-2">
              <Side title="This device" file={c.local} newer={newerSide(c) === 'local'} />
              <Side title="Cloud" file={c.remote} newer={newerSide(c) === 'remote'} from={c.remote.machine} />
            </div>
            <div className="mt-3">
              <SegRow<Choice>
                label={`Choice for ${c.path}`}
                value={choiceOf(c.path)}
                onChange={(v) => setChoices((prev) => ({ ...prev, [c.path]: v }))}
                options={[
                  { value: 'local', label: 'Keep this device' },
                  { value: 'remote', label: 'Keep cloud' },
                  { value: 'later', label: 'Decide later' },
                ]}
              />
            </div>
          </li>
        ))}
      </ul>

      {status.backupDir && <p className="mt-3 font-mono text-[11px] text-white/40 select-text">backups: {status.backupDir}</p>}
      {error && <p role="alert" className="mt-3 font-mono text-[11.5px] text-red-300">{error}</p>}

      <div className="mt-5 flex flex-wrap items-center justify-end gap-2">
        {onLaunchAnyway && (
          <Button variant="neutral" onClick={onLaunchAnyway} disabled={busy} className="mr-auto">
            Launch without resolving
          </Button>
        )}
        <Button variant="ghost" onClick={onClose} disabled={busy}>Close</Button>
        <Button variant="primary" onClick={apply} disabled={busy || chosen.length === 0}>
          {busy && <Loader2 className="h-4 w-4 animate-spin" />}
          {chosen.length === 0 ? 'Apply choices' : `Apply ${chosen.length} choice${chosen.length === 1 ? '' : 's'}`}
        </Button>
      </div>
    </Modal>
  );
}

function newerSide(c: CloudConflict): 'local' | 'remote' | null {
  if (c.local.mtime === c.remote.mtime || !c.local.mtime || !c.remote.mtime) return null;
  return c.local.mtime > c.remote.mtime ? 'local' : 'remote';
}

function Side({ title, file, newer, from }: { title: string; file: CloudConflict['local']; newer: boolean; from?: string }) {
  return (
    <div className={cx('rounded-xl px-3 py-2.5 ring-1', newer ? 'bg-amber-300/[0.06] ring-amber-300/25' : 'ring-white/[0.07]')}>
      <div className="flex items-center justify-between gap-2">
        <MonoLabel>{title}</MonoLabel>
        {newer && <Badge variant="accent">newer</Badge>}
      </div>
      <div className="mt-1.5 font-mono text-[12px] text-white/85">{fileSize(file.size)}</div>
      <div className="font-mono text-[11.5px] text-white/50">{whenLabel(file.mtime)}</div>
      {from && <div className="font-mono text-[11px] text-white/40">from {from}</div>}
    </div>
  );
}
