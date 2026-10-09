import React from 'react';
import { cn } from '@/lib/utils';
import { Dialog, DialogContent } from './ds/dialog';

interface ModalProps {
  label: string;
  onClose: () => void;
  className?: string;
  /** false = Esc and click-outside are ignored (e.g. unsaved edits); the owner's own buttons still close. */
  dismissable?: boolean;
  children: React.ReactNode;
}

/** Centered modal on ds/Dialog (Radix): Esc + click-outside close, focus trapped and restored on close (FE-11).
 *  Narrow by default (max-w-md); a dialog that needs more passes its own max-w-* in className. */
export function Modal({ label, onClose, className, dismissable = true, children }: ModalProps) {
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        label={label}
        className={cn('max-w-md', className)}
        onEscapeKeyDown={(e) => !dismissable && e.preventDefault()}
        onInteractOutside={(e) => !dismissable && e.preventDefault()}
      >
        {children}
      </DialogContent>
    </Dialog>
  );
}
