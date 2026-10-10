import { describe, it, expect, vi } from 'vitest';
import { openExternal, webURL } from './external';

// Every source file under src, read as text, so the lint below can look at the markup.
const sources = import.meta.glob('../**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>;

// An <a> element with an href attribute. The webview opens target="_blank"
// anchors inside itself, so links must render through ExternalLink, which calls openExternal.
const RAW_ANCHOR = /<a\s[^<]*?\bhref=/;

describe('external links', () => {
  it('only http and https addresses are web links', () => {
    expect(webURL('https://protondb.com/app/620')).toBe('https://protondb.com/app/620');
    expect(webURL('http://example.com/x')).toBe('http://example.com/x');
    expect(webURL('javascript:alert(1)')).toBe('');
    expect(webURL('file:///etc/passwd')).toBe('');
    expect(webURL('not a url')).toBe('');
    expect(webURL(undefined)).toBe('');
  });

  it('openExternal goes through the shell bridge', () => {
    const shellOpen = vi.fn();
    window.platypus = { invoke: vi.fn(), on: vi.fn(), openExternal: shellOpen };
    expect(openExternal('https://www.pcgamingwiki.com/wiki/Foo')).toBe(true);
    expect(shellOpen).toHaveBeenCalledWith('https://www.pcgamingwiki.com/wiki/Foo');
    expect(openExternal('file:///etc/passwd')).toBe(false);
    expect(shellOpen).toHaveBeenCalledTimes(1);
  });

  it('openExternal hands a web link to the browser and nothing else', () => {
    const shellOpen = vi.fn();
    window.platypus = { invoke: vi.fn(), on: vi.fn(), openExternal: shellOpen };
    try {
      expect(openExternal('https://www.pcgamingwiki.com/wiki/Foo')).toBe(true);
      expect(shellOpen).toHaveBeenCalledWith('https://www.pcgamingwiki.com/wiki/Foo');

      expect(openExternal('javascript:alert(1)')).toBe(false);
      expect(openExternal(undefined)).toBe(false);
      expect(shellOpen).toHaveBeenCalledTimes(1);
    } finally {
      delete window.platypus;
    }
  });

  it('lint: no raw <a href> outside ExternalLink', () => {
    const offenders = Object.entries(sources)
      .filter(([path]) => !path.includes('.test.') && !path.includes('__tests__') && !path.endsWith('components/ExternalLink.tsx'))
      .filter(([, text]) => RAW_ANCHOR.test(text))
      .map(([path]) => path);
    expect(offenders).toEqual([]);
  });

  it('the lint pattern catches a raw anchor, multi-line included', () => {
    expect(RAW_ANCHOR.test('<a href="https://steamdb.info">x</a>')).toBe(true);
    expect(RAW_ANCHOR.test('<a\n  className="x"\n  href={url}>x</a>')).toBe(true);
    expect(RAW_ANCHOR.test('<ExternalLink href={url}>x</ExternalLink>')).toBe(false);
  });
});
