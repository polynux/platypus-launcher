// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { ImagePlus, RotateCcw, Search } from 'lucide-react';
import type { ArtCandidate, ArtKind, Customization, Game } from '../types';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { Cover, MonoLabel, cx } from './ui';
import { LauncherApi, onLibrary } from '../effect/runtime';
import { ART_KINDS, currentArt, hasCustomization, type ArtKindInfo } from '../lib/customize';
import { ErrorNotice } from './ErrorNotice';
import { coreErrMsg } from '../lib/coreError';

const SHAPE: Record<ArtKindInfo['shape'], string> = {
  portrait: 'aspect-[2/3] w-14',
  wide: 'aspect-[16/6] w-32',
  square: 'aspect-square w-14',
};

type T = (key: string, vars?: Record<string, unknown>) => string;

/** The per-kind catalog copy (labels, hints, accessible names, picker titles),
 *  keyed statically so the key check sees every key. */
function kindCopy(t: T, kind: ArtKind) {
  switch (kind) {
    case 'cover':
      return {
        label: t('game:custom.art.cover.label'),
        hint: t('game:custom.art.cover.hint'),
        chooseAria: t('game:custom.art.cover.chooseAria'),
        resetAria: t('game:custom.art.cover.resetAria'),
        pick: (title: string) => t('game:custom.art.cover.pick', { title }),
      };
    case 'banner':
      return {
        label: t('game:custom.art.banner.label'),
        hint: t('game:custom.art.banner.hint'),
        chooseAria: t('game:custom.art.banner.chooseAria'),
        resetAria: t('game:custom.art.banner.resetAria'),
        pick: (title: string) => t('game:custom.art.banner.pick', { title }),
      };
    case 'logo':
      return {
        label: t('game:custom.art.logo.label'),
        hint: t('game:custom.art.logo.hint'),
        chooseAria: t('game:custom.art.logo.chooseAria'),
        resetAria: t('game:custom.art.logo.resetAria'),
        pick: (title: string) => t('game:custom.art.logo.pick', { title }),
      };
    case 'icon':
      return {
        label: t('game:custom.art.icon.label'),
        hint: t('game:custom.art.icon.hint'),
        chooseAria: t('game:custom.art.icon.chooseAria'),
        resetAria: t('game:custom.art.icon.resetAria'),
        pick: (title: string) => t('game:custom.art.icon.pick', { title }),
      };
    case 'background':
      return {
        label: t('game:custom.art.background.label'),
        hint: t('game:custom.art.background.hint'),
        chooseAria: t('game:custom.art.background.chooseAria'),
        resetAria: t('game:custom.art.background.resetAria'),
        pick: (title: string) => t('game:custom.art.background.pick', { title }),
      };
  }
}

/** The attribution line for art the lookup stored. */
function artSourceText(t: T, artSource: string, name?: string): string {
  const label = artSource.startsWith('gog:')
    ? t('game:artSource.gog')
    : artSource.startsWith('lutris:')
      ? t('game:artSource.lutris')
      : t('game:artSource.steam');
  return name ? t('game:artSource.withName', { label, name }) : label;
}

/** The small source label of one candidate row. */
function candidateSource(t: T, c: ArtCandidate): string {
  if (c.source === 'gog') return t('game:source.gog');
  if (c.source === 'lutris') return t('game:source.lutris');
  return t('game:source.steam');
}

/** The user's own title and art for a game. It is Platypus's, not the store's: the
 *  images are copied into Platypus's data folder, and a reset brings the store's
 *  back. The plugin never hears of any of it. */
export function CustomizationTab({ game, onChanged }: {
  game: Game;
  /** The customization changed: the library should reload to carry it. */
  onChanged: () => void;
}) {
  const { t } = useTranslation('game');
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
      (e) => live && setError(coreErrMsg(e)),
    );
    return () => { live = false; };
  }, [game.id]);

  // Reload for this game on a targeted library event or an empty-id batch broadcast.
  useEffect(() => {
    if (typeof onLibrary !== 'function') return;
    return onLibrary((id: string) => {
      if (id !== '' && id !== game.id) return;
      Effect.runPromise(LauncherApi.getCustomization(game.id)).then(
        (c) => apply(c),
        () => {},
      );
    });
  }, [game.id]);

  /** Runs one change, keeps the form in step with what core answers and tells the owner. */
  // i18n:allow: TypeScript Promise generic in the parameter type, not user-facing text
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
      setError(coreErrMsg(e));
    } finally {
      setBusy(null);
    }
  };

  const choose = (info: ArtKindInfo) => run(info.kind, async () => {
    const path = await Effect.runPromise(LauncherApi.pickImage(kindCopy(t, info.kind).pick(game.title)));
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
        {t('game:custom.intro', { title: storeTitle })}
      </p>

      <section>
        <MonoLabel className="mb-1.5">{t('game:custom.titleSection')}</MonoLabel>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-[12px] text-white/55">{t('game:custom.titleLabel')}</span>
            <Input aria-label={t('game:custom.customTitleAria')} value={title} placeholder={storeTitle} maxLength={200} onChange={(e) => setTitle(e.target.value)} disabled={!custom} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[12px] text-white/55">{t('game:custom.sortLabel')}</span>
            <Input aria-label={t('game:custom.sortTitleAria')} value={sortTitle} placeholder={title || storeTitle} maxLength={200} onChange={(e) => setSortTitle(e.target.value)} disabled={!custom} />
          </label>
        </div>
        <div className="mt-2 flex gap-2">
          <Button variant="primary" size="sm" disabled={!titleDirty || busy != null} onClick={() => void saveTitle()}>{t('game:custom.saveTitle')}</Button>
          <Button variant="neutral" size="sm" disabled={!custom || (custom.title === '' && custom.sort_title === '') || busy != null}
            onClick={() => void run('title', () => Effect.runPromise(LauncherApi.setCustomTitle(game.id, '', '')))}>
            {t('game:custom.useStoreTitle')}
          </Button>
        </div>
      </section>

      <section>
        <MonoLabel className="mb-1.5">{t('game:custom.artSection')}</MonoLabel>
        {custom?.art_source && (
          <p className="mb-2 font-mono text-hint text-white/55" data-testid="art-attribution">
            {artSourceText(t, custom.art_source, custom.art_source_name || undefined)}
          </p>
        )}
        <ul className="space-y-2">
          {ART_KINDS.map((info) => {
            const copy = kindCopy(t, info.kind);
            const mine = custom?.art[info.kind];
            const src = mine ?? currentArt(game, info.kind);
            return (
              <li key={info.kind} className="flex items-center gap-3 rounded-xl bg-white/[0.03] px-3 py-2.5 ring-1 ring-white/[0.06]">
                <Cover src={src} title="" className={cx('flex-none rounded-lg ring-1 ring-white/10', SHAPE[info.shape])} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-[12.5px] font-semibold text-white/90">
                    {copy.label}
                    {mine && <span className="rounded bg-amber-300/15 px-1.5 py-0.5 font-mono text-micro uppercase tracking-wider text-amber-200">{t('game:custom.yours')}</span>}
                  </div>
                  <div className="truncate text-hint text-white/55">{copy.hint}</div>
                </div>
                <div className="flex flex-none gap-1.5">
                  <Button variant="neutral" size="sm" disabled={!custom || busy != null} onClick={() => void choose(info)} aria-label={copy.chooseAria}>
                    <ImagePlus className="h-3.5 w-3.5" /> {mine ? t('game:custom.replace') : t('game:custom.choose')}
                  </Button>
                  {mine && (
                    <Button variant="ghost" size="sm" disabled={busy != null} onClick={() => void reset(info.kind)} aria-label={copy.resetAria}>
                      <RotateCcw className="h-3.5 w-3.5" /> {t('game:custom.reset')}
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

      {error && <ErrorNotice message={error} fallback={t('game:custom.saveFailed')} />}

      {hasCustomization(custom) && (
        <Button variant="neutral" size="sm" disabled={busy != null} onClick={() => void resetAll()}>
          <RotateCcw className="h-3.5 w-3.5" /> {t('game:custom.resetAll')}
        </Button>
      )}
    </div>
  );
}

/** One candidate's identity across sources (Steam app ids are numbers, GOG
 *  product ids and Lutris slugs are strings). */
function applyKey(c: ArtCandidate): string {
  return `${c.source || 'steam'}:${c.id || c.app_id}`;
}

/** The small id detail of one candidate row. */
function candidateDetail(t: T, c: ArtCandidate): string {
  if (c.source === 'steam') return t('game:source.steamApp', { id: c.app_id });
  return c.id;
}

/** Find cover and banner art online for a sideloaded game (issues #36, art
 *  fallbacks): Steam's public store search first, GOG and Lutris (IGDB) as
 *  keyless fallbacks when Steam is thin, prefilled with the game's title.
 *  Picking a candidate applies its art like a manual pick (it replaces what
 *  is there); failures show here, never silently. */
function OnlineArtFinder({ game, onApplied, onChanged }: {
  game: Game;
  /** The apply answered: keep the form in step with what core stored. */
  onApplied: (c: Customization) => void;
  /** The customization changed: the library should reload to carry it. */
  onChanged: () => void;
}) {
  const { t } = useTranslation('game');
  const storeTitle = game.store_title || game.title;
  const [query, setQuery] = useState(game.title);
  const [candidates, setCandidates] = useState<ArtCandidate[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [applying, setApplying] = useState<string | null>(null);
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
    setApplying(applyKey(c));
    setError(null);
    Effect.runPromise(LauncherApi.applyOnlineArt(game.id, c.source, c.id)).then(
      (custom) => { setApplying(null); onApplied(custom); onChanged(); },
      (e) => { setApplying(null); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  return (
    <section>
      <MonoLabel className="mb-1.5">{t('game:custom.finder.heading')}</MonoLabel>
      <p className="mb-2 text-[12px] leading-relaxed text-white/55">
        {t('game:custom.finder.help', { title: storeTitle })}
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); search(); }}
      >
        <Input
          aria-label={t('game:custom.finder.searchAria')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={storeTitle}
          className="min-w-0 max-w-xs flex-1"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={!query.trim() || searching || applying != null}>
          <Search className="h-3.5 w-3.5" /> {searching ? t('game:custom.finder.searching') : t('game:custom.finder.search')}
        </Button>
      </form>
      {candidates != null && candidates.length === 0 && (
        <p className="mt-2 text-[12px] text-white/55">{t('game:custom.finder.noneFound')}</p>
      )}
      {candidates != null && candidates.length > 0 && (
        <ul className="mt-2 space-y-2">
          {candidates.map((c) => (
            <li key={applyKey(c)} className="flex items-center gap-3 rounded-xl bg-white/[0.03] px-3 py-2.5 ring-1 ring-white/[0.06]">
              {c.thumb_url ? (
                <img src={c.thumb_url} alt="" loading="lazy" className="h-10 w-24 flex-none rounded-lg object-cover ring-1 ring-white/10" />
              ) : (
                <span className="h-10 w-24 flex-none rounded-lg bg-white/[0.05] ring-1 ring-white/10" />
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate text-[12.5px] font-semibold text-white/90">{c.name}</div>
                <div className="font-mono text-hint text-white/55">
                  <span data-testid="candidate-source">{candidateSource(t, c)}</span>
                  {c.year ? ` · ${c.year}` : ''} · {candidateDetail(t, c)}
                </div>
              </div>
              <Button variant="primary" size="sm" disabled={searching || applying != null} onClick={() => apply(c)}>
                {applying === applyKey(c) ? t('game:custom.finder.applying') : t('game:custom.finder.apply')}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {error && <ErrorNotice message={error} fallback={t('game:custom.finder.failed')} />}
    </section>
  );
}
