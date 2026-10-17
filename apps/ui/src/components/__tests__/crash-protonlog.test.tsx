import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import type { CrashHints, Game } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    runWinetricksVerb: () => Effect.succeed(undefined),
    openCrashLog: () => Effect.succeed(undefined),
  },
}));

import { CrashHintsPanel } from '../CrashHints';

const game = { id: 'steam_620', title: 'Portal' } as unknown as Game;
const hints: CrashHints = {
  log: '/home/you/steam-620.log',
  winetricks: false,
  hints: [{ rule: 'protonLog', param: '/home/you/steam-620.log' }],
} as unknown as CrashHints;

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage('en');
});

describe('the Proton log hint for a Steam launch', () => {
  it('says to turn the log on, with the launch option and where the log appears', async () => {
    await i18n.changeLanguage('en');
    render(<CrashHintsPanel game={game} hints={hints} onRunner={() => {}} />);
    expect(screen.getByText('Turn on the Proton log to find out why.')).toBeTruthy();
    expect(screen.getByText('PROTON_LOG=1 %command%')).toBeTruthy();
    expect(screen.getByText('The log will appear at /home/you/steam-620.log.')).toBeTruthy();
    // No log exists yet, so there is nothing to open.
    expect(screen.queryByRole('button', { name: 'Open the log' })).toBeNull();
  });

  it('says the same in French', async () => {
    await i18n.changeLanguage('fr');
    render(<CrashHintsPanel game={game} hints={hints} onRunner={() => {}} />);
    expect(screen.getByText('Activez le journal Proton pour savoir pourquoi.')).toBeTruthy();
    expect(screen.getByText('PROTON_LOG=1 %command%')).toBeTruthy();
    expect(screen.getByText('Le journal apparaîtra dans /home/you/steam-620.log.')).toBeTruthy();
  });
});
