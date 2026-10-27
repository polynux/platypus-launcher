import * as React from 'react';
import { Minus, Plus } from 'lucide-react';
import { cn } from '@/lib/utils';
import { fieldClass } from './input';

export interface NumberFieldProps {
  /** undefined = unset (the input is empty). */
  value: number | undefined;
  /** Called with a clamped number, or undefined when the field is emptied. */
  onChange: (v: number | undefined) => void;
  min?: number;
  max?: number;
  step?: number;
  /** Hide the -/+ buttons. */
  hideSteppers?: boolean;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  id?: string;
  'aria-label'?: string;
}

const clamp = (n: number, min?: number, max?: number) => Math.min(max ?? Infinity, Math.max(min ?? -Infinity, n));
const decimals = (n: number) => (String(n).split('.')[1] ?? '').length;

/** Numeric setting: text input (no native spinner) with optional -/+ buttons. Clamps on blur; empty means unset. */
export function NumberField({ value, onChange, min, max, step = 1, hideSteppers, placeholder, disabled, className, id, 'aria-label': ariaLabel }: NumberFieldProps) {
  const [draft, setDraft] = React.useState<string | null>(null);
  const text = draft ?? (value === undefined ? '' : String(value));

  const commit = (raw: string) => {
    setDraft(null);
    const t = raw.trim();
    if (t === '') return onChange(undefined);
    const n = Number(t);
    if (!Number.isFinite(n)) return; // revert to the last good value
    const c = clamp(n, min, max);
    if (c !== value) onChange(c);
  };

  const bump = (dir: 1 | -1) => {
    const base = value ?? min ?? 0;
    const prec = Math.max(decimals(step), decimals(base));
    const next = value === undefined && dir === 1 ? clamp(base, min, max) : clamp(Number((base + dir * step).toFixed(prec)), min, max);
    setDraft(null);
    onChange(next);
  };

  const atMin = value !== undefined && min !== undefined && value <= min;
  const atMax = value !== undefined && max !== undefined && value >= max;
  const stepBtn = 'grid size-7 flex-none place-items-center rounded-lg bg-surface-strong text-text-muted transition hover:bg-white/[0.1] hover:text-text disabled:pointer-events-none disabled:opacity-40';

  return (
    <div data-slot="number-field" className={cn('flex items-center gap-1.5', className)}>
      <input
        id={id}
        aria-label={ariaLabel}
        type="text"
        inputMode={step % 1 === 0 && (min ?? 0) >= 0 ? 'numeric' : 'decimal'}
        autoComplete="off"
        role="spinbutton"
        aria-valuemin={min}
        aria-valuemax={max}
        aria-valuenow={value}
        value={text}
        placeholder={placeholder}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={(e) => commit(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') commit(e.currentTarget.value);
          else if (e.key === 'ArrowUp') (e.preventDefault(), bump(1));
          else if (e.key === 'ArrowDown') (e.preventDefault(), bump(-1));
        }}
        onWheel={(e) => e.currentTarget.blur()}
        className={cn(fieldClass, 'h-9 min-w-0 tabular-nums')}
      />
      {!hideSteppers && (
        <>
          <button type="button" aria-label="Decrease" disabled={disabled || atMin} onClick={() => bump(-1)} className={stepBtn}>
            <Minus className="size-3.5" aria-hidden />
          </button>
          <button type="button" aria-label="Increase" disabled={disabled || atMax} onClick={() => bump(1)} className={stepBtn}>
            <Plus className="size-3.5" aria-hidden />
          </button>
        </>
      )}
    </div>
  );
}
