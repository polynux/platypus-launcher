// shadcn-style Sonner wrapper, themed with the app's design tokens (no
// library default look, no Radix). Sonner's positioning/stack/animation CSS
// comes from the bundled `sonner/dist/styles.css` import below (a same-origin
// stylesheet, allowed by style-src 'self'); Sonner also injects the same CSS
// as a <style> tag at runtime, which the current style-src 'unsafe-inline'
// permits — no CSP change for scripts, no style-src change. `unstyled` drops
// Sonner's default colours so only our tokens show; the type icons replace the
// library's default icons.
import 'sonner/dist/styles.css';
import { CheckCircle2, Info, Loader2, X, XCircle } from 'lucide-react';
import { Toaster as SonnerToaster } from 'sonner';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useVariant } from '@/lib/theme';

// The launcher's floating-panel family (dialog surface, JobProgress): raised
// ground, ring-1 instead of a border, the rounded-2xl card radius.
const TOAST_BASE =
  'platypus-toast group pointer-events-auto relative grid w-[360px] max-w-[calc(100vw-2rem)] grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3.5 gap-y-2.5 rounded-2xl bg-raised p-3.5 font-sans text-[14.5px] leading-snug text-text shadow-[0_16px_40px_rgba(0,0,0,.5)] ring-1 ring-hairline';

/** A type glyph in a small tinted square, the same tints as the app's badges. */
function TypeIcon({ Icon, tone, spin }: { Icon: typeof Info; tone: string; spin?: boolean }) {
  return (
    <span aria-hidden className={cn('flex size-8 flex-none items-center justify-center rounded-lg', tone)}>
      <Icon className={cn('size-[18px]', spin && 'animate-spin')} />
    </span>
  );
}

/** The app's toast layer: top right, below the window controls, at most 3
 *  visible with the rest collapsed until hover (Sonner's visibleToasts /
 *  expand). Hover or focus pauses the timer (Sonner pauses while expanded or
 *  interacting); the close button and swipe dismiss. Animations are Sonner's
 *  built-in; prefers-reduced-motion and the platypus-paused class (a game
 *  running, #49) disable them in style.css, while new toasts still appear. */
export function Toaster({ storeOpen = false }: { storeOpen?: boolean }) {
  const { t } = useTranslation('common');
  const plain = useVariant('label.style') === 'plain';
  return (
    <SonnerToaster
      position={plain && !storeOpen ? 'bottom-center' : 'top-right'}
      visibleToasts={3}
      expand={false}
      closeButton
      theme="dark"
      richColors={false}
      gap={8}
      offset={plain && !storeOpen ? { bottom: 40 } : { top: 64, right: 16 }}
      mobileOffset={plain && !storeOpen ? { bottom: 40 } : { top: 64, right: 16 }}
      containerAriaLabel={t('toast.region', 'Notifications')}
      className={cn('platypus-toaster')}
      icons={{
        success: plain ? undefined : <TypeIcon Icon={CheckCircle2} tone="bg-ok/15 text-ok" />,
        info: plain ? undefined : <TypeIcon Icon={Info} tone="bg-info/10 text-info" />,
        error: plain ? <XCircle className="size-3.5 text-danger" /> : <TypeIcon Icon={XCircle} tone="bg-danger/15 text-danger" />,
        loading: plain ? <Loader2 className="size-3.5 animate-spin" /> : <TypeIcon Icon={Loader2} tone="bg-accent-strong/15 text-accent-strong" spin />,
        close: <X className="size-3.5" />,
      }}
      toastOptions={{
        unstyled: true,
        closeButtonAriaLabel: t('toast.close', 'Close notification'),
        classNames: {
          toast: plain ? 'platypus-toast group pointer-events-auto relative flex w-auto max-w-[min(560px,calc(100vw-2rem))] items-center gap-2 whitespace-nowrap rounded-[6px] bg-surface-strong px-3.5 py-2 font-sans text-[13.5px] text-text shadow-[0_10px_30px_rgba(0,0,0,.45)] ring-1 ring-hairline-strong' : TOAST_BASE,
          title: plain ? 'platypus-toast-title-wrap [&_[data-title]]:truncate' : 'platypus-toast-title-wrap',
          description: 'platypus-toast-description mt-0.5 text-[13px] leading-snug',
          actionButton:
            'platypus-toast-action col-start-2 inline-flex h-8 justify-self-start items-center rounded-xl bg-accent-strong px-3 text-[12px] font-extrabold text-on-accent transition hover:brightness-110',
          cancelButton:
            'platypus-toast-cancel col-start-2 inline-flex h-8 justify-self-start items-center rounded-xl bg-surface-strong px-3 text-[12px] font-bold text-text-body transition hover:bg-hairline',
          closeButton:
            'platypus-toast-close absolute flex size-7 items-center justify-center rounded-xl p-0 text-text-muted transition hover:bg-surface-strong hover:text-text',
          content: 'platypus-toast-content flex min-w-0 flex-1 flex-col pr-5',
          icon: plain ? 'hidden' : 'platypus-toast-icon flex flex-none items-center',
        },
      }}
    />
  );
}
