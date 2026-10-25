import * as React from 'react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;
export const DialogTitle = DialogPrimitive.Title;
export const DialogDescription = DialogPrimitive.Description;

/**
 * Overlay + centered raised panel. The overlay is the grid that centers the panel;
 * the panel clips and an inner region scrolls, so the scrollbar sits inside the rounded corners.
 */
export function DialogContent({
  className,
  children,
  label,
  sheet = false,
  sheetWidth,
  overlayClassName,
  contentClassName,
  innerClassName,
  ...props
}: Omit<React.ComponentProps<typeof DialogPrimitive.Content>, 'aria-label'> & {
  label: string;
  sheet?: boolean;
  sheetWidth?: string;
  /** Atlas drawer overrides (game-settings.tabs = accordion only): scrim etc. */
  overlayClassName?: string;
  contentClassName?: string;
  innerClassName?: string;
}) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay data-slot="dialog-overlay" className={cn('fixed inset-0 z-50 grid place-items-center bg-black/65 p-6', sheet && 'place-items-stretch justify-items-end p-0', overlayClassName)}>
        <DialogPrimitive.Content
          data-slot="dialog-content"
          aria-label={label}
          aria-describedby={undefined}
          className={cn(
            'relative flex max-h-[86vh] w-full animate-modal-in flex-col overflow-hidden rounded-3xl bg-raised outline-none ring-1 ring-white/10',
            sheet && 'h-full max-h-full w-[min(42rem,92vw)] animate-sheet-in motion-reduce:animate-none rounded-l-3xl rounded-r-none',
            contentClassName,
            className,
          )}
          style={{ boxShadow: '0 40px 100px rgba(0,0,0,.6)', ...(sheetWidth ? { width: sheetWidth } : {}) }}
          {...props}
        >
          <div className={cn('min-h-0 flex-1 overflow-y-auto p-7 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]', innerClassName)}>{children}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Overlay>
    </DialogPrimitive.Portal>
  );
}
