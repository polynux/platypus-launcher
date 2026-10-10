import React from 'react';
import type { ActivityType } from '../types';
import { cn } from '@/lib/utils';
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
  return <Tag className={cx('rounded-3xl bg-white/[0.045] p-5 ring-1 ring-white/[0.07]', className)}>{children}</Tag>;
}

export const cardClass =
  'group overflow-hidden rounded-2xl bg-white/[0.045] text-left ring-1 ring-white/[0.07] transition duration-300 hover:-translate-y-0.5 hover:ring-amber-300/40 focus-visible:ring-amber-300/60';

export function MonoLabel({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cx('font-mono text-[11px] uppercase tracking-[0.16em] text-white/40', className)}>{children}</div>;
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
    <button onClick={onClick} className="rounded font-mono text-[11.5px] uppercase tracking-widest text-white/40 transition hover:text-white/80">
      {children}
    </button>
  );
}

export interface SegOption<T extends string> {
  value: T;
  label: string;
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
    <span aria-hidden className={cx('relative h-4 w-7 flex-none rounded-full transition-colors', on ? 'bg-amber-300' : 'bg-white/15')}>
      <span className={cx('absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-[#191410] transition-transform', on && 'translate-x-3')} />
    </span>
  );
}

const TIERS = ['platinum', 'gold', 'silver', 'bronze', 'borked'] as const;

export function TierBadge({ tier, suffix }: { tier?: string | null; suffix?: string }) {
  if (!tier) return null;
  const variant = (TIERS as readonly string[]).includes(tier) ? (tier as (typeof TIERS)[number]) : 'silver';
  return (
    <Badge variant={variant} title={TIER_MEANING[tier]}>
      {tier}
      {suffix ? ` ${suffix}` : ''}
    </Badge>
  );
}

export const TIER_TEXT: Record<string, string> = {
  platinum: 'text-amber-300',
  gold: 'text-amber-300/80',
  silver: 'text-white/55',
  bronze: 'text-orange-300',
  borked: 'text-red-300',
};

export const TIER_DOT: Record<string, string> = {
  platinum: 'bg-amber-300',
  gold: 'bg-amber-300/50',
  silver: 'bg-white/40',
  bronze: 'bg-orange-300/70',
  borked: 'bg-red-400',
};

export function TweakDot({ className }: { className?: string }) {
  return (
    <span
      title="Community fix available"
      className={cx('h-2 w-2 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,.8)]', className)}
    />
  );
}

// Derived from ds/Button variants (look only: callers size and lay these out themselves).
export const btnPrimary = buttonVariantClass.primary;
export const btnNeutral = buttonVariantClass.neutral;

export const EVENT_TEXT: Record<ActivityType, string> = {
  session: 'text-emerald-300',
  unlock: 'text-amber-300',
  fix: 'text-sky-300',
  update: 'text-white/60',
  add: 'text-white/60',
  download: 'text-violet-400',
  tweak: 'text-orange-300',
  cloud: 'text-sky-300',
};

export const EVENT_DOT: Record<ActivityType, string> = {
  session: 'bg-emerald-400',
  unlock: 'bg-amber-400',
  fix: 'bg-[#7dd3fc]',
  update: 'bg-white/30',
  add: 'bg-white/30',
  download: 'bg-[#a78bfa]',
  tweak: 'bg-orange-400',
  cloud: 'bg-[#7dd3fc]',
};

export function EmptyState({ label, children, action }: { label: string; children: React.ReactNode; action?: React.ReactNode }) {
  return (
    <div className="rounded-3xl border border-dashed border-white/15 p-10 text-center">
      <MonoLabel>{label}</MonoLabel>
      <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-white/55">{children}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

/** Cover image with a typographic fallback when the CDN has no art. */
export function Cover({ src, title, className }: { src?: string | string[]; title: string; className?: string }) {
  const sources = (Array.isArray(src) ? src : [src]).filter(Boolean) as string[];
  const key = sources.join('|');
  // Track which source list the failure index belongs to so a new list starts at 0.
  const [failed, setFailed] = React.useState({ key, idx: 0 });
  const idx = failed.key === key ? failed.idx : 0;
  const current = sources[idx];
  const fail = React.useCallback(() => setFailed({ key, idx: idx + 1 }), [key, idx]);
  // A cached 404 can fail before React attaches onError; catch it on mount.
  const img = React.useRef<HTMLImageElement>(null);
  React.useEffect(() => {
    const el = img.current;
    if (el?.complete && el.naturalWidth === 0) fail();
  }, [current, fail]);
  if (!current) {
    return (
      <div className={cx('grid place-items-center bg-gradient-to-br from-white/[0.08] to-white/[0.02] p-3 text-center', className)}>
        <span className="line-clamp-3 text-[13px] font-extrabold leading-tight text-white/45">{title}</span>
      </div>
    );
  }
  return <img ref={img} src={current} alt="" loading="lazy" onError={fail} className={cx('object-cover', className)} />;
}

export function relTime(ts: number, now = Date.now()): string {
  const s = Math.max(0, Math.round((now - ts) / 1000));
  if (s < 60) return 'just now';
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

export function clock(ts: number): string {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
}
