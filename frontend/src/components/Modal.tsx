import React from 'react';
import { Dialog, DialogContent } from './ds/dialog';
import { cn } from '@/lib/utils';

/** The shared dialog widths: a dialog states a size instead of a class, so the
 *  small ones (verify, move) don't stretch to the window's full width. */
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
  children: React.ReactNode;
}

/** Centered modal on ds/Dialog (Radix): Esc + click-outside close, focus trapped and restored on close (FE-11). */
export function Modal({ label, onClose, size, className, dismissable = true, children }: ModalProps) {
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        label={label}
        className={cn(size && MODAL_SIZES[size], className)}
        onEscapeKeyDown={(e) => !dismissable && e.preventDefault()}
        onInteractOutside={(e) => !dismissable && e.preventDefault()}
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
