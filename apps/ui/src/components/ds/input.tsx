import * as React from 'react';
import { cn } from '@/lib/utils';

/** Shared look for every text-like control (Input, Textarea, Select trigger, NumberField). */
export const fieldClass =
  'w-full rounded-xl bg-surface-strong px-3 font-mono text-[12px] text-text placeholder:text-muted outline-none ring-focus/50 transition focus:ring-1 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-1 aria-invalid:ring-danger/60';

export function Input({ className, type = 'text', ...props }: React.ComponentProps<'input'>) {
  return <input data-slot="input" type={type} className={cn(fieldClass, 'h-9', className)} {...props} />;
}
