import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import { CrashHintsPanel } from '../CrashHints';
import type { CrashHints, Game } from '../../types';

const api = vi.hoisted(() => ({
  runWinetricksVerb: vi.fn(),
  openCrashLog: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({ LauncherApi: api }));

const game = { id: 'foo', title: 'Foo' } as Game;

const hints = (o: Partial<CrashHints> = {}): CrashHints => ({
  log: '/tmp/logs/foo.log',
  winetricks: true,
  hints: [{ rule: 'vcredist', param: 'vcruntime140_1.dll', verb: 'vcrun2022' }],
  ...o,
});

beforeEach(() => {
  api.runWinetricksVerb.mockReset().mockReturnValue(Effect.succeed(undefined));
  api.openCrashLog.mockReset().mockReturnValue(Effect.succeed(undefined));
});

describe('CrashHintsPanel', () => {
  it('names the missing DLL and offers the winetricks verb that fixes it', () => {
    const { getByText } = render(<CrashHintsPanel game={game} hints={hints()} onRunner={() => {}} />);
    expect(getByText('vcruntime140_1.dll is missing: the Visual C++ runtime isn\'t installed in this prefix.')).toBeTruthy();
    expect(getByText('Install vcrun2022 with winetricks')).toBeTruthy();
  });

  it('runs the verb in the game\'s prefix and says it started', async () => {
    const onNotify = vi.fn();
    const { getByText } = render(<CrashHintsPanel game={game} hints={hints()} onNotify={onNotify} onRunner={() => {}} />);
    fireEvent.click(getByText('Install vcrun2022 with winetricks'));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.stringContaining('vcrun2022')));
    expect(api.runWinetricksVerb).toHaveBeenCalledWith(game, 'vcrun2022');
  });

  it('reports a refused install as an error, with the core message', async () => {
    api.runWinetricksVerb.mockReturnValue(Effect.fail(new Error('winetricks isn\'t installed')));
    const onNotify = vi.fn();
    const { getByText } = render(<CrashHintsPanel game={game} hints={hints()} onNotify={onNotify} onRunner={() => {}} />);
    fireEvent.click(getByText('Install vcrun2022 with winetricks'));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('winetricks isn\'t installed', 'err'));
  });

  it('shows the command instead of a button when winetricks is not on this machine', () => {
    const { queryByText, getByText } = render(<CrashHintsPanel game={game} hints={hints({ winetricks: false })} onRunner={() => {}} />);
    expect(queryByText('Install vcrun2022 with winetricks')).toBeNull();
    expect(getByText('Run: winetricks vcrun2022')).toBeTruthy();
  });

  it('names the DLL the log named when no rule knows it', () => {
    const { getByText } = render(
      <CrashHintsPanel game={game} hints={hints({ hints: [{ rule: 'missingDll', param: 'bar.dll' }] })} onRunner={() => {}} />
    );
    expect(getByText("Wine couldn't load bar.dll.")).toBeTruthy();
  });

  it('offers to switch the runner for a media failure', () => {
    const onRunner = vi.fn();
    const { getByText } = render(
      <CrashHintsPanel game={game} hints={hints({ hints: [{ rule: 'media', action: 'runner' }] })} onRunner={onRunner} />
    );
    fireEvent.click(getByText('Switch runner'));
    expect(onRunner).toHaveBeenCalledTimes(1);
  });

  it('opens the log of the failed launch', () => {
    const { getByText } = render(<CrashHintsPanel game={game} hints={hints()} onRunner={() => {}} />);
    fireEvent.click(getByText('Open the log'));
    expect(api.openCrashLog).toHaveBeenCalledWith(game);
  });
});
