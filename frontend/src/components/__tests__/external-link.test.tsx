import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { MouseEvent } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { ExternalLink } from '../ExternalLink';

vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

describe('ExternalLink', () => {
  beforeEach(() => {
    vi.mocked(BrowserOpenURL).mockClear();
  });

  it('a click opens the page in the browser, not in the webview', () => {
    render(<ExternalLink href="https://www.protondb.com/app/620">protondb ↗</ExternalLink>);
    const link = screen.getByRole('link', { name: 'protondb ↗' });
    expect(link).toHaveAttribute('href', 'https://www.protondb.com/app/620');

    const notPrevented = fireEvent.click(link);
    expect(notPrevented).toBe(false);
    expect(BrowserOpenURL).toHaveBeenCalledWith('https://www.protondb.com/app/620');
  });

  it('an address that is not http(s) renders with no href and opens nothing', () => {
    render(<ExternalLink href="javascript:alert(1)">bad</ExternalLink>);
    const link = screen.getByText('bad');
    expect(link).not.toHaveAttribute('href');
    fireEvent.click(link);
    expect(BrowserOpenURL).not.toHaveBeenCalled();
  });

  it('a caller onClick still runs and can cancel the open', () => {
    const onClick = vi.fn((e: MouseEvent<HTMLAnchorElement>) => e.preventDefault());
    render(<ExternalLink href="https://example.com" onClick={onClick}>x</ExternalLink>);
    fireEvent.click(screen.getByText('x'));
    expect(onClick).toHaveBeenCalled();
    expect(BrowserOpenURL).not.toHaveBeenCalled();
  });
});
