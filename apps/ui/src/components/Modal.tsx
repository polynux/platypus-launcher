import React, { useRef } from 'react';
import { cn } from '@/lib/utils';
import { Dialog, DialogContent } from './ds/dialog';

/** The shared dialog widths: a dialog states a size instead of a class. Without
 *  one a dialog is `sm`, so a small dialog never stretches to the window's full width. */
export const MODAL_SIZES = {
  sm: 'max-w-[440px]',
  md: 'max-w-[560px]',
  lg: 'max-w-[720px]',
  xl: 'max-w-[920px]',
} as const;
export type ModalSize = keyof typeof MODAL_SIZES;

interface ModalProps {
  label: string;
  onClose: () => void;
  /** Width of the dialog; className can still override it. */
  size?: ModalSize;
  className?: string;
  /** false = Esc and click-outside are ignored (e.g. unsaved edits); the owner's own buttons still close. */
  dismissable?: boolean;
  presentation?: 'dialog' | 'sheet';
  /** A sheet's own width (a CSS length), when it isn't the default 42rem. */
  sheetWidth?: string;
  /** Radix's open autofocus, for surfaces (like the atlas drawer) that focus
   *  their own panel instead of the first control, so a mouse open shows no
   *  focus ring. */
  onOpenAutoFocus?: (e: Event) => void;
  children: React.ReactNode;
}

/** Centered modal on ds/Dialog (Radix): Esc + click-outside close, focus trapped and restored on close (FE-11).
 *  Narrow by default (`sm`); a dialog that needs more passes a `size`. */
export function Modal({ label, onClose, size = 'sm', className, dismissable = true, presentation = 'dialog', sheetWidth, onOpenAutoFocus, children }: ModalProps) {
  const restoreFocus = useRef<HTMLElement | null>(typeof document === 'undefined' ? null : document.activeElement as HTMLElement);
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        label={label}
        sheet={presentation === 'sheet'}
        sheetWidth={sheetWidth}
        className={cn(MODAL_SIZES[size], className)}
        onEscapeKeyDown={(e) => !dismissable && e.preventDefault()}
        onInteractOutside={(e) => !dismissable && e.preventDefault()}
        onOpenAutoFocus={onOpenAutoFocus}
        onCloseAutoFocus={(e) => {
          if (!restoreFocus.current?.isConnected) return;
          e.preventDefault();
          restoreFocus.current.focus();
        }}
      >
        {children}
      </DialogContent>
    </Dialog>
  );
}

/** A dialog's content in its own Modal, or inline (`embedded`) where a tab of
 *  the game settings shows it. */
export function ModalOrInline({ embedded, children, ...modal }: ModalProps & { embedded?: boolean }) {
  return embedded ? <>{children}</> : <Modal {...modal}>{children}</Modal>;
}
