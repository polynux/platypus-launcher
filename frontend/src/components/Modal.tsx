import React from 'react';
import { Dialog, DialogContent } from './ds/dialog';

interface ModalProps {
  label: string;
  onClose: () => void;
  className?: string;
  /** false = Esc and click-outside are ignored (e.g. unsaved edits); the owner's own buttons still close. */
  dismissable?: boolean;
  children: React.ReactNode;
}

/** Centered modal on ds/Dialog (Radix): Esc + click-outside close, focus trapped and restored on close (FE-11). */
export function Modal({ label, onClose, className, dismissable = true, children }: ModalProps) {
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        label={label}
        className={className}
        onEscapeKeyDown={(e) => !dismissable && e.preventDefault()}
        onInteractOutside={(e) => !dismissable && e.preventDefault()}
      >
        {children}
      </DialogContent>
    </Dialog>
  );
}
