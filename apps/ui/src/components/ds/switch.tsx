import * as React from 'react';
import { Switch as SwitchPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

/** Boolean control. Amber track when on; track h-4 w-7, thumb h-3 w-3 (DESIGN toggle pill). */
export function Switch({ className, ...props }: React.ComponentProps<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'relative inline-flex h-4 w-7 flex-none items-center rounded-full bg-hairline-strong transition-colors data-[state=checked]:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50',
        'plain:h-5 plain:w-[34px] plain:bg-surface-strong plain:ring-1 plain:ring-inset plain:ring-hairline-strong plain:data-[state=checked]:bg-accent plain:data-[state=checked]:ring-0',
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb className="pointer-events-none block h-3 w-3 translate-x-0.5 rounded-full bg-raised transition-transform data-[state=checked]:translate-x-[14px] plain:size-4 plain:bg-text-muted plain:data-[state=checked]:translate-x-[16px] plain:data-[state=checked]:bg-on-accent" />
    </SwitchPrimitive.Root>
  );
}
