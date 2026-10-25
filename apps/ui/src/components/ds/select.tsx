import * as React from 'react';
import { Select as SelectPrimitive } from 'radix-ui';
import { Check, ChevronDown } from 'lucide-react';
import { cn } from '@/lib/utils';
import { fieldClass } from './input';

export const Select = SelectPrimitive.Root;
export const SelectGroup = SelectPrimitive.Group;
export const SelectValue = SelectPrimitive.Value;

export function SelectTrigger({ className, children, ...props }: React.ComponentProps<typeof SelectPrimitive.Trigger>) {
  return (
    <SelectPrimitive.Trigger
      data-slot="select-trigger"
      className={cn(fieldClass, 'flex h-9 items-center justify-between gap-2 text-left data-[placeholder]:text-text-muted [&>span]:truncate', className)}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon asChild>
        <ChevronDown className="size-4 flex-none text-text-muted" aria-hidden />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
}

export function SelectContent({ className, children, position = 'popper', ...props }: React.ComponentProps<typeof SelectPrimitive.Content>) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        data-slot="select-content"
        position={position}
        sideOffset={6}
        className={cn(
          'relative z-[70] max-h-[min(20rem,var(--radix-select-content-available-height))] min-w-[var(--radix-select-trigger-width)] origin-[var(--radix-select-content-transform-origin)] overflow-hidden rounded-xl bg-popover text-text ring-1 ring-hairline-strong animate-modal-in',
          className,
        )}
        style={{ boxShadow: '0 20px 50px rgba(0,0,0,.55)' }}
        {...props}
      >
        <SelectPrimitive.Viewport className="max-h-[inherit] overflow-y-auto p-1 [scrollbar-width:thin]">{children}</SelectPrimitive.Viewport>
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  );
}

export function SelectLabel({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.Label>) {
  return <SelectPrimitive.Label data-slot="select-label" className={cn('px-2.5 pb-1 pt-2 font-mono text-micro uppercase tracking-[0.16em] text-text-muted plain:font-sans plain:normal-case plain:tracking-normal plain:text-[12px] plain:font-bold plain:text-text-faint', className)} {...props} />;
}

export function SelectItem({ className, children, ...props }: React.ComponentProps<typeof SelectPrimitive.Item>) {
  return (
    <SelectPrimitive.Item
      data-slot="select-item"
      className={cn(
        'relative flex w-full items-center gap-2 rounded-lg py-1.5 pl-2.5 pr-8 font-mono text-[12px] text-text-body outline-none select-none data-[highlighted]:bg-surface-strong data-[highlighted]:text-text data-[state=checked]:text-accent-strong data-[disabled]:pointer-events-none data-[disabled]:opacity-40 plain:font-sans plain:text-[13px]',
        className,
      )}
      {...props}
    >
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
      <SelectPrimitive.ItemIndicator className="absolute right-2.5 flex items-center">
        <Check className="size-3.5 text-accent-strong" aria-hidden />
      </SelectPrimitive.ItemIndicator>
    </SelectPrimitive.Item>
  );
}

export function SelectSeparator({ className, ...props }: React.ComponentProps<typeof SelectPrimitive.Separator>) {
  return <SelectPrimitive.Separator data-slot="select-separator" className={cn('-mx-1 my-1 h-px bg-hairline', className)} {...props} />;
}
