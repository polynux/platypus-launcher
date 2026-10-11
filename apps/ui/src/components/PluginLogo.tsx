import React from 'react';
import { cx } from './ui';

/** Monogram tints, picked by name so a source keeps its tile across views. */
const TINTS = [
  'bg-amber-300/15 text-amber-200',
  'bg-sky-300/15 text-sky-200',
  'bg-emerald-300/15 text-emerald-200',
  'bg-violet-300/15 text-violet-200',
  'bg-rose-300/15 text-rose-200',
  'bg-orange-300/15 text-orange-200',
] as const;

/** The tint class for a name (exported for tests). */
export function tintFor(name: string): string {
  let h = 0;
  for (const c of name.trim().toLowerCase()) h = (h * 31 + c.codePointAt(0)!) >>> 0;
  return TINTS[h % TINTS.length];
}

/** The monogram letter for a name (exported for tests). */
export function monogramFor(name: string): string {
  const t = name.trim();
  return t ? t.charAt(0).toUpperCase() : '?';
}

const BOX: Record<'xs' | 'md', string> = {
  // Sidebar Sources rows (~16 px).
  xs: 'h-4 w-4 rounded text-[9px]',
  // Plugins view cards.
  md: 'h-10 w-10 rounded-xl text-[15px]',
};

/**
 * A store plugin's logo: the locally served file from its folder, drawn
 * through <img> (scripts in SVG never run there). When there is no logo or
 * it fails to load, a monogram tile with a consistent tint — the Cover-like
 * onError approach.
 */
export function PluginLogo({ src, name, size = 'md' }: { src?: string | null; name: string; size?: keyof typeof BOX }) {
  const [broken, setBroken] = React.useState(false);
  React.useEffect(() => setBroken(false), [src]);
  const box = BOX[size];
  if (!src || broken) {
    return (
      <span aria-hidden className={cx('grid flex-none place-items-center font-extrabold ring-1 ring-white/10', tintFor(name), box)}>
        {monogramFor(name)}
      </span>
    );
  }
  return (
    <img
      src={src}
      alt=""
      loading="lazy"
      decoding="async"
      onError={() => setBroken(true)}
      className={cx('flex-none object-contain', box)}
    />
  );
}
