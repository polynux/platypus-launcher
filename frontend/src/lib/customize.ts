import type { ArtKind, Customization, Game } from '../types';

// Customization (core only): the user's own title and art for a game. The
// library carries the result in the game's own fields (see Game.customized);
// these helpers describe the kinds for the form.

export interface ArtKindInfo {
  kind: ArtKind;
  label: string;
  /** Where it shows and the shape that suits it. */
  hint: string;
  /** Preview box: portrait art is tall, the others wide or square. */
  shape: 'portrait' | 'wide' | 'square';
}

export const ART_KINDS: ArtKindInfo[] = [
  { kind: 'cover', label: 'Cover', hint: 'Library cards · portrait, about 2:3', shape: 'portrait' },
  { kind: 'banner', label: 'Banner', hint: 'Top of the game page and wide cards · wide, about 3:1', shape: 'wide' },
  { kind: 'logo', label: 'Logo', hint: 'Over the banner in place of the title · transparent PNG', shape: 'wide' },
  { kind: 'icon', label: 'Icon', hint: 'Small lists · square', shape: 'square' },
  { kind: 'background', label: 'Background', hint: 'Faint backdrop of the game page · wide', shape: 'wide' },
];

/** The image the game shows for a kind now (the user's if they set one). */
export function currentArt(game: Game, kind: ArtKind): string {
  switch (kind) {
    case 'cover': return game.cover_url;
    case 'banner': return game.hero_url || game.banner_url;
    case 'logo': return game.logo_url ?? '';
    case 'icon': return game.icon_url ?? '';
    case 'background': return game.background_url ?? '';
  }
}

/** Whether the user changed this part of the game (a kind of art, "title" or "sort_title"). */
export const isCustomized = (game: Game, part: ArtKind | 'title' | 'sort_title'): boolean => !!game.customized?.includes(part);

/** Whether the user changed anything about the game's look. */
export const hasCustomization = (c: Customization | null): boolean => !!c && (c.title !== '' || c.sort_title !== '' || Object.keys(c.art).length > 0);

/** The key a game sorts by in the library: the sort title if there is one, else the title. */
export const sortKey = (g: Game): string => (g.sort_title || g.title).toLocaleLowerCase();
