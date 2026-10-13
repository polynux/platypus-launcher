import { describe, it, expect, vi, beforeEach } from 'vitest';
import { addGameFlow, type AddGameDeps } from './addGame';
import type { Game } from '../types';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom', title: 'Doom',
  cover_url: '', banner_url: '', install_dir: '/g/doom', executable: '/g/doom/doom.exe',
  is_installed: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, use_gamemode: true },
  ...o,
}) as Game;

function deps(over: Partial<Record<keyof AddGameDeps, ReturnType<typeof vi.fn>>> = {}) {
  return {
    addCustomGame: vi.fn(async () => game()),
    saveGameSettings: vi.fn(async () => {}),
    removeCustomGame: vi.fn(async () => {}),
    setCustomGameYear: vi.fn(async (id: string, year: number) => game({ id, release_year: year })),
    ...over,
  } as unknown as AddGameDeps & { addCustomGame: ReturnType<typeof vi.fn>; saveGameSettings: ReturnType<typeof vi.fn>; removeCustomGame: ReturnType<typeof vi.fn>; setCustomGameYear: ReturnType<typeof vi.fn> };
}

describe('addGameFlow run mode (issue #47)', () => {
  it('saves an explicit run mode with the new game', async () => {
    const d = deps();
    const { settings } = await addGameFlow(d, 'Doom', '/g/doom/game', { workingDir: '', winePrefix: '', runMode: 'native' });
    expect(d.saveGameSettings).toHaveBeenCalledWith('custom-1', expect.objectContaining({ run_mode: 'native' }));
    expect(settings?.run_mode).toBe('native');
  });

  it('stores nothing for the automatic (detected) default', async () => {
    const d = deps();
    const { settings } = await addGameFlow(d, 'Doom', '/g/doom/game', { workingDir: '', winePrefix: '', runMode: 'auto' });
    expect(d.saveGameSettings).not.toHaveBeenCalled();
    expect(settings).toBeNull();
  });
});
