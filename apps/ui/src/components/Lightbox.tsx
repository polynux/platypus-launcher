import React, { useEffect, useRef } from 'react';
import { ChevronLeft, ChevronRight, X } from 'lucide-react';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ds/tooltip';
import { cx } from './ui';

const FOCUSABLE = 'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])';

/** Full-window image viewer. Wraps around at both ends; the parent owns the
 *  index so the gallery behind it stays in sync. Focus moves into the dialog
 *  on open and returns to the previously focused element on close. */
export function Lightbox({ images, index, alt, onIndex, onClose }: {
  images: string[];
  index: number;
  alt: string;
  onIndex: (i: number) => void;
  onClose: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const n = images.length;
  const go = (d: number) => onIndex((index + d + n) % n);

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

  /** Icon button with a tooltip; the content sits above the lightbox (z-100). */
  const tip = (label: string, btn: React.ReactNode, side: 'left' | 'right' | 'bottom') => (
    <Tooltip>
      <TooltipTrigger asChild>{btn}</TooltipTrigger>
      <TooltipContent side={side} className="z-[110]">{label}</TooltipContent>
    </Tooltip>
  );

  const nav = 'absolute z-10 h-11 w-11 rounded-full bg-black/60 text-white/85 ring-1 ring-white/15 hover:bg-black/80 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300';

  return (
    <TooltipProvider>
    <div
      ref={root}
      role="dialog"
      aria-modal="true"
      aria-label={`${alt} screenshots`}
      onKeyDown={onKeyDown}
      onClick={onClose}
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/90 motion-safe:animate-[fadeIn_.15s_ease-out]"
    >
      <style>{'@keyframes fadeIn{from{opacity:0}to{opacity:1}}'}</style>
      {tip('Close', <Button
        data-autofocus
        size="none"
        onClick={(e) => { e.stopPropagation(); onClose(); }}
        aria-label="Close slideshow"
        className={cx(nav, 'right-5 top-5')}
      >
        <X size={18} />
      </Button>, 'left')}
      {n > 1 && (
        tip('Previous', <Button size="none" onClick={(e) => { e.stopPropagation(); go(-1); }} aria-label="Previous screenshot" className={cx(nav, 'left-5 top-1/2 -translate-y-1/2')}>
          <ChevronLeft size={22} />
        </Button>, 'right')
      )}
      <img
        src={images[index]}
        alt={`${alt} screenshot ${index + 1}`}
        onClick={(e) => e.stopPropagation()}
        className="max-h-[calc(100vh-6rem)] max-w-[calc(100vw-9rem)] select-none object-contain"
      />
      {n > 1 && (
        tip('Next', <Button size="none" onClick={(e) => { e.stopPropagation(); go(1); }} aria-label="Next screenshot" className={cx(nav, 'right-5 top-1/2 -translate-y-1/2')}>
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
