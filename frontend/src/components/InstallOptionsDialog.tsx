import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { ExternalLink, TriangleAlert } from 'lucide-react';
import type { Game, InstallChoices, OptionGroup, StorePlugin } from '../types';
import { Modal } from './Modal';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { formatBytes } from '../lib/games';
import { addedBytes, canPick, chosenIds, pickValue, setFlag, toggleValue } from '../lib/options';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface InstallOptionsDialogProps {
  game: Game;
  plugin: StorePlugin;
  /** install: the first install with these choices. update: change an installed game's options. */
  mode: 'install' | 'update';
  onClose: () => void;
  /** A job was queued: the library should reload. */
  onChanged: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

/** Choose what an install includes (language, add-ons, packs, toggles), and
 *  change it on an installed game. The store lists the options; Hydra remembers
 *  the choices and checks them again when the install is queued. */
export function InstallOptionsDialog({ game, plugin, mode, onClose, onChanged, onNotify }: InstallOptionsDialogProps) {
  const [groups, setGroups] = useState<OptionGroup[] | null>(null);
  const [choices, setChoices] = useState<InstallChoices>({});
  const [free, setFree] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getGameOptions(game)).then(
      (o) => { if (live) { setGroups(o.groups); setChoices(o.chosen); } },
      (e) => { if (live) setError(errMsg(e)); },
    );
    Effect.runPromise(LauncherApi.getInstallFreeSpace(game)).then(
      (bytes) => { if (live) setFree(bytes); },
      () => { /* the free space is a hint: the install checks it again */ },
    );
    return () => { live = false; };
  }, [game]);

  const installing = mode === 'install';
  const added = groups ? addedBytes(groups, choices) : 0;
  // The game's own size is known for an installed game; a fresh install adds
  // the options to what the store reports, which the queue checks again. An
  // installed game already has its own files on disk, so only the add-ons need room.
  const total = added + (game.size_bytes ?? 0);
  const needed = installing ? total : added;
  const short = free != null && needed > free;

  const submit = async () => {
    setBusy(true);
    try {
      if (installing) await Effect.runPromise(LauncherApi.installGameWithOptions(game, choices));
      else await Effect.runPromise(LauncherApi.updateGameWithOptions(game, choices));
      onNotify(`${installing ? 'Install' : 'Update'} of ${game.title} queued`, 'busy');
      onChanged();
      onClose();
    } catch (e) {
      onNotify(errMsg(e), 'err');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label={`Install options for ${game.title}`} onClose={onClose} className="max-w-[560px]" dismissable={!busy}>
      <h2 className="text-[20px] font-extrabold tracking-tight">{installing ? 'Install options' : 'Add-ons & options'}</h2>
      <p className="mt-1 text-[12.5px] text-white/50">
        {game.title} · {plugin.name}
      </p>

      {!groups && !error && <p className="mt-6 font-mono text-[11.5px] uppercase tracking-widest text-white/40">loading options…</p>}
      {error && (
        <p role="alert" className="mt-5 rounded-xl bg-red-500/10 px-4 py-3 text-[12.5px] text-red-200">
          Couldn't read the options: {error}
        </p>
      )}

      {groups && groups.length === 0 && (
        <p className="mt-5 rounded-xl bg-white/[0.04] px-4 py-3 text-[12.5px] text-white/60">
          {plugin.name} has nothing to choose for {game.title}.
        </p>
      )}

      {groups && groups.map((g) => (
        <section key={g.id} className="mt-5">
          <MonoLabel className="mb-1.5">{g.label ?? g.id}</MonoLabel>
          <OptionGroupBody group={g} choices={choices} onChange={setChoices} />
        </section>
      ))}

      {groups && groups.length > 0 && (
        <div className="mt-6 rounded-2xl bg-white/[0.04] px-4 py-3 font-mono text-[11.5px]">
          <div className="flex justify-between gap-4">
            <span className="text-white/45">Add-ons & packs</span>
            <span className="text-white/85" data-testid="added-size">{added > 0 ? `+${formatBytes(added)}` : '—'}</span>
          </div>
          {game.size_bytes != null && (
            <div className="mt-1.5 flex justify-between gap-4">
              <span className="text-white/45">Total size</span>
              <span className="text-white/85" data-testid="total-size">{formatBytes(total)}</span>
            </div>
          )}
          {free != null && (
            <div className="mt-1.5 flex justify-between gap-4">
              <span className="text-white/45">Free space</span>
              <span className={cx(short ? 'text-red-300' : 'text-white/85')} data-testid="free-space">{formatBytes(free)}</span>
            </div>
          )}
          {short && (
            <p role="alert" className="mt-2 flex items-center gap-1.5 text-red-300">
              <TriangleAlert className="h-3.5 w-3.5" /> Not enough space for these options.
            </p>
          )}
        </div>
      )}

      <div className="mt-6 flex justify-end gap-2.5">
        <Button variant="neutral" size="sm" disabled={busy} onClick={onClose}>Cancel</Button>
        <Button variant="primary" size="sm" disabled={busy || !groups || short} onClick={submit}>
          {installing ? 'Install' : 'Apply'}
        </Button>
      </div>
    </Modal>
  );
}

/** One group: a dropdown for select, a checkbox list for multi, a switch for bool. */
function OptionGroupBody({ group, choices, onChange }: {
  group: OptionGroup;
  choices: InstallChoices;
  onChange: (next: InstallChoices) => void;
}) {
  const values = group.values ?? [];
  if (group.kind === 'bool') {
    const on = choices[group.id] === true;
    return (
      <label className="flex items-center justify-between gap-3 text-[12.5px] text-white/75">
        {group.label ?? group.id}
        <Switch aria-label={group.label ?? group.id} checked={on} onCheckedChange={(v) => onChange(setFlag(choices, group, v))} />
      </label>
    );
  }
  if (group.kind === 'select') {
    const current = chosenIds(group, choices)[0] ?? '';
    return (
      <Select value={current} onValueChange={(v) => onChange(pickValue(choices, group, v))}>
        <SelectTrigger aria-label={group.label ?? group.id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {values.map((v) => <SelectItem key={v.id} value={v.id}>{v.label ?? v.id}</SelectItem>)}
        </SelectContent>
      </Select>
    );
  }
  const picked = chosenIds(group, choices);
  return (
    <ul className="space-y-1.5">
      {values.map((v) => {
        const allowed = canPick(group, v);
        const checked = picked.includes(v.id);
        return (
          <li key={v.id} className={cx('flex items-center gap-2.5 text-[12.5px]', allowed ? 'text-white/75' : 'text-white/35')}>
            <input
              type="checkbox"
              aria-label={v.label ?? v.id}
              checked={checked}
              disabled={!allowed}
              onChange={(e) => onChange(toggleValue(group, choices, v.id, e.target.checked))}
              className="h-3.5 w-3.5 accent-amber-300"
            />
            <span className="min-w-0 flex-1 truncate">{v.label ?? v.id}</span>
            {v.installed && <Badge variant="ok">installed</Badge>}
            {!v.owned && group.dlc && <Badge variant="neutral">not owned</Badge>}
            {v.sizeBytes != null && v.sizeBytes > 0 && <span className="font-mono text-[11px] text-white/45">{formatBytes(v.sizeBytes)}</span>}
            {!v.owned && group.dlc && v.storeUrl && (
              <Button variant="link" size="sm" onClick={() => BrowserOpenURL(v.storeUrl!)} className="gap-1">
                Store <ExternalLink className="h-3 w-3" />
              </Button>
            )}
          </li>
        );
      })}
    </ul>
  );
}
