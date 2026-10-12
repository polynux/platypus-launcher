import { describe, it, expect, vi } from 'vitest';
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

describe('addGameFlow (all-or-nothing add)', () => {
  it('saves the advanced options with the new game', async () => {
    const d = deps();
    const { game: got, settings } = await addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '/g/doom', winePrefix: '/p/doom' });
    expect(d.addCustomGame).toHaveBeenCalledWith('Doom', '/g/doom/doom.exe');
    expect(d.saveGameSettings).toHaveBeenCalledWith('custom-1', expect.objectContaining({ working_dir: '/g/doom', wine_prefix: '/p/doom' }));
    expect(d.removeCustomGame).not.toHaveBeenCalled();
    expect(got.settings.working_dir).toBe('/g/doom');
    expect(settings?.wine_prefix).toBe('/p/doom');
  });

  it('skips the save when no advanced options are set', async () => {
    const d = deps();
    const { settings } = await addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '', winePrefix: '' });
    expect(d.saveGameSettings).not.toHaveBeenCalled();
    expect(d.removeCustomGame).not.toHaveBeenCalled();
    expect(settings).toBeNull();
  });

  it('removes the new game and rethrows when saving the options fails', async () => {
    const err = new Error('working directory: no such file or directory');
    const d = deps({ saveGameSettings: vi.fn(async () => { throw err; }) });
    await expect(addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '/missing', winePrefix: '' })).rejects.toBe(err);
    expect(d.addCustomGame).toHaveBeenCalledOnce();
    expect(d.removeCustomGame).toHaveBeenCalledWith('custom-1');
  });

  it('still surfaces the original error when the rollback fails too', async () => {
    const err = new Error('working directory: no such file or directory');
    const d = deps({
      saveGameSettings: vi.fn(async () => { throw err; }),
      removeCustomGame: vi.fn(async () => { throw new Error('rollback failed'); }),
    });
    await expect(addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '/missing', winePrefix: '' })).rejects.toBe(err);
  });

  it('rejects without side effects when adding itself fails', async () => {
    const err = new Error('not a file');
    const d = deps({ addCustomGame: vi.fn(async () => { throw err; }) });
    await expect(addGameFlow(d, 'Bad', 'relative/path', { workingDir: '', winePrefix: '' })).rejects.toBe(err);
    expect(d.saveGameSettings).not.toHaveBeenCalled();
    expect(d.removeCustomGame).not.toHaveBeenCalled();
  });

  it('saves the release year with the new game', async () => {
    const d = deps();
    const { game: got } = await addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '', winePrefix: '', releaseYear: 1993 });
    expect(d.setCustomGameYear).toHaveBeenCalledWith('custom-1', 1993);
    expect(d.removeCustomGame).not.toHaveBeenCalled();
    expect(got.release_year).toBe(1993);
  });

  it('removes the new game when saving the year fails', async () => {
    const err = new Error('release year must be between 1970 and 2027, got 1800');
    const d = deps({ setCustomGameYear: vi.fn(async () => { throw err; }) });
    await expect(addGameFlow(d, 'Doom', '/g/doom/doom.exe', { workingDir: '', winePrefix: '', releaseYear: 1800 })).rejects.toBe(err);
    expect(d.removeCustomGame).toHaveBeenCalledWith('custom-1');
  });
});
