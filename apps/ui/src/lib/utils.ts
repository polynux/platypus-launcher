import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// text-micro and text-hint are the type-floor sizes from style.css (@theme). Without this, twMerge reads
// them as text colours and drops them when a text colour follows.
const twMerge = extendTailwindMerge({ extend: { classGroups: { 'font-size': [{ text: ['micro', 'hint'] }] } } });

/** Merge class names, letting later Tailwind utilities win over earlier ones. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
