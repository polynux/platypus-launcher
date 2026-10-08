import React from 'react';
import clsx from 'clsx';
import type { ActivityType } from '../types';

export const cx = clsx;

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
    <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button
          key={o.value}
          role="radio"
          aria-checked={value === o.value}
          title={o.title}
          onClick={() => onChange(o.value)}
          className={cx(
            'rounded-lg px-3 py-1.5 font-mono text-[12px] transition',
            value === o.value ? 'bg-amber-300 font-bold text-[#130f0a]' : 'bg-white/[0.06] text-white/60 hover:bg-white/[0.1]'
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function TogglePill({ on }: { on: boolean }) {
  return (
    <span aria-hidden className={cx('relative h-4 w-7 flex-none rounded-full transition-colors', on ? 'bg-amber-300' : 'bg-white/15')}>
      <span className={cx('absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-[#191410] transition-transform', on && 'translate-x-3')} />
    </span>
  );
}

const TIER_STYLE: Record<string, string> = {
  platinum: 'bg-amber-300 text-[#130f0a]',
  gold: 'bg-amber-300/15 text-amber-300 ring-1 ring-amber-300/40',
  silver: 'bg-white/[0.06] text-white/60',
  bronze: 'bg-orange-400/10 text-orange-300',
  borked: 'bg-red-500/15 text-red-300',
};

export function TierBadge({ tier, suffix }: { tier?: string | null; suffix?: string }) {
  if (!tier) return null;
  return (
    <span className={cx('inline-flex h-6 items-center rounded-md px-2 pt-px font-mono leading-none text-[11px] font-bold uppercase tracking-[0.14em]', TIER_STYLE[tier] ?? TIER_STYLE.silver)}>
      {tier}
      {suffix ? ` ${suffix}` : ''}
    </span>
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

export const btnPrimary =
  'rounded-xl bg-amber-300 font-extrabold text-[#130f0a] transition hover:bg-amber-200 disabled:cursor-wait disabled:opacity-60';
export const btnNeutral = 'rounded-xl bg-white/[0.06] font-bold text-white/80 transition hover:bg-white/[0.1]';

export const EVENT_TEXT: Record<ActivityType, string> = {
  session: 'text-emerald-300',
  unlock: 'text-amber-300',
  fix: 'text-sky-300',
  update: 'text-white/60',
  add: 'text-white/60',
  download: 'text-violet-400',
  tweak: 'text-orange-300',
};

export const EVENT_DOT: Record<ActivityType, string> = {
  session: 'bg-emerald-400',
  unlock: 'bg-amber-400',
  fix: 'bg-[#7dd3fc]',
  update: 'bg-white/30',
  add: 'bg-white/30',
  download: 'bg-[#a78bfa]',
  tweak: 'bg-orange-400',
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
