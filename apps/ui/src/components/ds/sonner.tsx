// shadcn-style Sonner wrapper, themed with the app's design tokens (no
// library default look, no Radix). Sonner's positioning/stack/animation CSS
// comes from the bundled `sonner/dist/styles.css` import below (a same-origin
// stylesheet, allowed by style-src 'self'); Sonner also injects the same CSS
// as a <style> tag at runtime, which the current style-src 'unsafe-inline'
// permits — no CSP change for scripts, no style-src change. `unstyled` drops
// Sonner's default colours so only our tokens show; the dots replace the
// library's default icons.
import 'sonner/dist/styles.css';
import { X } from 'lucide-react';
import { Toaster as SonnerToaster } from 'sonner';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';

const TOAST_BASE =
  'platypus-toast group pointer-events-auto relative flex w-[356px] max-w-[calc(100vw-2rem)] items-start gap-2.5 rounded-[3px] border bg-[#191410] px-3 py-2 font-mono text-[12px] leading-relaxed shadow-[0_10px_30px_rgba(0,0,0,.5)]';

function Dot({ className }: { className: string }) {
  return <span aria-hidden className={cn('mt-[5px] h-1.5 w-1.5 flex-none rounded-full', className)} />;
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
        success: <Dot className="bg-emerald-400" />,
        info: <Dot className="bg-sky-300" />,
        error: <Dot className="bg-red-400" />,
        loading: <Dot className="bg-amber-300" />,
        close: <X className="h-3 w-3" />,
      }}
      toastOptions={{
        unstyled: true,
        closeButtonAriaLabel: t('toast.close', 'Close notification'),
        classNames: {
          toast: TOAST_BASE,
          title: 'platypus-toast-title-wrap',
          description: 'platypus-toast-description',
          actionButton:
            'platypus-toast-action inline-flex h-6 flex-none items-center rounded-xl bg-amber-300 px-3 font-extrabold text-[#130f0a] transition hover:bg-amber-200',
          cancelButton:
            'platypus-toast-cancel inline-flex h-6 flex-none items-center rounded-xl bg-white/[0.06] px-3 font-bold text-white/80 hover:bg-white/[0.1]',
          closeButton:
            'platypus-toast-close absolute flex h-5 w-5 items-center justify-center rounded-full border border-white/10 bg-[#191410] p-0 text-white/60 transition hover:text-white/90',
          success: 'border-emerald-300/50 text-emerald-100',
          info: 'border-sky-300/50 text-sky-100',
          error: 'border-red-400/50 text-red-100',
          warning: 'border-amber-300/50 text-amber-100',
          default: 'border-white/10 text-[#ece6da]',
          loading: 'border-amber-300/50 text-amber-100',
          content: 'platypus-toast-content min-w-0 flex-1',
          icon: 'platypus-toast-icon flex flex-none items-center',
        },
      }}
    />
  );
}
