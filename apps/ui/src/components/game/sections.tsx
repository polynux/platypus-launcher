// The game page's sections (issue #119): the store description, the media
// strip, the status notes, the collections, the installation facts, the
// achievements and the info finder. Props only: the page owns the data.
import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Maximize2, Play, Search, Trophy } from 'lucide-react';
import type { Achievement, ArtCandidate, Collection, Game, GameDetails, GameStatus, Job, StorePlugin } from '../../types';
import { ExternalLink } from '../ExternalLink';
import type { MediaItem } from '../Lightbox';
import { openExternal } from '../../lib/external';
import { JobProgress } from '../JobProgress';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import { ScrollArea } from '@/components/ds/scroll-area';
import { MonoLabel, Panel, cx } from '../ui';
import { PluginLogo } from '../PluginLogo';
import { formatBytes, storeLabel } from '../../lib/games';
import { formatCount, formatDate, formatPercentValue } from '../../lib/format';
import { pinLabel } from '../../lib/versions';
import { ErrorNotice } from '../ErrorNotice';
import { Input } from '@/components/ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Textarea } from '@/components/ds/textarea';

/** The store's HTML (description, requirements): core sanitizes it to the
 *  allowlist, so rendering it as markup is safe. Links open in the browser. */
export function SafeHtml({ html, testid, className }: { html: string; testid?: string; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const onClick = (e: MouseEvent) => {
      const a = (e.target as HTMLElement).closest?.('a');
      if (!a) return;
      const href = a.getAttribute('href');
      if (!href) return;
      e.preventDefault();
      openExternal(href);
    };
    el.addEventListener('click', onClick);
    return () => el.removeEventListener('click', onClick);
  }, []);
  return (
    <div
      ref={ref}
      data-testid={testid}
      dangerouslySetInnerHTML={{ __html: html }}
      className={cx('space-y-2 text-[13px] leading-relaxed text-text-body [&_a]:text-amber-200/90 [&_a]:underline [&_a]:decoration-amber-200/40 [&_a]:underline-offset-2 [&_h2]:text-[14px] [&_h2]:font-bold [&_h2]:text-white/85 [&_h3]:text-[13px] [&_h3]:font-bold [&_h3]:text-white/85 [&_h4]:text-[13px] [&_h4]:font-bold [&_h4]:text-white/85 [&_li]:ml-4 [&_li]:list-disc [&_ul]:space-y-1', className)}
    />
  );
}

/** Screenshots + video thumbnails: lazy, nothing heavy on first paint. The
 *  hero shows the selected item; videos show their thumbnail with a play
 *  badge and open click-to-play in the lightbox (never autoplay with sound). */
export function MediaStrip({ game, media, index, onIndex, onOpen, stripRef }: {
  game: Game;
  media: MediaItem[];
  index: number;
  onIndex: (i: number) => void;
  onOpen: () => void;
  stripRef: React.RefObject<HTMLDivElement | null>;
}) {
  const { t } = useTranslation('game');
  const cur = media[index] ?? media[0];
  const thumbLabel = (m: MediaItem, i: number) =>
    m.kind === 'video'
      ? t('game:media.trailerLabel', { suffix: m.title ? `"${m.title}" ` : '', index: i + 1, total: media.length })
      : t('game:media.screenshotLabel', { index: i + 1 });
  return (
    <div className="space-y-3">
      <Tooltip>
      <TooltipTrigger asChild>
      <button
        onClick={onOpen}
        aria-label={cur?.kind === 'video' ? t('game:media.openTrailer') : t('game:media.openShots')}
        className="group relative block w-full cursor-zoom-in overflow-hidden rounded-3xl bg-black/40 ring-1 ring-hairline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {cur?.kind === 'video' ? (
          cur.thumbnail ? (
            <img key={cur.src} src={cur.thumbnail} alt={`${game.title} trailer thumbnail`} loading="lazy" className="aspect-video w-full object-cover" />
          ) : (
            <span key={cur.src} className="grid aspect-video w-full place-items-center bg-black/60">
              <Play className="h-10 w-10 text-white/80" />
            </span>
          )
        ) : (
          cur && <img key={cur.src} src={cur.src} alt={`${game.title} screenshot ${index + 1}`} className="aspect-video w-full object-cover" />
        )}
        {cur?.kind === 'video' && (
          <span aria-hidden className="absolute inset-0 grid place-items-center">
            <span className="grid h-14 w-14 place-items-center rounded-full bg-black/65 ring-1 ring-white/20 transition group-hover:scale-105">
              <Play className="h-6 w-6 fill-white text-white" />
            </span>
          </span>
        )}
        <span aria-hidden className="absolute right-3 top-3 grid h-8 w-8 place-items-center rounded-lg bg-black/60 text-white/80 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
          <Maximize2 size={15} />
        </span>
      </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{t('game:media.openViewer')}</TooltipContent>
      </Tooltip>
      <ScrollArea orientation="horizontal" type="always" viewportRef={stripRef} data-testid="shot-strip">
        {/* Padding keeps the active ring and rounded corners inside the viewport; pb leaves room for the overlay bar. */}
        <div data-testid="shot-strip-inner" className="flex w-max gap-2.5 px-1.5 pb-3.5 pt-1.5">
          {media.map((m, i) => (
            <button
              key={`${m.kind}-${m.src}`}
              data-thumb
              onClick={() => onIndex(i)}
              onDoubleClick={onOpen}
              aria-label={thumbLabel(m, i)}
              aria-pressed={i === index}
              className={cx('relative aspect-video h-[84px] flex-none overflow-hidden rounded-xl bg-black/50 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus', i === index ? 'ring-2 ring-accent-strong/80' : 'opacity-55 ring-1 ring-hairline hover:opacity-90')}
            >
              {m.kind === 'video' ? (
                m.thumbnail ? (
                  <img src={m.thumbnail} alt="" loading="lazy" className="h-full w-full object-cover" />
                ) : (
                  <span className="grid h-full w-full place-items-center"><Play className="h-5 w-5 text-white/80" /></span>
                )
              ) : (
                <img src={m.src} alt="" loading="lazy" className="h-full w-full object-contain" />
              )}
              {m.kind === 'video' && (
                <span aria-hidden className="absolute inset-0 grid place-items-center">
                  <span className="grid h-7 w-7 place-items-center rounded-full bg-black/65 ring-1 ring-white/20">
                    <Play className="h-3.5 w-3.5 fill-white text-white" />
                  </span>
                </span>
              )}
            </button>
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}

function controllerLabel(t: (key: string) => string, c?: string): string | null {
  if (c === 'full') return t('game:about.controllerFull');
  if (c === 'partial') return t('game:about.controllerPartial');
  return null;
}

/** The About section: formatted description (collapsed + Show more), details
 *  grid, requirements tabs and links. Calm by default: secondary details live
 *  here, not in the hero. Sideloaded games (issue #38) get a "Find info
 *  online" action when nothing is linked, and an attribution + Unlink control
 *  once linked. */
export function AboutSection({ game, details, loading, error, compatUrls, onOpenStore, onReload, onSearchInfo, onLinkInfo, onUnlinkInfo, hideDetails }: {
  game: Game;
  details: GameDetails | null;
  loading: boolean;
  error: string | null;
  compatUrls: { protondb?: string; wiki?: string };
  /** Opens the store page in the embedded store view, when the plugin has one. */
  onOpenStore?: () => void;
  /** The info link changed: details were evicted and refetched. */
  onReload?: () => void;
  onSearchInfo: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLinkInfo: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  onUnlinkInfo: (id: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  /** The tabs store page shows Details in its side column: hide the grid here. */
  hideDetails?: boolean;
}) {
  const { t } = useTranslation('game');
  const [expanded, setExpanded] = useState(false);
  const [reqTab, setReqTab] = useState<'minimum' | 'recommended'>('minimum');
  useEffect(() => { setExpanded(false); setReqTab('minimum'); }, [game.id]);
  // The tabs clamp (760px) applies only when the description actually
  // overflows it: a short store description shows with no mask and no
  // "Read the full description" toggle. Measured off scrollHeight (the full
  // content height even when clamped).
  const wrapRef = useRef<HTMLDivElement>(null);
  const [overflows, setOverflows] = useState(false);
  useEffect(() => { setOverflows(false); }, [game.id, details]);
  useLayoutEffect(() => {
    if (!hideDetails) return;
    const el = wrapRef.current;
    if (!el) return;
    const check = () => {
      if (expanded) return;
      setOverflows(el.scrollHeight > 761);
    };
    check();
    window.addEventListener('resize', check);
    let ro: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      ro = new ResizeObserver(check);
      ro.observe(el);
    }
    return () => {
      window.removeEventListener('resize', check);
      ro?.disconnect();
    };
  }, [hideDetails, details, expanded]);
  const d = details as unknown as (GameDetails & {
    description_html?: string; website?: string; store_url?: string;
    info_source?: string; info_source_name?: string; info_stale?: boolean;
    languages?: string[]; controller?: string;
    requirements?: { minimum?: string; recommended?: string } | null;
    age_rating?: string; tags?: string[]; videos?: unknown[];
  }) | null;
  const html = d?.description_html || '';
  const summary = details?.summary || '';
  const isCustom = game.store === 'custom' || game.source === 'custom';
  const linked = !!((d?.info_source || d?.info_source_name) && isCustom);
  const genres = [...(details?.genres ?? []), ...((d?.tags ?? []).filter((t) => !(details?.genres ?? []).includes(t)) ?? [])];
  const controller = controllerLabel(t, d?.controller);
  const rows: [string, string][] = [
    ...(details?.developers?.length ? [[t('game:about.developer'), details.developers.join(', ')] as [string, string]] : []),
    ...(details?.publishers?.length ? [[t('game:about.publisher'), details.publishers.join(', ')] as [string, string]] : []),
    // The user's own year wins over the source's release date (issue #38):
    // the source's title is never written anywhere.
    ...(isCustom && game.release_year
      ? [[t('game:about.release'), String(game.release_year)] as [string, string]]
      : details?.release_date
        ? [[t('game:about.release'), details.release_date] as [string, string]]
        : []),
    ...(genres.length ? [[t('game:about.genres'), genres.join(' · ')] as [string, string]] : []),
    ...((d?.languages?.length ?? 0) ? [[t('game:about.languages'), (d?.languages ?? []).join(', ')] as [string, string]] : []),
    ...(controller ? [[t('game:about.controller'), controller] as [string, string]] : []),
    ...(d?.age_rating ? [[t('game:about.ageRating'), d.age_rating] as [string, string]] : []),
  ];
  const req = d?.requirements;
  const links: [string, string][] = [
    ...(d?.website ? [[t('game:about.linkWebsite'), d.website] as [string, string]] : []),
    ...(d?.store_url ? [[t('game:about.linkStore'), d.store_url] as [string, string]] : []),
    ...(compatUrls.protondb ? [[t('game:about.linkProtondb'), compatUrls.protondb] as [string, string]] : []),
    ...(compatUrls.wiki ? [[t('game:about.linkPcgw'), compatUrls.wiki] as [string, string]] : []),
  ];
  const infoName = d?.info_source_name || infoSourceText(t, d?.info_source);
  const storeLinkLabel = t('game:about.linkStore');
  // The tabs store page (atlas) shows the description as plain text: 14px,
  // max 72ch, 1.62 line height, with no card and no About heading. An
  // untitled game with no description shows one muted line.
  if (hideDetails) {
    return (
      <div className="max-w-[72ch] text-[14px] leading-[1.62] text-text-body">
        {loading ? (
          <div className="space-y-2">
            <div className="h-3 w-full animate-pulse rounded bg-surface-strong" />
            <div className="h-3 w-4/5 animate-pulse rounded bg-surface-strong" />
          </div>
        ) : html ? (
          <>
            <div ref={wrapRef} className={expanded || !overflows ? '' : 'max-h-[760px] overflow-hidden [mask-image:linear-gradient(#000_70%,transparent)]'} data-testid="about-html-wrap">
              <SafeHtml html={html} testid="about-html" className="text-[14px] leading-[1.62] !space-y-3.5 [&_h2]:!text-[15px] [&_h2]:!font-[750] [&_h2]:!mt-2.5 [&_h3]:!text-[15px] [&_h3]:!font-[750] [&_h3]:!mt-2.5 [&_h4]:text-[13px] [&_li]:text-[14px] [&_li]:leading-[1.62] [&_li::marker]:!text-accent [&_p]:text-[14px] [&_p]:leading-[1.62] [&_ul]:text-[14px] [&_a]:!text-accent" />
            </div>
            {overflows && (
              <Button variant="ghost" size="none" onClick={() => setExpanded((v) => !v)} data-testid="about-toggle" className="mt-2.5 rounded px-0 font-sans text-[14px] font-semibold text-accent hover:bg-transparent hover:text-text hover:underline hover:underline-offset-[3px]">
                {expanded ? t('game:about.showLess') : t('game:about.readFull')}
              </Button>
            )}
          </>
        ) : summary ? (
          <p className="leading-[1.62]">{summary}</p>
        ) : (
          <p className="text-text-muted">{error ? t('game:about.storeUnavailable', { error }) : t('game:about.noDescription')}</p>
        )}
        {isCustom && !linked && !loading && (
          <InfoFinder game={game} onLinked={() => onReload?.()} onSearch={onSearchInfo} onLink={onLinkInfo} />
        )}
      </div>
    );
  }
  return (
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:about.heading')}</h2>
      {loading ? (
        <div className="space-y-2">
          <div className="h-3 w-full animate-pulse rounded bg-surface-strong" />
          <div className="h-3 w-4/5 animate-pulse rounded bg-surface-strong" />
        </div>
      ) : html ? (
        <>
          <div className={expanded ? '' : 'max-h-[104px] overflow-hidden'} data-testid="about-html-wrap">
            <SafeHtml html={html} testid="about-html" />
          </div>
          <Button variant="ghost" size="none" onClick={() => setExpanded((v) => !v)} data-testid="about-toggle" className="mt-1.5 rounded px-0 font-mono text-[12px] underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/85">
            {expanded ? t('game:about.showLess') : t('game:about.showMore')}
          </Button>
        </>
      ) : summary ? (
        <p className="text-[13px] leading-relaxed text-text-body">{summary}</p>
      ) : (
        <p className="text-[13px] text-text-body">{error ? t('game:about.storeUnavailable', { error }) : t('game:about.noDescription')}</p>
      )}
      {isCustom && !linked && !loading && (
        <InfoFinder game={game} onLinked={() => onReload?.()} onSearch={onSearchInfo} onLink={onLinkInfo} />
      )}
      {!hideDetails && rows.length > 0 && (
        <dl data-testid="details-grid" className="mt-4 grid grid-cols-2 gap-x-6 gap-y-2 border-t border-hairline pt-3">
          {rows.map(([k, v]) => (
            <div key={k} className="min-w-0">
              <dt className="font-mono text-hint uppercase tracking-widest text-white/45">{k}</dt>
              <dd className="truncate text-[12.5px] text-white/80" title={v}>{v}</dd>
            </div>
          ))}
        </dl>
      )}
      {req && (req.minimum || req.recommended) && (
        <div className="mt-4 border-t border-hairline pt-3" data-testid="requirements">
          <div role="tablist" aria-label={t('game:about.reqLabel')} className="mb-2 flex gap-1.5">
            {(['minimum', 'recommended'] as const).filter((k) => req[k]).map((k) => (
              <Button
                key={k}
                role="tab"
                aria-selected={reqTab === k}
                data-testid={`req-tab-${k}`}
                variant="ghost"
                size="none"
                onClick={() => setReqTab(k)}
                className={reqTab === k ? 'rounded-lg bg-white/[0.08] px-2.5 py-1 font-mono text-[12px] text-white' : 'rounded-lg px-2.5 py-1 font-mono text-[12px] text-text-muted hover:bg-transparent hover:text-white/80'}
              >
                {k === 'minimum' ? t('game:about.reqMinimum') : t('game:about.reqRecommended')}
              </Button>
            ))}
          </div>
          <div role="tabpanel" data-testid="req-panel">
            <SafeHtml html={req[reqTab] ?? req.minimum ?? req.recommended ?? ''} />
          </div>
        </div>
      )}
      {links.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-x-4 gap-y-1.5 border-t border-hairline pt-3 font-mono text-[12px]" data-testid="about-links">
          {links.map(([label, href]) =>
            label === storeLinkLabel && onOpenStore ? (
              <button key={label} type="button" onClick={onOpenStore} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
                {label}
              </button>
            ) : (
            <ExternalLink key={label} href={href} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
              {label} ↗
            </ExternalLink>
            ),
          )}
        </div>
      )}
      {linked && (
        <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-hairline pt-3 font-mono text-[12px] text-text-muted" data-testid="info-attribution">
          <span>
            {t('game:about.infoFrom')} {d?.store_url ? (
              <ExternalLink href={d.store_url} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
                {infoName} ↗
              </ExternalLink>
            ) : infoName}
          </span>
          <InfoUnlink game={game} onUnlinked={() => onReload?.()} onUnlink={onUnlinkInfo} />
        </div>
      )}
      {d?.info_stale && (
        <p className="mt-2 font-mono text-[12px] text-amber-200/70" data-testid="info-stale">
          {t('game:about.infoStale')}
        </p>
      )}
    </Panel>
  );
}

/** The small source label of one info candidate row. */
function infoSourceText(t: (key: string) => string, source?: string): string {
  if (source === 'gog') return t('game:source.gog');
  if (source === 'lutris') return t('game:source.lutris');
  if (source === 'steam') return t('game:source.steam');
  return '';
}

/** One candidate's identity across sources. */
function infoCandidateKey(c: ArtCandidate): string {
  return `${c.source || 'steam'}:${c.id || c.app_id}`;
}

/** The small id detail of one candidate row. */
function infoCandidateDetail(t: (key: string, vars?: Record<string, unknown>) => string, c: ArtCandidate): string {
  if ((c.source || 'steam') === 'steam') return t('game:source.steamApp', { id: c.app_id || c.id });
  return c.id;
}

/** The user's own status and plain-text notes for a game (issue #51): the
 *  select saves at once, the notes autosave on a debounce and on blur, one
 *  write at a time so a slow save can't overwrite a newer one. The page owns
 *  the data: saves go through the callbacks it passes (the core calls live
 *  there), so this block is props only and can move in a layout. */
export function StatusNotesSection({ game, onChanged, onSaveStatus, onSaveNotes }: {
  game: Game;
  onChanged?: () => void;
  /** Saves the status through the page (the core call lives there). */
  onSaveStatus: (id: string, status: GameStatus) => Promise<{ status: GameStatus }>; // i18n:allow: Promise return type, not user-facing text
  /** Saves the notes through the page (the core call lives there). */
  onSaveNotes: (id: string, text: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
}) {
  const { t } = useTranslation('game');
  const [status, setStatus] = useState(game.status ?? '');
  const [statusError, setStatusError] = useState<string | null>(null);
  const [notes, setNotes] = useState(game.user_notes ?? '');
  const [notesState, setNotesState] = useState<'idle' | 'saving' | 'saved' | 'failed'>('idle');
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Every notes write runs through one promise chain, so writes are strictly
  // ordered and a slow save can't overwrite a newer one. The game id is
  // captured when a write is enqueued, so a save of the previous game never
  // touches this game's state.
  const chain = useRef<Promise<void>>(Promise.resolve()); // i18n:allow: Promise return type, not user-facing text
  // The text waiting on the debounce, not yet enqueued: { id, text } for the
  // game it was typed on. null when nothing is pending.
  const pending = useRef<{ id: string; text: string } | null>(null);
  // The last text enqueued for the game on screen: a blur with nothing
  // changed enqueues nothing.
  const lastText = useRef(game.user_notes ?? '');
  const shownId = useRef(game.id);

  const enqueueNotes = (id: string, text: string) => {
    lastText.current = text;
    setNotesState('saving');
    chain.current = chain.current.then(() =>
      onSaveNotes(id, text).then(
        () => { if (shownId.current === id) setNotesState('saved'); },
        () => { if (shownId.current === id) setNotesState('failed'); },
      ),
    );
  };

  useEffect(() => {
    // A text typed inside the debounce is flushed for the old game before it
    // is dropped, so navigating away never loses it.
    if (pending.current) {
      clearTimeout(timer.current);
      enqueueNotes(pending.current.id, pending.current.text);
      pending.current = null;
    }
    shownId.current = game.id;
    lastText.current = game.user_notes ?? '';
    setStatus(game.status ?? '');
    setNotes(game.user_notes ?? '');
    setStatusError(null);
    // The label is about the game on screen: an in-flight save's "Saving…"
    // belongs to the game we just left.
    setNotesState('idle');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  const saveStatus = (next: GameStatus) => {
    setStatusError(null);
    const at = game.id;
    onSaveStatus(at, next).then(
      (info) => { if (shownId.current === at) setStatus(info.status); onChanged?.(); },
      (e) => { if (shownId.current === at) setStatusError(e instanceof Error ? e.message : String(e)); },
    );
  };

  // The debounce timer: it enqueues the pending text for its own game.
  const requestNotesSave = (text: string, delay: number) => {
    if (text === lastText.current) { // nothing changed since the last enqueued text
      pending.current = null;
      clearTimeout(timer.current);
      return;
    }
    const id = game.id;
    pending.current = { id, text };
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      pending.current = null;
      enqueueNotes(id, text);
    }, delay);
  };

  return (
    <Panel className="bg-surface-strong" data-testid="status-notes">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:status.heading')}</h2>
      <Select value={status || 'none'} onValueChange={(v) => saveStatus((v === 'none' ? '' : v) as GameStatus)}>
        <SelectTrigger aria-label={t('game:status.aria')} className="w-full" data-testid="status-select">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(['none', 'backlog', 'playing', 'finished', 'abandoned'] as const).map((v) => (
            <SelectItem key={v} value={v}>
              {t(`game:status.${v}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {statusError && (
        <p role="alert" className="mt-1.5 text-[12px] text-red-300/80">
          {statusError}
        </p>
      )}
      <MonoLabel className="mb-1.5 mt-4">{t('game:status.notesLabel')}</MonoLabel>
      <Textarea
        value={notes}
        rows={4}
        aria-label={t('game:status.notesLabel')}
        placeholder={t('game:status.notesPlaceholder')}
        data-testid="notes-input"
        onChange={(e) => {
          setNotes(e.target.value);
          requestNotesSave(e.target.value, 500);
        }}
        onBlur={() => requestNotesSave(notes, 0)}
      />
      <MonoLabel className={cx('mt-1.5 text-right', notesState === 'failed' && 'text-red-300/80')}>
        {notesState === 'saving' ? t('game:status.saving') : notesState === 'saved' ? t('game:status.saved') : notesState === 'failed' ? t('game:status.failed') : ''}
      </MonoLabel>
    </Panel>
  );
}

/** The manual collections a game is in (issue #45), and the way to change them. */
export function CollectionsSection({ game, collections = [], onManage }: { game: Game; collections?: Collection[]; onManage?: (gameId: string) => void }) {
  const { t } = useTranslation('collections');
  if (!onManage) return null;
  const mine = collections.filter((c) => !c.smart && c.games?.includes(game.id));
  return (
    <Panel className="bg-surface-strong" data-testid="game-collections">
      <h2 className="mb-2 text-[15px] font-bold">{t('collections:gameHeading')}</h2>
      <p className="text-[13px] text-white/75">
        {mine.length > 0 ? mine.map((c) => (c.builtin ? t(`builtin.${c.id}`) : c.name)).join(', ') : t('collections:notInCollections')}
      </p>
      <Button variant="neutral" className="mt-3 w-full" onClick={() => onManage(game.id)}>
        {t('collections:addToCollection')}
      </Button>
    </Panel>
  );
}

/** The Unlink control next to a linked game's attribution. The page owns the
 *  data: the unlink goes through the callback it passes, so this block is
 *  props only and can move in a layout. */
function InfoUnlink({ game, onUnlinked, onUnlink }: { game: Game; onUnlinked: () => void; onUnlink: (id: string) => Promise<void> }) { // i18n:allow: Promise return type, not user-facing text
  const { t } = useTranslation('game');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const unlink = () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    onUnlink(game.id).then(
      () => { setBusy(false); onUnlinked(); },
      (e) => { setBusy(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };
  return (
    <span className="inline-flex items-center gap-2">
      <Button variant="ghost" size="none" onClick={unlink} disabled={busy} data-testid="info-unlink" className="rounded px-0 underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/85">
        {busy ? t('game:about.unlinking') : t('game:about.unlink')}
      </Button>
      {error && <span className="text-red-300/80">{error}</span>}
    </span>
  );
}

/** Find a sideloaded game's info online (issue #38): the same candidate
 *  search as Find art online (SearchArtOnline), the same list (source label,
 *  year), and a pick links via LinkGameInfo. The page owns the data: search
 *  and link go through the callbacks it passes, so this block is props only
 *  and can move in a layout. */
function InfoFinder({ game, onLinked, onSearch, onLink }: {
  game: Game;
  onLinked: () => void;
  onSearch: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLink: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
}) {
  const { t } = useTranslation('game');
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState(game.title);
  const [candidates, setCandidates] = useState<ArtCandidate[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [linking, setLinking] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const search = () => {
    const q = query.trim();
    if (!q || searching || linking != null) return;
    setSearching(true);
    setError(null);
    onSearch(q).then(
      (cs) => { setSearching(false); setCandidates(cs); },
      (e) => { setSearching(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  const link = (c: ArtCandidate) => {
    if (searching || linking != null) return;
    const source = c.source || 'steam';
    const id = source === 'steam' ? (c.id || String(c.app_id)) : c.id;
    setLinking(infoCandidateKey(c));
    setError(null);
    onLink(game.id, source, id).then(
      () => { setLinking(null); onLinked(); },
      (e) => { setLinking(null); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  if (!open) {
    return (
      <div className="mt-3">
        <Button variant="neutral" size="sm" onClick={() => setOpen(true)} data-testid="find-info-online">
          <Search className="h-3.5 w-3.5" /> {t('game:info.find')}
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-3 border-t border-hairline pt-3">
      <MonoLabel className="mb-1.5">{t('game:info.heading')}</MonoLabel>
      <p className="mb-2 text-[12px] leading-relaxed text-text-muted">
        {t('game:info.help', { title: game.title })}
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); search(); }}
      >
        <Input
          aria-label={t('game:info.searchAria')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={game.title}
          className="min-w-0 max-w-xs flex-1"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={!query.trim() || searching || linking != null}>
          <Search className="h-3.5 w-3.5" /> {searching ? t('game:info.searching') : t('game:info.search')}
        </Button>
      </form>
      {candidates != null && candidates.length === 0 && (
        <p className="mt-2 text-[12px] text-text-muted">{t('game:info.noneFound')}</p>
      )}
      {candidates != null && candidates.length > 0 && (
        <ul className="mt-2 space-y-2">
          {candidates.map((c) => (
            <li key={infoCandidateKey(c)} className="flex items-center gap-3 rounded-xl bg-surface px-3 py-2.5 ring-1 ring-hairline">
              <div className="min-w-0 flex-1">
                <div className="truncate text-[12.5px] font-semibold text-text">{c.name}</div>
                <div className="font-mono text-hint text-text-muted">
                  <span data-testid="candidate-source">{infoSourceText(t, c.source)}</span>
                  {c.year ? ` · ${c.year}` : ''} · {infoCandidateDetail(t, c)}
                </div>
              </div>
              <Button variant="primary" size="sm" disabled={searching || linking != null} onClick={() => link(c)}>
                {linking === infoCandidateKey(c) ? t('game:info.linking') : t('game:info.link')}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {error && <ErrorNotice message={error} fallback={t('game:info.linkFailed')} />}
    </div>
  );
}

/** Where a plugin game lives: key facts only. What can be done with it is in the game settings. */
export function InstallationPanel({
  game,
  plugin,
  job,
  onJobAction,
  sourceLogos,
}: {
  game: Game;
  plugin: StorePlugin;
  job?: Job;
  onJobAction: (action: 'cancel' | 'retry' | 'pause' | 'resume', id: string) => void;
  sourceLogos?: Record<string, string>;
}) {
  const { t } = useTranslation('game');
  const active = job && (job.state === 'queued' || job.state === 'running');
  const storeLogo = sourceLogos?.[game.source] ?? sourceLogos?.[game.store];
  const storeName = storeLabel(game.store);
  const storeText = `${storeName} · ${plugin.name}`;
  const storeValue = (
    <span className="inline-flex items-center gap-1.5" data-testid="store-row">
      <PluginLogo src={storeLogo} name={storeName} size="xs" />
      <span>{storeText}</span>
    </span>
  );
  const rows: [string, React.ReactNode, string][] = [
    [t('game:installation.store'), storeValue, storeText],
    // A game the store lists as installed may have no folder Platypus can name: say so.
    [t('game:installation.folder'), game.install_dir || (game.is_installed ? t('game:installation.unknownFolder') : '—'), game.install_dir || ''],
    [t('game:installation.version'), game.version || '—', game.version || ''],
    ...(game.pinned ? [[t('game:installation.pinnedTo'), pinLabel(game.pinned), pinLabel(game.pinned)] as [string, React.ReactNode, string]] : []),
    [t('game:installation.size'), formatBytes(game.size_bytes) || '—', formatBytes(game.size_bytes) || ''],
  ];
  return (
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:installation.heading')}</h2>
      <div className="space-y-2 font-mono text-hint">
        {rows.map(([k, v, title]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-hairline pt-2">
            <span className="text-text-muted">{k}</span>
            <span className="truncate text-right text-white/85 select-text" title={title}>{v}</span>
          </div>
        ))}
      </div>
      {active && <div className="mt-4"><JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} /></div>}
      {job?.state === 'failed' && !active && (
        <ErrorNotice className="mt-2" message={job.error ?? ''} fallback={t('game:installation.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
      )}
    </Panel>
  );
}

export function AchievementsSection({
  ach,
  known,
  viaSteam,
}: {
  ach: NonNullable<GameDetails['achievements']>;
  known: boolean;
  viaSteam: boolean;
}) {
  const { t } = useTranslation('game');
  const pct = known && ach.total > 0 ? Math.round((ach.unlocked / ach.total) * 100) : 0;
  // Backend may send list:null for uncached progress; normalized at the
  // edge in runtime.ts, guarded here too for safety (FE-10).
  const list = ach.list ?? [];
  return (
    <section className="pb-8">
      <div className="mb-3.5 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">{t('game:achievements.heading')}</h2>
        <span className="font-mono text-[12px] tabular-nums text-text-muted">
          {known ? t('game:achievements.countKnown', { unlocked: formatCount(ach.unlocked), total: formatCount(ach.total), percent: formatCount(pct) }) : t('game:achievements.countTotal', { total: formatCount(ach.total) })}
        </span>
      </div>
      {known ? (
        <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
          <div className="h-full rounded-full bg-accent-strong" style={{ width: `${pct}%` }} />
        </div>
      ) : (
        <p className="mb-4 text-[12px] text-text-body">
          {viaSteam
            ? t('game:achievements.steamHint')
            : t('game:achievements.storeHint')}
        </p>
      )}
      <div className="ach-grid">
        {list.map((a) => (
          <AchievementCard key={a.id || a.name} a={a} known={known} />
        ))}
      </div>
      {list.length < ach.total && (
        <p className="mt-3 font-mono text-[12px] text-text-muted">
          {t('game:achievements.showing', { shown: formatCount(list.length), total: formatCount(ach.total), reason: known ? t('game:achievements.reasonSample') : t('game:achievements.reasonHighlight') })}
        </p>
      )}
    </section>
  );
}

function AchievementCard({ a, known }: { a: Achievement; known: boolean }) {
  const { t } = useTranslation('game');
  const dim = known && !a.unlocked;
  const meta = a.unlocked && a.unlocked_at
    ? formatDate(a.unlocked_at)
    : a.global_percent
      ? t('game:achievements.ofPlayers', { share: formatPercentValue(a.global_percent) })
      : '';
  return (
    <div title={a.description} className={cx('flex gap-3 rounded-2xl p-3 ring-1', a.unlocked ? 'bg-accent-strong/[0.07] ring-accent-strong/30' : 'ring-hairline')}>
      {a.icon ? (
        <img src={a.icon} alt="" loading="lazy" className={cx('h-11 w-11 flex-none rounded-lg', dim && 'opacity-40 grayscale')} />
      ) : (
        <span className="grid h-11 w-11 flex-none place-items-center rounded-lg bg-surface-strong"><Trophy className="h-4 w-4 text-text-muted" /></span>
      )}
      <div className={cx('min-w-0', dim && 'opacity-60')}>
        <div className="truncate text-[12px] font-bold">{a.name}</div>
        {a.description && <div className="line-clamp-2 text-hint leading-snug text-text-muted">{a.description}</div>}
        {meta && <div className="mt-0.5 font-mono text-hint text-text-muted">{meta}</div>}
      </div>
    </div>
  );
}
