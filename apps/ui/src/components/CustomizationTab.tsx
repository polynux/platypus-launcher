import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { ImagePlus, RotateCcw, Search } from 'lucide-react';
import type { ArtCandidate, ArtKind, Customization, Game } from '../types';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { Cover, MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { ART_KINDS, currentArt, hasCustomization, type ArtKindInfo } from '../lib/customize';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

const SHAPE: Record<ArtKindInfo['shape'], string> = {
  portrait: 'aspect-[2/3] w-14',
  wide: 'aspect-[16/6] w-32',
  square: 'aspect-square w-14',
};

/** The user's own title and art for a game. It is Platypus's, not the store's: the
 *  images are copied into Platypus's data folder, and a reset brings the store's
 *  back. The plugin never hears of any of it. */
export function CustomizationTab({ game, onChanged }: {
  game: Game;
  /** The customization changed: the library should reload to carry it. */
  onChanged: () => void;
}) {
  const [custom, setCustom] = useState<Customization | null>(null);
  const [title, setTitle] = useState('');
  const [sortTitle, setSortTitle] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const apply = (c: Customization) => {
    setCustom(c);
    setTitle(c.title);
    setSortTitle(c.sort_title);
  };

  useEffect(() => {
    let live = true;
    setCustom(null);
    setError(null);
    Effect.runPromise(LauncherApi.getCustomization(game.id)).then(
      (c) => live && apply(c),
      (e) => live && setError(errMsg(e)),
    );
    return () => { live = false; };
  }, [game.id]);

  /** Runs one change, keeps the form in step with what core answers and tells the owner. */
  const run = async (what: string, change: () => Promise<Customization | null>) => {
    setBusy(what);
    setError(null);
    try {
      const c = await change();
      if (c) {
        apply(c);
        onChanged();
      }
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(null);
    }
  };

  const choose = (info: ArtKindInfo) => run(info.kind, async () => {
    const path = await Effect.runPromise(LauncherApi.pickImage(`Choose the ${info.label.toLowerCase()} of ${game.title}`));
    if (!path) return null;
    return Effect.runPromise(LauncherApi.setCustomArt(game.id, info.kind, path));
  });
  const reset = (kind: ArtKind) => run(kind, () => Effect.runPromise(LauncherApi.resetCustomArt(game.id, kind)));
  const saveTitle = () => run('title', () => Effect.runPromise(LauncherApi.setCustomTitle(game.id, title, sortTitle)));
  const resetAll = () => run('all', () => Effect.runPromise(LauncherApi.resetCustomization(game.id)));

  const storeTitle = game.store_title || game.title;
  const titleDirty = custom != null && (title.trim() !== custom.title || sortTitle.trim() !== custom.sort_title);

  return (
    <div className="space-y-6">
      <p className="text-[12.5px] leading-relaxed text-white/70">
        Change how {storeTitle} looks in Platypus. Images are copied into Platypus's own folder, so the file you pick can move or go.
        This stays on this computer and never reaches the store.
      </p>

      <section>
        <MonoLabel className="mb-1.5">title</MonoLabel>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-[12px] text-white/55">Title</span>
            <Input aria-label="Custom title" value={title} placeholder={storeTitle} maxLength={200} onChange={(e) => setTitle(e.target.value)} disabled={!custom} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] text-white/55">Sort as</span>
            <Input aria-label="Sort title" value={sortTitle} placeholder={title || storeTitle} maxLength={200} onChange={(e) => setSortTitle(e.target.value)} disabled={!custom} />
          </label>
        </div>
        <div className="mt-2 flex gap-2">
          <Button variant="primary" size="sm" disabled={!titleDirty || busy != null} onClick={() => void saveTitle()}>Save title</Button>
          <Button variant="neutral" size="sm" disabled={!custom || (custom.title === '' && custom.sort_title === '') || busy != null}
            onClick={() => void run('title', () => Effect.runPromise(LauncherApi.setCustomTitle(game.id, '', '')))}>
            Use the store's title
          </Button>
        </div>
      </section>

      <section>
        <MonoLabel className="mb-1.5">art</MonoLabel>
        {custom?.art_source && (
          <p className="mb-2 font-mono text-hint text-white/55" data-testid="art-attribution">
            Art from Steam{custom.art_source_name ? ` · ${custom.art_source_name}` : ''}
          </p>
        )}
        <ul className="space-y-2">
          {ART_KINDS.map((info) => {
            const mine = custom?.art[info.kind];
            const src = mine ?? currentArt(game, info.kind);
            return (
              <li key={info.kind} className="flex items-center gap-3 rounded-xl bg-white/[0.03] px-3 py-2.5 ring-1 ring-white/[0.06]">
                <Cover src={src} title="" className={cx('flex-none rounded-lg ring-1 ring-white/10', SHAPE[info.shape])} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-[12.5px] font-semibold text-white/90">
                    {info.label}
                    {mine && <span className="rounded bg-amber-300/15 px-1.5 py-0.5 font-mono text-micro uppercase tracking-wider text-amber-200">yours</span>}
                  </div>
                  <div className="truncate text-hint text-white/55">{info.hint}</div>
                </div>
                <div className="flex flex-none gap-1.5">
                  <Button variant="neutral" size="sm" disabled={!custom || busy != null} onClick={() => void choose(info)} aria-label={`Choose ${info.label.toLowerCase()} image`}>
                    <ImagePlus className="h-3.5 w-3.5" /> {mine ? 'Replace' : 'Choose'}
                  </Button>
                  {mine && (
                    <Button variant="ghost" size="sm" disabled={busy != null} onClick={() => void reset(info.kind)} aria-label={`Reset ${info.label.toLowerCase()} to the store's`}>
                      <RotateCcw className="h-3.5 w-3.5" /> Reset
                    </Button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      {game.store === 'custom' && (
        <OnlineArtFinder game={game} onApplied={apply} onChanged={onChanged} />
      )}

      {error && <ErrorNotice message={error} fallback="Platypus couldn't save the customization." />}

      {hasCustomization(custom) && (
        <Button variant="neutral" size="sm" disabled={busy != null} onClick={() => void resetAll()}>
          <RotateCcw className="h-3.5 w-3.5" /> Reset everything to the store's
        </Button>
      )}
    </div>
  );
}

/** Find cover and banner art online for a sideloaded game (issue #36):
 *  Steam's public store search, prefilled with the game's title. Picking a
 *  candidate applies its art like a manual pick (it replaces what is there);
 *  failures show here, never silently. */
function OnlineArtFinder({ game, onApplied, onChanged }: {
  game: Game;
  /** The apply answered: keep the form in step with what core stored. */
  onApplied: (c: Customization) => void;
  /** The customization changed: the library should reload to carry it. */
  onChanged: () => void;
}) {
  const storeTitle = game.store_title || game.title;
  const [query, setQuery] = useState(game.title);
  const [candidates, setCandidates] = useState<ArtCandidate[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [applying, setApplying] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const search = () => {
    const q = query.trim();
    if (!q || searching || applying != null) return;
    setSearching(true);
    setError(null);
    Effect.runPromise(LauncherApi.searchArtOnline(q)).then(
      (cs) => { setSearching(false); setCandidates(cs); },
      (e) => { setSearching(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  const apply = (c: ArtCandidate) => {
    if (searching || applying != null) return;
    setApplying(c.app_id);
    setError(null);
    Effect.runPromise(LauncherApi.applyOnlineArt(game.id, c.app_id)).then(
      (custom) => { setApplying(null); onApplied(custom); onChanged(); },
      (e) => { setApplying(null); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  return (
    <section>
      <MonoLabel className="mb-1.5">find art online</MonoLabel>
      <p className="mb-2 text-[12px] leading-relaxed text-white/55">
        Search Steam's public store for {storeTitle}'s cover, banner and logo. Applying replaces the art above;
        art you picked by hand is only replaced when you choose it here.
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); search(); }}
      >
        <Input
          aria-label="Search for art online"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={storeTitle}
          className="min-w-0 max-w-xs flex-1"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={!query.trim() || searching || applying != null}>
          <Search className="h-3.5 w-3.5" /> {searching ? 'Searching…' : 'Search'}
        </Button>
      </form>
      {candidates != null && candidates.length === 0 && (
        <p className="mt-2 text-[12px] text-white/55">No artwork found for that title. The placeholder stays.</p>
      )}
      {candidates != null && candidates.length > 0 && (
        <ul className="mt-2 space-y-2">
          {candidates.map((c) => (
            <li key={c.app_id} className="flex items-center gap-3 rounded-xl bg-white/[0.03] px-3 py-2.5 ring-1 ring-white/[0.06]">
              {c.thumb_url ? (
                <img src={c.thumb_url} alt="" loading="lazy" className="h-10 w-24 flex-none rounded-lg object-cover ring-1 ring-white/10" />
              ) : (
                <span className="h-10 w-24 flex-none rounded-lg bg-white/[0.05] ring-1 ring-white/10" />
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate text-[12.5px] font-semibold text-white/90">{c.name}</div>
                <div className="font-mono text-hint text-white/55">Steam app {c.app_id}</div>
              </div>
              <Button variant="primary" size="sm" disabled={searching || applying != null} onClick={() => apply(c)}>
                {applying === c.app_id ? 'Applying…' : 'Apply'}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {error && <ErrorNotice message={error} fallback="Platypus couldn't find art online." />}
    </section>
  );
}
