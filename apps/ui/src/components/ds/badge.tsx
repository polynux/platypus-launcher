import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

export const badgeVariants = cva(
  'inline-flex h-6 items-center rounded-md px-2 pt-px font-mono text-micro font-bold uppercase leading-none tracking-[0.14em] whitespace-nowrap',
  {
    variants: {
      variant: {
        platinum: 'bg-accent-strong text-on-accent',
        gold: 'bg-accent-strong/15 text-accent-strong ring-1 ring-accent-strong/40',
        silver: 'bg-surface-strong text-text-muted',
        bronze: 'bg-attention/10 text-orange-300',
        borked: 'bg-danger/15 text-red-300',
        neutral: 'bg-surface-strong text-text-muted',
        accent: 'bg-accent-strong/15 text-accent-strong',
        ok: 'bg-emerald-400/10 text-emerald-300',
        info: 'bg-info/10 text-info',
      },
    },
    defaultVariants: { variant: 'neutral' },
  },
);

export function Badge({ className, variant, ...props }: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants>) {
  return <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />;
}
