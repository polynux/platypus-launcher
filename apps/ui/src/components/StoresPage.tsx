// i18n: converted
// The /22 stores page (issue #138, Atlas only): one tab per storefront
// plugin plus the redeem entry, with the embedded StoreView for the selected
// store below. Tab switches navigate (the router URL stays the store), so the
// browser back/forward and the native view follow the selection. Other themes
// keep the bare StoreView.
import React from 'react';
import { useTranslation } from 'react-i18next';
import { Ticket } from 'lucide-react';
import type { StorePlugin } from '../types';
import { Button } from './ds/button';
import { PluginLogo } from './PluginLogo';
import { StoreView } from './StoreView';
import { cx } from './ui';

interface StoresPageProps {
  plugins: StorePlugin[];
  selected: StorePlugin;
  onSelect: (id: string) => void;
  canRedeem: boolean;
  onRedeem: () => void;
  initialUrl?: string;
  onClose: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

export function StoresPage({ plugins, selected, onSelect, canRedeem, onRedeem, initialUrl, onClose, onNotify }: StoresPageProps) {
  const { t } = useTranslation(['store', 'nav']);
  // The lab's single shop bar (StorefrontPage .ap-shop-bar): the store switch
  // as segmented buttons, then the browser row, then Redeem. No logo tabs row.
  const seg = (
    <div role="tablist" aria-label={t('store:shop.storeLabel')} className="inline-flex min-w-0 flex-none gap-0.5 overflow-x-auto rounded-[9px] bg-surface p-[3px]">
      {plugins.map((p) => {
        const on = p.id === selected.id;
        return (
          <button
            key={p.id}
            type="button"
            role="tab"
            aria-selected={on}
            onClick={() => onSelect(p.id)}
            className={cx(
              'inline-flex h-7 flex-none items-center gap-1.5 rounded-[7px] px-[11px] text-[13.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              on
                ? 'bg-[color-mix(in_srgb,var(--color-accent)_20%,var(--color-surface-strong))] text-text ring-1 ring-inset ring-accent/40'
                : 'text-text-muted hover:text-text',
            )}
          >
            <span className={cx('[&>svg]:h-[13px] [&>svg]:w-[13px]', on && 'text-accent')}>
              <PluginLogo src={p.icon_url} name={p.name} size="xs" />
            </span>
            <span className="whitespace-nowrap">{p.name}</span>
          </button>
        );
      })}
    </div>
  );
  return (
    <div className="flex h-full min-h-[480px] flex-col">
      <div className="min-h-0 flex-1">
        <StoreView
          key={selected.id}
          plugin={selected}
          initialUrl={initialUrl}
          onClose={onClose}
          onNotify={onNotify}
          shopBar
          toolbarLead={seg}
          toolbarTrail={canRedeem ? (
              <Button variant="neutral" size="md" onClick={onRedeem} className="flex-none font-medium">
              <Ticket className="h-3.5 w-3.5" /> {t('store:shop.redeem')}
            </Button>
          ) : undefined}
        />
      </div>
    </div>
  );
}
