// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { ExternalLink, TriangleAlert } from 'lucide-react';
import type { Game, InstallChoices, OptionGroup, StorePlugin } from '../types';
import { ModalOrInline } from './Modal';
import { InstallLocationPicker } from './InstallLocationPicker';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Switch } from '@/components/ds/switch';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { openExternal } from '../lib/external';
import { formatList } from '../lib/format';
import { formatBytes } from '../lib/games';
import { ensureTerms } from '../lib/termsRequest';
import { addedBytes, canPick, chosenIds, pickValue, removedAddOns, sameChoices, setFlag, toggleValue } from '../lib/options';
import { ErrorNotice } from './ErrorNotice';

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
  /** Show the content inline (a tab of the game settings) instead of in a dialog of its own. */
  embedded?: boolean;
}

/** Choose what an install includes (language, add-ons, packs, toggles), and
 *  change it on an installed game. The store lists the options; Platypus remembers
 *  the choices and checks them again when the install is queued. */
export function InstallOptionsDialog({ game, plugin, mode, onClose, onChanged, onNotify, embedded }: InstallOptionsDialogProps) {
  const { t } = useTranslation(['gameOps', 'common']);
  const [groups, setGroups] = useState<OptionGroup[] | null>(null);
  const [choices, setChoices] = useState<InstallChoices>({});
  // What the choices were when the dialog opened: an update compares with it.
  const [saved, setSaved] = useState<InstallChoices>({});
  const [free, setFree] = useState<number | null>(null);
  // Where a fresh install goes: '' is the store's own folder. The room there replaces the default's.
  const [folder, setFolder] = useState('');
  const [folderFree, setFolderFree] = useState<number | null>(null);
  const [baseSize, setBaseSize] = useState<number | null>(null);
  // The removal the user has already been warned about (see removed below).
  const [confirmed, setConfirmed] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getGameOptions(game)).then(
      (o) => { if (live) { setGroups(o.groups); setChoices(o.chosen); setSaved(o.chosen); } },
      (e) => { if (live) setError(errMsg(e)); },
    );
    Effect.runPromise(LauncherApi.getInstallFreeSpace(game)).then(
      (bytes) => { if (live) setFree(bytes); },
      () => { /* the free space is a hint: the install checks it again */ },
    );
    Effect.runPromise(LauncherApi.getInstallSize(game)).then(
      (bytes) => { if (live) setBaseSize(bytes); },
      () => { /* unknown: the total says so, and the install isn't blocked on it */ },
    );
    return () => { live = false; };
  }, [game]);

  const installing = mode === 'install';
  const added = groups ? addedBytes(groups, choices) : 0;
  // The base is the store's figure, or the library's. An installed game already
  // has its own files on disk, so only the add-ons need room. While the base
  // is unknown the total is unknown too, and a fresh install isn't blocked.
  const base = baseSize ?? game.size_bytes ?? null;
  const total = base == null ? null : base + added;
  const needed = installing ? total : added;
  const room = installing && folderFree != null && folderFree >= 0 ? folderFree : free;
  const short = room != null && needed != null && needed > room;
  // Apply does nothing for an update that changes nothing.
  const unchanged = !installing && groups != null && sameChoices(groups, saved, choices);
  // An update that switches off an add-on already on disk removes it: the
  // user confirms that with a second Apply, and the warning goes if they change the choice again.
  const removed = !installing && groups ? removedAddOns(groups, saved, choices) : [];
  const removedKey = removed.join('\n');

  const apply = () => {
    if (removed.length > 0 && confirmed !== removedKey) {
      setConfirmed(removedKey);
      return;
    }
    void submit();
  };

  const submit = async () => {
    setBusy(true);
    try {
      if (!(await ensureTerms(game))) return;
      if (installing) {
        await Effect.runPromise(folder ? LauncherApi.installGameWithOptionsTo(game, choices, folder) : LauncherApi.installGameWithOptions(game, choices));
      }
      else await Effect.runPromise(LauncherApi.updateGameWithOptions(game, choices));
      if (installing) onNotify(t('gameOps:options.queuedInstall', { title: game.title }), 'busy');
      else onNotify(t('gameOps:options.queuedUpdate', { title: game.title }), 'busy');
      onChanged();
      onClose();
    } catch (e) {
      onNotify(errMsg(e), 'err');
    } finally {
      setBusy(false);
    }
  };

  return (
    <ModalOrInline embedded={embedded} label={installing ? t('gameOps:options.installLabel', { title: game.title }) : t('gameOps:options.updateLabel', { title: game.title })} onClose={onClose} size="md" dismissable={!busy}>
      {!embedded && (
        <>
          <h2 className="text-[20px] font-extrabold tracking-tight">{installing ? t('gameOps:options.installTitle') : t('gameOps:options.updateTitle')}</h2>
          <p className="mt-1 text-[12.5px] text-white/70">
            {game.title} · {plugin.name}
          </p>
        </>
      )}

      {!groups && !error && <p className="mt-6 font-mono text-micro uppercase tracking-widest text-white/55">{t('gameOps:options.loading')}</p>}
      {error && (
        <ErrorNotice className="mt-5 rounded-xl bg-red-500/10 px-4 py-3" message={error} fallback={t('gameOps:options.loadError')} />
      )}

      {groups && groups.length === 0 && (
        <p className="mt-5 rounded-xl bg-white/[0.04] px-4 py-3 text-[12.5px] text-white/60">
          {t('gameOps:options.empty', { plugin: plugin.name, title: game.title })}
        </p>
      )}

      {groups && groups.map((g) => (
        <section key={g.id} className="mt-5">
          <MonoLabel className="mb-1.5">{g.label ?? g.id}</MonoLabel>
          <OptionGroupBody group={g} choices={choices} onChange={setChoices} />
        </section>
      ))}

      {groups && groups.length > 0 && (
        <div className="mt-6 rounded-2xl bg-white/[0.04] px-4 py-3 font-mono text-hint">
          <div className="flex justify-between gap-4">
            <span className="text-white/55">{t('gameOps:options.addonsLabel')}</span>
            <span className="text-white/85" data-testid="added-size">{added > 0 ? t('gameOps:options.addedSome', { size: formatBytes(added) }) : t('gameOps:options.addedNone')}</span>
          </div>
          <div className="mt-1.5 flex justify-between gap-4">
            <span className="text-white/55">{t('gameOps:options.totalLabel')}</span>
            <span className="text-white/85" data-testid="total-size">{total != null ? formatBytes(total) : t('gameOps:options.totalUnknown')}</span>
          </div>
          {room != null && room >= 0 && (
            <div className="mt-1.5 flex justify-between gap-4">
              <span className="text-white/55">{t('gameOps:options.freeLabel')}</span>
              <span className={cx(short ? 'text-red-300' : 'text-white/85')} data-testid="free-space">{formatBytes(room)}</span>
            </div>
          )}
          {short && (
            <p role="alert" className="mt-2 flex items-center gap-1.5 text-red-300">
              <TriangleAlert className="h-3.5 w-3.5" /> {t('gameOps:options.short')}
            </p>
          )}
        </div>
      )}

      {installing && (
        <div className="mt-6">
          <InstallLocationPicker
            game={game}
            needBytes={total}
            value={folder}
            onChange={(f, fr) => { setFolder(f); setFolderFree(fr); }}
            disabled={busy}
          />
        </div>
      )}

      {removed.length > 0 && (
        <p role="alert" className="mt-5 flex items-start gap-1.5 text-[12.5px] text-amber-200">
          <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>{t('gameOps:options.removal', { list: formatList(removed), title: game.title })}</span>
        </p>
      )}

      <div className="mt-6 flex justify-end gap-2.5">
        <Button variant="neutral" size="sm" disabled={busy} onClick={onClose}>{t('common:cancel')}</Button>
        <Button variant="primary" size="sm" disabled={busy || !groups || short || unchanged} onClick={apply}>
          {installing ? t('gameOps:options.install') : t('gameOps:options.apply')}
        </Button>
      </div>
    </ModalOrInline>
  );
}

/** One group: a dropdown for select, a checkbox list for multi, a switch for bool. */
function OptionGroupBody({ group, choices, onChange }: {
  group: OptionGroup;
  choices: InstallChoices;
  onChange: (next: InstallChoices) => void;
}) {
  const { t } = useTranslation('gameOps');
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
          <li key={v.id} className={cx('flex items-center gap-2.5 text-[12.5px]', allowed ? 'text-white/75' : 'text-white/55')}>
            <input
              type="checkbox"
              aria-label={v.label ?? v.id}
              checked={checked}
              disabled={!allowed}
              onChange={(e) => onChange(toggleValue(group, choices, v.id, e.target.checked))}
              className="h-3.5 w-3.5 accent-amber-300"
            />
            <span className="min-w-0 flex-1 truncate">{v.label ?? v.id}</span>
            {v.installed && <Badge variant="ok">{t('gameOps:options.installedBadge')}</Badge>}
            {!v.owned && group.dlc && <Badge variant="neutral">{t('gameOps:options.notOwned')}</Badge>}
            {v.sizeBytes != null && v.sizeBytes > 0 && <span className="font-mono text-hint text-white/55">{formatBytes(v.sizeBytes)}</span>}
            {!v.owned && group.dlc && v.storeUrl && (
              <Button variant="link" size="sm" onClick={() => openExternal(v.storeUrl)} className="gap-1">
                {t('gameOps:options.store')} <ExternalLink className="h-3 w-3" />
              </Button>
            )}
          </li>
        );
      })}
    </ul>
  );
}
