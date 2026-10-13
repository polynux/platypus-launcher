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

// The launcher's floating-panel family (dialog surface, JobProgress): raised
// ground, ring-1 instead of a border, the rounded-2xl card radius.
const TOAST_BASE =
  'platypus-toast group pointer-events-auto relative grid w-[360px] max-w-[calc(100vw-2rem)] grid-cols-[auto_minmax(0,1fr)] items-start gap-x-3 gap-y-2.5 rounded-2xl bg-[#191410] p-3.5 font-sans text-[13px] text-[#ece6da] shadow-[0_16px_40px_rgba(0,0,0,.5)] ring-1 ring-white/10';

/** A type glyph in a small tinted square, the same tints as the app's badges. */
function TypeIcon({ Icon, tone, spin }: { Icon: typeof Info; tone: string; spin?: boolean }) {
  return (
    <span aria-hidden className={cn('flex size-7 flex-none items-center justify-center rounded-lg', tone)}>
      <Icon className={cn('size-4', spin && 'animate-spin')} />
    </span>
  );
}

/** The app's toast layer: top right, below the window controls, at most 3
 *  visible with the rest collapsed until hover (Sonner's visibleToasts /
 *  expand). Hover or focus pauses the timer (Sonner pauses while expanded or
 *  interacting); the close button and swipe dismiss. Animations are Sonner's
 *  built-in; prefers-reduced-motion and the platypus-paused class (a game
 *  running, #49) disable them in style.css, while new toasts still appear. */
export function Toaster() {
  const { t } = useTranslation('common');
  return (
    <SonnerToaster
      position="top-right"
      visibleToasts={3}
      expand={false}
      closeButton
      theme="dark"
      richColors={false}
      gap={8}
      offset={{ top: 64, right: 16 }}
      mobileOffset={{ top: 64, right: 16 }}
      containerAriaLabel={t('toast.region', 'Notifications')}
      className={cn('platypus-toaster')}
      icons={{
        success: <TypeIcon Icon={CheckCircle2} tone="bg-emerald-400/15 text-emerald-300" />,
        info: <TypeIcon Icon={Info} tone="bg-sky-300/10 text-sky-300" />,
        error: <TypeIcon Icon={XCircle} tone="bg-red-500/15 text-red-300" />,
        loading: <TypeIcon Icon={Loader2} tone="bg-amber-300/15 text-amber-300" spin />,
        close: <X className="size-3.5" />,
      }}
      toastOptions={{
        unstyled: true,
        closeButtonAriaLabel: t('toast.close', 'Close notification'),
        classNames: {
          toast: TOAST_BASE,
          title: 'platypus-toast-title-wrap',
          description: 'platypus-toast-description mt-0.5 text-[12.5px] leading-snug',
          actionButton:
            'platypus-toast-action col-start-2 inline-flex h-8 justify-self-start items-center rounded-xl bg-amber-300 px-3 text-[12px] font-extrabold text-[#130f0a] transition hover:bg-amber-200',
          cancelButton:
            'platypus-toast-cancel col-start-2 inline-flex h-8 justify-self-start items-center rounded-xl bg-white/[0.06] px-3 text-[12px] font-bold text-white/80 transition hover:bg-white/[0.1]',
          closeButton:
            'platypus-toast-close absolute flex size-7 items-center justify-center rounded-xl p-0 text-white/60 transition hover:bg-white/[0.06] hover:text-white/90',
          content: 'platypus-toast-content flex min-w-0 flex-1 flex-col pr-5',
          icon: 'platypus-toast-icon flex flex-none items-center',
        },
      }}
    />
  );
}
