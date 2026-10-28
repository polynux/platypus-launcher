import React, { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Dialog, DialogContent } from './ds/dialog';
import { useVariant } from '../lib/theme';

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
  /** Where focus goes when the saved opener is gone (unmounted before close,
   *  e.g. the drawer opened from Tune's All settings): called only then. */
  returnFocus?: () => HTMLElement | null;
  /** Atlas drawer chrome (game-settings.tabs = accordion only): scrim, radius
   *  and inner scroll. Other sheets keep the shared look. */
  overlayClassName?: string;
  contentClassName?: string;
  innerClassName?: string;
  /** A dialog's own shadow, when it isn't the shared one (the /22 palette). */
  contentStyle?: React.CSSProperties;
  /** Radix's Escape hook, for dialogs that leave it (the /22 palette's sub-mode). */
  onEscapeKeyDown?: (e: KeyboardEvent) => void;
  children: React.ReactNode;
}

/** Centered modal on ds/Dialog (Radix): Esc + click-outside close, focus trapped and restored on close (FE-11).
 *  Narrow by default (`sm`); a dialog that needs more passes a `size`. */
export function Modal({ label, onClose, size = 'sm', className, dismissable = true, presentation = 'dialog', sheetWidth, onOpenAutoFocus, returnFocus, overlayClassName, contentClassName, innerClassName, contentStyle, onEscapeKeyDown, children }: ModalProps) {
  const restoreFocus = useRef<HTMLElement | null>(typeof document === 'undefined' ? null : document.activeElement as HTMLElement);
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        label={label}
        sheet={presentation === 'sheet'}
        sheetWidth={sheetWidth}
        overlayClassName={overlayClassName}
        contentClassName={contentClassName}
        innerClassName={innerClassName}
        contentStyle={contentStyle}
        className={cn(MODAL_SIZES[size], className)}
        onEscapeKeyDown={(e) => { if (!dismissable) e.preventDefault(); else onEscapeKeyDown?.(e); }}
        onInteractOutside={(e) => !dismissable && e.preventDefault()}
        onOpenAutoFocus={onOpenAutoFocus}
        onCloseAutoFocus={(e) => {
          if (restoreFocus.current?.isConnected) {
            e.preventDefault();
            restoreFocus.current.focus();
            return;
          }
          const target = returnFocus?.();
          if (!target?.isConnected) return;
          e.preventDefault();
          target.focus();
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

/** The shared dialog head (the lab's .dialog-head): title, muted sub under
 *  it, and the X close button top right. A kicker renders only outside Atlas:
 *  under `label.style = plain` it stays hidden, so Atlas dialogs stop
 *  rebuilding it.
 *
 *  The head spans the Modal's inner padding (which is `p-7`): callers wrap
 *  the dialog in `-m-7` and give the body its own padding. */
export function ModalHead({ title, sub, kicker, onClose }: {
  title: React.ReactNode;
  sub?: React.ReactNode;
  kicker?: React.ReactNode;
  onClose: () => void;
}) {
  const { t } = useTranslation('common');
  const plain = useVariant('label.style') === 'plain';
  return (
    <div className="flex items-start justify-between gap-4 border-b border-hairline px-5 pb-2.5 pt-4">
      <div className="min-w-0">
        {kicker && !plain ? <div className="mb-1">{kicker}</div> : null}
        <h2 className="text-[16px] font-extrabold text-text">{title}</h2>
        {sub && <span className="mt-0.5 block text-[12.5px] text-text-muted">{sub}</span>}
      </div>
      <button
        type="button"
        onClick={onClose}
        aria-label={t('common:close')}
        className="grid size-8 flex-none place-items-center rounded-lg text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <X className="size-[15px]" aria-hidden />
      </button>
    </div>
  );
}

/** The shared dialog footer (the lab's .ap-dialog-foot): quiet Cancel on the
 *  left, the primary action on the right. Never full-width equal buttons. */
export function ModalFoot({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-end gap-2 border-t border-hairline px-4 py-3">
      {children}
    </div>
  );
}
