import React, { useEffect, useRef } from 'react';
import { cx } from './ui';

interface ModalProps {
  label: string;
  onClose: () => void;
  className?: string;
  children: React.ReactNode;
}

/** Centered modal: Esc + click-outside close, focus moves in on open and back on close. Tab cycles inside (FE-11). */
export function Modal({ label, onClose, className, children }: ModalProps) {
  const panel = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    panel.current?.focus();
    const focusables = () => {
      const root = panel.current;
      if (!root) return [] as HTMLElement[];
      const nodes = root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      return [...nodes].filter((el) => !el.hasAttribute('disabled'));
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        closeRef.current();
        return;
      }
      if (e.key !== 'Tab') return;
      const list = focusables();
      if (list.length === 0) {
        e.preventDefault();
        return;
      }
      const first = list[0];
      const last = list[list.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (e.shiftKey && (active === first || !panel.current?.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
      prev?.focus?.();
    };
  }, []);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/65 p-6" onMouseDown={onClose}>
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        onMouseDown={(e) => e.stopPropagation()}
        className={cx(
          'max-h-[86vh] w-full animate-modal-in overflow-y-auto rounded-3xl bg-[#191410] p-7 outline-none ring-1 ring-white/10',
          className
        )}
        style={{ boxShadow: '0 40px 100px rgba(0,0,0,.6)' }}
      >
        {children}
      </div>
    </div>
  );
}
