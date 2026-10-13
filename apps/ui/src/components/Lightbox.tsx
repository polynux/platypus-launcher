// i18n: converted
import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronLeft, ChevronRight, X } from 'lucide-react';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ds/tooltip';
import { cx } from './ui';

const FOCUSABLE = 'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])';

/** One media item of the game page's strip: a screenshot or a trailer. */
export interface MediaItem {
  kind: 'image' | 'video';
  src: string;
  thumbnail?: string;
  title?: string;
  mime?: string;
}

export function toMediaItems(images: string[], videos?: { url: string; thumbnail?: string; title?: string; mime?: string }[]): MediaItem[] {
  const out: MediaItem[] = (images ?? []).map((src) => ({ kind: 'image' as const, src }));
  for (const v of videos ?? []) {
    if (v?.url) out.push({ kind: 'video', src: v.url, thumbnail: v.thumbnail, title: v.title, mime: v.mime });
  }
  return out;
}

/** Full-window media viewer. Wraps around at both ends; the parent owns the
 *  index so the gallery behind it stays in sync. Images show full size;
 *  videos play click-to-play (controls, muted, never autoplay with sound).
 *  Keyboard arrows move, Esc closes; wheel and touch swipes move too. Focus
 *  moves into the dialog on open and returns on close. */
export function Lightbox({ images, items, index, alt, onIndex, onClose }: {
  images?: string[];
  items?: MediaItem[];
  index: number;
  alt: string;
  onIndex: (i: number) => void;
  onClose: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const touchX = useRef<number | null>(null);
  const { t } = useTranslation('common');
  const list: MediaItem[] = items ?? (images ?? []).map((src) => ({ kind: 'image' as const, src }));
  const n = list.length;
  const go = (d: number) => onIndex((index + d + n) % n);
  const [failed, setFailed] = useState<string | null>(null);

  useEffect(() => {
    setFailed(null);
  }, [index]);

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    root.current?.querySelector<HTMLElement>('[data-autofocus]')?.focus();
    return () => prev?.focus?.();
  }, []);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') { e.stopPropagation(); onClose(); }
    else if (e.key === 'ArrowRight' && n > 1) { e.preventDefault(); go(1); }
    else if (e.key === 'ArrowLeft' && n > 1) { e.preventDefault(); go(-1); }
    else if (e.key === 'Tab') {
      const els = Array.from(root.current?.querySelectorAll<HTMLElement>(FOCUSABLE) ?? []);
      if (els.length === 0) return;
      const first = els[0];
      const last = els[els.length - 1];
      const cur = document.activeElement;
      if (e.shiftKey && (cur === first || !root.current?.contains(cur))) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && (cur === last || !root.current?.contains(cur))) { e.preventDefault(); first.focus(); }
    }
  };

  const onWheel = (e: React.WheelEvent) => {
    if (n <= 1) return;
    if (Math.abs(e.deltaY) < 12 && Math.abs(e.deltaX) < 12) return;
    // A video's own wheel (volume/scroll) stays with the player.
    if ((e.target as HTMLElement)?.closest?.('video')) return;
    e.preventDefault();
    go((e.deltaY || e.deltaX) > 0 ? 1 : -1);
  };

  /** Icon button with a tooltip; the content sits above the lightbox (z-100). */
  const tip = (label: string, btn: React.ReactNode, side: 'left' | 'right' | 'bottom') => (
    <Tooltip>
      <TooltipTrigger asChild>{btn}</TooltipTrigger>
      <TooltipContent side={side} className="z-[110]">{label}</TooltipContent>
    </Tooltip>
  );

  const nav = 'absolute z-10 h-11 w-11 rounded-full bg-black/60 text-white/85 ring-1 ring-white/15 hover:bg-black/80 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300';
  const cur = list[index];
  const label = cur
    ? cur.kind === 'video'
      ? t('common:lightbox.video', { alt, index: index + 1 }) + (cur.title ? t('common:lightbox.videoTitle', { title: cur.title }) : '')
      : t('common:lightbox.screenshot', { alt, index: index + 1 })
    : t('common:lightbox.media', { alt, index: index + 1 });

  return (
    <TooltipProvider>
    <div
      ref={root}
      role="dialog"
      aria-modal="true"
      aria-label={t('common:lightbox.viewer', { alt })}
      onKeyDown={onKeyDown}
      onClick={onClose}
      onWheel={onWheel}
      onTouchStart={(e) => { touchX.current = e.touches[0]?.clientX ?? null; }}
      onTouchEnd={(e) => {
        const x0 = touchX.current;
        touchX.current = null;
        if (x0 == null || n <= 1) return;
        const dx = (e.changedTouches[0]?.clientX ?? x0) - x0;
        if (Math.abs(dx) > 40) go(dx < 0 ? 1 : -1);
      }}
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/90 motion-safe:animate-[fadeIn_.15s_ease-out]"
    >
      <style>{'@keyframes fadeIn{from{opacity:0}to{opacity:1}}'}</style>
      {tip(t('common:lightbox.close'), <Button
        data-autofocus
        size="none"
        onClick={(e) => { e.stopPropagation(); onClose(); }}
        aria-label={t('common:lightbox.closeViewer')}
        className={cx(nav, 'right-5 top-5')}
      >
        <X size={18} />
      </Button>, 'left')}
      {n > 1 && (
        tip(t('common:lightbox.previous'), <Button size="none" onClick={(e) => { e.stopPropagation(); go(-1); }} aria-label={t('common:lightbox.previousMedia')} className={cx(nav, 'left-5 top-1/2 -translate-y-1/2')}>
          <ChevronLeft size={22} />
        </Button>, 'right')
      )}
      <div onClick={(e) => e.stopPropagation()} className="flex max-h-[calc(100vh-6rem)] max-w-[calc(100vw-9rem)] flex-col items-center gap-2">
        {cur?.kind === 'video' ? (
          failed ? (
            <p role="alert" className="max-w-md text-center text-[13px] text-white/70">{t('common:lightbox.videoError')}</p>
          ) : (
            <video
              key={cur.src}
              src={cur.src}
              controls
              muted
              playsInline
              preload="metadata"
              aria-label={label}
              data-testid="lightbox-video"
              onError={() => setFailed(cur.src)}
              className="max-h-[calc(100vh-9rem)] max-w-[calc(100vw-9rem)] select-none bg-black"
            />
          )
        ) : cur ? (
          <img
            src={cur.src}
            alt={label}
            className="max-h-[calc(100vh-6rem)] max-w-[calc(100vw-9rem)] select-none object-contain"
          />
        ) : null}
        {cur?.kind === 'video' && cur.title && (
          <p className="max-w-full truncate text-[12px] text-white/70">{cur.title}</p>
        )}
      </div>
      {n > 1 && (
        tip(t('common:lightbox.next'), <Button size="none" onClick={(e) => { e.stopPropagation(); go(1); }} aria-label={t('common:lightbox.nextMedia')} className={cx(nav, 'right-5 top-1/2 -translate-y-1/2')}>
          <ChevronRight size={22} />
        </Button>, 'left')
      )}
      <div
        aria-live="polite"
        onClick={(e) => e.stopPropagation()}
        className="absolute bottom-5 left-1/2 -translate-x-1/2 rounded-md bg-black/60 px-2.5 py-1 font-mono text-[12px] leading-none text-white/75"
      >
        {index + 1} / {n}
      </div>
    </div>
    </TooltipProvider>
  );
}
