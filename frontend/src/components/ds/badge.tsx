import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

export const badgeVariants = cva(
  'inline-flex h-6 items-center rounded-md px-2 pt-px font-mono text-[11px] font-bold uppercase leading-none tracking-[0.14em] whitespace-nowrap',
  {
    variants: {
      variant: {
        platinum: 'bg-amber-300 text-[#130f0a]',
        gold: 'bg-amber-300/15 text-amber-300 ring-1 ring-amber-300/40',
        silver: 'bg-white/[0.06] text-white/60',
        bronze: 'bg-orange-400/10 text-orange-300',
        borked: 'bg-red-500/15 text-red-300',
        neutral: 'bg-white/[0.06] text-white/60',
        accent: 'bg-amber-300/15 text-amber-300',
        ok: 'bg-emerald-400/10 text-emerald-300',
        info: 'bg-sky-300/10 text-sky-300',
      },
    },
    defaultVariants: { variant: 'neutral' },
  },
);

export function Badge({ className, variant, ...props }: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants>) {
  return <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />;
}
