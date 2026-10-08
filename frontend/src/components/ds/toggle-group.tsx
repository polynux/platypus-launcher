import * as React from 'react';
import { ToggleGroup as ToggleGroupPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export function ToggleGroup({ className, ...props }: React.ComponentProps<typeof ToggleGroupPrimitive.Root>) {
  return <ToggleGroupPrimitive.Root data-slot="toggle-group" className={cn('flex flex-wrap gap-1.5', className)} {...props} />;
}

/** Segmented chip: inactive translucent, active solid amber. */
export function ToggleGroupItem({ className, ...props }: React.ComponentProps<typeof ToggleGroupPrimitive.Item>) {
  return (
    <ToggleGroupPrimitive.Item
      data-slot="toggle-group-item"
      className={cn(
        'rounded-lg bg-white/[0.06] px-3 py-1.5 font-mono text-[12px] text-white/60 transition hover:bg-white/[0.1] data-[state=on]:bg-amber-300 data-[state=on]:font-bold data-[state=on]:text-[#130f0a] disabled:opacity-50',
        className,
      )}
      {...props}
    />
  );
}
