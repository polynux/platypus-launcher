import * as React from 'react';
import { Label as LabelPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

/** Mono uppercase micro-label for a form field. */
export function Label({ className, ...props }: React.ComponentProps<typeof LabelPrimitive.Root>) {
  return (
    <LabelPrimitive.Root
      data-slot="label"
      className={cn('font-mono text-[11px] uppercase tracking-[0.16em] text-white/40 select-none', className)}
      {...props}
    />
  );
}
