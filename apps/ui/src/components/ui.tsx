import React from 'react';
import type { ActivityType } from '../types';
import { cn } from '@/lib/utils';
import { Inbox } from 'lucide-react';
import { coverMisses } from '../lib/coverMisses';
import { useVariant } from '../lib/theme';
import { buttonVariantClass } from './ds/button';
import { Badge } from './ds/badge';
import { ToggleGroup, ToggleGroupItem } from './ds/toggle-group';
import { TIER_MEANING } from '../lib/compat';

/*
 * App-level primitives and legacy helpers. src/components/ds/ (shadcn/Radix, restyled to
 * DESIGN.md) is the source of truth for controls: Button, Input, Select, NumberField, Switch,
 * ToggleGroup, ScrollArea, Dialog, Badge, Tooltip. Prefer those in new code; the btn* strings,
 * SegRow, TogglePill and TierBadge below are thin adapters kept so existing surfaces keep working.
 */

export const cx = cn;

export function Panel({ className, children, as: Tag = 'div' }: { className?: string; children: React.ReactNode; as?: 'div' | 'section' | 'aside' }) {
  return <Tag className={cx('rounded-3xl bg-surface p-5 ring-1 ring-hairline', className)}>{children}</Tag>;
}

export const cardClass =
  'group overflow-hidden rounded-2xl bg-surface text-left ring-1 ring-hairline transition duration-300 hover:-translate-y-0.5 hover:ring-accent-strong/40 focus-visible:ring-focus/60';

export function MonoLabel({ children, className, as: Tag = 'div' }: { children: React.ReactNode; className?: string; as?: 'div' | 'h2' }) {
  return <Tag className={cx('font-mono text-micro uppercase tracking-[0.16em] text-text-muted', className)}>{children}</Tag>;
}

export function SectionTitle({ children, right }: { children: React.ReactNode; right?: React.ReactNode }) {
  return (
    <div className="mb-3 flex items-baseline justify-between gap-4">
      <h2 className="text-[16px] font-bold tracking-tight">{children}</h2>
      {right}
    </div>
  );
}

export function LinkButton({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} className="rounded font-mono text-micro uppercase tracking-widest text-text-muted transition hover:text-white/80">
      {children}
    </button>
  );
}

export interface SegOption<T extends string> {
  value: T;
  label: React.ReactNode;
  title?: string;
}

export function SegRow<T extends string>({ options, value, onChange, label }: { options: SegOption<T>[]; value: T; onChange: (v: T) => void; label: string }) {
  return (
    <ToggleGroup
      type="single"
      role="radiogroup"
      aria-label={label}
      value={value}
      // Single groups report '' when the active item is clicked again; a radio group never deselects.
      onValueChange={(v) => v && onChange(v as T)}
    >
      {options.map((o) => (
        <ToggleGroupItem key={o.value} value={o.value} title={o.title}>
          {o.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}

/** Visual-only switch for use inside a clickable row/button. For a standalone control use ds/Switch. */
export function TogglePill({ on }: { on: boolean }) {
  return (
    <span aria-hidden className={cx('relative h-4 w-7 flex-none rounded-full transition-colors', on ? 'bg-accent-strong' : 'bg-white/15')}>
      <span className={cx('absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-raised transition-transform', on && 'translate-x-3')} />
    </span>
  );
}

const TIERS = ['platinum', 'gold', 'silver', 'bronze', 'borked'] as const;

export function TierBadge({ tier, suffix, title }: { tier?: string | null; suffix?: string; /** Translated tooltip; defaults to the English meaning. */ title?: string }) {
  if (!tier) return null;
  const variant = (TIERS as readonly string[]).includes(tier) ? (tier as (typeof TIERS)[number]) : 'silver';
  return (
    <Badge variant={variant} title={title ?? TIER_MEANING[tier]}>
      {tier}
      {suffix ? ` ${suffix}` : ''}
    </Badge>
  );
}

export const TIER_TEXT: Record<string, string> = {
  platinum: 'text-accent-strong',
  gold: 'text-accent-strong/80',
  silver: 'text-text-muted',
  bronze: 'text-orange-300',
  borked: 'text-red-300',
};

export const TIER_DOT: Record<string, string> = {
  platinum: 'bg-accent-strong',
  gold: 'bg-accent-strong/50',
  silver: 'bg-white/40',
  bronze: 'bg-orange-300/70',
  borked: 'bg-danger',
};

export function TweakDot({ className }: { className?: string }) {
  return (
    <span
      title="Community fix available"
      className={cx('h-2 w-2 rounded-full bg-attention shadow-[0_0_8px_rgba(251,146,60,.8)]', className)}
    />
  );
}

// Derived from ds/Button variants (look only: callers size and lay these out themselves).
export const btnPrimary = buttonVariantClass.primary;
export const btnNeutral = buttonVariantClass.neutral;

export const EVENT_TEXT: Record<ActivityType, string> = {
  session: 'text-emerald-300',
  unlock: 'text-accent-strong',
  fix: 'text-info',
  update: 'text-text-muted',
  add: 'text-text-muted',
  remove: 'text-text-muted',
  download: 'text-dl',
  tweak: 'text-orange-300',
  cloud: 'text-info',
};

export const EVENT_DOT: Record<ActivityType, string> = {
  session: 'bg-emerald-400',
  unlock: 'bg-accent',
  fix: 'bg-info',
  update: 'bg-white/30',
  add: 'bg-white/30',
  remove: 'bg-white/30',
  download: 'bg-dl',
  tweak: 'bg-attention',
  cloud: 'bg-info',
};

export function EmptyState({ label, children, action }: { label: string; children: React.ReactNode; action?: React.ReactNode }) {
  const chrome = useVariant('empty-state.chrome');
  const icon = useVariant('empty-state.icon');
  return (
    <div className={chrome === 'plain' ? 'rounded-3xl p-10 text-center' : 'rounded-3xl border border-dashed border-white/15 p-10 text-center'}>
      {icon === 'on' && <Inbox aria-hidden className="mx-auto mb-3 h-6 w-6 text-text-faint" />}
      <MonoLabel>{label}</MonoLabel>
      <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-text-body">{children}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

/** A view's header (issue #100): title, subtitle and right-side actions.
 *  The subtitle and actions are the caller's own nodes, so each view keeps
 *  its copy; the variants only move or hide them. Warm (right/shown/regular)
 *  is today's layout. */
export function ViewHeader({ title, subtitle, actions, below, tight }: { title: React.ReactNode; subtitle?: React.ReactNode; actions?: React.ReactNode; below?: React.ReactNode; tight?: boolean }) {
  const actionsAt = useVariant('view-header.actions');
  const showSubtitle = useVariant('view-header.subtitle');
  const size = useVariant('view-header.size');
  const titleClass = size === 'compact' ? 'text-[20px] font-extrabold tracking-tight' : 'text-[23px] font-extrabold tracking-tight';
  const subtitleNode = subtitle && showSubtitle === 'shown' ? subtitle : null;
  if (!actions) {
    return (
      <header className={tight ? 'mb-2' : size === 'compact' ? 'mb-4' : 'mb-6'}>
        <h1 className={titleClass}>{title}</h1>
        {subtitleNode}
        {below}
      </header>
    );
  }
  if (actionsAt === 'below') {
    return (
      <header className={tight ? 'mb-2' : size === 'compact' ? 'mb-4' : 'mb-6'}>
        <h1 className={titleClass}>{title}</h1>
        {subtitleNode}
        <div className="mt-3 flex flex-wrap items-center gap-2.5">{actions}</div>
        {below}
      </header>
    );
  }
  if (below) {
    return (
      <header className="mb-6 space-y-3">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className={titleClass}>{title}</h1>
            {subtitleNode}
          </div>
          {actions}
        </div>
        {below}
      </header>
    );
  }
  return (
    <header className={tight ? 'mb-2 flex flex-wrap items-end justify-between gap-4' : 'mb-6 flex flex-wrap items-end justify-between gap-4'}>
      <div>
        <h1 className={titleClass}>{title}</h1>
        {subtitleNode}
      </div>
      {actions}
    </header>
  );
}

/** One row in a simple list (issue #100): runner, fix and job rows. The kind
 *  keeps each row's box as it is today; the media variant hides the fix
 *  cover in text-only themes, and density tightens the box. Warm
 *  (media on, comfortable) is pixel-identical to today. */
const LISTROW_BOX: Record<'fix' | 'runner' | 'job', { comfortable: string; compact: string }> = {
  fix: {
    comfortable: 'flex w-full gap-4 rounded-2xl bg-surface p-4 text-left ring-1 ring-hairline transition hover:ring-attention/40',
    compact: 'flex w-full gap-3 rounded-2xl bg-surface p-3 text-left ring-1 ring-hairline transition hover:ring-attention/40',
  },
  runner: {
    comfortable: 'flex items-center justify-between gap-4 rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline',
    compact: 'flex items-center justify-between gap-3 rounded-2xl bg-surface px-3 py-2 ring-1 ring-hairline',
  },
  job: {
    comfortable: 'rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline',
    compact: 'rounded-2xl bg-surface px-3 py-2 ring-1 ring-hairline',
  },
};

export function ListRow({ kind, media, children, as: Tag = 'div', buttonProps }: { kind: 'fix' | 'runner' | 'job'; media?: React.ReactNode; children: React.ReactNode; as?: 'div' | 'button'; buttonProps?: React.ButtonHTMLAttributes<HTMLButtonElement> }) {
  const showMedia = useVariant('list-row.media');
  const density = useVariant('list-row.density');
  const box = density === 'compact' ? LISTROW_BOX[kind].compact : LISTROW_BOX[kind].comfortable;
  const mediaNode = kind === 'fix' && media && showMedia === 'on' ? media : null;
  if (Tag === 'button') {
    return (
      <button {...buttonProps} className={box}>
        {mediaNode}
        {children}
      </button>
    );
  }
  return (
    <div className={box}>
      {mediaNode}
      {children}
    </div>
  );
}

/** A group in a simple view (issue #100): the queue and finished groups in
 *  Downloads, the feed panel in Activity. The chrome prop is the view's
 *  current box, so warm stays pixel-identical; the catalog variant drives
 *  the Settings sections, and cool's framed/compact/split choices show in
 *  the headers, empty states and rows instead. */
export function Section({ children, className }: { children: React.ReactNode; className?: string }) {
  const chrome = useVariant('section.chrome');
  return (
    <section data-chrome={chrome} className={className}>
      {children}
    </section>
  );
}

/** Whether a loaded image holds no real art: a 200 with an empty or
 *  undecodable body decodes to 0x0, and some CDNs answer 200 with a 1x1
 *  placeholder instead of a 404. Real covers, banners and heroes are
 *  hundreds of pixels across, so anything at most 1px in either dimension
 *  is a miss, just like a 404. */
function isBadRender(img: HTMLImageElement): boolean {
  return img.naturalWidth <= 1 || img.naturalHeight <= 1;
}

/** Cover image with a typographic fallback when the CDN has no art. Tries
 *  each source in order; a URL that 404'd recently is skipped without a
 *  request (see lib/coverMisses) instead of refetching on every render. */
export function Cover({ src, title, className }: { src?: string | string[]; title: string; className?: string }) {
  const all = (Array.isArray(src) ? src : [src]).filter(Boolean) as string[];
  const sources = all.filter((s) => !coverMisses.isBad(s));
  const key = sources.join('|');
  // Track which source list the failure index belongs to so a new list starts at 0.
  const [failed, setFailed] = React.useState({ key, idx: 0 });
  const idx = failed.key === key ? failed.idx : 0;
  const current = sources[idx];
  const fail = React.useCallback(() => {
    if (current) coverMisses.markBad(current);
    setFailed({ key, idx: idx + 1 });
  }, [key, idx, current]);
  // A missing image can fail before React attaches onError (a cached 404) or
  // load with no pixels at all: a 200 with an empty or undecodable body, or
  // the 1x1 placeholder some CDNs return instead of a 404. Neither fires
  // onError yet nothing shows, so the decoded dimensions decide too: real
  // covers, banners and heroes are hundreds of pixels across.
  const loaded = React.useCallback(
    (e: React.SyntheticEvent<HTMLImageElement>) => {
      if (isBadRender(e.currentTarget)) fail();
    },
    [fail],
  );
  // A cached failure can finish before React attaches any handler; catch it on mount.
  const img = React.useRef<HTMLImageElement>(null);
  React.useEffect(() => {
    const el = img.current;
    if (el?.complete && isBadRender(el)) fail();
  }, [current, fail]);
  if (!current) {
    return (
      <div className={cx('grid place-items-center bg-gradient-to-br from-white/[0.08] to-white/[0.02] p-3 text-center', className)}>
        <span className="line-clamp-3 text-[13px] font-extrabold leading-tight text-white/85">{title}</span>
      </div>
    );
  }
  return <img ref={img} src={current} alt="" loading="lazy" decoding="async" onError={fail} onLoad={loaded} className={cx('object-cover', className)} />;
}

import { formatClock, formatRelativeAge } from '../lib/format';

export function relTime(ts: number, now = Date.now()): string {
  return formatRelativeAge(ts, now);
}

export function clock(ts: number): string {
  return formatClock(ts);
}
