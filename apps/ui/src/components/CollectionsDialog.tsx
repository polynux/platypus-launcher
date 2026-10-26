// i18n: converted
import React, { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import type { Collection, Game, SmartFilter } from '../types';
import { Modal, ModalHead, ModalFoot } from './Modal';
import { Button } from './ds/button';
import { Input } from './ds/input';
import { Switch } from './ds/switch';
import { MonoLabel, cx } from './ui';
import { ErrorNotice } from './ErrorNotice';
import { LauncherApi } from '../effect/runtime';
import { coreErrMsg } from '../lib/coreError';
import { collectionCounts, missingFrom, moveCollection } from '../lib/collections';
import { storeLabel } from '../lib/games';
import { TIER_FILTERS } from '../lib/compat';

/** The fields of a smart filter as the form holds them: strings and '' for "any". */
export interface SmartDraft {
  store: string;
  installed: '' | 'yes' | 'no';
  played: '' | 'yes' | 'no';
  badgeKind: string;
  badgeValue: string;
  status: string;
  minHours: string;
  yearFrom: string;
  yearTo: string;
  collection: string;
}

export const EMPTY_DRAFT: SmartDraft = { store: '', installed: '', played: '', badgeKind: '', badgeValue: '', status: '', minHours: '', yearFrom: '', yearTo: '', collection: '' };

const positive = (s: string): number => {
  const n = Math.floor(Number(s));
  return Number.isFinite(n) && n > 0 ? n : 0;
};

/** The filter a draft describes: only the fields the user set. A draft with
 *  none set is the zero filter, which makes a manual collection. */
export function smartFromDraft(d: SmartDraft): SmartFilter {
  const f: SmartFilter = {};
  if (d.store) f.store = d.store;
  if (d.installed) f.installed = d.installed === 'yes';
  if (d.played) f.played = d.played === 'yes';
  if (d.badgeKind) f.badge_kind = d.badgeKind;
  if (d.badgeValue) f.badge_value = d.badgeValue;
  if (d.status) f.status = d.status;
  const hours = Number(d.minHours);
  if (Number.isFinite(hours) && hours > 0) f.min_playtime_minutes = Math.round(hours * 60);
  if (positive(d.yearFrom)) f.year_from = positive(d.yearFrom);
  if (positive(d.yearTo)) f.year_to = positive(d.yearTo);
  if (d.collection) f.collection = d.collection;
  return f;
}

export function draftFromSmart(f: SmartFilter): SmartDraft {
  return {
    ...EMPTY_DRAFT,
    store: f.store ?? '',
    installed: f.installed === undefined ? '' : f.installed ? 'yes' : 'no',
    played: f.played === undefined ? '' : f.played ? 'yes' : 'no',
    badgeKind: f.badge_kind ?? '',
    badgeValue: f.badge_value ?? '',
    status: f.status ?? '',
    minHours: f.min_playtime_minutes === undefined ? '' : String(f.min_playtime_minutes / 60),
    yearFrom: f.year_from === undefined ? '' : String(f.year_from),
    yearTo: f.year_to === undefined ? '' : String(f.year_to),
    collection: f.collection ?? '',
  };
}

const STATUSES = ['none', 'backlog', 'playing', 'finished', 'abandoned'] as const;

/** A tier filter value's label: the library's own words for native and unrated games. */
function tierLabel(t: (key: string) => string, v: string): string {
  if (v === 'native') return t('library:tierNative');
  if (v === 'none') return t('library:tierNone');
  return t(`library:tierName.${v}`);
}

interface DialogProps {
  collections: Collection[];
  games: Game[];
  /** The game the picker is for. Absent: the manager (pin, order, rename, create, delete). */
  gameId?: string;
  mode?: 'new';
  onChanged: (list: Collection[]) => void;
  onClose: () => void;
}

/** Collections (issue #45). With a game: a checkbox per manual collection, and a
 *  new one that takes the game. Without: the manager for the sidebar. Every
 *  change answers the whole list, which goes back up through onChanged. */
export function CollectionsDialog({ collections, games, gameId, mode, onChanged, onClose }: DialogProps) {
  const { t } = useTranslation(['collections', 'library', 'common']);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [smart, setSmart] = useState(false);
  const [draft, setDraft] = useState<SmartDraft>(EMPTY_DRAFT);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState<SmartDraft>(EMPTY_DRAFT);
  const [editName, setEditName] = useState('');
  const game = gameId ? games.find((g) => g.id === gameId) : undefined;
  const nowSec = Math.floor(Date.now() / 1000);
  const counts = useMemo(() => collectionCounts(collections, games, nowSec), [collections, games, nowSec]);

  const label = (c: Collection) => (c.builtin ? t(`collections:builtin.${c.id}`) : c.name);

  const run = (call: Effect.Effect<Collection[], unknown>, after?: (list: Collection[]) => void) => {
    setBusy(true);
    setError(null);
    return Effect.runPromise(call).then(
      (list) => {
        onChanged(list);
        after?.(list);
      },
      (e) => setError(coreErrMsg(e)),
    ).finally(() => setBusy(false));
  };

  // A new collection takes the game when the picker is open for one: the new
  // id is the one the list did not have before.
  const create = (e: React.FormEvent) => {
    e.preventDefault();
    const clean = name.trim();
    if (!clean || busy) return;
    const f = smart ? smartFromDraft(draft) : {};
    if (smart && Object.keys(f).length === 0) {
      setError(t('collections:smart.needOne'));
      return;
    }
    const before = new Set(collections.map((c) => c.id));
    // Picking a game creates a manual collection and adds the game to it, in one call chain.
    const created = LauncherApi.createCollection(clean, f);
    const call = gameId
      ? created.pipe(
          Effect.flatMap((list) => {
            const added = list.find((c) => !before.has(c.id));
            return added && !added.smart ? LauncherApi.addCollectionGame(added.id, gameId) : Effect.succeed(list);
          }),
        )
      : created;
    run(call, () => {
      setName('');
      setSmart(false);
      setDraft(EMPTY_DRAFT);
      if (mode === 'new') onClose();
    });
  };

  const toggle = (c: Collection, on: boolean) => {
    if (!gameId) return;
    run(on ? LauncherApi.addCollectionGame(c.id, gameId) : LauncherApi.removeCollectionGame(c.id, gameId));
  };

  const move = (c: Collection, delta: -1 | 1) => {
    const ids = moveCollection(
      collections.map((x) => x.id),
      c.id,
      delta,
    );
    if (ids) void run(LauncherApi.reorderCollections(ids));
  };

  const rename = (c: Collection, next: string) => {
    const clean = next.trim();
    if (!clean || clean === c.name) return;
    void run(LauncherApi.updateCollection(c.id, clean, c.smart ?? {}));
  };

  const edit = (c: Collection) => {
    if (!c.smart) return;
    setEditingId(c.id);
    setEditName(c.name);
    setEditDraft(draftFromSmart(c.smart));
    setError(null);
  };

  const saveEdit = (c: Collection) => {
    const clean = editName.trim();
    const f = smartFromDraft(editDraft);
    if (!clean || busy) return;
    if (Object.keys(f).length === 0) {
      setError(t('collections:smart.needOne'));
      return;
    }
    void run(LauncherApi.updateCollection(c.id, clean, f), () => setEditingId(null));
  };

  const pickable = collections.filter((c) => !c.smart);
  const title = mode === 'new' ? t('collections:newHeading') : game ? t('collections:pickLabel', { title: game.title }) : t('collections:title');

  return (
    <Modal label={title} onClose={onClose} size="md" innerClassName={mode === 'new' ? 'p-0' : undefined}>
      {mode === 'new' && <ModalHead title={title} onClose={onClose} />}
      <div className={mode === 'new' ? 'p-5' : 'p-6'}>
        {mode !== 'new' && <MonoLabel className="mb-2">{t('collections:title')}</MonoLabel>}
        {mode !== 'new' && <h2 className="mb-4 text-[18px] font-bold">{title}</h2>}

        {gameId ? (
          <ul className="mb-5 space-y-1.5" aria-label={t('collections:pickLabel', { title: game?.title ?? gameId })}>
            {pickable.length === 0 && <li className="text-[13px] text-text-muted">{t('collections:pickerEmpty')}</li>}
            {pickable.map((c) => {
              const on = c.games?.includes(gameId) ?? false;
              return (
                <li key={c.id}>
                  <label className="flex cursor-pointer items-center gap-2.5 rounded-xl px-2 py-1.5 text-[14px] hover:bg-surface">
                    <input type="checkbox" checked={on} disabled={busy} onChange={(e) => toggle(c, e.target.checked)} className="h-4 w-4 accent-attention" />
                    <span className="flex-1">{label(c)}</span>
                    <span className="font-mono text-hint tabular-nums text-text-muted">{counts[c.id] ?? 0}</span>
                  </label>
                </li>
              );
            })}
          </ul>
        ) : mode !== 'new' ? (
          <ul className="mb-5 space-y-2" aria-label={t('collections:title')}>
            {collections.map((c, i) => {
              const missing = missingFrom(c, games).length;
              return (
                <React.Fragment key={c.id}>
                <li className="flex flex-wrap items-center gap-2.5 rounded-xl bg-surface px-3 py-2 ring-1 ring-hairline">
                  {c.builtin ? (
                    <span className="min-w-0 flex-1 text-[14px] font-semibold">{label(c)}</span>
                  ) : (
                    <Input
                      key={`${c.name}-${editingId === c.id}`}
                      defaultValue={editingId === c.id ? undefined : c.name}
                      maxLength={80}
                      aria-label={t('collections:renameAria', { name: c.name })}
                      disabled={busy}
                      onChange={editingId === c.id ? (e) => setEditName(e.target.value) : undefined}
                      onBlur={(e) => { if (editingId !== c.id) rename(c, e.target.value); }}
                      value={editingId === c.id ? editName : undefined}
                      className="min-w-0 flex-1"
                    />
                  )}
                  <span className="font-mono text-hint tabular-nums text-text-muted">
                    {t('collections:count', { count: counts[c.id] ?? 0 })}
                    {missing > 0 && <span className="ml-2 text-attention">{t('collections:missing', { count: missing })}</span>}
                  </span>
                  <label className="flex items-center gap-1.5 text-hint text-text-muted">
                    <Switch checked={c.pinned ?? false} disabled={busy} onCheckedChange={(v) => void run(LauncherApi.pinCollection(c.id, v))} aria-label={t('collections:pinAria', { name: label(c) })} />
                    {t('collections:pin')}
                  </label>
                  <Button variant="neutral" size="sm" disabled={busy || i === 0} aria-label={t('collections:moveUpAria', { name: label(c) })} onClick={() => move(c, -1)}>
                    {t('collections:moveUp')}
                  </Button>
                  <Button variant="neutral" size="sm" disabled={busy || i === collections.length - 1} aria-label={t('collections:moveDownAria', { name: label(c) })} onClick={() => move(c, 1)}>
                    {t('collections:moveDown')}
                  </Button>
                  {!c.builtin && (
                    c.smart ? (
                      <Button variant="neutral" size="sm" disabled={busy} aria-label={t(editingId === c.id ? 'collections:cancelEditAria' : 'collections:editAria', { name: label(c) })} onClick={() => editingId === c.id ? setEditingId(null) : edit(c)}>
                        {t(editingId === c.id ? 'collections:cancelEdit' : 'collections:edit')}
                      </Button>
                    ) : null
                  )}
                  {!c.builtin && (
                    <Button variant="ghost" size="sm" disabled={busy} aria-label={t('collections:deleteAria', { name: c.name })} onClick={() => void run(LauncherApi.deleteCollection(c.id))} className="text-danger">
                      {t('collections:delete')}
                    </Button>
                  )}
                </li>
                {editingId === c.id && c.smart && (
                  <li className="rounded-xl bg-surface p-3">
                    <SmartFields draft={editDraft} setDraft={setEditDraft} games={games} collections={collections} disabled={busy} />
                    <div className="mt-3 flex justify-end">
                      <Button variant="primary" size="sm" disabled={busy || !editName.trim()} onClick={() => saveEdit(c)}>{t('collections:save')}</Button>
                    </div>
                  </li>
                )}
                </React.Fragment>
              );
            })}
          </ul>
        ) : null}

        <form id="atlas-new-collection" onSubmit={create} className={cx('space-y-3', mode === 'new' ? '' : 'border-t border-hairline pt-4')}>
          {mode !== 'new' && <MonoLabel>{t('collections:newHeading')}</MonoLabel>}
          <div className="flex flex-wrap items-end gap-2.5">
            <label className="min-w-0 flex-1 text-[13px] text-text-muted">
              {t('collections:newName')}
              <Input autoFocus={mode === 'new'} value={name} maxLength={80} onChange={(e) => setName(e.target.value)} placeholder={t('collections:newNamePlaceholder')} disabled={busy} className="mt-1.5" />
            </label>
            {!gameId && mode !== 'new' && (
              <label className="text-[13px] text-text-muted">
                {t('collections:kind')}
                <select
                  value={smart ? 'smart' : 'manual'}
                  onChange={(e) => setSmart(e.target.value === 'smart')}
                  disabled={busy}
                  className="mt-1.5 block rounded-xl border border-hairline bg-surface px-3 py-2 text-[14px] text-text outline-none focus-visible:ring-2 focus-visible:ring-focus/50"
                >
                  <option value="manual">{t('collections:kindManual')}</option>
                  <option value="smart">{t('collections:kindSmart')}</option>
                </select>
              </label>
            )}
            {mode !== 'new' && <Button type="submit" variant="primary" disabled={busy || !name.trim()}>
              {t('collections:create')}
            </Button>}
          </div>
          {mode === 'new' && !gameId && <div className="flex items-center justify-between gap-4 border-t border-hairline py-3"><div><div className="text-[13px] font-semibold text-text">{t('collections:kind')}</div><div className="text-[12px] text-text-muted">{smart ? t('collections:kindSmartDesc') : t('collections:kindManualDesc')}</div></div><div className="flex rounded-[9px] bg-surface p-[3px]"><button type="button" onClick={() => setSmart(false)} className={cx('rounded-[7px] px-3 py-1.5 text-[13px] font-semibold', !smart ? 'bg-accent text-on-accent' : 'text-text-muted')}>{t('collections:kindManual')}</button><button type="button" onClick={() => setSmart(true)} className={cx('rounded-[7px] px-3 py-1.5 text-[13px] font-semibold', smart ? 'bg-accent text-on-accent' : 'text-text-muted')}>{t('collections:kindSmart')}</button></div></div>}
          {smart && !gameId && <SmartFields draft={draft} setDraft={setDraft} games={games} collections={collections} disabled={busy} />}
          {error && <ErrorNotice className="text-[13px]" message={error} fallback={t('collections:error')} />}
        </form>

        <div className="mt-6 flex justify-end gap-2">
          {mode !== 'new' && <Button variant="neutral" onClick={onClose}>{t('common:done')}</Button>}
        </div>
      </div>
      {mode === 'new' && <ModalFoot><Button variant="ghost" disabled={busy} onClick={onClose}>{t('common:cancel')}</Button><Button type="submit" form="atlas-new-collection" variant="primary" disabled={busy || !name.trim()}>{t('collections:create')}</Button></ModalFoot>}
    </Modal>
  );
}

const selectClass = 'mt-1.5 block w-full rounded-xl border border-hairline bg-surface px-3 py-2 text-[14px] text-text outline-none focus-visible:ring-2 focus-visible:ring-focus/50 disabled:opacity-60';

/** The smart filter's fields: each one is "any" until the user picks a value. */
function SmartFields({ draft, setDraft, games, collections, disabled }: { draft: SmartDraft; setDraft: (d: SmartDraft) => void; games: Game[]; collections: Collection[]; disabled: boolean }) {
  const { t } = useTranslation(['collections', 'library']);
  const set = (k: keyof SmartDraft) => (e: { target: { value: string } }) => setDraft({ ...draft, [k]: e.target.value });
  const stores = [...new Set(games.map((g) => g.store))];
  const select = (k: keyof SmartDraft, label: string, options: { value: string; label: string }[]) => (
    <label className="text-[13px] text-text-muted" key={k}>
      {label}
      <select value={draft[k]} onChange={set(k)} disabled={disabled} className={selectClass}>
        <option value="">{t('collections:smart.any')}</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </label>
  );
  const yesNo = [{ value: 'yes', label: t('collections:smart.yes') }, { value: 'no', label: t('collections:smart.no') }];
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3" data-testid="smart-fields">
      {select('store', t('collections:smart.store'), stores.map((s) => ({ value: s, label: storeLabel(s) })))}
      {select('installed', t('collections:smart.installed'), yesNo)}
      {select('played', t('collections:smart.played'), yesNo)}
      {select('badgeKind', t('collections:smart.badgeKind'), ['rating', 'verdict', 'protection', 'note', 'platform'].map((v) => ({ value: v, label: t(`collections:smart.badgeKinds.${v}`) })))}
      {draft.badgeKind === 'rating' ? select('badgeValue', t('collections:smart.badgeValue'), TIER_FILTERS.filter((v) => v !== 'all').map((v) => ({ value: v, label: tierLabel(t, v) }))) : draft.badgeKind === 'verdict' ? select('badgeValue', t('collections:smart.badgeValue'), ['verified', 'playable', 'unsupported'].map((v) => ({ value: v, label: t(`collections:smart.deckName.${v}`) }))) : draft.badgeKind ? (
        <label className="text-[13px] text-text-muted">
          {t('collections:smart.badgeValue')}
          <Input value={draft.badgeValue} onChange={set('badgeValue')} disabled={disabled} className="mt-1.5" />
        </label>
      ) : null}
      {select('status', t('collections:smart.status'), STATUSES.map((v) => ({ value: v, label: t(`library:statusFilter.${v}`) })))}
      {select('collection', t('collections:smart.collection'), collections.filter((c) => !c.smart).map((c) => ({ value: c.id, label: c.builtin ? t(`collections:builtin.${c.id}`) : c.name })))}
      <label className="text-[13px] text-text-muted">
        {t('collections:smart.minHours')}
        <Input type="number" min={0} inputMode="numeric" value={draft.minHours} onChange={set('minHours')} disabled={disabled} className="mt-1.5" />
      </label>
      <label className="text-[13px] text-text-muted">
        {t('collections:smart.yearFrom')}
        <Input type="number" min={0} inputMode="numeric" value={draft.yearFrom} onChange={set('yearFrom')} disabled={disabled} className="mt-1.5" />
      </label>
      <label className="text-[13px] text-text-muted">
        {t('collections:smart.yearTo')}
        <Input type="number" min={0} inputMode="numeric" value={draft.yearTo} onChange={set('yearTo')} disabled={disabled} className="mt-1.5" />
      </label>
    </div>
  );
}
