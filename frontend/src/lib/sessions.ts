import type { SessionState } from '../types';

/** A session the backend still tracks: the game is starting, running or being stopped. */
export const isLiveState = (state: SessionState): boolean => state === 'starting' || state === 'running' || state === 'stopping';
