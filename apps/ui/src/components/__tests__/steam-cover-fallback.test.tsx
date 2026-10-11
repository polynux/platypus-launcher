import React from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { coverMisses } from '../../lib/coverMisses';
import type { Game } from '../../types';

// An owned Steam game whose portrait 404s (older title): the card must show
// the landscape header cropped, without ever requesting the known 404.
const COVER = 'https://cdn.cloudflare.steamstatic.com/steam/apps/400/library_600x900.jpg';
const HEADER = 'https://cdn.cloudflare.steamstatic.com/steam/apps/400/header.jpg';

const game = {
  id: 'p.steam-standalone.400',
  source: 'steam-standalone',
  store: 'steam',
  store_app_id: '400',
  title: 'Portal',
  cover_url: COVER,
  banner_url: HEADER,
  hero_url: `https://cdn.cloudflare.steamstatic.com/steam/apps/400/library_hero_2x.jpg`,
  install_dir: '',
  executable: '',
  is_installed: false,
  installable: true,
  settings: { runner_id: '' },
} as unknown as Game;

const props = {
  games: [game],
  runners: [],
  activity: [],
  dismissedHero: new Set<string>(),
  onDismissHero: () => {},
  onOpenGame: () => {},
  onNavigate: () => {},
};

beforeEach(() => {
  coverMisses.clear();
});

describe('Steam cover fallback in the library UI (issue #26)', () => {
  it('shows the header on the card when the portrait is a known miss', () => {
    coverMisses.markBad(COVER);
    const { container } = render(<Dashboard {...props} />);
    const srcs = [...container.querySelectorAll('img')].map((i) => i.getAttribute('src'));
    expect(srcs).toContain(HEADER);
    expect(srcs).not.toContain(COVER);
  });
});
