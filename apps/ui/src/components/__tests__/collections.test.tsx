import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Collection, Game } from '../../types';
import { TooltipProvider } from '../ds/tooltip';
import { LibraryView } from '../LibraryView';
import { Sidebar } from '../Sidebar';
import { CollectionsDialog, EMPTY_DRAFT, smartFromDraft } from '../CollectionsDialog';

const h = vi.hoisted(() => ({
  api: {} as Record<string, ReturnType<typeof vi.fn>>,
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, name: string) => h.api[name] ?? (() => Effect.succeed(null)) }),
}));

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const COUCH: Collection = { id: 'couch', name: 'Couch', pinned: true, games: ['a', 'gone'], titles: { gone: 'Gone Game' } };
const OLD: Collection = { id: 'old', name: 'Old', pinned: true, games: ['a', 'untitled'] };
const INSTALLED: Collection = { id: 'installed', name: 'Installed', builtin: true, pinned: true, smart: { installed: true } };
const HIDDEN: Collection = { id: 'hidden', name: 'Hidden list', pinned: false, games: [] };
const LIST = [INSTALLED, COUCH, HIDDEN, OLD];

const games = [mk('a'), mk('b'), mk('c', { is_installed: false })];

const titles = () => [...document.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);

const library = (extra: Partial<React.ComponentProps<typeof LibraryView>> = {}) =>
  render(
    <TooltipProvider>
      <LibraryView
        games={games}
        runners={[]}
        activity={[]}
        search=""
        onSearch={() => {}}
        source="all"
        onSource={() => {}}
        mode="cards"
        onMode={() => {}}
        ownership="owned"
        onOwnership={() => {}}
        warnings={[]}
        plugins={[]}
        jobs={[]}
        lastScan={null}
        onOpenGame={() => {}}
        onAddGame={() => {}}
        onNavigate={() => {}}
        collections={LIST}
        onCollection={() => {}}
        onManageCollections={() => {}}
        {...extra}
      />
    </TooltipProvider>,
  );

beforeEach(() => {
  for (const k of Object.keys(h.api)) delete h.api[k];
});

describe('library collection filter', () => {
  it('shows a manual collection: its games in the library, the rest hidden', () => {
    library({ collection: 'couch' });
    expect(titles()).toEqual(['a']);
  });

  it('applies a smart collection to the loaded games', () => {
    library({ collection: 'installed' });
    expect(titles()).toEqual(['a', 'b']);
  });

  it('lists the games a manual collection holds that left the library as missing', () => {
    library({ collection: 'couch' });
    expect(screen.getByTestId('collection-missing')).toHaveTextContent('1 not in the library');
    expect(screen.getByTestId('collection-missing')).toHaveAttribute('title', 'Gone Game');
  });

  it('falls back to the id for a missing game whose title was never remembered', () => {
    library({ collection: 'old' });
    expect(screen.getByTestId('collection-missing')).toHaveAttribute('title', 'untitled');
  });

  it('shows every game when no collection is active', () => {
    library();
    expect(titles()).toEqual(['a', 'b', 'c']);
    expect(screen.queryByTestId('collection-chip')).toBeNull();
  });

  it('clears the active collection from the chip', () => {
    const onCollection = vi.fn();
    library({ collection: 'couch', onCollection });
    fireEvent.click(screen.getByRole('button', { name: 'Show all games' }));
    expect(onCollection).toHaveBeenCalledWith(undefined);
  });

  it('opens the collections dialog for a game on right-click', () => {
    const onManageCollections = vi.fn();
    library({ onManageCollections });
    const card = [...document.querySelectorAll('.cover-grid-sm > button')].find((b) => b.querySelector('.truncate')?.textContent === 'b')!;
    fireEvent.contextMenu(card);
    expect(onManageCollections).toHaveBeenCalledWith('b');
  });
});

describe('sidebar collections', () => {
  const base = {
    view: { name: 'library' as const, collection: 'couch' },
    onNavigate: vi.fn(),
    search: '',
    onSearch: () => {},
    storeCounts: { steam: 3 },
    onPickSource: () => {},
    gameCount: 3,
    downloadCount: 0,
    authNeeded: false,
    fixCount: 0,
    runnerCount: 0,
    lastScan: null,
    sessions: [],
    onStop: () => {},
    friends: [],
    friendLabels: {},
    onOpenFriends: () => {},
    canRedeem: false,
    onRedeem: () => {},
  };

  it('lists the pinned collections with their counts, and opens one', () => {
    const onOpenCollection = vi.fn();
    render(
      <TooltipProvider>
        <Sidebar {...base} collections={LIST} collectionCounts={{ installed: 2, couch: 1, hidden: 0 }} onOpenCollection={onOpenCollection} onManageCollections={() => {}} />
      </TooltipProvider>,
    );
    expect(screen.queryByRole('button', { name: /Hidden list/ })).toBeNull();
    const couch = screen.getByRole('button', { name: /Couch/ });
    expect(couch).toHaveAttribute('aria-current', 'page');
    expect(within(couch).getByText('1')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Installed/ }));
    expect(onOpenCollection).toHaveBeenCalledWith('installed');
  });
});

describe('smartFromDraft', () => {
  it('sets only the fields the user chose', () => {
    expect(smartFromDraft(EMPTY_DRAFT)).toEqual({});
    expect(smartFromDraft({ ...EMPTY_DRAFT, installed: 'no', minHours: '2', yearFrom: '2000', tier: 'native' })).toEqual({
      installed: false,
      min_playtime_minutes: 120,
      year_from: 2000,
      tier: 'native',
    });
  });

  it('ignores values that are not positive numbers', () => {
    expect(smartFromDraft({ ...EMPTY_DRAFT, minHours: 'many', yearTo: '-3' })).toEqual({});
  });
});

describe('collections dialog', () => {
  it('adds the game to a manual collection when its box is ticked', async () => {
    const after = [INSTALLED, { ...COUCH, games: ['a', 'gone', 'b'] }, HIDDEN];
    h.api.addCollectionGame = vi.fn(() => Effect.succeed(after));
    const onChanged = vi.fn();
    render(<CollectionsDialog collections={LIST} games={games} gameId="b" onChanged={onChanged} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('checkbox', { name: /^Couch/ }));
    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(after));
    expect(h.api.addCollectionGame).toHaveBeenCalledWith('couch', 'b');
  });

  it('removes the game from a manual collection when its box is unticked', async () => {
    h.api.removeCollectionGame = vi.fn(() => Effect.succeed(LIST));
    render(<CollectionsDialog collections={[INSTALLED, COUCH, HIDDEN]} games={games} gameId="a" onChanged={() => {}} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('checkbox', { name: /^Couch/ }));
    await waitFor(() => expect(h.api.removeCollectionGame).toHaveBeenCalledWith('couch', 'a'));
  });

  it('pins and unpins a collection from the manager', async () => {
    h.api.pinCollection = vi.fn(() => Effect.succeed(LIST));
    render(<CollectionsDialog collections={LIST} games={games} onChanged={() => {}} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('switch', { name: 'Show Couch in the sidebar' }));
    await waitFor(() => expect(h.api.pinCollection).toHaveBeenCalledWith('couch', false));
  });

  it('refuses a smart filter with no condition, before it reaches the core', () => {
    h.api.createCollection = vi.fn(() => Effect.succeed(LIST));
    render(<CollectionsDialog collections={LIST} games={games} onChanged={() => {}} onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText(/^Kind/), { target: { value: 'smart' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Nothing' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    expect(screen.getByText('A smart filter needs at least one condition.')).toBeInTheDocument();
    expect(h.api.createCollection).not.toHaveBeenCalled();
  });

  it('creates a smart filter from the fields the user set', async () => {
    h.api.createCollection = vi.fn(() => Effect.succeed([...LIST, { id: 'new', name: 'Night', pinned: true, smart: { installed: true } }]));
    render(<CollectionsDialog collections={LIST} games={games} onChanged={() => {}} onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText(/^Kind/), { target: { value: 'smart' } });
    fireEvent.change(screen.getByRole('textbox', { name: 'Name' }), { target: { value: 'Night' } });
    fireEvent.change(screen.getByLabelText('Installed'), { target: { value: 'yes' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(h.api.createCollection).toHaveBeenCalledWith('Night', { installed: true }));
  });
});
