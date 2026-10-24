// Hero frame fixtures (issue #101): the dashboard and game heroes in every
// variant, with axe clean. Warm is today's look; cool picks the compact
// values.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { axe } from 'vitest-axe';
import { Hero } from '../Hero';
import { DashboardAttention } from '../Dashboard';
import { GameHeroActions, GameHeroHeading } from '../GamePage';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

const themeState = vi.hoisted(() => ({ variants: {} as Record<string, string> }));
vi.mock('../../lib/theme', () => ({ useVariant: (axis: string) => themeState.variants[axis] }));

expect.extend(matchers);
afterEach(() => { cleanup(); themeState.variants = {}; });

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const fix = { source: 'curated', tier: 'gold', description: 'Needs this fix' } as Game['fix_info'];

const attentionProps = (o: Partial<Game> = {}) => {
  const hero = mk('hero-game', { title: 'Hero Game', fix_info: fix, ...o });
  return {
    hero,
    installed: 4,
    owned: 6,
    waitingCount: 2,
    pending: [hero],
    onOpenGame: () => {},
    onDismissHero: () => {},
  };
};

const headingProps = (o: Partial<Game> = {}) => {
  const game = mk('game-page', { title: 'Game Page', fix_info: fix, ...o });
  return { game, fix: game.fix_info, storeLogo: undefined, storeName: 'steam', needsConnection: false };
};

const actionsProps = (o: Partial<Game> = {}) => {
  const game = mk('game-page', { title: 'Game Page', playtime_minutes: 90, ...o });
  return {
    game,
    plugin: undefined,
    job: undefined,
    active: false,
    session: undefined,
    launching: false,
    hours: 1.5 as number | null,
    last: 'today' as string | undefined,
    playtimeTitle: 'played' as string | undefined,
    canChooseOptions: false,
    onJob: () => {},
    onJobAction: () => {},
    onPlay: () => {},
    onStop: () => {},
    onForceStop: () => {},
    onInstall: () => {},
    onInstallTo: undefined as (() => void) | undefined,
    onSettings: () => {},
    onOpenStore: undefined as (() => void) | undefined,
  };
};

// Warm reads the built-in defaults: the dashboard's tall glass banner, the
// game page's regular solid one.
const warm = () => {
  themeState.variants = {
    'hero.height': 'tall',
    'hero.gradient': 'warm',
    'hero.chips': 'glass',
    'game-hero.height': 'regular',
    'game-hero.gradient': 'warm',
    'game-hero.chips': 'solid',
  };
};

// Cool picks the compact values, with the chips swapped per use site.
const cool = () => {
  themeState.variants = {
    'hero.height': 'compact',
    'hero.gradient': 'neutral',
    'hero.chips': 'solid',
    'game-hero.height': 'compact',
    'game-hero.gradient': 'neutral',
    'game-hero.chips': 'glass',
  };
};

describe('hero frame', () => {
  it('dashboard warm: tall glass banner with the attention content, axe clean', async () => {
    warm();
    const { container } = render(
      <Hero site="dashboard" art={[]} title="Hero Game">
        <DashboardAttention {...attentionProps()} />
      </Hero>
    );
    const section = container.querySelector('section');
    expect(section?.className).toContain('min-h-[290px]');
    expect(section?.textContent).toContain('Hero Game');
    const scrim = [...container.querySelectorAll('section > div')].find((d) => (d as HTMLElement).style.background !== '');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('84deg');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('19,15,10');
    expect(container.querySelector('[class*="bg-black/45"]')).toBeTruthy();
    (expect(await axe(container)) as any).toHaveNoViolations();
  });

  it('dashboard cool: compact neutral banner with solid chips, axe clean', async () => {
    cool();
    const { container } = render(
      <Hero site="dashboard" art={[]} title="Hero Game">
        <DashboardAttention {...attentionProps()} />
      </Hero>
    );
    expect(container.querySelector('section')?.className).toContain('min-h-[210px]');
    const scrim = [...container.querySelectorAll('section > div')].find((d) => (d as HTMLElement).style.background !== '');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('11,17,24');
    expect(container.querySelector('[class*="bg-black/50"]')).toBeTruthy();
    (expect(await axe(container)) as any).toHaveNoViolations();
  });

  it('dashboard gradient none: no scrim over the art, axe clean', async () => {
    warm();
    themeState.variants['hero.gradient'] = 'none';
    const { container } = render(
      <Hero site="dashboard" art={[]} title="Hero Game">
        <DashboardAttention {...attentionProps()} />
      </Hero>
    );
    const scrims = [...container.querySelectorAll('section > div')].filter((d) => (d as HTMLElement).style.background !== '');
    expect(scrims).toHaveLength(0);
    (expect(await axe(container)) as any).toHaveNoViolations();
  });

  it('game warm: regular solid hero with heading and actions, axe clean', async () => {
    warm();
    const { container } = render(
      <Hero site="game" art={[]} title="Game Page">
        <GameHeroHeading {...headingProps()} />
        <GameHeroActions {...actionsProps()} />
      </Hero>
    );
    const section = container.querySelector('section');
    expect(section?.className).toContain('min-h-[280px]');
    expect(section?.className).toContain('mb-6');
    const scrim = [...container.querySelectorAll('section > div')].find((d) => (d as HTMLElement).style.background !== '');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('0deg');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('19,15,10');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Game Page');
    expect(container.querySelector('[data-testid="store-badge"]')).toBeTruthy();
    (expect(await axe(container)) as any).toHaveNoViolations();
  });

  it('game cool: compact neutral hero with glass chips, axe clean', async () => {
    cool();
    const { container } = render(
      <Hero site="game" art={[]} title="Game Page">
        <GameHeroHeading {...headingProps()} />
        <GameHeroActions {...actionsProps()} />
      </Hero>
    );
    expect(container.querySelector('section')?.className).toContain('min-h-[210px]');
    const scrim = [...container.querySelectorAll('section > div')].find((d) => (d as HTMLElement).style.background !== '');
    expect((scrim as HTMLElement | undefined)?.style.background.replace(/\s+/g, '')).toContain('11,17,24');
    expect(container.querySelector('[class*="backdrop-blur"]')).toBeTruthy();
    (expect(await axe(container)) as any).toHaveNoViolations();
  });
});
