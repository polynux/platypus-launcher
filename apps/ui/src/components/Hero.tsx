// The themeable hero frame (issue #101): one frame for the dashboard and
// game page heroes. It owns the art background, the scrim gradient, the
// height and the chip style through the hero catalog axes; the content blocks
// (dashboard.attention, game.heading, game.actions) render inside it in fixed
// positions, so the game page (#103) can place them in a layout later. Warm
// keeps today's looks through the defaults: the dashboard reads hero.* (its
// tall glass banner), the game reads game-hero.* (its regular solid one,
// following hero.* unless the theme overrides it). Cool picks the compact
// values.
//
// The library's last-played strip stays its own tile: it is a navigation
// button with a progress bar and a hover play affordance, not a banner with
// content blocks, so forcing it into this frame would add button, overlay
// and progress props for one use.
import type { ReactNode } from 'react';
import { Cover, cx } from './ui';
import { useVariant } from '../lib/theme';

/** Which hero use site: the dashboard reads the global hero axes, the game
 *  page reads game-hero (which follows hero unless the theme names a value
 *  for the game itself). */
export type HeroSite = 'dashboard' | 'game';

const HEIGHT_CLASS: Record<string, string> = {
  tall: 'min-h-[290px]',
  regular: 'min-h-[280px]',
  compact: 'min-h-[210px]',
};

// The scrim keeps its direction per use site (structural, as today): the
// dashboard fades sideways, the game page from below. The variant picks the
// palette: warm is today's dark scrim, neutral its slate counterpart, none
// leaves the art bare.
function heroGradient(site: HeroSite, gradient: string): string | null {
  if (gradient === 'none') return null;
  const ink = gradient === 'neutral' ? '11,17,24' : '19,15,10';
  return site === 'game'
    ? `linear-gradient(0deg, rgba(${ink},.98) 10%, rgba(${ink},.35) 62%, rgba(${ink},.25) 100%)`
    : `linear-gradient(84deg, rgba(${ink},.97) 30%, rgba(${ink},.35) 62%, transparent)`;
}

/** The chip style the hero content uses: glass is today's dashboard stat box
 *  (a blurred dark chip), solid today's game badge and button. */
export function heroChipClass(chips: string): string {
  return chips === 'solid' ? 'bg-black/50' : 'bg-black/45 backdrop-blur';
}

interface HeroProps {
  art: string[];
  title: string;
  site: HeroSite;
  children: ReactNode;
}

export function Hero({ art, title, site, children }: HeroProps) {
  const height = useVariant(site === 'game' ? 'game-hero.height' : 'hero.height');
  const gradient = useVariant(site === 'game' ? 'game-hero.gradient' : 'hero.gradient');
  const scrim = heroGradient(site, gradient);
  return (
    <section className={cx('relative flex min-h-[280px] flex-col justify-end overflow-hidden rounded-3xl ring-1 ring-hairline', HEIGHT_CLASS[height] ?? HEIGHT_CLASS.tall, site === 'game' && 'mb-6 mt-4')}>
      <Cover src={art} title={title} className="absolute inset-0 h-full w-full" />
      {scrim && <div className="absolute inset-0" style={{ background: scrim }} />}
      {site === 'game' ? (
        <div className="relative flex flex-wrap items-end justify-between gap-x-6 gap-y-4 px-8 pb-6 pt-16">{children}</div>
      ) : (
        <>{children}</>
      )}
    </section>
  );
}
