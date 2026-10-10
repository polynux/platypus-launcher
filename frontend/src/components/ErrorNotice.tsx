import { RotateCw } from 'lucide-react';
import { Button } from '@/components/ds/button';
import { hasDetails, plainError } from '../lib/plainError';
import { cx } from './ui';

export interface ErrorNoticeProps {
  /** The raw message the plugin or core returned. */
  message: string;
  /** The sentence to show when the raw message is developer text. */
  fallback?: string;
  /** Re-runs the action; the button is shown only where a retry makes sense. */
  onRetry?: () => void;
  className?: string;
}

const DEFAULT_FALLBACK = 'Platypus couldn\'t finish that. Try again.';

/** An error a person can act on: a plain sentence, a Retry where it helps, and
 *  the raw message only behind Details. */
export function ErrorNotice({ message, fallback = DEFAULT_FALLBACK, onRetry, className }: ErrorNoticeProps) {
  const summary = plainError(message, fallback);
  return (
    <div role="alert" className={cx('text-[12.5px] leading-relaxed text-red-200', className)}>
      <p>{summary}</p>
      {(onRetry || hasDetails(message, fallback)) && (
        <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1">
          {onRetry && (
            <Button variant="link" size="none" onClick={onRetry} className="font-mono text-[12px] text-red-100 underline underline-offset-2">
              <RotateCw className="h-3 w-3" /> Try again
            </Button>
          )}
          {hasDetails(message, fallback) && <RawDetails text={message} />}
        </div>
      )}
    </div>
  );
}

/** The raw text a plain sentence stands for, behind a Details disclosure. */
export function RawDetails({ text }: { text: string }) {
  return (
    <details className="font-mono text-[12px] text-white/55">
      <summary className="cursor-pointer select-none hover:text-white/80">Details</summary>
      <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-black/30 p-2 text-[11.5px] text-white/65">{text}</pre>
    </details>
  );
}
