import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';

const state = vi.hoisted(() => ({ variants: {
  'view-header.actions': 'right',
  'view-header.subtitle': 'shown',
  'view-header.size': 'regular',
  'empty-state.chrome': 'dashed',
  'empty-state.icon': 'off',
  'list-row.media': 'on',
  'list-row.density': 'comfortable',
  'section.chrome': 'hairline',
} as Record<string, string> }));
vi.mock('../lib/theme', () => ({ useVariant: (axis: string) => state.variants[axis] }));

import { EmptyState, ListRow, Section, ViewHeader } from './ui';

expect.extend(matchers);
afterEach(() => { cleanup(); });

function HeaderFixture() {
  return (
    <ViewHeader
      title="Runners"
      subtitle={<p className="mt-0.5 text-[12.5px] text-white/70">Every runner</p>}
      actions={<button>Rescan</button>}
    />
  );
}

describe('ViewHeader catalog variants', () => {
  for (const actions of ['right', 'below']) {
    for (const subtitle of ['shown', 'hidden']) {
      for (const size of ['regular', 'compact']) {
        it(`renders header ${actions} / ${subtitle} / ${size} without axe violations`, async () => {
          state.variants = { ...state.variants, 'view-header.actions': actions, 'view-header.subtitle': subtitle, 'view-header.size': size };
          const { container } = render(<HeaderFixture />);
          expect(screen.getByRole('heading', { name: 'Runners' })).toBeInTheDocument();
          if (subtitle === 'shown') expect(screen.getByText('Every runner')).toBeInTheDocument();
          else expect(screen.queryByText('Every runner')).not.toBeInTheDocument();
          (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
        });
      }
    }
  }
});

describe('EmptyState catalog variants', () => {
  for (const chrome of ['dashed', 'plain']) {
    for (const icon of ['on', 'off']) {
      it(`renders empty state ${chrome} / icon ${icon} without axe violations`, async () => {
        state.variants = { ...state.variants, 'empty-state.chrome': chrome, 'empty-state.icon': icon };
        const { container } = render(<EmptyState label="Nothing here">Nothing matches.</EmptyState>);
        expect(screen.getByText('Nothing here')).toBeInTheDocument();
        (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
      });
    }
  }
});

describe('ListRow catalog variants', () => {
  for (const media of ['on', 'off']) {
    for (const density of ['comfortable', 'compact']) {
      it(`renders fix row media ${media} / ${density} without axe violations`, async () => {
        state.variants = { ...state.variants, 'list-row.media': media, 'list-row.density': density };
        const { container } = render(
          <ListRow kind="fix" as="button" buttonProps={{ onClick: () => {} }} media={<span data-testid="row-cover">cover</span>}>
            <div><span>Game title</span></div>
          </ListRow>,
        );
        expect(screen.getByRole('button', { name: /Game title/ })).toBeInTheDocument();
        if (media === 'on') expect(screen.getByTestId('row-cover')).toBeInTheDocument();
        else expect(screen.queryByTestId('row-cover')).not.toBeInTheDocument();
        (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
      });
    }
  }

  it('renders runner and job rows without axe violations', async () => {
    state.variants = { ...state.variants, 'list-row.media': 'on', 'list-row.density': 'comfortable' };
    const { container } = render(<>
      <ListRow kind="runner"><div><span>Runner name</span></div><span>found</span></ListRow>
      <ListRow kind="job"><div><span>Job title</span></div></ListRow>
    </>);
    expect(screen.getByText('Runner name')).toBeInTheDocument();
    expect(screen.getByText('Job title')).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});

describe('Section catalog variants', () => {
  for (const chrome of ['hairline', 'framed', 'plain']) {
    it(`renders section ${chrome} without axe violations`, async () => {
      state.variants = { ...state.variants, 'section.chrome': chrome };
      const { container } = render(<Section><p>Group body</p></Section>);
      expect(screen.getByText('Group body')).toBeInTheDocument();
      (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
    });
  }
});
