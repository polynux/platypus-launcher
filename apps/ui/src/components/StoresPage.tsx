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
  return (
    <div className="flex h-full min-h-[480px] flex-col">
      <div className="flex flex-none flex-wrap items-center gap-1.5 px-5 pb-1 pt-5">
        <div role="tablist" aria-label={t('nav:store')} className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
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
                  'relative flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  on ? 'text-text' : 'text-text-muted hover:bg-surface hover:text-text',
                )}
              >
                <PluginLogo src={p.icon_url} name={p.name} />
                <span>{p.name}</span>
                {on && <span aria-hidden className="absolute inset-x-2.5 -bottom-[5px] h-0.5 rounded-full bg-accent" />}
              </button>
            );
          })}
        </div>
        {canRedeem && (
          <Button variant="neutral" size="sm" onClick={onRedeem}>
            <Ticket className="h-3.5 w-3.5" /> {t('nav:sourcebar.redeem')}
          </Button>
        )}
      </div>
      <div className="min-h-0 flex-1">
        <StoreView
          key={selected.id}
          plugin={selected}
          initialUrl={initialUrl}
          onClose={onClose}
          onNotify={onNotify}
        />
      </div>
    </div>
  );
}
