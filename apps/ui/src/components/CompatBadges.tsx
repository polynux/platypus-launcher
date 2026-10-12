import { Badge } from '@/components/ds/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import type { Compat, CompatBadge, Game, WikiCredit } from '../types';
import { appIdSourceLabel, asOfDate, badgeTitle, cardCompat, deckTitle, DECK_LABEL, PCGW_ATTRIBUTION, ratingLine, TIER_LABEL } from '../lib/compat';
import { ExternalLink } from './ExternalLink';
import { cx } from './ui';

/** The badge on a library card: Native for a Linux build, else the game's tier,
 *  with the Steam Deck verdict beside it when Steam has one. Nothing when
 *  there's no answer worth showing. */
export function CompatChip({ game, className }: { game: Game; className?: string }) {
  const c = cardCompat(game);
  const deck = game.fix_info?.deck;
  if (!c && !deck) return null;
  const deckBadge = deck ? <DeckBadge rating={deck} notes={game.fix_info?.deck_notes} /> : null;
  if (c?.kind === 'native') {
    return (
      <span className={cx('inline-flex max-w-full flex-wrap items-center gap-1.5', className)}>
        <Badge variant="info" title="Native Linux build from the store: runs without Proton" className="max-w-full truncate">
          Native
        </Badge>
        {deckBadge}
      </span>
    );
  }
  const tier = c?.kind === 'tier' ? c.tier : '';
  const source = c?.kind === 'tier' ? c.source : '';
  const prefix = source === 'protondb' ? 'ProtonDB' : 'Fix';
  const variant = (['platinum', 'gold', 'silver', 'bronze', 'borked'] as const).find((t) => t === tier) ?? 'silver';
  return (
    <span className={cx('inline-flex max-w-full flex-wrap items-center gap-1.5', className)}>
      {c && (
        <Badge variant={variant} title={`${prefix}: ${TIER_LABEL[tier] ?? tier}`} className="max-w-full truncate">
          <span className="truncate">{prefix} · {TIER_LABEL[tier] ?? tier}</span>
        </Badge>
      )}
      {deckBadge}
    </span>
  );
}

/** One Steam Deck verdict badge, with its caveats in a tooltip. Nothing when
 *  Steam has no verdict. Verified reads green, Playable amber, Unsupported red. */
export function DeckBadge({ rating, notes, className }: { rating: string; notes?: string[] | null; className?: string }) {
  if (!rating) return null;
  const variant = rating === 'verified' ? 'ok' : rating === 'playable' ? 'accent' : 'borked';
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className={cx('cursor-help', className)}>
          Deck · {DECK_LABEL[rating] ?? rating}
        </Badge>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">{deckTitle(rating, notes)}</TooltipContent>
    </Tooltip>
  );
}

/** One DRM or anti-cheat badge, with its source and check date in a tooltip. */
export function ProtectionBadge({ badge }: { badge: CompatBadge }) {
  const variant = badge.kind === 'drm-free' ? 'ok' : badge.kind === 'denuvo' || badge.kind === 'anticheat' ? 'bronze' : 'neutral';
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className="cursor-help">
          {badge.label}
        </Badge>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">{badgeTitle(badge)}</TooltipContent>
    </Tooltip>
  );
}

/** The compatibility and protection block of the game page: the ProtonDB rating
 *  (with a link), the native build, and the DRM and anti-cheat badges. A fact
 *  nobody reported is left out, and then the block says so. */
export function CompatFacts({ compat }: { compat: Compat }) {
  const drm = compat.drm ?? [];
  const anticheat = compat.anticheat ?? [];
  const pdb = compat.protondb;
  return (
    <div className="mt-4 space-y-3">
      {(drm.length > 0 || anticheat.length > 0) ? (
        <div className="space-y-2">
          {drm.length > 0 && (
            <div className="flex flex-wrap gap-1.5" aria-label="DRM">
              {drm.map((b) => <ProtectionBadge key={`${b.kind}-${b.source}-${b.label}`} badge={b} />)}
            </div>
          )}
          {anticheat.length > 0 && (
            <div className="flex flex-wrap gap-1.5" aria-label="Anti-cheat">
              {anticheat.map((b) => <ProtectionBadge key={`${b.kind}-${b.source}-${b.label}`} badge={b} />)}
            </div>
          )}
          {compat.drm_notes && <p className="text-[12px] leading-relaxed text-white/70">{compat.drm_notes}</p>}
        </div>
      ) : (
        <p className="font-mono text-hint text-white/55">No DRM or anti-cheat reported for this game.</p>
      )}
      {pdb && compat.steam_app_id && (
        <p className="font-mono text-hint text-white/55">
          {ratingLine(pdb)}
          {pdb.fetched_at > 0 && ` · as of ${asOfDate(pdb.fetched_at)}`}
          {pdb.stale && ' · refreshing'}
          {' · '}
          <ExternalLink href={pdb.url} className="text-amber-200/80 hover:text-amber-200">
            protondb ↗
          </ExternalLink>
        </p>
      )}
      {compat.deck?.rating && compat.steam_app_id && (
        <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-hint text-white/55">
          <DeckBadge rating={compat.deck.rating} notes={compat.deck.notes} />
          {compat.deck.fetched_at > 0 && <span>as of {asOfDate(compat.deck.fetched_at)}</span>}
          {compat.deck.stale && <span>refreshing</span>}
          <ExternalLink href={compat.deck.url} className="text-amber-200/80 hover:text-amber-200">
            steam ↗
          </ExternalLink>
        </p>
      )}
      {compat.wiki && <WikiCreditLine credit={compat.wiki} />}
      {compat.steam_app_id && (
        <p className="font-mono text-hint text-white/55">
          Steam AppID {compat.steam_app_id} ({appIdSourceLabel(compat.steam_app_id_source)})
        </p>
      )}
    </div>
  );
}

/** The credit under the facts read from PCGamingWiki: the licence line, the page
 *  and the licence as links, and the day Platypus read the page. */
export function WikiCreditLine({ credit }: { credit: WikiCredit }) {
  return (
    <p className="font-mono text-hint leading-relaxed text-white/55" aria-label="PCGamingWiki credit">
      {PCGW_ATTRIBUTION}
      {' · '}
      <ExternalLink href={credit.url} className="text-amber-200/80 hover:text-amber-200">
        {credit.title} on PCGamingWiki ↗
      </ExternalLink>
      {' · '}
      <ExternalLink href={credit.licence_url} className="text-amber-200/80 hover:text-amber-200">
        licence ↗
      </ExternalLink>
      {credit.fetched_at > 0 && ` · read ${asOfDate(credit.fetched_at)}`}
    </p>
  );
}
