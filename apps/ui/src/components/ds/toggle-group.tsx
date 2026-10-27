import * as React from 'react';
import { ToggleGroup as ToggleGroupPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export function ToggleGroup({ className, ...props }: React.ComponentProps<typeof ToggleGroupPrimitive.Root>) {
  return <ToggleGroupPrimitive.Root data-slot="toggle-group" className={cn('flex flex-wrap gap-1.5', className)} {...props} />;
}

/** Segmented chip: inactive translucent, active solid accent. */
export function ToggleGroupItem({ className, ...props }: React.ComponentProps<typeof ToggleGroupPrimitive.Item>) {
  return (
    <ToggleGroupPrimitive.Item
      data-slot="toggle-group-item"
      className={cn(
        'inline-flex items-center justify-center rounded-lg bg-surface-strong px-3 py-1.5 font-mono text-[12px] text-text-muted transition hover:bg-hairline data-[state=on]:bg-accent-strong data-[state=on]:font-bold data-[state=on]:text-on-accent plain:font-sans plain:text-[13px] plain:font-semibold plain:data-[state=on]:bg-[color-mix(in_srgb,var(--color-accent)_20%,var(--color-surface-strong))] plain:data-[state=on]:text-text plain:data-[state=on]:ring-1 plain:data-[state=on]:ring-inset plain:data-[state=on]:ring-accent/40 disabled:opacity-50',
        className,
      )}
      {...props}
    />
  );
}
