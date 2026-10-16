// i18n: converted
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ds/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import type { Compat, CompatBadge, Game, WikiCredit } from '../types';
import { asOfDate, badgeTitle, cardCompat, TIER_LABEL } from '../lib/compat';
import { ExternalLink } from './ExternalLink';
import { cx } from './ui';

type T = (key: string, vars?: Record<string, unknown>) => string;

// The display strings of lib/compat.ts live in the game catalog now; the lib
// keeps its logic (cardCompat, tierFilterValue) and its English unit-pinned
// helpers. These map tiers and verdicts to the catalog, falling back to the
// raw id for values the catalog never heard of.
export function tierLabel(t: T, tier: string): string {
  switch (tier) {
    case 'platinum':
      return t('game:tier.platinum');
    case 'gold':
      return t('game:tier.gold');
    case 'silver':
      return t('game:tier.silver');
    case 'bronze':
      return t('game:tier.bronze');
    case 'borked':
      return t('game:tier.borked');
    default:
      return tier;
  }
}

export function deckLabel(t: T, rating: string): string {
  switch (rating) {
    case 'verified':
      return t('game:deck.verified');
    case 'playable':
      return t('game:deck.playable');
    case 'unsupported':
      return t('game:deck.unsupported');
    default:
      return rating;
  }
}

function sourceLabel(t: T, source?: string): string {
  switch (source) {
    case 'user':
      return t('game:appIdSource.user');
    case 'plugin':
      return t('game:appIdSource.plugin');
    case 'steam':
      return t('game:appIdSource.steam');
    default:
      return t('game:appIdSource.none');
  }
}

/** What a ProtonDB tier means for the player, through the catalog. */
export function tierMeaning(t: T, tier: string): string | undefined {
  switch (tier) {
    case 'platinum':
      return t('game:tierMeaning.platinum');
    case 'gold':
      return t('game:tierMeaning.gold');
    case 'silver':
      return t('game:tierMeaning.silver');
    case 'bronze':
      return t('game:tierMeaning.bronze');
    case 'borked':
      return t('game:tierMeaning.borked');
    default:
      return undefined;
  }
}

/** The tooltip of a Steam Deck badge: the verdict, its caveats, and the source. */
function deckTooltip(t: T, rating: string, notes?: string[] | null): string {
  const body = (notes ?? []).filter(Boolean).join(' ');
  return t('game:deck.tooltip', { verdict: deckLabel(t, rating), notes: body ? ` ${body}` : '' });
}

/** The badge on a library card: Native for a Linux build, else the game's tier,
 *  with the Steam Deck verdict beside it when Steam has one. Nothing when
 *  there's no answer worth showing.
 *
 *  Card layout (issues #61, #62): the row wraps, every badge stays inside the
 *  card (`min-w-0 max-w-full`, shrink-capped with ellipsis as a last resort),
 *  and each badge carries a full and a compact label — the card (`.card-compat`,
 *  see style.css) shows the full "ProtonDB · Gold" text on wide cards and the
 *  tier/verdict only on narrow ones, the full text kept in the tooltip. */
export function CompatChip({ game, className, hideNative }: { game: Game; className?: string; /** Hide the whole chip for native builds (dashboard). */ hideNative?: boolean }) {
  const { t } = useTranslation('game');
  if (hideNative && game.native) return null;
  const c = cardCompat(game);
  const deck = game.fix_info?.deck;
  if (!c && !deck) return null;
  const deckBadge = deck ? <DeckBadge rating={deck} notes={game.fix_info?.deck_notes} /> : null;
  if (c?.kind === 'native') {
    return (
      <span className={cx('inline-flex min-w-0 max-w-full flex-wrap content-start items-start gap-1.5', className)}>
        <Badge variant="info" title={t('game:compat.nativeTitle')} className="min-w-0 max-w-full shrink-0 truncate">
          {t('game:compat.native')}
        </Badge>
        {deckBadge}
      </span>
    );
  }
  const tier = c?.kind === 'tier' ? c.tier : '';
  const source = c?.kind === 'tier' ? c.source : '';
  const prefix = source || t('game:compat.fixPrefix');
  const label = TIER_LABEL[tier] ? tierLabel(t, tier) : tier;
  const variant = (['platinum', 'gold', 'silver', 'bronze', 'borked'] as const).find((x) => x === tier) ?? 'silver';
  const chip = c && (
    <Badge variant={variant} title={c.source ? `${c.source} · ${label}${c.checkedAt ? ` · ${asOfDate(c.checkedAt)}` : ''}` : t('game:compat.tierTitle', { prefix, tier: label })} className="min-w-0 max-w-full shrink-0 truncate">
      <span className="badge-full truncate leading-normal">{prefix} · {label}</span>
      <span className="badge-short truncate leading-normal">{label}</span>
    </Badge>
  );
  return (
    <span className={cx('inline-flex min-w-0 max-w-full flex-wrap content-start items-start gap-1.5', className)}>
      {c?.link ? <ExternalLink href={c.link} title={c.checkedAt ? `${source} · ${label} · ${asOfDate(c.checkedAt)}` : `${source} · ${label}`}>{chip}</ExternalLink> : chip}
      {deckBadge}
    </span>
  );
}

/** The shared text block of a game card (issues #61, #62): title, meta line
 *  and badges, top-aligned, with a badge row of stable height so cards in a
 *  row stay aligned however many badges (0, 1 or 2) each card shows. */
export function GameCardBody({ title, meta, game, hideNative, className }: {
  title: string;
  /** One-line summary under the title (playtime, recency, store…); kept whole in a tooltip. */
  meta: string;
  game: Game;
  /** Hide the whole chip for native builds (dashboard). */
  hideNative?: boolean;
  className?: string;
}) {
  return (
    <div className={cx('flex min-w-0 flex-1 flex-col justify-start', className)}>
      <div className="truncate text-hint font-bold" title={title}>{title}</div>
      <div className="truncate font-mono text-micro uppercase tracking-wider text-white/55" title={meta || undefined}>{meta || ' '}</div>
      <div className="mt-1.5 min-h-[3.375rem] min-w-0 max-w-full">
        <CompatChip game={game} hideNative={hideNative} />
      </div>
    </div>
  );
}

/** One Steam Deck verdict badge, with its caveats in a tooltip. Nothing when
 *  Steam has no verdict. Verified reads green, Playable amber, Unsupported red.
 *  Like the tier badge it carries a full ("Deck · Verified") and a compact
 *  (verdict only) label for narrow cards, the full text kept in the tooltip. */
export function DeckBadge({ rating, notes, className }: { rating: string; notes?: string[] | null; className?: string }) {
  const { t } = useTranslation('game');
  if (!rating) return null;
  const variant = rating === 'verified' ? 'ok' : rating === 'playable' ? 'accent' : 'borked';
  const verdict = deckLabel(t, rating);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className={cx('min-w-0 max-w-full shrink-0 cursor-help truncate', className)}>
          <span className="badge-full truncate leading-normal">{t('game:deck.badge', { verdict })}</span>
          <span className="badge-short truncate leading-normal">{verdict}</span>
        </Badge>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">{deckTooltip(t, rating, notes)}</TooltipContent>
    </Tooltip>
  );
}

/** One DRM or anti-cheat badge, with its source and check date in a tooltip. */
export function ProtectionBadge({ badge }: { badge: CompatBadge }) {
  const { t } = useTranslation('game');
  const variant = badge.kind === 'drm-free' ? 'ok' : badge.kind === 'denuvo' || badge.kind === 'anticheat' ? 'bronze' : 'neutral';
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className="cursor-help">
          {badge.label}
        </Badge>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">{`${badge.detail ? `${badge.detail} ` : ''}${t('game:compat.badgeSource', { source: badge.source, date: asOfDate(badge.checked_at) })}`}</TooltipContent>
    </Tooltip>
  );
}

/** The compatibility and protection block of the game page: the ProtonDB rating
 *  (with a link), the native build, and the DRM and anti-cheat badges. A fact
 *  nobody reported is left out, and then the block says so. */
export function CompatFacts({ compat }: { compat: Compat }) {
  const { t } = useTranslation('game');
  const drm = compat.drm ?? [];
  const anticheat = compat.anticheat ?? [];
  const badges = compat.badges ?? [];
  return (
    <div className="mt-4 space-y-3">
      {(badges.length > 0 || drm.length > 0 || anticheat.length > 0) ? (
        <div className="space-y-2">
          {badges.length > 0 && (
            <div className="flex flex-wrap gap-1.5" aria-label={t('game:compat.providerLabel')}>
              {badges.map((b) => {
                const variant = (['platinum', 'gold', 'silver', 'bronze', 'borked'] as const).find((x) => x === b.tone) ?? 'neutral';
                const chip = <Badge variant={variant} title={badgeTitle(b)}>{b.label}</Badge>;
                return b.link ? <ExternalLink key={`${b.kind}-${b.source}-${b.label}`} href={b.link}>{chip}</ExternalLink> : <span key={`${b.kind}-${b.source}-${b.label}`}>{chip}</span>;
              })}
            </div>
          )}
          {drm.length > 0 && (
            <div className="flex flex-wrap gap-1.5" aria-label={t('game:compat.drmLabel')}>
              {drm.map((b) => <ProtectionBadge key={`${b.kind}-${b.source}-${b.label}`} badge={b} />)}
            </div>
          )}
          {anticheat.length > 0 && (
            <div className="flex flex-wrap gap-1.5" aria-label={t('game:compat.anticheatLabel')}>
              {anticheat.map((b) => <ProtectionBadge key={`${b.kind}-${b.source}-${b.label}`} badge={b} />)}
            </div>
          )}
          {compat.drm_notes && <p className="text-[12px] leading-relaxed text-white/70">{compat.drm_notes}</p>}
        </div>
      ) : (
        <p className="font-mono text-hint text-white/55">{t('game:compat.noneReported')}</p>
      )}
      {compat.notes?.map((note, i) => <p key={i} className="text-[12px] leading-relaxed text-white/70">{note}</p>)}
      {compat.deck?.rating && compat.steam_app_id && (
        <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-hint text-white/55">
          <DeckBadge rating={compat.deck.rating} notes={compat.deck.notes} />
          {compat.deck.fetched_at > 0 && <span>{t('game:compat.asOf', { date: asOfDate(compat.deck.fetched_at) })}</span>}
          {compat.deck.stale && <span>{t('game:compat.refreshing')}</span>}
          <ExternalLink href={compat.deck.url} className="text-amber-200/80 hover:text-amber-200">
            {t('game:compat.steamLink')}
          </ExternalLink>
        </p>
      )}
      {compat.wiki && <WikiCreditLine credit={compat.wiki} />}
      {compat.steam_app_id && (
        <p className="font-mono text-hint text-white/55">
          {t('game:compat.appIdLine', { id: compat.steam_app_id, source: sourceLabel(t, compat.steam_app_id_source) })}
        </p>
      )}
    </div>
  );
}

/** The credit under the facts read from PCGamingWiki: the licence line, the page
 *  and the licence as links, and the day Platypus read the page. */
export function WikiCreditLine({ credit }: { credit: WikiCredit }) {
  const { t } = useTranslation('game');
  return (
    <p className="font-mono text-hint leading-relaxed text-white/55" aria-label={t('game:wiki.creditLabel')}>
      {t('game:wiki.attribution')}
      {' · '}
      <ExternalLink href={credit.url} className="text-amber-200/80 hover:text-amber-200">
        {t('game:wiki.onWiki', { title: credit.title })}
      </ExternalLink>
      {' · '}
      <ExternalLink href={credit.licence_url} className="text-amber-200/80 hover:text-amber-200">
        {t('game:wiki.licence')}
      </ExternalLink>
      {credit.fetched_at > 0 && t('game:wiki.readOn', { date: asOfDate(credit.fetched_at) })}
    </p>
  );
}
