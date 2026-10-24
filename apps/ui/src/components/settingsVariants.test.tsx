import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Modal } from './Modal';
import { useState } from 'react';

expect.extend(matchers);
afterEach(cleanup);

function Fixture({ presentation, navigation, chrome, onClose }: { presentation: string; navigation: string; chrome: string; onClose: () => void }) {
  const [tab, setTab] = useState(0);
  const body = <div>
    {navigation === 'tabs' ? <>
      <div role="tablist" aria-label="Settings sections" aria-orientation="vertical">
        {['Language', 'Theme'].map((name, index) => <button key={name} role="tab" id={`tab-${index}`} aria-controls={`panel-${index}`} aria-selected={tab === index} tabIndex={tab === index ? 0 : -1} onClick={() => setTab(index)} onKeyDown={(e) => {
          if (!['ArrowDown', 'ArrowUp', 'ArrowRight', 'ArrowLeft'].includes(e.key)) return;
          e.preventDefault(); const next = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? (index + 1) % 2 : (index + 1) % 2; setTab(next); document.getElementById(`tab-${next}`)?.focus();
        }}>{name}</button>)}
      </div>
      <section role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`}><h2>{tab === 0 ? 'Language' : 'Theme'}</h2><label htmlFor="value">Value</label><input id="value" /></section>
    </> : <section><h2>Settings</h2><label htmlFor="value">Value</label><input id="value" /></section>}
    <button onClick={onClose}>Close settings</button>
    <div aria-hidden="true" data-chrome={chrome} />
  </div>;
  return presentation === 'page' ? body : <Modal label="Settings" presentation={presentation === 'sheet' ? 'sheet' : 'dialog'} onClose={onClose}>{body}</Modal>;
}

describe('Settings catalog variants', () => {
  for (const presentation of ['page', 'dialog', 'sheet']) {
    for (const navigation of ['scroll', 'tabs']) {
      for (const chrome of ['hairline', 'framed', 'plain']) {
        it(`renders ${presentation} / ${navigation} / ${chrome} without axe violations`, async () => {
          const { container } = render(<Fixture presentation={presentation} navigation={navigation} chrome={chrome} onClose={() => {}} />);
          (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
        });
      }
    }
  }

  it('traps focus, closes with Escape and restores focus to the Settings entry', async () => {
    const user = userEvent.setup();
    const closed = vi.fn();
    function Host() {
      const [open, setOpen] = useState(false);
      return <><button onClick={() => setOpen(true)}>Settings entry</button>{open && <Modal label="Settings" onClose={() => { closed(); setOpen(false); }}><button>Inside</button><button>Second inside</button></Modal>}</>;
    }
    render(<Host />);
    const trigger = screen.getByRole('button', { name: 'Settings entry' });
    await user.click(trigger);
    expect(screen.getByRole('dialog', { name: 'Settings' })).toBeInTheDocument();
    await user.tab();
    expect(trigger).not.toHaveFocus();
    await user.keyboard('{Escape}');
    expect(closed).toHaveBeenCalledOnce();
    expect(trigger).toHaveFocus();
  });

  it('moves focus between tabs with arrow keys', async () => {
    const user = userEvent.setup();
    render(<Fixture presentation="page" navigation="tabs" chrome="hairline" onClose={() => {}} />);
    const language = screen.getByRole('tab', { name: 'Language' });
    language.focus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('tab', { name: 'Theme' })).toHaveFocus();
    expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', 'tab-1');
  });
});
