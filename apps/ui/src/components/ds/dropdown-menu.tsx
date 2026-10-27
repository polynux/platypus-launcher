import * as React from 'react';
import { DropdownMenu as DropdownMenuPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuGroup = DropdownMenuPrimitive.Group;

/** Floating panel, styled like the select's content: raised surface, hairline ring. */
export function DropdownMenuContent({ className, sideOffset = 4, children, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        className={cn(
          'z-50 min-w-44 overflow-hidden rounded-xl bg-raised p-1.5 text-text ring-1 ring-hairline animate-modal-in motion-reduce:animate-none plain:bg-popover plain:ring-hairline-strong plain:rounded-[10px] plain:min-w-[210px]',
          className,
        )}
        style={{ boxShadow: '0 20px 50px rgba(0,0,0,.55)' }}
        {...props}
      >
        {children}
      </DropdownMenuPrimitive.Content>
    </DropdownMenuPrimitive.Portal>
  );
}

export function DropdownMenuItem({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Item>) {
  return (
    <DropdownMenuPrimitive.Item
      data-slot="dropdown-menu-item"
      className={cn(
        'relative flex w-full cursor-pointer items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12.5px] font-semibold text-text-body outline-none select-none data-[highlighted]:bg-surface-strong data-[highlighted]:text-text data-[disabled]:pointer-events-none data-[disabled]:opacity-40 plain:h-8 plain:text-[13.5px] plain:font-normal plain:text-text plain:gap-[9px]',
        className,
      )}
      {...props}
    />
  );
}

export function DropdownMenuSeparator({ className, ...props }: React.ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
  return <DropdownMenuPrimitive.Separator data-slot="dropdown-menu-separator" className={cn('-mx-1 my-1 h-px bg-hairline', className)} {...props} />;
}
