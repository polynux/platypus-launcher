import React, { useEffect, useRef } from 'react';
import { cx } from './ui';

interface ModalProps {
  label: string;
  onClose: () => void;
  className?: string;
  children: React.ReactNode;
}

/** Centered modal: Esc + click-outside close, focus moves in on open and back on close. */
export function Modal({ label, onClose, className, children }: ModalProps) {
  const panel = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    panel.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        closeRef.current();
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
