import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { ImagePlus, RotateCcw } from 'lucide-react';
import type { ArtKind, Customization, Game } from '../types';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { Cover, MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';
import { ART_KINDS, currentArt, hasCustomization, type ArtKindInfo } from '../lib/customize';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

const SHAPE: Record<ArtKindInfo['shape'], string> = {
  portrait: 'aspect-[2/3] w-14',
  wide: 'aspect-[16/6] w-32',
  square: 'aspect-square w-14',
};

/** The user's own title and art for a game. It is Hydra's, not the store's: the
 *  images are copied into Hydra's data folder, and a reset brings the store's
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
      <p className="text-[12.5px] leading-relaxed text-white/55">
        Change how {storeTitle} looks in Hydra. Images are copied into Hydra's own folder, so the file you pick can move or go.
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
                    {mine && <span className="rounded bg-amber-300/15 px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider text-amber-200">yours</span>}
                  </div>
                  <div className="truncate text-[11.5px] text-white/45">{info.hint}</div>
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

      {error && <p role="alert" className="font-mono text-[11.5px] text-red-300">{error}</p>}

      {hasCustomization(custom) && (
        <Button variant="neutral" size="sm" disabled={busy != null} onClick={() => void resetAll()}>
          <RotateCcw className="h-3.5 w-3.5" /> Reset everything to the store's
        </Button>
      )}
    </div>
  );
}
