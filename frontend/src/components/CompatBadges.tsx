import { Badge } from '@/components/ds/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ds/tooltip';
import type { Compat, CompatBadge, Game, WikiCredit } from '../types';
import { appIdSourceLabel, asOfDate, badgeTitle, cardCompat, PCGW_ATTRIBUTION, ratingLine, TIER_LABEL } from '../lib/compat';
import { cx } from './ui';

/** The badge on a library card: Native for a Linux build, else the game's tier.
 *  Nothing when there's no answer worth showing. */
export function CompatChip({ game, className }: { game: Game; className?: string }) {
  const c = cardCompat(game);
  if (!c) return null;
  if (c.kind === 'native') {
    return (
      <Badge variant="info" title="Native Linux build from the store: runs without Proton" className={cx('max-w-full truncate', className)}>
        Native
      </Badge>
    );
  }
  const prefix = c.source === 'protondb' ? 'ProtonDB' : 'Fix';
  const variant = (['platinum', 'gold', 'silver', 'bronze', 'borked'] as const).find((t) => t === c.tier) ?? 'silver';
  return (
    <Badge variant={variant} title={`${prefix}: ${TIER_LABEL[c.tier] ?? c.tier}`} className={cx('max-w-full truncate', className)}>
      <span className="truncate">{prefix} · {TIER_LABEL[c.tier] ?? c.tier}</span>
    </Badge>
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
          {compat.drm_notes && <p className="text-[12px] leading-relaxed text-white/55">{compat.drm_notes}</p>}
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
          <a href={pdb.url} target="_blank" rel="noreferrer" className="text-amber-200/80 hover:text-amber-200">
            protondb ↗
          </a>
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
      <a href={credit.url} target="_blank" rel="noreferrer" className="text-amber-200/80 hover:text-amber-200">
        {credit.title} on PCGamingWiki ↗
      </a>
      {' · '}
      <a href={credit.licence_url} target="_blank" rel="noreferrer" className="text-amber-200/80 hover:text-amber-200">
        licence ↗
      </a>
      {credit.fetched_at > 0 && ` · read ${asOfDate(credit.fetched_at)}`}
    </p>
  );
}
