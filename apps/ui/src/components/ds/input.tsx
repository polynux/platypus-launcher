import * as React from 'react';
import { cn } from '@/lib/utils';

/** Shared look for every text-like control (Input, Textarea, Select trigger, NumberField). */
export const fieldClass =
  'w-full rounded-xl bg-white/[0.06] px-3 font-mono text-[12px] text-white placeholder-white/55 outline-none ring-amber-300/50 transition focus:ring-1 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-1 aria-invalid:ring-red-400/60';

export function Input({ className, type = 'text', ...props }: React.ComponentProps<'input'>) {
  return <input data-slot="input" type={type} className={cn(fieldClass, 'h-9', className)} {...props} />;
}
