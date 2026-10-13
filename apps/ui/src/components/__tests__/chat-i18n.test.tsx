import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { ChatPanel } from '../ChatPanel';
import { FriendsModal } from '../FriendsModal';
import { FriendsSummary } from '../FriendsSummary';
import type { ChatMessage, ChatView, SocialFriend, SocialView } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getFamilyMembers: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
}));

const msg = (over: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'm1',
  from: 'ana',
  text: 'hello',
  sentAt: new Date(2026, 9, 9, 7, 5).getTime(),
  ...over,
});

const view = (over: Partial<ChatView> = {}): ChatView => ({
  seq: 1,
  plugin: 'echo',
  friendId: 'ana',
  open: true,
  loaded: true,
  more: false,
  unread: 0,
  typing: false,
  messages: [msg()],
  ...over,
});

const friend = (id: string, name: string, state: SocialFriend['state'], game?: SocialFriend['game']): SocialFriend =>
  ({ id, name, state, game });

const echo: SocialView = {
  seq: 1,
  plugin: 'echo',
  enabled: true,
  showPlaying: true,
  running: true,
  connection: { state: 'online' },
  self: { name: 'Echo Player', state: 'online' },
  friends: [
    friend('1', 'Ana Ruiz', 'online', { id: 'p.echo.tiny', title: 'Echo Tiny' }),
    friend('2', 'Bo Lindqvist', 'offline'),
  ],
};

const labels = { echo: 'Echo' };

describe('chat live language switching', () => {
  it('re-renders the conversation in French without a reload', async () => {
    render(
      <ChatPanel
        view={view({
          more: true,
          typing: true,
          messages: [
            msg({ id: '', from: 'self', text: 'on its way', status: 'sending', clientId: 'c-1' }),
            msg({ id: 'm2', from: 'self', text: 'refused', status: 'failed', clientId: 'c-2' }),
          ],
        })}
        friendName="Ana Ruiz"
        storeName="Echo"
        onLoadOlder={() => {}}
        onSend={() => {}}
        onResend={() => {}}
        onClose={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: 'Send' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Write a message')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Load older messages' })).toBeInTheDocument();
    expect(screen.getByText('Ana Ruiz is typing…')).toBeInTheDocument();
    expect(screen.getByText('Sending…')).toBeInTheDocument();
    expect(screen.getByText('Not sent')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Send again' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('button', { name: 'Envoyer' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Écrivez un message')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Charger les messages plus anciens' })).toBeInTheDocument();
    expect(screen.getByText("Ana Ruiz est en train d'écrire…")).toBeInTheDocument();
    expect(screen.getByText('Envoi…')).toBeInTheDocument();
    expect(screen.getByText('Non envoyé')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Renvoyer' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: 'Send' })).toBeInTheDocument();
  });

  it('re-renders the friends modal in French without a reload', async () => {
    render(
      <FriendsModal
        views={[echo]}
        labels={labels}
        plugins={[]}
        inLibrary={() => false}
        onClose={() => {}}
        onSetEnabled={() => {}}
        onSetStatus={() => {}}
        onSetPlaying={() => {}}
        onReconnect={() => {}}
        onOpenGame={() => {}}
      />,
    );
    expect(screen.getByRole('heading', { name: 'Friends' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search friends…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Turn off' })).toBeInTheDocument();
    expect(screen.getByText('In game · 1')).toBeInTheDocument();
    expect(screen.getByText('Offline · 1')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Your status on Echo' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByRole('heading', { name: 'Amis' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Rechercher des amis…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Désactiver' })).toBeInTheDocument();
    expect(screen.getByText('En jeu · 1')).toBeInTheDocument();
    expect(screen.getByText('Hors ligne · 1')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Votre statut sur Echo' })).toBeInTheDocument();
    expect(screen.getByText('Joue à Echo Tiny')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('heading', { name: 'Friends' })).toBeInTheDocument();
  });

  it('re-renders the friends summary in French without a reload', async () => {
    render(<FriendsSummary views={[echo]} labels={labels} onOpen={() => {}} unread={5} />);
    const block = screen.getByRole('button', { name: /Friends: 1 online · 1 in game/ });
    expect(within(block).getByText('1 online · 1 in game')).toBeInTheDocument();
    expect(within(block).getByText('Echo · Online')).toBeInTheDocument();
    expect(within(block).getByLabelText('5 unread messages')).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    const frBlock = screen.getByRole('button', { name: /Amis : 1 en ligne · 1 en jeu/ });
    expect(within(frBlock).getByText('1 en ligne · 1 en jeu')).toBeInTheDocument();
    expect(within(frBlock).getByText('Echo · En ligne')).toBeInTheDocument();
    expect(within(frBlock).getByLabelText('5 messages non lus')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByRole('button', { name: /Friends: 1 online · 1 in game/ })).toBeInTheDocument();
  });
});
