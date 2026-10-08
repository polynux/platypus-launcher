import * as React from 'react';
import { ScrollArea as ScrollAreaPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

/**
 * Scroll container with a thin overlay thumb that widens and brightens on hover.
 * Give it a bounded size (h-*, max-h-*, flex-1 min-h-0). `viewportClassName`/`viewportRef` reach the scrolling element.
 */
export function ScrollArea({
  className,
  viewportClassName,
  viewportRef,
  orientation = 'vertical',
  children,
  ...props
}: React.ComponentProps<typeof ScrollAreaPrimitive.Root> & {
  viewportClassName?: string;
  viewportRef?: React.Ref<HTMLDivElement>;
  /** Which scrollbars to render. */
  orientation?: 'vertical' | 'horizontal' | 'both';
}) {
  return (
    <ScrollAreaPrimitive.Root data-slot="scroll-area" className={cn('relative overflow-hidden', className)} {...props}>
      <ScrollAreaPrimitive.Viewport ref={viewportRef} className={cn('size-full rounded-[inherit] [&>div]:!block', viewportClassName)}>
        {children}
      </ScrollAreaPrimitive.Viewport>
      {orientation !== 'horizontal' && <ScrollBar orientation="vertical" />}
      {orientation !== 'vertical' && <ScrollBar orientation="horizontal" />}
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}

export function ScrollBar({ className, orientation = 'vertical', ...props }: React.ComponentProps<typeof ScrollAreaPrimitive.ScrollAreaScrollbar>) {
  return (
    <ScrollAreaPrimitive.ScrollAreaScrollbar
      data-slot="scroll-bar"
      orientation={orientation}
      className={cn(
        'group/sb flex touch-none select-none p-0.5',
        orientation === 'vertical' && 'h-full w-3 justify-center',
        orientation === 'horizontal' && 'h-3 flex-col justify-center',
        className,
      )}
      {...props}
    >
      <ScrollAreaPrimitive.ScrollAreaThumb
        className={cn(
          'relative rounded-full bg-white/10 opacity-70 transition-[width,height,opacity,background-color] duration-200 ease-out group-hover/sb:bg-white/25 group-hover/sb:opacity-100 group-active/sb:bg-amber-300/50',
          orientation === 'vertical' ? 'w-1 group-hover/sb:w-2' : 'h-1 group-hover/sb:h-2',
        )}
      />
    </ScrollAreaPrimitive.ScrollAreaScrollbar>
  );
}
