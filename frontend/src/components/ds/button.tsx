import * as React from 'react';
import { Slot } from 'radix-ui';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

/** Per-variant look only (no layout/size); ui.tsx derives the legacy btnPrimary/btnNeutral strings from these. */
export const buttonVariantClass = {
  primary: 'rounded-xl bg-amber-300 font-extrabold text-[#130f0a] transition hover:bg-amber-200 disabled:cursor-wait disabled:opacity-60',
  neutral: 'rounded-xl bg-white/[0.06] font-bold text-white/80 transition hover:bg-white/[0.1]',
  ghost: 'rounded-xl font-bold text-white/60 hover:bg-white/[0.06] hover:text-white/90',
  link: 'rounded font-mono uppercase tracking-widest text-white/40 hover:text-white/80',
  danger: 'rounded-xl bg-red-500/15 font-bold text-red-300 hover:bg-red-500/25',
} as const;

export const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap transition disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: buttonVariantClass,
      size: {
        sm: 'h-8 px-3 text-[12px]',
        md: 'h-9 px-4 text-[13px]',
        lg: 'h-11 px-6 text-[14px]',
        icon: 'size-9 text-[13px]',
        /** No size classes: the caller sizes it (used by the legacy btn* strings). */
        none: '',
      },
    },
    compoundVariants: [{ variant: 'link', size: ['sm', 'md', 'lg'], class: 'h-auto px-0 text-[11.5px]' }],
    defaultVariants: { variant: 'neutral', size: 'md' },
  },
);

export interface ButtonProps extends React.ComponentProps<'button'>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export function Button({ className, variant, size, asChild = false, type, ...props }: ButtonProps) {
  const Comp = asChild ? Slot.Root : 'button';
  return <Comp data-slot="button" type={asChild ? undefined : (type ?? 'button')} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
